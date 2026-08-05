import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { ApolloProvider } from "@apollo/client/react";
import { Toaster } from "react-hot-toast";
import { apolloClient } from "./lib/apollo";
import { initSentry } from "./lib/sentry";
import "./index.css";
import App from "./App";
import reportWebVitals from "./reportWebVitals";

// Before render, so an error thrown during the first paint is caught
initSentry();

const root = ReactDOM.createRoot(document.getElementById("root")!);

root.render(
  <React.StrictMode>
    <ApolloProvider client={apolloClient}>
      <BrowserRouter>
        <App />
        <Toaster position="top-right" />
      </BrowserRouter>
    </ApolloProvider>
  </React.StrictMode>,
);

reportWebVitals();
