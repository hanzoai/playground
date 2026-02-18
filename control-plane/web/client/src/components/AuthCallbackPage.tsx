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

  useEffect(() => {
    if (!handleCallback) {
      // Not in IAM mode — redirect home
      navigate("/", { replace: true });
      return;
    }

    handleCallback()
      .then(() => navigate("/", { replace: true }))
      .catch((err) =>
        setError(err instanceof Error ? err.message : String(err)),
      );
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
