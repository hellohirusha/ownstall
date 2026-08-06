// Ownstall has three separate audiences and a visitor can plausibly be
// signed in as more than one at a time — a store owner shopping on someone
// else's stall, or an operator with their own test store. Keeping one token
// per scope means signing in as a buyer never logs you out of your dashboard.
//
// The tenant keys keep their original names: the mobile app and any browser
// with a live session already store tokens under them.

export type SessionScope = "tenant" | "buyer" | "admin";

export interface SessionUser {
  id: string;
  email: string;
  role?: string;
  tenant_id?: string;
  first_name?: string;
  last_name?: string;
}

interface ScopeKeys {
  access: string;
  refresh: string;
  user: string;
}

const KEYS: Record<SessionScope, ScopeKeys> = {
  tenant: {
    access: "access_token",
    refresh: "refresh_token",
    user: "user",
  },
  buyer: {
    access: "buyer_access_token",
    refresh: "buyer_refresh_token",
    user: "buyer_user",
  },
  admin: {
    access: "admin_access_token",
    refresh: "admin_refresh_token",
    user: "admin_user",
  },
};

export function saveSession(
  scope: SessionScope,
  tokens: { access_token: string; refresh_token: string; user: SessionUser },
): void {
  const keys = KEYS[scope];
  localStorage.setItem(keys.access, tokens.access_token);
  localStorage.setItem(keys.refresh, tokens.refresh_token);
  localStorage.setItem(keys.user, JSON.stringify(tokens.user));
}

export function clearSession(scope: SessionScope): void {
  const keys = KEYS[scope];
  localStorage.removeItem(keys.access);
  localStorage.removeItem(keys.refresh);
  localStorage.removeItem(keys.user);
}

export function getAccessToken(scope: SessionScope): string | null {
  return localStorage.getItem(KEYS[scope].access);
}

export function getRefreshToken(scope: SessionScope): string | null {
  return localStorage.getItem(KEYS[scope].refresh);
}

export function getSessionUser(scope: SessionScope): SessionUser | null {
  const raw = localStorage.getItem(KEYS[scope].user);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as SessionUser;
  } catch {
    return null;
  }
}

export function isSignedIn(scope: SessionScope): boolean {
  return !!getAccessToken(scope);
}

// Which session a request belongs to is decided by where the visitor is,
// not by which tokens happen to exist. /platform is the operator console,
// /admin is the store owner's dashboard, and everything else — storefronts,
// the store finder, the cart — is buyer or guest territory.
export function scopeForPath(pathname: string): SessionScope {
  if (pathname.startsWith("/platform")) return "admin";
  if (pathname.startsWith("/admin")) return "tenant";
  return "buyer";
}

export function currentScope(): SessionScope {
  return scopeForPath(window.location.pathname);
}

// Where to send someone who needs to sign in before seeing this page.
export const LOGIN_PATH: Record<SessionScope, string> = {
  tenant: "/login",
  buyer: "/account/login",
  admin: "/platform/login",
};

const API_URL = process.env.REACT_APP_API_URL || "http://localhost:8080";

// Refresh endpoints are per-audience because the tokens are minted with
// different scopes and must not be interchangeable.
const REFRESH_ENDPOINT: Record<SessionScope, string> = {
  tenant: "/api/refresh",
  buyer: "/api/buyer/refresh",
  admin: "/api/admin/refresh",
};

// Exchanges the stored refresh token for a fresh access token.
// Returns null when the refresh token is missing, expired or revoked —
// the caller is then responsible for sending the visitor to sign in.
export async function refreshAccessToken(
  scope: SessionScope,
): Promise<string | null> {
  const refreshToken = getRefreshToken(scope);
  if (!refreshToken) return null;

  try {
    const res = await fetch(`${API_URL}${REFRESH_ENDPOINT[scope]}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refreshToken }),
    });
    if (!res.ok) return null;

    const data = await res.json();
    if (!data.access_token) return null;

    localStorage.setItem(KEYS[scope].access, data.access_token);
    return data.access_token as string;
  } catch {
    return null;
  }
}
