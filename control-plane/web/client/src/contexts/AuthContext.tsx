import { createContext, useContext, useEffect, useState, useMemo } from "react";
import type { ReactNode } from "react";
import { setGlobalApiKey } from "../services/api";
import { resetAllStores } from "../stores/resetAll";
import { IamProvider as IamProviderBase, useIam as useIamHook } from "@hanzo/iam/react";
import type { TokenResponse, IamUser } from "@hanzo/iam";

// ---------------------------------------------------------------------------
// IAM Configuration (from environment variables)
// ---------------------------------------------------------------------------

const IAM_SERVER_URL = import.meta.env.VITE_IAM_SERVER_URL as string | undefined;
const IAM_CLIENT_ID = import.meta.env.VITE_IAM_CLIENT_ID as string | undefined;
const IAM_REDIRECT_URI = import.meta.env.VITE_IAM_REDIRECT_URI as string | undefined;

/** Whether IAM auth mode is active (determined by env vars). */
export const isIamMode = !!(IAM_SERVER_URL && IAM_CLIENT_ID);

const basePath = (import.meta.env.VITE_BASE_PATH || "").replace(/\/+$/, "");

const iamConfig = isIamMode
  ? {
      serverUrl: IAM_SERVER_URL!,
      clientId: IAM_CLIENT_ID!,
      redirectUri:
        IAM_REDIRECT_URI ||
        `${window.location.origin}${basePath}/auth/callback`,
    }
  : null;

// ---------------------------------------------------------------------------
// Auth Context
// ---------------------------------------------------------------------------

interface AuthContextType {
  apiKey: string | null;
  setApiKey: (key: string | null) => void;
  isAuthenticated: boolean;
  authRequired: boolean;
  clearAuth: () => void;
  /** Current auth mode. */
  authMode: "api-key" | "iam";
  /** Start IAM login (redirect). Only in IAM mode. */
  iamLogin?: () => Promise<void>;
  /** Start IAM login (popup). Only in IAM mode. */
  iamLoginPopup?: () => Promise<void>;
  /** Handle IAM OAuth callback. Only in IAM mode. */
  handleCallback?: (url?: string) => Promise<TokenResponse>;
  /** IAM user info. Only in IAM mode. */
  iamUser?: IamUser | null;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);
const STORAGE_KEY = "af_api_key";

// Simple obfuscation for localStorage; not meant as real security.
const encryptKey = (key: string): string => btoa(key.split("").reverse().join(""));
const decryptKey = (value: string): string => {
  try {
    return atob(value).split("").reverse().join("");
  } catch {
    return "";
  }
};

// Initialize global API key from localStorage BEFORE any React rendering
// Skip in IAM mode — tokens are managed by BrowserIamSdk
const initStoredKey = (() => {
  if (isIamMode) return null;
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored) {
      const key = decryptKey(stored);
      if (key) {
        setGlobalApiKey(key);
        return key;
      }
    }
  } catch { /* ignore */ }
  return null;
})();

// ---------------------------------------------------------------------------
// IAM Auth Bridge — maps useIam() into AuthContextType
// ---------------------------------------------------------------------------

function IamAuthBridge({ children }: { children: ReactNode }) {
  const iam = useIamHook();

  // Sync IAM access token to global API key so REST calls work
  useEffect(() => {
    setGlobalApiKey(iam.accessToken);
  }, [iam.accessToken]);

  const value = useMemo<AuthContextType>(
    () => ({
      apiKey: iam.accessToken,
      setApiKey: () => {}, // No-op in IAM mode
      isAuthenticated: iam.isAuthenticated,
      authRequired: true,
      clearAuth: () => {
        iam.logout();
        resetAllStores();
      },
      authMode: "iam",
      iamLogin: iam.login,
      iamLoginPopup: iam.loginPopup,
      handleCallback: iam.handleCallback,
      iamUser: iam.user,
    }),
    [iam],
  );

  if (iam.isLoading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        Loading...
      </div>
    );
  }

  return (
    <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
  );
}

// ---------------------------------------------------------------------------
// API Key Auth Provider (existing behavior)
// ---------------------------------------------------------------------------

function ApiKeyAuthProvider({ children }: { children: ReactNode }) {
  const [apiKey, setApiKeyState] = useState<string | null>(initStoredKey);
  const [authRequired, setAuthRequired] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const checkAuth = async () => {
      try {
        const stored = localStorage.getItem(STORAGE_KEY);
        const storedKey = stored ? decryptKey(stored) : null;
        if (stored && !storedKey) {
          localStorage.removeItem(STORAGE_KEY);
        }
        const headers: HeadersInit = {};
        if (apiKey) {
          headers["X-API-Key"] = apiKey;
        }
        const response = await fetch("/api/ui/v1/dashboard/summary", {
          headers,
        });
        if (response.ok) {
          if (storedKey) {
            setApiKeyState(storedKey);
            setGlobalApiKey(storedKey);
            setAuthRequired(true);
          } else {
            setAuthRequired(false);
          }
        } else if (response.status === 401) {
          setAuthRequired(true);
          setGlobalApiKey(null);
          if (stored) {
            localStorage.removeItem(STORAGE_KEY);
          }
        }
      } catch (err) {
        console.error("Auth check failed:", err);
        // Backend unreachable — allow through so gateway-only mode works
        setAuthRequired(false);
      } finally {
        setLoading(false);
      }
    };
    void checkAuth();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const setApiKey = (key: string | null) => {
    setApiKeyState(key);
    setGlobalApiKey(key);
    if (key) {
      localStorage.setItem(STORAGE_KEY, encryptKey(key));
    } else {
      localStorage.removeItem(STORAGE_KEY);
    }
  };

  const clearAuth = () => {
    setApiKeyState(null);
    setGlobalApiKey(null);
    localStorage.removeItem(STORAGE_KEY);
    resetAllStores();
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        Loading...
      </div>
    );
  }

  return (
    <AuthContext.Provider
      value={{
        apiKey,
        setApiKey,
        isAuthenticated: !authRequired || !!apiKey,
        authRequired,
        clearAuth,
        authMode: "api-key",
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

// ---------------------------------------------------------------------------
// AuthProvider — delegates to IAM or API key mode
// ---------------------------------------------------------------------------

export function AuthProvider({ children }: { children: ReactNode }) {
  if (isIamMode && iamConfig) {
    return (
      <IamProviderBase config={iamConfig}>
        <IamAuthBridge>{children}</IamAuthBridge>
      </IamProviderBase>
    );
  }
  return <ApiKeyAuthProvider>{children}</ApiKeyAuthProvider>;
}

// ---------------------------------------------------------------------------
// useAuth hook
// ---------------------------------------------------------------------------

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used within AuthProvider");
  }
  return ctx;
}
