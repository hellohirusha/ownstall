import { gql } from "@apollo/client";
import { useQuery } from "@apollo/client/react";

const GET_TENANT = gql`
  query GetTenantBySubdomain($subdomain: String!) {
    tenant(subdomain: $subdomain) {
      id
      name
      subdomain
      plan
    }
  }
`;

export function useTenant() {
  // The ?store= param takes precedence — hosted domains (e.g. *.vercel.app)
  // would otherwise be misread as store subdomains. Hostname parsing kicks in
  // for real per-store subdomains; localStorage is the dev fallback.
  let subdomain =
    new URLSearchParams(window.location.search).get("store") || "";

  if (!subdomain) {
    const parts = window.location.hostname.split("."); // e.g. "teststore.ownstall.app"
    if (parts.length >= 3) {
      subdomain = parts[0]; // "teststore"
    } else {
      subdomain = localStorage.getItem("demo_subdomain") || "";
    }
  } else {
    // Remember the store so /store, /cart and /order/success work without ?store=
    localStorage.setItem("demo_subdomain", subdomain);
  }

  const { data, loading, error } = useQuery<{
    tenant?: { id: string; name: string; subdomain: string; plan: string };
  }>(GET_TENANT, {
    variables: { subdomain },
    skip: !subdomain,
  });

  return {
    tenant: data?.tenant,
    subdomain,
    loading,
    error,
  };
}
