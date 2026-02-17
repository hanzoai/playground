import { createContext, useContext, useEffect, useState } from "react";
import type { ReactNode } from "react";
import { setGlobalApiKey, setGlobalIamToken } from "../services/api";
import { resetAllStores } from "../stores/resetAll";

// IAM OAuth configuration
const IAM_PUBLIC_ENDPOINT = import.meta.env.VITE_IAM_PUBLIC_ENDPOINT || "https://hanzo.id";
const IAM_CLIENT_ID = import.meta.env.VITE_IAM_CLIENT_ID || "hanzobot-client-id";
const IAM_REDIRECT_URI = import.meta.env.VITE_IAM_REDIRECT_URI || `${window.location.origin}/auth/callback`;

interface IAMUser {
  sub: string;
  name: string;
  email: string;
  organization: string;
  isAdmin: boolean;
}

type AuthMode = "iam" | "apikey" | "none";

interface AuthContextType {
  apiKey: string | null;
  setApiKey: (key: string | null) => void;
  iamToken: string | null;
  iamUser: IAMUser | null;
  isAuthenticated: boolean;
  authRequired: boolean;
  authMode: AuthMode;
  clearAuth: () => void;
  loginWithIAM: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);
const STORAGE_KEY = "af_api_key";
const IAM_TOKEN_KEY = "af_iam_token";
const IAM_STATE_KEY = "af_iam_state";

// Simple obfuscation for localStorage; not meant as real security.
const encryptKey = (key: string): string => btoa(key.split("").reverse().join(""));
const decryptKey = (value: string): string => {
  try {
    return atob(value).split("").reverse().join("");
  } catch {
    return "";
  }
};

/** Parse OAuth token from URL hash (implicit flow) */
function parseOAuthCallback(): { accessToken: string; state: string } | null {
  const hash = window.location.hash;
  if (!hash || !hash.includes("access_token")) return null;

  const params = new URLSearchParams(hash.substring(1));
  const accessToken = params.get("access_token");
  const state = params.get("state") || "";

  if (accessToken) {
    // Clear the hash from URL
    window.history.replaceState(null, "", window.location.pathname + window.location.search);
    return { accessToken, state };
  }
  return null;
}

/** Generate random state for CSRF protection */
function generateState(): string {
  const array = new Uint8Array(16);
  crypto.getRandomValues(array);
  return Array.from(array, (b) => b.toString(16).padStart(2, "0")).join("");
}

// Pre-initialize from OAuth callback or localStorage
const initAuth = (() => {
  // Check OAuth callback first (hash fragment)
  const callback = parseOAuthCallback();
  if (callback) {
    const savedState = sessionStorage.getItem(IAM_STATE_KEY);
    if (savedState && callback.state === savedState) {
      sessionStorage.removeItem(IAM_STATE_KEY);
      setGlobalIamToken(callback.accessToken);
      return { iamToken: callback.accessToken, apiKey: null };
    }
  }

  // Check stored IAM token
  try {
    const storedToken = localStorage.getItem(IAM_TOKEN_KEY);
    if (storedToken) {
      setGlobalIamToken(storedToken);
      return { iamToken: storedToken, apiKey: null };
    }
  } catch { /* ignore */ }

  // Check stored API key
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored) {
      const key = decryptKey(stored);
      if (key) {
        setGlobalApiKey(key);
        return { iamToken: null, apiKey: key };
      }
    }
  } catch { /* ignore */ }

  return { iamToken: null, apiKey: null };
})();

export function AuthProvider({ children }: { children: ReactNode }) {
  const [apiKey, setApiKeyState] = useState<string | null>(initAuth.apiKey);
  const [iamToken, setIamTokenState] = useState<string | null>(initAuth.iamToken);
  const [iamUser, setIamUser] = useState<IAMUser | null>(null);
  const [authRequired, setAuthRequired] = useState(false);
  const [loading, setLoading] = useState(true);

  // Determine current auth mode
  const authMode: AuthMode = iamToken ? "iam" : apiKey ? "apikey" : "none";

  useEffect(() => {
    const checkAuth = async () => {
      try {
        const headers: HeadersInit = {};
        if (iamToken) {
          headers["Authorization"] = `Bearer ${iamToken}`;
        } else if (apiKey) {
          headers["X-API-Key"] = apiKey;
        }

        const response = await fetch("/api/ui/v1/dashboard/summary", { headers });

        if (response.ok) {
          if (iamToken || apiKey) {
            setAuthRequired(true);
          } else {
            setAuthRequired(false);
          }
        } else if (response.status === 401) {
          setAuthRequired(true);
          // Token expired or invalid
          if (iamToken) {
            setGlobalIamToken(null);
            setIamTokenState(null);
            localStorage.removeItem(IAM_TOKEN_KEY);
          }
          if (apiKey) {
            setGlobalApiKey(null);
            setApiKeyState(null);
            localStorage.removeItem(STORAGE_KEY);
          }
        }
      } catch {
        setAuthRequired(true);
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

  const loginWithIAM = () => {
    const state = generateState();
    sessionStorage.setItem(IAM_STATE_KEY, state);

    const params = new URLSearchParams({
      client_id: IAM_CLIENT_ID,
      response_type: "token",
      redirect_uri: IAM_REDIRECT_URI,
      scope: "openid profile email",
      state,
    });

    window.location.href = `${IAM_PUBLIC_ENDPOINT}/login/oauth/authorize?${params}`;
  };

  const clearAuth = () => {
    setApiKeyState(null);
    setIamTokenState(null);
    setIamUser(null);
    setGlobalApiKey(null);
    setGlobalIamToken(null);
    localStorage.removeItem(STORAGE_KEY);
    localStorage.removeItem(IAM_TOKEN_KEY);
    resetAllStores();
  };

  if (loading) {
    return <div className="flex items-center justify-center min-h-screen">Loading...</div>;
  }

  return (
    <AuthContext.Provider
      value={{
        apiKey,
        setApiKey,
        iamToken,
        iamUser,
        isAuthenticated: !authRequired || !!iamToken || !!apiKey,
        authRequired,
        authMode,
        clearAuth,
        loginWithIAM,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used within AuthProvider");
  }
  return ctx;
}
