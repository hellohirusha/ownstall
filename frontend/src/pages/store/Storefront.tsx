import { useMemo, useState } from "react";
import { gql } from "@apollo/client";
import { useQuery } from "@apollo/client/react";
import { Link } from "react-router-dom";
import { ArrowLeft, Search, ShoppingCart, X } from "lucide-react";
import { useTenant } from "../../hooks/useTenant";
import { useCart } from "../../lib/cart";
import { LogoMark } from "../../components/Logo";
import { SiteFooter } from "../../components/SiteFooter";

const GET_STORE_PRODUCTS = gql`
  query GetStoreProducts($tenantId: UUID!) {
    products(tenantId: $tenantId, status: "active") {
      id
      name
      slug
      basePrice
      comparePrice
      shortDesc
      images {
        url
        position
      }
      variants {
        isInStock
        stockQuantity
      }
      tags
    }
  }
`;

export function StorefrontPage() {
  const { tenant, subdomain, loading: tenantLoading } = useTenant();
  const { totalItems } = useCart();
  const [query, setQuery] = useState("");

  const { data, loading: productsLoading } = useQuery<{ products: any[] }>(
    GET_STORE_PRODUCTS,
    {
      variables: { tenantId: tenant?.id },
      skip: !tenant?.id,
    },
  );

  const products = useMemo(() => data?.products ?? [], [data]);

  // Searching a single stall is a client-side filter: the catalogue is
  // already loaded, so round-tripping for it would only add latency.
  // Cross-stall search is a different feature and lives on /stores.
  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return products;
    return products.filter((product: any) => {
      const haystack = [
        product.name,
        product.shortDesc ?? "",
        ...(product.tags ?? []),
      ]
        .join(" ")
        .toLowerCase();
      return haystack.includes(q);
    });
  }, [products, query]);

  if (tenantLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="w-8 h-8 border-2 border-brand-600 border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  if (!tenant) {
    return (
      <div className="min-h-screen flex items-center justify-center px-4">
        <div className="text-center max-w-sm">
          <h1 className="text-2xl font-bold text-ink-900">Stall not found</h1>
          <p className="text-ink-500 mt-2">
            {subdomain ? (
              <>
                No stall exists at <strong>{subdomain}</strong>, or it is not
                open yet.
              </>
            ) : (
              <>No stall was specified.</>
            )}
          </p>
          <Link
            to="/stores"
            className="inline-flex items-center gap-2 mt-6 px-5 py-2.5 bg-brand-600
                       hover:bg-brand-700 text-white text-sm font-medium rounded-lg transition-colors"
          >
            <Search size={16} />
            Browse all stalls
          </Link>
        </div>
      </div>
    );
  }

  const loading = productsLoading;

  return (
    <div className="min-h-screen bg-white flex flex-col">
      {/* Storefront header */}
      <header className="sticky top-0 z-10 bg-white/95 backdrop-blur border-b border-ink-200">
        <div className="max-w-6xl mx-auto px-4 py-3">
          <div className="flex items-center justify-between gap-4">
            <div className="min-w-0">
              <Link
                to="/stores"
                className="inline-flex items-center gap-1 text-xs text-ink-400
                           hover:text-brand-700 transition-colors"
              >
                <ArrowLeft size={12} />
                All stalls
              </Link>
              <h1 className="text-xl font-bold text-ink-900 truncate">
                {tenant.name}
              </h1>
            </div>

            <div className="flex items-center gap-2">
              <Link
                to="/cart"
                className="flex items-center gap-2 px-4 py-2 bg-ink-900 text-white
                           rounded-lg text-sm font-medium hover:bg-ink-800 transition-colors"
              >
                <ShoppingCart size={16} />
                <span>Cart ({totalItems()})</span>
              </Link>
            </div>
          </div>

          {/* Search this stall */}
          <div className="relative mt-3">
            <Search
              size={16}
              className="absolute left-3 top-1/2 -translate-y-1/2 text-ink-400 pointer-events-none"
            />
            <input
              type="search"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={`Search ${tenant.name}`}
              aria-label={`Search products in ${tenant.name}`}
              className="w-full pl-9 pr-9 py-2 border border-ink-200 rounded-lg text-sm
                         focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
            />
            {query && (
              <button
                onClick={() => setQuery("")}
                aria-label="Clear search"
                className="absolute right-2 top-1/2 -translate-y-1/2 p-1 text-ink-400
                           hover:text-ink-700 transition-colors"
              >
                <X size={14} />
              </button>
            )}
          </div>
        </div>
      </header>

      <main className="flex-1 max-w-6xl w-full mx-auto px-4 py-8">
        {/* Hero — hidden while searching, the results are the subject then */}
        {!query && (
          <div className="mb-10 text-center">
            <h2 className="text-4xl font-bold text-ink-900 mb-3">
              Welcome to {tenant.name}
            </h2>
            <p className="text-ink-500 max-w-xl mx-auto">
              Browse our collection below
            </p>
          </div>
        )}

        {/* Product grid — skeleton while loading */}
        {loading ? (
          <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
            {[...Array(8)].map((_, i) => (
              <div key={i} className="animate-pulse">
                <div className="aspect-square bg-ink-100 rounded-xl mb-3" />
                <div className="h-4 bg-ink-100 rounded w-3/4 mb-2" />
                <div className="h-4 bg-ink-100 rounded w-1/4" />
              </div>
            ))}
          </div>
        ) : visible.length === 0 ? (
          <div className="text-center py-20">
            <p className="text-ink-400 text-lg">
              {query
                ? `No products match “${query}”`
                : "No products available yet"}
            </p>
            {query && (
              <button
                onClick={() => setQuery("")}
                className="mt-4 text-sm font-medium text-brand-700 hover:underline"
              >
                Clear search
              </button>
            )}
          </div>
        ) : (
          <>
            {query && (
              <p className="text-sm text-ink-500 mb-4">
                {visible.length}{" "}
                {visible.length === 1 ? "product" : "products"} matching “
                {query}”
              </p>
            )}
            <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
              {visible.map((product: any) => {
                const primaryImage = product.images.find(
                  (img: any) => img.position === 0,
                );
                const inStock = product.variants.some((v: any) => v.isInStock);
                const hasDiscount =
                  product.comparePrice &&
                  product.comparePrice > product.basePrice;
                const discountPercent = hasDiscount
                  ? Math.round(
                      ((product.comparePrice - product.basePrice) /
                        product.comparePrice) *
                        100,
                    )
                  : 0;

                return (
                  <Link
                    key={product.id}
                    to={`/store/${subdomain}/products/${product.slug}`}
                    className="group"
                  >
                    {/* Image */}
                    <div className="aspect-square bg-ink-50 rounded-xl overflow-hidden relative mb-3">
                      {primaryImage ? (
                        <img
                          src={primaryImage.url}
                          alt={product.name}
                          className="w-full h-full object-cover group-hover:scale-105
                                     transition-transform duration-300"
                          loading="lazy"
                        />
                      ) : (
                        <div className="w-full h-full bg-gradient-to-br from-ink-100 to-ink-200" />
                      )}

                      {/* Discount badge */}
                      {hasDiscount && (
                        <div
                          className="absolute top-2 left-2 bg-accent-500 text-white
                                        text-xs font-bold px-2 py-0.5 rounded-full"
                        >
                          -{discountPercent}%
                        </div>
                      )}

                      {/* Out of stock overlay */}
                      {!inStock && (
                        <div className="absolute inset-0 bg-ink-900/40 flex items-center justify-center rounded-xl">
                          <span className="bg-white text-ink-900 text-xs font-medium px-3 py-1 rounded-full">
                            Out of stock
                          </span>
                        </div>
                      )}
                    </div>

                    {/* Info */}
                    <div>
                      <h3
                        className="text-sm font-medium text-ink-900 group-hover:text-brand-700
                                     transition-colors line-clamp-2"
                      >
                        {product.name}
                      </h3>
                      <div className="flex items-center gap-2 mt-1">
                        <span className="text-sm font-bold text-ink-900">
                          ${product.basePrice.toFixed(2)}
                        </span>
                        {hasDiscount && (
                          <span className="text-xs text-ink-400 line-through">
                            ${product.comparePrice.toFixed(2)}
                          </span>
                        )}
                      </div>
                    </div>
                  </Link>
                );
              })}
            </div>
          </>
        )}

        {/* Powered-by — the storefront is the tenant's brand, so Ownstall
            stays a quiet footnote rather than a second header. */}
        <div className="mt-16 pt-8 border-t border-ink-100 text-center">
          <Link
            to="/"
            className="inline-flex items-center gap-1.5 text-xs text-ink-400
                       hover:text-brand-700 transition-colors"
          >
            <LogoMark size={16} />
            Powered by Ownstall
          </Link>
        </div>
      </main>

      <SiteFooter />
    </div>
  );
}
