import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  ArrowRight,
  BadgeCheck,
  CreditCard,
  Package,
  Search,
  ShieldCheck,
  Sparkles,
  Store,
  Truck,
} from "lucide-react";

// What a shopper does here, in the order they do it.
const BUYER_STEPS = [
  {
    icon: Search,
    title: "Find a stall",
    desc: "Search every approved stall on Ownstall by name, category or what they sell.",
  },
  {
    icon: Package,
    title: "Pick your items",
    desc: "Each stall keeps its own storefront, its own products and its own prices.",
  },
  {
    icon: CreditCard,
    title: "Check out",
    desc: "Pay as a guest, or create an account to keep your order history in one place.",
  },
];

// What a seller does here.
const SELLER_STEPS = [
  {
    icon: Store,
    title: "Open your stall",
    desc: "Claim a name, agree to the seller terms and your storefront is reserved.",
  },
  {
    icon: Package,
    title: "Add your products",
    desc: "Photos, variants, stock and pricing — with AI-assisted product copy if you want it.",
  },
  {
    icon: BadgeCheck,
    title: "Get approved",
    desc: "We review every stall before it goes public, so shoppers can trust what they find.",
  },
];

const PROMISES = [
  {
    icon: ShieldCheck,
    title: "Every stall is reviewed",
    desc: "No stall appears in search until it has been approved. Stalls that break the rules get restricted or suspended.",
  },
  {
    icon: CreditCard,
    title: "Payments handled properly",
    desc: "Checkout runs through Stripe. Card details never touch our servers, and sellers are paid out directly.",
  },
  {
    icon: Truck,
    title: "Order tracking built in",
    desc: "Confirmation emails, status updates and production tracking come as standard for every stall.",
  },
];

