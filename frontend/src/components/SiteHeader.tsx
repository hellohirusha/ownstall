import { Link, NavLink } from "react-router-dom";
import { ShoppingCart, Store, User } from "lucide-react";
import { Logo } from "./Logo";
import { useCart } from "../lib/cart";
import { getSessionUser, isSignedIn } from "../lib/session";

const navLinkClass = ({ isActive }: { isActive: boolean }) =>
  `px-3 py-2 text-sm font-medium rounded-lg transition-colors ${
    isActive
      ? "text-brand-700 bg-brand-50"
      : "text-ink-600 hover:text-ink-900 hover:bg-ink-50"
  }`;

interface SiteHeaderProps {
  /** Storefronts hide the cart in their own header, so it can be suppressed. */
  showCart?: boolean;
}

export function SiteHeader({ showCart = true }: SiteHeaderProps) {
  const { totalItems } = useCart();
  const buyer = getSessionUser("buyer");
  const sellerSignedIn = isSignedIn("tenant");
  const cartCount = totalItems();

  return (
    <header className="sticky top-0 z-20 bg-white/90 backdrop-blur border-b border-ink-200">
      <div className="max-w-6xl mx-auto px-4 h-16 flex items-center justify-between gap-4">
        <Logo />

        <nav className="hidden md:flex items-center gap-1">
          <NavLink to="/stores" className={navLinkClass}>
            Browse stalls
          </NavLink>
          <NavLink to="/about" className={navLinkClass}>
            About
          </NavLink>
          <NavLink to="/contact" className={navLinkClass}>
            Contact
          </NavLink>
        </nav>

        <div className="flex items-center gap-2">
          {showCart && (
            <Link
              to="/cart"
              aria-label={`Cart, ${cartCount} item${cartCount === 1 ? "" : "s"}`}
              className="relative p-2 text-ink-600 hover:text-ink-900 transition-colors"
            >
              <ShoppingCart size={20} />
              {cartCount > 0 && (
                <span
                  className="absolute -top-0.5 -right-0.5 min-w-[18px] h-[18px] px-1
                             bg-brand-600 text-white text-[11px] font-bold rounded-full
                             flex items-center justify-center"
                >
                  {cartCount}
                </span>
              )}
            </Link>
          )}

          {buyer ? (
            <Link
              to="/account"
              className="flex items-center gap-1.5 px-3 py-2 text-sm font-medium
                         text-ink-600 hover:text-ink-900 transition-colors"
            >
              <User size={16} />
              <span className="hidden sm:inline">Account</span>
            </Link>
          ) : (
            <Link
              to="/account/login"
              className="hidden sm:flex items-center gap-1.5 px-3 py-2 text-sm font-medium
                         text-ink-600 hover:text-ink-900 transition-colors"
            >
              <User size={16} />
              Sign in
            </Link>
          )}

          {sellerSignedIn ? (
            <Link
              to="/admin/products"
              className="flex items-center gap-1.5 px-4 py-2 bg-ink-900 hover:bg-ink-800
                         text-white text-sm font-medium rounded-lg transition-colors"
            >
              <Store size={16} />
              <span className="hidden sm:inline">Dashboard</span>
            </Link>
          ) : (
            <Link
              to="/signup"
              className="px-4 py-2 bg-brand-600 hover:bg-brand-700 text-white text-sm
                         font-medium rounded-lg transition-colors whitespace-nowrap"
            >
              Open your stall
            </Link>
          )}
        </div>
      </div>
    </header>
  );
}
