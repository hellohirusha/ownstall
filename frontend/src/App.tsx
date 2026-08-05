import { Routes, Route, Navigate } from "react-router-dom";
import { LandingPage } from "./pages/Landing";
import { Signup } from "./pages/auth/Signup";
import { ProductsPage } from "./pages/admin/Products";
import { NewProductPage } from "./pages/admin/NewProduct";
import { OrdersPage } from "./pages/admin/Orders";
import { NotifyPage } from "./pages/admin/Notify";
import { ReplyPage } from "./pages/admin/Reply";
import { ReplyMetricsPage } from "./pages/admin/ReplyMetrics";
import { TicketDetailPage } from "./pages/admin/TicketDetail";
import { HireMePage } from "./pages/admin/HireMe";
import { ManufacturingPage } from "./pages/admin/Manufacturing";
import { BookingConversationPage } from "./pages/admin/BookingConversation";
import { CreatorProfilePage } from "./pages/store/CreatorProfile";
import { StorefrontPage } from "./pages/store/Storefront";
import { ProductDetailPage } from "./pages/store/ProductDetail";
import { CartPage } from "./pages/store/Cart";
import { OrderSuccessPage } from "./pages/store/OrderSuccess";

function App() {
  return (
    <Routes>
      <Route path="/" element={<LandingPage />} />
      <Route path="/signup" element={<Signup />} />
      {/* No login page yet — send visitors to signup until it exists */}
      <Route path="/login" element={<Navigate to="/signup" replace />} />

      {/* Admin (store owner) */}
      <Route path="/admin/products" element={<ProductsPage />} />
      <Route path="/admin/products/new" element={<NewProductPage />} />
      {/* No admin product-detail page yet — send stale links back to the list */}
      <Route
        path="/admin/products/:id"
        element={<Navigate to="/admin/products" replace />}
      />
      <Route path="/admin/orders" element={<OrdersPage />} />
      <Route path="/admin/notify" element={<NotifyPage />} />
      <Route path="/admin/reply" element={<ReplyPage />} />
      {/* Static segment outranks :id in v7 route ranking */}
      <Route path="/admin/reply/metrics" element={<ReplyMetricsPage />} />
      <Route path="/admin/reply/:id" element={<TicketDetailPage />} />
      <Route path="/admin/hire" element={<HireMePage />} />
      <Route path="/admin/manufacturing" element={<ManufacturingPage />} />
      {/* Stripe onboarding return/refresh URL — back to the dashboard */}
      <Route
        path="/admin/hire/onboarding"
        element={<Navigate to="/admin/hire" replace />}
      />
      <Route
        path="/admin/hire/bookings/:id"
        element={<BookingConversationPage />}
      />

      {/* Storefront (shoppers) */}
      <Route path="/store" element={<StorefrontPage />} />
      <Route
        path="/store/:subdomain/products/:slug"
        element={<ProductDetailPage />}
      />
      <Route path="/store/:subdomain/hire" element={<CreatorProfilePage />} />
      <Route path="/cart" element={<CartPage />} />
      <Route path="/order/success" element={<OrderSuccessPage />} />

      {/* Unknown URLs go home instead of rendering a blank page */}
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

export default App;