export function LandingPage() {
  const navigate = useNavigate();
  const [query, setQuery] = useState("");

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = query.trim();
    navigate(trimmed ? `/stores?q=${encodeURIComponent(trimmed)}` : "/stores");
  };

  return (
    <>
      {/* Hero */}
      <section className="bg-brand-gradient">
        <div className="max-w-6xl mx-auto px-4 pt-20 pb-24 text-center">
          <span
            className="inline-flex items-center gap-1.5 mb-5 px-3 py-1 bg-white/15
                       text-white text-xs font-semibold rounded-full backdrop-blur"
          >
            <Sparkles size={13} />
            Independent sellers, one marketplace
          </span>

          <h1 className="text-4xl sm:text-5xl lg:text-6xl font-bold text-white tracking-tight mb-5">
            Every stall, one marketplace.
          </h1>
          <p className="text-lg text-brand-50/90 max-w-2xl mx-auto mb-8">
            Ownstall gives independent sellers a storefront of their own — and
            gives you one place to find all of them.
          </p>

          {/* Finding a stall is the primary action on this page, so the search
              box is the hero rather than a sign-up form. */}
          <form
            onSubmit={handleSearch}
            className="max-w-xl mx-auto flex flex-col sm:flex-row gap-2"
          >
            <div className="relative flex-1">
              <Search
                size={18}
                className="absolute left-4 top-1/2 -translate-y-1/2 text-ink-400 pointer-events-none"
              />
              <input
                type="search"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Search stalls — ceramics, prints, coffee…"
                aria-label="Search stalls"
                className="w-full pl-11 pr-4 py-3.5 rounded-xl text-sm text-ink-900
                           bg-white shadow-lift focus:outline-none focus:ring-2
                           focus:ring-white/70"
              />
            </div>
            <button
              type="submit"
              className="px-6 py-3.5 bg-ink-900 hover:bg-ink-800 text-white font-semibold
                         rounded-xl transition-colors whitespace-nowrap"
            >
              Browse stalls
            </button>
          </form>

          <p className="mt-4 text-sm text-brand-50/80">
            Or{" "}
            <Link to="/signup" className="font-semibold text-white underline">
              open your own stall
            </Link>{" "}
            — free to start.
          </p>
        </div>
      </section>

      {/* How it works */}
      <section className="max-w-6xl mx-auto px-4 py-20">
        <div className="grid gap-12 lg:grid-cols-2">
          {/* Buyers */}
          <div>
            <h2 className="text-2xl font-bold text-ink-900 mb-1">
              Buying on Ownstall
            </h2>
            <p className="text-sm text-ink-500 mb-8">
              No account needed to start.
            </p>
            <ol className="space-y-6">
              {BUYER_STEPS.map((step, i) => {
                const Icon = step.icon;
                return (
                  <li key={step.title} className="flex gap-4">
                    <div
                      className="w-10 h-10 flex-shrink-0 bg-brand-50 text-brand-700
                                 rounded-xl flex items-center justify-center"
                    >
                      <Icon size={19} />
                    </div>
                    <div>
                      <h3 className="font-semibold text-ink-900">
                        <span className="text-ink-300 mr-1.5">{i + 1}.</span>
                        {step.title}
                      </h3>
                      <p className="text-sm text-ink-500 mt-0.5">{step.desc}</p>
                    </div>
                  </li>
                );
              })}
            </ol>
            <Link
              to="/stores"
              className="inline-flex items-center gap-2 mt-8 px-5 py-2.5 bg-brand-600
                         hover:bg-brand-700 text-white text-sm font-semibold rounded-lg
                         transition-colors"
            >
              Browse stalls
              <ArrowRight size={16} />
            </Link>
          </div>

          {/* Sellers */}
          <div>
            <h2 className="text-2xl font-bold text-ink-900 mb-1">
              Selling on Ownstall
            </h2>
            <p className="text-sm text-ink-500 mb-8">
              Your stall, your brand, your customers.
            </p>
            <ol className="space-y-6">
              {SELLER_STEPS.map((step, i) => {
                const Icon = step.icon;
                return (
                  <li key={step.title} className="flex gap-4">
                    <div
                      className="w-10 h-10 flex-shrink-0 bg-accent-100 text-accent-700
                                 rounded-xl flex items-center justify-center"
                    >
                      <Icon size={19} />
                    </div>
                    <div>
                      <h3 className="font-semibold text-ink-900">
                        <span className="text-ink-300 mr-1.5">{i + 1}.</span>
                        {step.title}
                      </h3>
                      <p className="text-sm text-ink-500 mt-0.5">{step.desc}</p>
                    </div>
                  </li>
                );
              })}
            </ol>
            <Link
              to="/signup"
              className="inline-flex items-center gap-2 mt-8 px-5 py-2.5 bg-ink-900
                         hover:bg-ink-800 text-white text-sm font-semibold rounded-lg
                         transition-colors"
            >
              Open your stall
              <ArrowRight size={16} />
            </Link>
          </div>
        </div>
      </section>

      {/* Promises */}
      <section className="bg-ink-50 border-y border-ink-200">
        <div className="max-w-6xl mx-auto px-4 py-16">
          <h2 className="text-2xl font-bold text-ink-900 text-center mb-2">
            What you can count on
          </h2>
          <p className="text-sm text-ink-500 text-center mb-10 max-w-xl mx-auto">
            A marketplace only works if both sides trust it.
          </p>
          <div className="grid gap-4 md:grid-cols-3">
            {PROMISES.map((promise) => {
              const Icon = promise.icon;
              return (
                <div
                  key={promise.title}
                  className="bg-white rounded-2xl border border-ink-200 p-6 shadow-card"
                >
                  <div
                    className="w-10 h-10 bg-brand-50 rounded-xl flex items-center
                               justify-center mb-4"
                  >
                    <Icon size={20} className="text-brand-700" />
                  </div>
                  <h3 className="font-semibold text-ink-900 mb-1">
                    {promise.title}
                  </h3>
                  <p className="text-sm text-ink-500">{promise.desc}</p>
                </div>
              );
            })}
          </div>
        </div>
      </section>

      {/* Closing CTA */}
      <section className="max-w-6xl mx-auto px-4 py-20">
        <div className="rounded-3xl bg-ink-900 px-6 py-14 text-center sm:px-14">
          <h2 className="text-3xl font-bold text-white mb-3">
            Ready to set up your stall?
          </h2>
          <p className="text-ink-300 max-w-lg mx-auto mb-8">
            Claim your name, add your first products and submit for review. It
            takes a few minutes and costs nothing to start.
          </p>
          <div className="flex flex-wrap items-center justify-center gap-3">
            <Link
              to="/signup"
              className="flex items-center gap-2 px-6 py-3 bg-brand-500 hover:bg-brand-400
                         text-ink-900 font-semibold rounded-xl transition-colors"
            >
              Open your stall
              <ArrowRight size={18} />
            </Link>
            <Link
              to="/about"
              className="px-6 py-3 bg-white/10 hover:bg-white/20 text-white font-medium
                         rounded-xl transition-colors backdrop-blur"
            >
              Learn about us
            </Link>
          </div>
        </div>
      </section>
    </>
  );
}
