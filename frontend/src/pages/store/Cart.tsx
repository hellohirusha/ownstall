import { Link, useNavigate } from "react-router-dom";
import { ArrowLeft, Trash2, Minus, Plus, ShoppingBag, Loader2 } from "lucide-react";
import { useState } from "react";
import toast from "react-hot-toast";
import { useCart } from "../../lib/cart";
import { getAccessToken, getSessionUser } from "../../lib/session";

export function CartPage() {
  const navigate = useNavigate();
  const { items, removeItem, updateQuantity, totalItems, totalPrice } =
    useCart();
  const [loading, setLoading] = useState(false);
  // A signed-in shopper should not have to retype the address we already have
  const buyer = getSessionUser("buyer");
  const [email, setEmail] = useState(buyer?.email ?? "");

  const handleCheckout = async () => {
    if (!email) {
      toast.error("Enter your email to continue");
      return;
    }
    if (items.length === 0) return;

    setLoading(true);

    try {
      // Checkout is open to guests. When a shopper IS signed in we send their
      // buyer token so the order is attached to their account — but never the
      // seller token, which is what used to make the stall owner the only
      // person who could complete a purchase.
      const token = getAccessToken("buyer");
      const res = await fetch(
        `${process.env.REACT_APP_API_URL}/api/checkout/session`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            ...(token ? { Authorization: `Bearer ${token}` } : {}),
          },
          body: JSON.stringify({
            items: items.map((i) => ({
              variant_id: i.variantId,
              quantity: i.quantity,
            })),
            customer_email: email,
          }),
        },
      );

      const data = await res.json();

      if (!res.ok) {
        toast.error(data.error || "Checkout failed");
        return;
      }

      // Redirect to Stripe hosted checkout page
      window.location.href = data.checkout_url;
    } catch {
      toast.error("Failed to start checkout");
    } finally {
      setLoading(false);
    }
  };

  if (items.length === 0) {
    return (
      <div className="min-h-screen flex items-center justify-center px-4">
        <div className="text-center">
          <ShoppingBag className="mx-auto h-20 w-20 text-gray-200 mb-4" />
          <h2 className="text-xl font-bold text-gray-900 mb-2">
            Your cart is empty
          </h2>
          <p className="text-gray-500 mb-6">Add some products to get started</p>
          <button
            onClick={() => navigate(-1)}
            className="px-6 py-2.5 bg-gray-900 text-white rounded-lg font-medium hover:bg-gray-800"
          >
            Continue shopping
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <div className="max-w-2xl mx-auto px-4 py-8">
        <Link
          to="/store"
          className="inline-flex items-center gap-1.5 text-sm text-gray-500
                     hover:text-gray-900 transition-colors mb-4"
        >
          <ArrowLeft size={16} />
          Continue shopping
        </Link>
        <h1 className="text-2xl font-bold text-gray-900 mb-6">
          Cart ({totalItems()} {totalItems() === 1 ? "item" : "items"})
        </h1>

        {/* Cart items */}
        <div className="bg-white rounded-2xl divide-y divide-gray-100 mb-4 overflow-hidden">
          {items.map((item) => (
            <div key={item.variantId} className="flex gap-4 p-4">
              {/* Image */}
              <div className="w-20 h-20 bg-gray-100 rounded-xl overflow-hidden flex-shrink-0">
                {item.imageUrl && (
                  <img
                    src={item.imageUrl}
                    alt=""
                    className="w-full h-full object-cover"
                  />
                )}
              </div>

              {/* Info */}
              <div className="flex-1 min-w-0">
                <h3 className="font-medium text-gray-900 truncate">
                  {item.productName}
                </h3>
                {item.variantTitle !== "Default" && (
                  <p className="text-sm text-gray-500">{item.variantTitle}</p>
                )}
                <p className="text-sm font-semibold text-gray-900 mt-1">
                  ${(item.price * item.quantity).toFixed(2)}
                </p>
              </div>

              {/* Quantity + remove */}
              <div className="flex flex-col items-end gap-2">
                <button
                  onClick={() => removeItem(item.variantId)}
                  className="text-gray-400 hover:text-red-500 transition-colors"
                >
                  <Trash2 size={16} />
                </button>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() =>
                      updateQuantity(item.variantId, item.quantity - 1)
                    }
                    className="w-7 h-7 flex items-center justify-center rounded-full
                               border border-gray-200 hover:border-gray-400 transition-colors"
                  >
                    <Minus size={12} />
                  </button>
                  <span className="text-sm font-medium w-6 text-center">
                    {item.quantity}
                  </span>
                  <button
                    onClick={() =>
                      updateQuantity(item.variantId, item.quantity + 1)
                    }
                    className="w-7 h-7 flex items-center justify-center rounded-full
                               border border-gray-200 hover:border-gray-400 transition-colors"
                  >
                    <Plus size={12} />
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>

        {/* Order summary */}
        <div className="bg-white rounded-2xl p-5 mb-4">
          <h2 className="font-semibold text-gray-900 mb-4">Order summary</h2>
          <div className="space-y-2 text-sm">
            <div className="flex justify-between text-gray-600">
              <span>Subtotal ({totalItems()} items)</span>
              <span>${totalPrice().toFixed(2)}</span>
            </div>
            <div className="flex justify-between text-gray-600">
              <span>Shipping</span>
              <span className="text-green-600">Free</span>
            </div>
          </div>
          <div className="border-t border-gray-100 mt-3 pt-3 flex justify-between font-bold">
            <span>Total</span>
            <span>${totalPrice().toFixed(2)}</span>
          </div>
        </div>

        {/* Email + Checkout */}
        <div className="bg-white rounded-2xl p-5">
          {!buyer && (
            <div className="mb-4 p-3 bg-ink-50 border border-ink-200 rounded-lg text-sm text-ink-600">
              Checking out as a guest.{" "}
              <Link
                to="/account/login"
                state={{ from: "/cart" }}
                className="text-brand-700 font-medium hover:underline"
              >
                Sign in
              </Link>{" "}
              to keep this order in your history — or just carry on below.
            </div>
          )}

          <label className="text-sm font-medium text-gray-700 block mb-1">
            Email for order confirmation
          </label>
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="you@example.com"
            className="w-full px-3 py-2 border border-gray-200 rounded-lg text-sm
                       focus:outline-none focus:ring-2 focus:ring-brand-500 mb-4"
          />

          <button
            onClick={handleCheckout}
            disabled={loading || !email}
            className="w-full py-4 bg-gray-900 hover:bg-gray-800 text-white font-semibold
                       rounded-xl transition-colors disabled:opacity-50 disabled:cursor-not-allowed
                       flex items-center justify-center gap-2"
          >
            {loading ? (
              <>
                <Loader2 className="animate-spin" size={20} /> Redirecting to
                payment...
              </>
            ) : (
              `Pay $${totalPrice().toFixed(2)} with Stripe`
            )}
          </button>
          <p className="text-xs text-center text-gray-400 mt-2">
            Secured by Stripe. Your card details are never stored on our
            servers.
          </p>
        </div>
      </div>
    </div>
  );
}
