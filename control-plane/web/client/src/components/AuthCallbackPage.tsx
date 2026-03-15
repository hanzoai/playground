/**
 * OAuth Callback Page
 *
 * Handles the IAM OIDC redirect callback.
 * Exchanges the authorization code for tokens, then redirects to home.
 *
 * If the PKCE code verifier is missing (e.g. stale session, browser restart,
 * or previous failed attempt consumed it), automatically starts a fresh login
 * flow instead of showing a dead-end error.
 */

import { useEffect, useState, useRef } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../contexts/AuthContext";

export function AuthCallbackPage() {
  const { handleCallback, iamLogin } = useAuth();
  const navigate = useNavigate();
  const [error, setError] = useState<string | null>(null);
  const retryAttempted = useRef(false);

  useEffect(() => {
    if (!handleCallback) {
      navigate("/", { replace: true });
      return;
    }

    handleCallback()
      .then(() => {
        // Use requestAnimationFrame to ensure React has flushed the
        // isAuthenticated=true state update from completeAuth() before we
        // navigate to "/" (which mounts AuthGuard).
        requestAnimationFrame(() => {
          navigate("/", { replace: true });
        });
      })
      .catch((err) => {
        const msg = err instanceof Error ? err.message : String(err);

        // If PKCE code verifier is missing, the login flow wasn't properly
        // initiated in this session (stale session, browser restart, or the
        // previous failed attempt consumed it). Start a fresh login.
        if (msg.includes("PKCE") && iamLogin && !retryAttempted.current) {
          retryAttempted.current = true;
          console.warn("[auth-callback] PKCE verifier missing, restarting login flow");
          iamLogin().catch(() => {
            setError("Login failed. Please try again.");
          });
          return;
        }

        setError(msg);
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
