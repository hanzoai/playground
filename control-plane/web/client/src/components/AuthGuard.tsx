import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { useAuth } from "../contexts/AuthContext";
import { setGlobalApiKey, setGlobalIamToken } from "../services/api";

export function AuthGuard({ children }: { children: React.ReactNode }) {
  const { apiKey, iamToken, setApiKey, isAuthenticated, authRequired, loginWithIAM } = useAuth();
  const [inputKey, setInputKey] = useState("");
  const [error, setError] = useState("");
  const [validating, setValidating] = useState(false);
  const [showApiKeyForm, setShowApiKeyForm] = useState(false);

  useEffect(() => {
    if (iamToken) {
      setGlobalIamToken(iamToken);
    } else {
      setGlobalApiKey(apiKey);
    }
  }, [apiKey, iamToken]);

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

        {/* Primary: Sign in with Hanzo IAM */}
        <button
          onClick={loginWithIAM}
          className="w-full bg-primary text-primary-foreground p-3 rounded-md font-medium mb-4 hover:opacity-90 transition-opacity"
        >
          Sign in with Hanzo
        </button>

        {/* Divider */}
        <div className="relative my-6">
          <div className="absolute inset-0 flex items-center">
            <div className="w-full border-t border-border" />
          </div>
          <div className="relative flex justify-center text-xs uppercase">
            <span className="bg-card px-2 text-muted-foreground">or</span>
          </div>
        </div>

        {/* Secondary: API Key */}
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
