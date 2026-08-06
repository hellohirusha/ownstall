import { Link } from "react-router-dom";
import { Mail, MapPin } from "lucide-react";
import { LogoMark } from "./Logo";

const CONTACT_EMAIL = "hello@ownstall.app";

// Every entry here resolves to a route that exists. Placeholder links in a
// footer are the fastest way to make a product feel unfinished, so anything
// without a page behind it lives as a section on /about instead.
const COLUMNS: {
  heading: string;
  links: { label: string; to: string }[];
}[] = [
  {
    heading: "Marketplace",
    links: [
      { label: "Browse stalls", to: "/stores" },
      { label: "Create an account", to: "/account/signup" },
      { label: "Buyer sign in", to: "/account/login" },
    ],
  },
  {
    heading: "Sell",
    links: [
      { label: "Open your stall", to: "/signup" },
      { label: "Seller sign in", to: "/login" },
      { label: "Seller terms", to: "/terms" },
    ],
  },
  {
    heading: "Company",
    links: [
      { label: "About us", to: "/about" },
      { label: "Contact", to: "/contact" },
    ],
  },
  {
    heading: "Legal",
    links: [
      { label: "Terms of service", to: "/terms" },
      { label: "Privacy policy", to: "/privacy" },
    ],
  },
];

export function SiteFooter() {
  return (
    <footer className="border-t border-ink-200 bg-white">
      <div className="max-w-6xl mx-auto px-4 py-12">
        <div className="grid grid-cols-2 gap-8 md:grid-cols-6">
          {/* Brand block */}
          <div className="col-span-2">
            <div className="flex items-center gap-2 mb-3">
              <LogoMark size={28} />
              <span className="font-bold tracking-tight text-ink-900">
                Ownstall
              </span>
            </div>
            <p className="text-sm text-ink-500 max-w-xs">
              A marketplace of independent stalls. Sellers keep their own
              storefront; shoppers get one place to find them all.
            </p>

            <div className="mt-4 space-y-1.5 text-sm text-ink-500">
              <a
                href={`mailto:${CONTACT_EMAIL}`}
                className="flex items-center gap-2 hover:text-brand-700 transition-colors"
              >
                <Mail size={14} />
                {CONTACT_EMAIL}
              </a>
              <p className="flex items-center gap-2">
                <MapPin size={14} />
                Colombo, Sri Lanka
              </p>
            </div>
          </div>

          {COLUMNS.map((column) => (
            <div key={column.heading}>
              <h3 className="text-xs font-semibold uppercase tracking-wider text-ink-900 mb-3">
                {column.heading}
              </h3>
              <ul className="space-y-2">
                {column.links.map((link) => (
                  <li key={link.label}>
                    <Link
                      to={link.to}
                      className="text-sm text-ink-500 hover:text-brand-700 transition-colors"
                    >
                      {link.label}
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>

        <div
          className="mt-10 pt-6 border-t border-ink-100 flex flex-col sm:flex-row
                     items-center justify-between gap-3"
        >
          <p className="text-xs text-ink-400">
            © {new Date().getFullYear()} Ownstall. All rights reserved.
          </p>
          <p className="text-xs text-ink-400">
            Payments are processed in Stripe test mode — no real money moves.
          </p>
        </div>
      </div>
    </footer>
  );
}
