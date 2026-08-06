import { Link } from "react-router-dom";
import { ArrowRight, Boxes, Compass, HeartHandshake } from "lucide-react";

const VALUES = [
  {
    icon: Compass,
    title: "Sellers keep their own front door",
    desc: "A stall on Ownstall is not a listing in someone else's catalogue. It has its own storefront, its own address and its own customer relationships.",
  },
  {
    icon: HeartHandshake,
    title: "Trust is the product",
    desc: "Every stall is reviewed before it goes public. When a stall stops meeting the bar, we restrict or suspend it rather than hoping shoppers notice.",
  },
  {
    icon: Boxes,
    title: "Small sellers deserve real tooling",
    desc: "Order pipelines, transactional email, support ticketing, production tracking — the things large retailers take for granted, available to a one-person stall.",
  },
];

export function AboutPage() {
  return (
    <>
      <section className="border-b border-ink-200 bg-ink-50">
        <div className="max-w-3xl mx-auto px-4 py-16">
          <p className="text-sm font-semibold text-brand-700 mb-2">About us</p>
          <h1 className="text-4xl font-bold text-ink-900 tracking-tight mb-4">
            A marketplace that doesn't flatten its sellers.
          </h1>
          <p className="text-lg text-ink-500">
            Ownstall exists because independent sellers keep being asked to
            choose between two bad options: run a storefront nobody can find, or
            become an anonymous row in a giant marketplace. We think you should
            get both — your own shop, and somewhere shoppers actually look.
          </p>
        </div>
      </section>

      <section className="max-w-3xl mx-auto px-4 py-16">
        <h2 className="text-2xl font-bold text-ink-900 mb-8">
          What we believe
        </h2>
        <div className="space-y-8">
          {VALUES.map((value) => {
            const Icon = value.icon;
            return (
              <div key={value.title} className="flex gap-4">
                <div
                  className="w-11 h-11 flex-shrink-0 bg-brand-50 text-brand-700
                             rounded-xl flex items-center justify-center"
                >
                  <Icon size={20} />
                </div>
                <div>
                  <h3 className="font-semibold text-ink-900 mb-1">
                    {value.title}
                  </h3>
                  <p className="text-ink-500">{value.desc}</p>
                </div>
              </div>
            );
          })}
        </div>
      </section>

      <section className="max-w-3xl mx-auto px-4 pb-16">
        <div className="rounded-2xl border border-ink-200 bg-white p-8 shadow-card">
          <h2 className="text-xl font-bold text-ink-900 mb-3">
            How Ownstall is built
          </h2>
          <p className="text-ink-500 mb-4">
            Ownstall runs on a Go API serving GraphQL and REST, PostgreSQL with
            row-level tenant isolation, Redis for the job queue, and Stripe for
            payments. The web app is React; there is a companion Expo app for
            sellers. Every stall shares one deployment while its data stays
            isolated from every other stall's.
          </p>
          <p className="text-sm text-ink-400">
            Ownstall is an independent project. Payments currently run in
            Stripe's test mode.
          </p>
        </div>
      </section>

      <section className="max-w-3xl mx-auto px-4 pb-20">
        <div className="flex flex-wrap gap-3">
          <Link
            to="/stores"
            className="inline-flex items-center gap-2 px-5 py-2.5 bg-brand-600
                       hover:bg-brand-700 text-white text-sm font-semibold rounded-lg
                       transition-colors"
          >
            Browse stalls
            <ArrowRight size={16} />
          </Link>
          <Link
            to="/contact"
            className="inline-flex items-center gap-2 px-5 py-2.5 border border-ink-200
                       hover:border-ink-400 text-ink-700 text-sm font-semibold rounded-lg
                       transition-colors"
          >
            Talk to us
          </Link>
        </div>
      </section>
    </>
  );
}
