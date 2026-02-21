import { useState } from "react";
import type { FormEvent } from "react";
import { useAuth } from "../contexts/AuthContext";
import { setGlobalApiKey } from "../services/api";

export function AuthGuard({ children }: { children: React.ReactNode }) {
  const {
    setApiKey,
    isAuthenticated,
    authRequired,
    authMode,
    iamLogin,
  } = useAuth();
  const [inputKey, setInputKey] = useState("");
  const [error, setError] = useState("");
  const [validating, setValidating] = useState(false);
  const [showApiKeyForm, setShowApiKeyForm] = useState(false);

  const handleApiKeySubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError("");
    setValidating(true);

    try {
      const response = await fetch("/api/ui/v1/dashboard/summary", {
        headers: { "X-API-Key": inputKey },
      });

      if (response.ok) {
        setApiKey(inputKey);
        setGlobalApiKey(inputKey);
      } else {
        setError("Invalid API key");
      }
    } catch {
      setError("Connection failed");
    } finally {
      setValidating(false);
    }
  };

  if (!authRequired || isAuthenticated) {
    return <>{children}</>;
  }

  // IAM mode — show "Sign in with Hanzo" button
  if (authMode === "iam" && iamLogin) {
    return (
      <div className="flex items-center justify-center min-h-screen bg-background">
        <div className="p-8 bg-card rounded-lg shadow-lg max-w-md w-full">
          {/* Hanzo Logo */}
          <div className="flex items-center gap-3 mb-6">
            <svg width="32" height="32" viewBox="0 0 64 64" xmlns="http://www.w3.org/2000/svg">
              <rect width="64" height="64" rx="8" fill="currentColor" className="text-primary" />
              <g transform="translate(8, 8) scale(0.716)">
                <path d="M22.21 67V44.6369H0V67H22.21Z" fill="currentColor" className="text-primary-foreground" />
                <path d="M66.7038 22.3184H22.2534L0.0878906 44.6367H44.4634L66.7038 22.3184Z" fill="currentColor" className="text-primary-foreground" />
                <path d="M22.21 0H0V22.3184H22.21V0Z" fill="currentColor" className="text-primary-foreground" />
                <path d="M66.7198 0H44.5098V22.3184H66.7198V0Z" fill="currentColor" className="text-primary-foreground" />
                <path d="M66.7198 67V44.6369H44.5098V67H66.7198Z" fill="currentColor" className="text-primary-foreground" />
              </g>
            </svg>
            <div>
              <h2 className="text-2xl font-semibold">Hanzo Playground</h2>
              <p className="text-muted-foreground text-sm">Agent Control Plane</p>
            </div>
          </div>

          {error && <p className="text-destructive mb-4">{error}</p>}

          <button
            onClick={() => {
              setError("");
              iamLogin().catch((err) =>
                setError(
                  err instanceof Error ? err.message : "Login failed",
                ),
              );
            }}
            className="w-full bg-primary text-primary-foreground p-3 rounded-md font-medium hover:opacity-90 transition-opacity"
          >
            Sign in with Hanzo
          </button>
        </div>
      </div>
    );
  }

  // API key mode — show Hanzo branding + API key form with optional IAM
  return (
    <div className="flex items-center justify-center min-h-screen bg-background">
      <div className="p-8 bg-card rounded-lg shadow-lg max-w-md w-full">
        {/* Hanzo Logo */}
        <div className="flex items-center gap-3 mb-6">
          <svg width="32" height="32" viewBox="0 0 512 512" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M256 0L512 256L256 512L0 256L256 0Z" fill="currentColor" className="text-primary" />
            <path d="M256 96L416 256L256 416L96 256L256 96Z" fill="currentColor" className="text-background" />
            <path d="M256 160L352 256L256 352L160 256L256 160Z" fill="currentColor" className="text-primary" />
          </svg>
          <div>
            <h2 className="text-2xl font-semibold">Hanzo Playground</h2>
            <p className="text-muted-foreground text-sm">Agent Control Plane</p>
          </div>
        </div>

        {/* API Key */}
        {showApiKeyForm ? (
          <form onSubmit={handleApiKeySubmit}>
            <input
              type="password"
              value={inputKey}
              onChange={(e) => setInputKey(e.target.value)}
              placeholder="API Key"
              className="w-full p-3 border rounded-md mb-4 bg-background"
              disabled={validating}
              autoFocus
            />
            {error && <p className="text-destructive mb-4">{error}</p>}
            <button
              type="submit"
              className="w-full border border-border text-foreground p-3 rounded-md font-medium disabled:opacity-50 hover:bg-accent transition-colors"
              disabled={validating || !inputKey}
            >
              {validating ? "Validating..." : "Connect with API Key"}
            </button>
          </form>
        ) : (
          <button
            onClick={() => setShowApiKeyForm(true)}
            className="w-full text-muted-foreground text-sm hover:text-foreground transition-colors"
          >
            Use API key instead
          </button>
        )}
      </div>
    </div>
  );
}
