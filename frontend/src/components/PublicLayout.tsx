import { Outlet } from "react-router-dom";
import { SiteHeader } from "./SiteHeader";
import { SiteFooter } from "./SiteFooter";

// Chrome shared by every page a logged-out visitor can reach. Used as a
// layout route so the header and footer are not remounted on navigation.
export function PublicLayout({ showCart = true }: { showCart?: boolean }) {
  return (
    <div className="min-h-screen flex flex-col bg-white">
      <SiteHeader showCart={showCart} />
      <main className="flex-1">
        <Outlet />
      </main>
      <SiteFooter />
    </div>
  );
}
