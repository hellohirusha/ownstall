import { Routes, Route, Navigate, Outlet } from "react-router-dom";
import { RequireAuth } from "./components/RequireAuth";
import { PublicLayout } from "./components/PublicLayout";
import { LandingPage } from "./pages/Landing";
import { AboutPage } from "./pages/company/About";
import { ContactPage } from "./pages/company/Contact";
import { TermsPage } from "./pages/legal/Terms";
import { PrivacyPage } from "./pages/legal/Privacy";
import { Signup } from "./pages/auth/Signup";
import { Login } from "./pages/auth/Login";
import { BuyerLogin } from "./pages/account/BuyerLogin";
import { BuyerSignup } from "./pages/account/BuyerSignup";
import { AccountHome } from "./pages/account/AccountHome";
import { PlatformLogin } from "./pages/platform/PlatformLogin";
import { PlatformDashboard } from "./pages/platform/PlatformDashboard";
import { StoreFinderPage } from "./pages/store/StoreFinder";
import { StallProfilePage } from "./pages/admin/StallProfile";
import { ProductsPage } from "./pages/admin/Products";
import { NewProductPage } from "./pages/admin/NewProduct";
import { OrdersPage } from "./pages/admin/Orders";
import { NotifyPage } from "./pages/admin/Notify";
import { ReplyPage } from "./pages/admin/Reply";
import { ReplyMetricsPage } from "./pages/admin/ReplyMetrics";
import { TicketDetailPage } from "./pages/admin/TicketDetail";
import { HireMePage } from "./pages/admin/HireMe";
import { ManufacturingPage } from "./pages/admin/Manufacturing";
import { AIFeaturesPage } from "./pages/admin/AIFeatures";
import { BookingConversationPage } from "./pages/admin/BookingConversation";
import { CreatorProfilePage } from "./pages/store/CreatorProfile";
import { StorefrontPage } from "./pages/store/Storefront";
import { ProductDetailPage } from "./pages/store/ProductDetail";
import { CartPage } from "./pages/store/Cart";
import { OrderSuccessPage } from "./pages/store/OrderSuccess";

function App() {
  return (
    <Routes>
      {/* Public marketing + company pages share one header and footer */}
      <Route element={<PublicLayout />}>
        <Route path="/" element={<LandingPage />} />
        <Route path="/about" element={<AboutPage />} />
        <Route path="/contact" element={<ContactPage />} />
        <Route path="/terms" element={<TermsPage />} />
        <Route path="/privacy" element={<PrivacyPage />} />
        <Route path="/stores" element={<StoreFinderPage />} />

        {/* Buyer account — same public chrome, but signed-in only */}
        <Route
          path="/account"
          element={
            <RequireAuth scope="buyer">
              <AccountHome />
            </RequireAuth>
          }
        />
      </Route>

      <Route path="/signup" element={<Signup />} />
      <Route path="/login" element={<Login />} />

      {/* Buyer auth — standalone, no site chrome */}
      <Route path="/account/login" element={<BuyerLogin />} />
      <Route path="/account/signup" element={<BuyerSignup />} />

      {/* Platform operator console — kept off the tenant /admin prefix so the
          two dashboards can never be confused for one another */}
      <Route path="/platform/login" element={<PlatformLogin />} />
      <Route
        path="/platform"
        element={
          <RequireAuth scope="admin">
            <PlatformDashboard />
          </RequireAuth>
        }
      />

      {/* Admin (store owner) — every page below needs a tenant session */}
      <Route
        element={
          <RequireAuth scope="tenant">
            <Outlet />
          </RequireAuth>
        }
      >
        <Route path="/admin/stall" element={<StallProfilePage />} />
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
        <Route path="/admin/ai" element={<AIFeaturesPage />} />
        {/* Stripe onboarding return/refresh URL — back to the dashboard */}
        <Route
          path="/admin/hire/onboarding"
          element={<Navigate to="/admin/hire" replace />}
        />
        <Route
          path="/admin/hire/bookings/:id"
          element={<BookingConversationPage />}
        />
      </Route>

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
