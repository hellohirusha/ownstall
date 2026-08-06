import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { ApolloProvider } from "@apollo/client/react";
import { apolloClient } from "./lib/apollo";
import App from "./App";

// App owns the routing table, so it has to be rendered inside a router and an
// Apollo provider. These are smoke tests: each asserts that one audience's
// entry point resolves to the right page, which is what breaks when a route is
// renamed or a guard is applied to the wrong scope.
function renderAt(path: string) {
  return render(
    <ApolloProvider client={apolloClient}>
      <MemoryRouter initialEntries={[path]}>
        <App />
      </MemoryRouter>
    </ApolloProvider>,
  );
}

beforeEach(() => {
  localStorage.clear();
});

test("landing page leads with the marketplace pitch", () => {
  renderAt("/");
  expect(
    screen.getByRole("heading", { name: /every stall, one marketplace/i }),
  ).toBeInTheDocument();
});

test("seller sign-in page is reachable at /login", () => {
  renderAt("/login");
  expect(
    screen.getByRole("heading", { name: /store sign in/i }),
  ).toBeInTheDocument();
});

test("buyer sign-in is separate from the seller's", () => {
  renderAt("/account/login");
  expect(
    screen.getByRole("heading", { name: /welcome back/i }),
  ).toBeInTheDocument();
});

test("the seller dashboard bounces a signed-out visitor to /login", () => {
  renderAt("/admin/products");
  expect(
    screen.getByRole("heading", { name: /store sign in/i }),
  ).toBeInTheDocument();
});

test("the operator console bounces a signed-out visitor to its own login", () => {
  renderAt("/platform");
  // Deliberately not the seller login — the two must never be interchangeable
  expect(screen.getByText(/restricted access/i)).toBeInTheDocument();
});
