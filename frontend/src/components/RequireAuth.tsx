import { Navigate, useLocation } from "react-router-dom";
import { isSignedIn, LOGIN_PATH, SessionScope } from "../lib/session";

interface RequireAuthProps {
  scope: SessionScope;
  children: React.ReactNode;
}

// Gates a route on a session for one audience. It only checks that a token
// exists — the API is what actually enforces access, and an expired token
// there triggers the refresh-or-redirect path in lib/apollo.ts.
export function RequireAuth({ scope, children }: RequireAuthProps) {
  const location = useLocation();

  if (!isSignedIn(scope)) {
    return (
      <Navigate
        to={LOGIN_PATH[scope]}
        replace
        state={{ from: location.pathname + location.search }}
      />
    );
  }

  return <>{children}</>;
}
