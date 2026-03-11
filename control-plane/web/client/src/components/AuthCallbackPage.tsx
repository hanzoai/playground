/**
 * OAuth Callback Page
 *
 * Handles the IAM OIDC redirect callback.
 * Exchanges the authorization code for tokens, then redirects to home.
 */

import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../contexts/AuthContext";

export function AuthCallbackPage() {
  const { handleCallback } = useAuth();
  const navigate = useNavigate();
  const [error, setError] = useState<string | null>(null);

  // Persistent debug log that survives page navigations
  const debugLog = (msg: string, data?: unknown) => {
    const entry = `${new Date().toISOString()} ${msg} ${data ? JSON.stringify(data) : ""}`;
    console.log(entry);
    try {
      const prev = localStorage.getItem("__auth_debug") || "";
      localStorage.setItem("__auth_debug", prev + entry + "\n");
    } catch { /* ok */ }
  };

  debugLog("[AuthCallbackPage] render", {
    hasHandleCallback: !!handleCallback,
    url: window.location.href,
    ssState: sessionStorage.getItem("hanzo_iam_state"),
    ssVerifier: sessionStorage.getItem("hanzo_iam_code_verifier")?.substring(0, 10),
    ssAccessToken: sessionStorage.getItem("hanzo_iam_access_token")?.substring(0, 20),
  });

  useEffect(() => {
    debugLog("[AuthCallbackPage] useEffect fired", {
      hasHandleCallback: !!handleCallback,
      url: window.location.href,
    });

    if (!handleCallback) {
      debugLog("[AuthCallbackPage] no handleCallback, navigating to /");
      navigate("/", { replace: true });
      return;
    }

    handleCallback()
      .then(() => {
        debugLog("[AuthCallbackPage] handleCallback SUCCESS", {
          ssAccessToken: sessionStorage.getItem("hanzo_iam_access_token")?.substring(0, 20),
        });
        // Use requestAnimationFrame to ensure React has flushed the
        // isAuthenticated=true state update from completeAuth() before we
        // navigate to "/" (which mounts AuthGuard).
        requestAnimationFrame(() => {
          debugLog("[AuthCallbackPage] navigating to /");
          navigate("/", { replace: true });
        });
      })
      .catch((err) => {
        debugLog("[AuthCallbackPage] handleCallback FAILED", {
          error: err instanceof Error ? err.message : String(err),
        });
        setError(err instanceof Error ? err.message : String(err));
      });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  if (error) {
    return (
      <div className="flex items-center justify-center min-h-screen bg-background">
        <div className="p-8 bg-card rounded-lg shadow-lg max-w-md w-full text-center">
          <h2 className="text-2xl font-semibold mb-2 text-destructive">
            Authentication Error
          </h2>
          <p className="text-muted-foreground mb-4">{error}</p>
          <button
            onClick={() => navigate("/", { replace: true })}
            className="bg-primary text-primary-foreground px-4 py-2 rounded-md"
          >
            Back to Home
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="flex items-center justify-center min-h-screen bg-background">
      <p className="text-muted-foreground">Completing sign-in...</p>
    </div>
  );
}
