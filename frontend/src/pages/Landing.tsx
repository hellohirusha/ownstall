import { Link } from "react-router-dom";
import {
  ArrowRight,
  Code,
  CreditCard,
  Database,
  Layers,
  Mail,
  ShoppingBag,
  Store,
  Zap,
} from "lucide-react";

const DEMO_STORE_PATH = "/store?store=hirusha";
const REPO_URL = "https://github.com/hellohirusha/ownstall";

const FEATURES = [
  {
    icon: Layers,
    title: "Multi-tenant by design",
    desc: "One deployment, many stores. Every tenant gets an isolated catalog, orders and email templates on shared infrastructure.",
  },
  {
    icon: Store,
    title: "Instant storefronts",
    desc: "Sign up, add products, and a shareable storefront is live: product pages, variants, cart and checkout included.",
  },
  {
    icon: CreditCard,
    title: "Stripe Checkout",
    desc: "Hosted payment pages with webhook-driven order updates: paid, expired and failed states handled server-side.",
  },
  {
    icon: Zap,
    title: "Redis job queue",
    desc: "A custom queue with scheduled jobs, exponential-backoff retries and a dead-letter queue, consumed by a Go worker.",
  },
  {
    icon: Mail,
    title: "Transactional email",
    desc: "Order confirmations rendered from per-tenant templates and delivered via Resend, with logging and suppression lists.",
  },
  {
    icon: Database,
    title: "Go + GraphQL + Postgres",
    desc: "A Go API serving GraphQL and REST, backed by PostgreSQL with migrations, JWT auth and CI/CD to Railway and Vercel.",
  },
];

const PIPELINE = [
  { step: "1", label: "Customer pays", detail: "Stripe Checkout session" },
  {
    step: "2",
    label: "Webhook fires",
    detail: "Order marked paid in Postgres",
  },
  { step: "3", label: "Job queued", detail: "Email job pushed to Redis" },
  {
    step: "4",
    label: "Email lands",
    detail: "Worker renders + sends via Resend",
  },
];

export function LandingPage() {
  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <header className="sticky top-0 z-10 bg-white/90 backdrop-blur border-b border-gray-100">
        <div className="max-w-6xl mx-auto px-4 py-4 flex items-center justify-between">
          <span className="text-xl font-bold text-gray-900">Ownstall</span>
          <nav className="flex items-center gap-3">
            <a
              href={REPO_URL}
              target="_blank"
              rel="noreferrer"
              className="hidden sm:flex items-center gap-1.5 px-3 py-2 text-sm font-medium
                         text-gray-600 hover:text-gray-900 transition-colors"
            >
              <Code size={16} />
              Source
            </a>
            <Link
              to={DEMO_STORE_PATH}
              className="px-3 py-2 text-sm font-medium text-gray-600 hover:text-gray-900 transition-colors"
            >
              Demo store
            </Link>
            <Link
              to="/signup"
              className="px-4 py-2 bg-gray-900 hover:bg-gray-800 text-white text-sm
                         font-medium rounded-lg transition-colors"
            >
              Create free store
            </Link>
          </nav>
        </div>
      </header>

      <main>
        {/* Hero */}
        <section className="max-w-6xl mx-auto px-4 pt-20 pb-16 text-center">
          <span
            className="inline-block mb-4 px-3 py-1 bg-green-100 text-green-700
                       text-xs font-semibold rounded-full"
          >
            Full-stack portfolio project
          </span>
          <h1 className="text-4xl sm:text-5xl font-bold text-gray-900 tracking-tight mb-4">
            Launch a store in minutes.
          </h1>
          <p className="text-lg text-gray-500 max-w-2xl mx-auto mb-8">
            Ownstall is a multi-tenant e-commerce platform built end-to-end
            with Go, React, PostgreSQL, Redis and Stripe. From signup to
            storefront to the order-confirmation email in your inbox.
          </p>
          <div className="flex flex-wrap items-center justify-center gap-3">
            <Link
              to={DEMO_STORE_PATH}
              className="flex items-center gap-2 px-6 py-3 bg-gray-900 hover:bg-gray-800
                         text-white font-medium rounded-xl transition-colors"
            >
              <ShoppingBag size={18} />
              Visit the demo store
            </Link>
            <Link
              to="/signup"
              className="flex items-center gap-2 px-6 py-3 bg-green-500 hover:bg-green-600
                         text-white font-medium rounded-xl transition-colors"
            >
              Build your own
              <ArrowRight size={18} />
            </Link>
            <a
              href={REPO_URL}
              target="_blank"
              rel="noreferrer"
              className="flex items-center gap-2 px-6 py-3 bg-white border border-gray-200
                         hover:border-gray-400 text-gray-700 font-medium rounded-xl transition-colors"
            >
              <Code size={18} />
              Read the code
            </a>
          </div>
        </section>

        {/* Feature grid */}
        <section className="max-w-6xl mx-auto px-4 pb-16">
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {FEATURES.map((feature) => {
              const Icon = feature.icon;
              return (
                <div
                  key={feature.title}
                  className="bg-white rounded-2xl border border-gray-100 p-6"
                >
                  <div
                    className="w-10 h-10 bg-green-100 rounded-xl flex items-center
                               justify-center mb-4"
                  >
                    <Icon size={20} className="text-green-600" />
                  </div>
                  <h3 className="font-semibold text-gray-900 mb-1">
                    {feature.title}
                  </h3>
                  <p className="text-sm text-gray-500">{feature.desc}</p>
                </div>
              );
            })}
          </div>
        </section>

        {/* Order pipeline */}
        <section className="max-w-6xl mx-auto px-4 pb-20">
          <div className="bg-white rounded-2xl border border-gray-100 p-8">
            <h2 className="text-xl font-bold text-gray-900 mb-1 text-center">
              What happens when someone buys
            </h2>
            <p className="text-sm text-gray-500 text-center mb-8">
              The full order pipeline, running live in this deployment
            </p>
            <div className="grid grid-cols-1 sm:grid-cols-4 gap-6">
              {PIPELINE.map((stage) => (
                <div key={stage.step} className="text-center">
                  <div
                    className="w-8 h-8 mx-auto mb-3 bg-gray-900 text-white text-sm
                               font-bold rounded-full flex items-center justify-center"
                  >
                    {stage.step}
                  </div>
                  <p className="font-medium text-gray-900 text-sm">
                    {stage.label}
                  </p>
                  <p className="text-xs text-gray-400 mt-1">{stage.detail}</p>
                </div>
              ))}
            </div>
          </div>
        </section>
      </main>

      {/* Footer */}
      <footer className="border-t border-gray-100 bg-white">
        <div className="max-w-6xl mx-auto px-4 py-8 text-center">
          <p className="text-sm text-gray-500">
            Built by{" "}
            <a
              href={REPO_URL}
              target="_blank"
              rel="noreferrer"
              className="font-medium text-gray-700 hover:underline"
            >
              Hirusha
            </a>{" "}
            as a portfolio project.
          </p>
          <p className="text-xs text-gray-400 mt-2">
            Non-commercial demo. Stripe runs in test mode and no real payments
            are processed.
          </p>
        </div>
      </footer>
    </div>
  );
}
