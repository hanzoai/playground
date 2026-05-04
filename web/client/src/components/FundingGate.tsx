import { useEffect, useState } from "react";
import type { ReactNode } from "react";
import { useAuth } from "../contexts/AuthContext";

/**
 * FundingGate — runs once after a successful IAM login.
 *
 *  1. Idempotently grants the $5 welcome credit via Commerce
 *     (POST /v1/billing/me/welcome — server-side dedupes by tag).
 *  2. Reads the user's balance (GET /v1/billing/me/balance).
 *  3. If balance > 0 (covers fresh users + topped-up returning users) → renders children.
 *  4. If balance == 0 (welcome credit already consumed) → redirects to pay.hanzo.ai
 *     with a return URL back to the current path. Commerce sends the user
 *     back here once a deposit settles.
 *
 * Designed to be cheap and unobtrusive: skipped entirely when the user
 * is unauthenticated, runs once per session per user, never blocks if
 * the balance API is unreachable (graceful degrade — better to let the
 * user in than to wedge the SPA on a transient network blip).
 */

const PAY_BASE = (import.meta.env.VITE_PAY_URL as string | undefined) ??
  "https://pay.hanzo.ai";
const COMMERCE_BASE = (import.meta.env.VITE_COMMERCE_URL as string | undefined) ??
  "https://api.hanzo.ai";

type GateState = "checking" | "ok" | "redirecting" | "error";

interface BalanceResponse {
  balance_cents?: number;
  cents?: number;
  amount?: number;
  currency?: string;
}

function readCents(body: BalanceResponse | null | undefined): number {
  if (!body) return 0;
  return body.balance_cents ?? body.cents ?? body.amount ?? 0;
}

async function postJson(path: string, token: string): Promise<Response> {
  return fetch(`${COMMERCE_BASE}${path}`, {
    method: "POST",
    headers: {
      authorization: `Bearer ${token}`,
      "content-type": "application/json",
    },
  });
}

async function getJson<T>(path: string, token: string): Promise<T | null> {
  const res = await fetch(`${COMMERCE_BASE}${path}`, {
    headers: { authorization: `Bearer ${token}` },
  });
  if (!res.ok) return null;
  return (await res.json()) as T;
}

export function FundingGate({ children }: { children: ReactNode }) {
  const { isAuthenticated, apiKey } = useAuth();
  const [state, setState] = useState<GateState>("checking");

  useEffect(() => {
    if (!isAuthenticated || !apiKey) {
      // Not authed yet — let upstream AuthGuard handle the IAM redirect.
      setState("ok");
      return;
    }

    let cancelled = false;
    (async () => {
      try {
        // Step 1: idempotent welcome credit. Commerce dedupes by tag.
        await postJson("/v1/billing/me/welcome", apiKey).catch(() => null);

        // Step 2: balance check.
        const body = await getJson<BalanceResponse>(
          "/v1/billing/me/balance?currency=usd",
          apiKey,
        );
        if (cancelled) return;

        const cents = readCents(body);
        if (cents > 0) {
          setState("ok");
          return;
        }

        // Zero balance — route through pay.hanzo.ai.
        const ret = encodeURIComponent(window.location.href);
        setState("redirecting");
        window.location.assign(`${PAY_BASE}/onboard?return=${ret}`);
      } catch {
        // Commerce unreachable — fail open so the SPA stays usable.
        if (!cancelled) setState("ok");
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [isAuthenticated, apiKey]);

  if (state === "checking" || state === "redirecting") {
    return (
      <div className="flex min-h-screen items-center justify-center text-sm text-muted-foreground">
        {state === "redirecting"
          ? "Redirecting to pay.hanzo.ai…"
          : "Verifying balance…"}
      </div>
    );
  }

  return <>{children}</>;
}
