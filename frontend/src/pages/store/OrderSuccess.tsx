import { useEffect } from "react";
import { useSearchParams, Link } from "react-router-dom";
import { CheckCircle } from "lucide-react";
import { useCart } from "../../lib/cart";

export function OrderSuccessPage() {
  const [params] = useSearchParams();
  const sessionId = params.get("session_id");
  const { clearCart } = useCart();

  // Clear cart once order is confirmed
  useEffect(() => {
    clearCart();
  }, [clearCart]);

  return (
    <div className="min-h-screen bg-gray-50 flex items-center justify-center px-4">
      <div className="text-center max-w-md">
        <div className="w-20 h-20 bg-green-100 rounded-full flex items-center justify-center mx-auto mb-6">
          <CheckCircle className="h-10 w-10 text-brand-600" />
        </div>
        <h1 className="text-3xl font-bold text-gray-900 mb-2">
          Order confirmed!
        </h1>
        <p className="text-gray-500 mb-2">
          Thank you for your purchase. You will receive a confirmation email
          shortly.
        </p>
        {sessionId && (
          <p className="text-xs text-gray-400 mb-8 font-mono">
            Reference: {sessionId.slice(-8).toUpperCase()}
          </p>
        )}
        <Link
          to="/store"
          className="px-6 py-3 bg-gray-900 text-white rounded-xl font-medium hover:bg-gray-800"
        >
          Continue shopping
        </Link>
        <p className="mt-6">
          <Link to="/" className="text-sm text-gray-400 hover:text-gray-600">
            Ownstall home
          </Link>
        </p>
      </div>
    </div>
  );
}
