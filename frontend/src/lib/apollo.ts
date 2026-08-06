import {
  ApolloClient,
  InMemoryCache,
  createHttpLink,
  from,
  ServerError,
} from "@apollo/client";
import { setContext } from "@apollo/client/link/context";
import { onError } from "@apollo/client/link/error";
import { EMPTY, from as observableFrom } from "rxjs";
import { mergeMap } from "rxjs/operators";

import {
  clearSession,
  currentScope,
  getAccessToken,
  isSignedIn,
  LOGIN_PATH,
  refreshAccessToken,
} from "./session";

// ── HTTP Link — points to Go backend GraphQL endpoint ────────
const httpLink = createHttpLink({
  uri: process.env.REACT_APP_GRAPHQL_URL || "http://localhost:8080/query",
});

// ── Auth Link — attaches the token for whichever audience owns this page ──
// Storefronts are browsed by guests, so a missing token is normal here and
// the header is simply omitted rather than sent empty.
const authLink = setContext((_, { headers }) => {
  const token = getAccessToken(currentScope());
  if (!token) return { headers };
  return {
    headers: { ...headers, authorization: `Bearer ${token}` },
  };
});

// ── Error Link — refresh the access token once, then retry ───────────────
// Access tokens live 15 minutes, so an expired token mid-session is routine.
// The retry has to be returned as an observable: returning `forward()` from
// inside a promise callback resolves after the link chain has already given
// up, which is why refresh never actually recovered a request before.
const errorLink = onError(({ error, operation, forward }) => {
  if (!ServerError.is(error) || error.statusCode !== 401) return;

  const scope = currentScope();

  // A guest hitting a protected field should see the error, not be bounced
  // to a sign-in page they never asked for.
  if (!isSignedIn(scope)) return;

  return observableFrom(refreshAccessToken(scope)).pipe(
    mergeMap((token) => {
      if (!token) {
        clearSession(scope);
        window.location.href = LOGIN_PATH[scope];
        return EMPTY;
      }

      operation.setContext(({ headers = {} }: { headers?: object }) => ({
        headers: { ...headers, authorization: `Bearer ${token}` },
      }));

      return forward(operation);
    }),
  );
});

// ── Apollo Client ─────────────────────────────────────────────
export const apolloClient = new ApolloClient({
  link: from([errorLink, authLink, httpLink]),
  cache: new InMemoryCache({
    typePolicies: {
      Product: {
        // Products are uniquely identified by id
        keyFields: ["id"],
      },
      Store: {
        keyFields: ["id"],
      },
    },
  }),
  defaultOptions: {
    watchQuery: {
      // Always check network for fresh data,
      // but show cache immediately while loading
      fetchPolicy: "cache-and-network",
    },
  },
});
