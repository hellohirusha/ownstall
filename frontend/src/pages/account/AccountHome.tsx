import { gql } from "@apollo/client";
import { useQuery } from "@apollo/client/react";
import { Link, useNavigate } from "react-router-dom";
import { LogOut, Package, ShoppingBag, Store } from "lucide-react";
import { clearSession, getSessionUser } from "../../lib/session";
import { apolloClient } from "../../lib/apollo";

const MY_ORDERS = gql`
  query MyOrders {
    myOrders {
      id
      storeName
      storeSubdomain
      status
      total
      trackingNumber
      createdAt
      items {
        productName
        variantTitle
        quantity
        totalPrice
        imageUrl
      }
    }
  }
`;

interface BuyerOrder {
  id: string;
  storeName: string;
  storeSubdomain: string;
  status: string;
  total: number;
  trackingNumber: string | null;
  createdAt: string;
  items: {
    productName: string;
    variantTitle: string | null;
    quantity: number;
    totalPrice: number;
    imageUrl: string | null;
  }[];
}

const STATUS_STYLE: Record<string, string> = {
  paid: "bg-green-100 text-green-700",
  processing: "bg-blue-100 text-blue-700",
  shipped: "bg-green-100 text-green-700",
  delivered: "bg-green-100 text-green-700",
  pending: "bg-ink-100 text-ink-600",
  cancelled: "bg-red-100 text-red-700",
  refunded: "bg-amber-100 text-amber-800",
  failed: "bg-red-100 text-red-700",
};

export function AccountHome() {
  const navigate = useNavigate();
  const buyer = getSessionUser("buyer");
  const { data, loading, error } = useQuery<{ myOrders: BuyerOrder[] }>(
    MY_ORDERS,
  );

  const handleSignOut = () => {
    clearSession("buyer");
    void apolloClient.clearStore();
    navigate("/", { replace: true });
  };

  const orders = data?.myOrders ?? [];
  const displayName =
    [buyer?.first_name, buyer?.last_name].filter(Boolean).join(" ") ||
    buyer?.email ||
    "there";

  return (
    <div className="max-w-4xl mx-auto px-4 py-10">
      <header className="flex flex-wrap items-start justify-between gap-4 mb-10">
        <div>
          <h1 className="text-3xl font-bold text-ink-900 tracking-tight">
            Hello, {displayName}
          </h1>
          <p className="text-ink-500 mt-1">{buyer?.email}</p>
        </div>
        <button
          onClick={handleSignOut}
          className="inline-flex items-center gap-1.5 px-4 py-2 border border-ink-200
                     rounded-lg text-sm font-medium text-ink-600 hover:border-ink-400
                     transition-colors"
        >
          <LogOut size={16} />
          Sign out
        </button>
      </header>

      <h2 className="text-lg font-bold text-ink-900 mb-4">Your orders</h2>

      {loading ? (
        <div className="space-y-3">
          {[...Array(3)].map((_, i) => (
            <div
              key={i}
              className="animate-pulse rounded-2xl border border-ink-200 p-5"
            >
              <div className="h-4 bg-ink-100 rounded w-1/3 mb-3" />
              <div className="h-3 bg-ink-100 rounded w-1/2" />
            </div>
          ))}
        </div>
      ) : error ? (
        <div className="rounded-2xl border border-red-200 bg-red-50 p-5 text-sm text-red-700">
          Could not load your orders. Please try again.
        </div>
      ) : orders.length === 0 ? (
        <div className="text-center py-16 border border-dashed border-ink-200 rounded-2xl">
          <ShoppingBag className="mx-auto h-12 w-12 text-ink-200 mb-4" />
          <p className="text-ink-500 mb-1">No orders yet.</p>
          <p className="text-sm text-ink-400 mb-6">
            Anything you buy — as a guest with this email, or signed in — shows
            up here.
          </p>
          <Link
            to="/stores"
            className="inline-flex items-center gap-2 px-5 py-2.5 bg-brand-600
                       hover:bg-brand-700 text-white text-sm font-semibold rounded-lg
                       transition-colors"
          >
            <Store size={16} />
            Browse stalls
          </Link>
        </div>
      ) : (
        <div className="space-y-4">
          {orders.map((order) => (
            <div
              key={order.id}
              className="rounded-2xl border border-ink-200 bg-white p-5 shadow-card"
            >
              <div className="flex flex-wrap items-start justify-between gap-3 mb-4">
                <div>
                  <Link
                    to={`/store?store=${order.storeSubdomain}`}
                    className="font-semibold text-ink-900 hover:text-brand-700 transition-colors"
                  >
                    {order.storeName}
                  </Link>
                  <p className="text-xs text-ink-400 mt-0.5">
                    {new Date(order.createdAt).toLocaleDateString(undefined, {
                      year: "numeric",
                      month: "short",
                      day: "numeric",
                    })}
                    {" · "}
                    <span className="font-mono">
                      {order.id.slice(-8).toUpperCase()}
                    </span>
                  </p>
                </div>
                <div className="text-right">
                  <span
                    className={`inline-block px-2.5 py-0.5 rounded-full text-xs font-semibold ${
                      STATUS_STYLE[order.status] ?? "bg-ink-100 text-ink-600"
                    }`}
                  >
                    {order.status}
                  </span>
                  <p className="font-bold text-ink-900 mt-1">
                    ${order.total.toFixed(2)}
                  </p>
                </div>
              </div>

              <ul className="divide-y divide-ink-100">
                {order.items.map((item, i) => (
                  <li key={i} className="flex items-center gap-3 py-2.5">
                    <div className="w-12 h-12 rounded-lg bg-ink-50 overflow-hidden flex-shrink-0">
                      {item.imageUrl ? (
                        <img
                          src={item.imageUrl}
                          alt=""
                          className="w-full h-full object-cover"
                          loading="lazy"
                        />
                      ) : (
                        <div className="w-full h-full flex items-center justify-center text-ink-300">
                          <Package size={16} />
                        </div>
                      )}
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-medium text-ink-900 truncate">
                        {item.productName}
                      </p>
                      {item.variantTitle && item.variantTitle !== "Default" && (
                        <p className="text-xs text-ink-400">
                          {item.variantTitle}
                        </p>
                      )}
                    </div>
                    <p className="text-sm text-ink-500">×{item.quantity}</p>
                    <p className="text-sm font-medium text-ink-900 w-16 text-right">
                      ${item.totalPrice.toFixed(2)}
                    </p>
                  </li>
                ))}
              </ul>

              {order.trackingNumber && (
                <p className="mt-3 pt-3 border-t border-ink-100 text-xs text-ink-500">
                  Tracking:{" "}
                  <span className="font-mono text-ink-700">
                    {order.trackingNumber}
                  </span>
                </p>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
