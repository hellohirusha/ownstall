import { NavLink, useNavigate } from "react-router-dom";
import {
  Briefcase,
  Factory,
  LogOut,
  Mail,
  MessageSquare,
  Package,
  ReceiptText,
  Sparkles,
  Store,
} from "lucide-react";
import { Logo } from "./Logo";
import { StoreStatusBanner } from "./StoreStatusBanner";
import { clearSession, getSessionUser } from "../lib/session";

const linkClass = ({ isActive }: { isActive: boolean }) =>
  `flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm font-medium transition-colors ${
    isActive
      ? "bg-brand-600 text-white"
      : "text-ink-600 hover:text-ink-900 hover:bg-ink-100"
  }`;

const SECTIONS = [
  { to: "/admin/stall", icon: Store, label: "Stall" },
  { to: "/admin/products", icon: Package, label: "Products" },
  { to: "/admin/orders", icon: ReceiptText, label: "Orders" },
  { to: "/admin/notify", icon: Mail, label: "Notify" },
  { to: "/admin/reply", icon: MessageSquare, label: "Reply" },
  { to: "/admin/hire", icon: Briefcase, label: "Hire Me" },
  { to: "/admin/manufacturing", icon: Factory, label: "Production" },
  { to: "/admin/ai", icon: Sparkles, label: "AI" },
];

export function AdminNav() {
  const navigate = useNavigate();
  const user = getSessionUser("tenant");

  const handleSignOut = () => {
    clearSession("tenant");
    navigate("/login", { replace: true });
  };

  return (
    <>
    <header className="sticky top-0 z-10 bg-white border-b border-ink-200">
      <div className="max-w-6xl mx-auto px-4 py-3 flex items-center justify-between gap-4">
        <Logo size={28} />

        <nav className="flex items-center gap-1 overflow-x-auto">
          {SECTIONS.map(({ to, icon: Icon, label }) => (
            <NavLink key={to} to={to} className={linkClass}>
              <Icon size={16} />
              <span className="whitespace-nowrap">{label}</span>
            </NavLink>
          ))}
        </nav>

        <div className="flex items-center gap-3">
          {user?.email && (
            <span className="hidden lg:block text-sm text-ink-500 truncate max-w-[180px]">
              {user.email}
            </span>
          )}
          <button
            onClick={handleSignOut}
            className="flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm font-medium
                       text-ink-600 hover:text-ink-900 hover:bg-ink-100 transition-colors"
          >
            <LogOut size={16} />
            <span className="hidden sm:inline">Sign out</span>
          </button>
        </div>
      </div>
    </header>
    <StoreStatusBanner />
    </>
  );
}
