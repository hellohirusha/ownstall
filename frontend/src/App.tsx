import { Routes, Route, Navigate } from "react-router-dom";
import { LandingPage } from "./pages/Landing";
import { Signup } from "./pages/auth/Signup";
import { ProductsPage } from "./pages/admin/Products";
import { NewProductPage } from "./pages/admin/NewProduct";
import { OrdersPage } from "./pages/admin/Orders";
import { NotifyPage } from "./pages/admin/Notify";
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

      {/* Storefront (shoppers) */}
      <Route path="/store" element={<StorefrontPage />} />
      <Route
        path="/store/:subdomain/products/:slug"
        element={<ProductDetailPage />}
      />
      <Route path="/cart" element={<CartPage />} />
      <Route path="/order/success" element={<OrderSuccessPage />} />

      {/* Unknown URLs go home instead of rendering a blank page */}
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

export default App;
