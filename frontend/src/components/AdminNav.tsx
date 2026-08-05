import { Link, NavLink } from "react-router-dom";
import { Briefcase, Mail, MessageSquare, Package, ReceiptText } from "lucide-react";

const linkClass = ({ isActive }: { isActive: boolean }) =>
  `flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm font-medium transition-colors ${
    isActive
      ? "bg-gray-900 text-white"
      : "text-gray-600 hover:text-gray-900 hover:bg-gray-100"
  }`;

export function AdminNav() {
  return (
    <header className="sticky top-0 z-10 bg-white border-b border-gray-100">
      <div className="max-w-6xl mx-auto px-4 py-3 flex items-center justify-between">
        <Link to="/" className="text-lg font-bold text-gray-900">
          Ownstall
        </Link>
        <nav className="flex items-center gap-1">
          <NavLink to="/admin/products" className={linkClass}>
            <Package size={16} />
            Products
          </NavLink>
          <NavLink to="/admin/orders" className={linkClass}>
            <ReceiptText size={16} />
            Orders
          </NavLink>
          <NavLink to="/admin/notify" className={linkClass}>
            <Mail size={16} />
            Notify
          </NavLink>
          <NavLink to="/admin/reply" className={linkClass}>
            <MessageSquare size={16} />
            Reply
          </NavLink>
          <NavLink to="/admin/hire" className={linkClass}>
            <Briefcase size={16} />
            Hire Me
          </NavLink>
        </nav>
      </div>
    </header>
  );
}
