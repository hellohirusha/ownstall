import { useEffect, useMemo, useState } from "react";
import { gql } from "@apollo/client";
import { useQuery } from "@apollo/client/react";
import { Link, useSearchParams } from "react-router-dom";
import { MapPin, Package, Search, SlidersHorizontal, Store, X } from "lucide-react";

const SEARCH_STORES = gql`
  query SearchStores($input: StoreSearchInput) {
    stores(input: $input) {
      total
      stores {
        id
        name
        subdomain
        tagline
        category
        location
        logoUrl
        productCount
      }
    }
  }
`;

const GET_CATEGORIES = gql`
  query StoreCategories {
    storeCategories {
      category
      count
    }
  }
`;

const SORTS = [
  { value: "newest", label: "Newest" },
  { value: "name", label: "Name A–Z" },
  { value: "products", label: "Most products" },
  { value: "oldest", label: "Longest running" },
];

const PAGE_SIZE = 24;

interface StoreRow {
  id: string;
  name: string;
  subdomain: string;
  tagline: string | null;
  category: string | null;
  location: string | null;
  logoUrl: string | null;
  productCount: number;
}

export function StoreFinderPage() {
  // The URL is the source of truth for the search, so a result page can be
  // shared, bookmarked and reached from the landing page's hero box.
  const [params, setParams] = useSearchParams();

  const query = params.get("q") ?? "";
  const category = params.get("category") ?? "";
  const sort = params.get("sort") ?? "newest";
  const page = Math.max(0, Number(params.get("page") ?? "0") || 0);

  const [draft, setDraft] = useState(query);
  const [showFilters, setShowFilters] = useState(false);

  // Keep the box in step when the URL changes underneath it (back button,
  // or arriving from the landing page with ?q= already set).
  useEffect(() => setDraft(query), [query]);

  const setParam = (key: string, value: string) => {
    const next = new URLSearchParams(params);
    if (value) next.set(key, value);
    else next.delete(key);
    // Any filter change invalidates the current page offset
    if (key !== "page") next.delete("page");
    setParams(next, { replace: false });
  };

  const input = useMemo(
    () => ({
      search: query || null,
      category: category || null,
      sort,
      limit: PAGE_SIZE,
      offset: page * PAGE_SIZE,
    }),
    [query, category, sort, page],
  );

  const { data, loading } = useQuery<{
    stores: { total: number; stores: StoreRow[] };
  }>(SEARCH_STORES, { variables: { input } });

  const { data: catData } = useQuery<{
    storeCategories: { category: string; count: number }[];
  }>(GET_CATEGORIES);

  const stores = data?.stores.stores ?? [];
  const total = data?.stores.total ?? 0;
  const categories = catData?.storeCategories ?? [];
  const lastPage = Math.max(0, Math.ceil(total / PAGE_SIZE) - 1);
  const hasFilters = Boolean(query || category);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setParam("q", draft.trim());
  };

  const clearAll = () => {
    setDraft("");
    setParams(new URLSearchParams());
  };

  return (
    <div className="max-w-6xl mx-auto px-4 py-10">
      <header className="mb-8">
        <h1 className="text-3xl font-bold text-ink-900 tracking-tight mb-2">
          Browse stalls
        </h1>
        <p className="text-ink-500">
          Every approved stall on Ownstall, in one place.
        </p>
      </header>

      {/* Search */}
      <form onSubmit={handleSubmit} className="flex gap-2 mb-4">
        <div className="relative flex-1">
          <Search
            size={18}
            className="absolute left-3.5 top-1/2 -translate-y-1/2 text-ink-400 pointer-events-none"
          />
          <input
            type="search"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            placeholder="Search by stall name, category or product"
            aria-label="Search stalls"
            className="w-full pl-11 pr-4 py-3 border border-ink-200 rounded-xl text-sm
                       focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
          />
        </div>
        <button
          type="submit"
          className="px-5 py-3 bg-brand-600 hover:bg-brand-700 text-white text-sm
                     font-semibold rounded-xl transition-colors"
        >
          Search
        </button>
        <button
          type="button"
          onClick={() => setShowFilters((v) => !v)}
          aria-expanded={showFilters}
          className="sm:hidden px-4 py-3 border border-ink-200 rounded-xl text-ink-600"
          aria-label="Toggle filters"
        >
          <SlidersHorizontal size={18} />
        </button>
      </form>

      {/* Filters */}
      <div
        className={`${showFilters ? "block" : "hidden"} sm:block mb-6 space-y-4`}
      >
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-xs font-semibold uppercase tracking-wider text-ink-400 mr-1">
            Category
          </span>
          <button
            onClick={() => setParam("category", "")}
            className={`px-3 py-1.5 rounded-full text-sm font-medium transition-colors ${
              !category
                ? "bg-brand-600 text-white"
                : "bg-ink-100 text-ink-600 hover:bg-ink-200"
            }`}
          >
            All
          </button>
          {categories.map((c) => (
            <button
              key={c.category}
              onClick={() => setParam("category", c.category)}
              className={`px-3 py-1.5 rounded-full text-sm font-medium transition-colors ${
                category === c.category
                  ? "bg-brand-600 text-white"
                  : "bg-ink-100 text-ink-600 hover:bg-ink-200"
              }`}
            >
              {c.category}
              <span className="ml-1.5 opacity-60">{c.count}</span>
            </button>
          ))}
        </div>

        <div className="flex flex-wrap items-center gap-3">
          <label
            htmlFor="sort"
            className="text-xs font-semibold uppercase tracking-wider text-ink-400"
          >
            Sort
          </label>
          <select
            id="sort"
            value={sort}
            onChange={(e) => setParam("sort", e.target.value)}
            className="px-3 py-2 border border-ink-200 rounded-lg text-sm bg-white
                       focus:outline-none focus:ring-2 focus:ring-brand-500"
          >
            {SORTS.map((s) => (
              <option key={s.value} value={s.value}>
                {s.label}
              </option>
            ))}
          </select>

          {hasFilters && (
            <button
              onClick={clearAll}
              className="inline-flex items-center gap-1 text-sm text-ink-500 hover:text-ink-900"
            >
              <X size={14} />
              Clear filters
            </button>
          )}
        </div>
      </div>

      {/* Result count */}
      {!loading && (
        <p className="text-sm text-ink-500 mb-5">
          {total === 0
            ? "No stalls found"
            : `${total} stall${total === 1 ? "" : "s"}${
                query ? ` matching “${query}”` : ""
              }`}
        </p>
      )}

      {/* Results */}
      {loading ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {[...Array(6)].map((_, i) => (
            <div
              key={i}
              className="animate-pulse rounded-2xl border border-ink-200 p-5"
            >
              <div className="w-12 h-12 bg-ink-100 rounded-xl mb-4" />
              <div className="h-4 bg-ink-100 rounded w-2/3 mb-2" />
              <div className="h-3 bg-ink-100 rounded w-full" />
            </div>
          ))}
        </div>
      ) : stores.length === 0 ? (
        <div className="text-center py-20 border border-dashed border-ink-200 rounded-2xl">
          <Store className="mx-auto h-12 w-12 text-ink-200 mb-4" />
          <p className="text-ink-500 mb-1">Nothing matched that search.</p>
          <p className="text-sm text-ink-400 mb-6">
            Try a broader term, or clear the filters.
          </p>
          {hasFilters && (
            <button
              onClick={clearAll}
              className="px-5 py-2.5 bg-brand-600 hover:bg-brand-700 text-white
                         text-sm font-semibold rounded-lg transition-colors"
            >
              Show all stalls
            </button>
          )}
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {stores.map((store) => (
            <Link
              key={store.id}
              to={`/store?store=${store.subdomain}`}
              className="group rounded-2xl border border-ink-200 bg-white p-5 shadow-card
                         hover:border-brand-400 hover:shadow-lift transition-all"
            >
              <div className="flex items-start gap-3 mb-3">
                {store.logoUrl ? (
                  <img
                    src={store.logoUrl}
                    alt=""
                    className="w-12 h-12 rounded-xl object-cover flex-shrink-0"
                    loading="lazy"
                  />
                ) : (
                  <div
                    className="w-12 h-12 rounded-xl bg-brand-50 text-brand-700
                               flex items-center justify-center flex-shrink-0"
                  >
                    <Store size={20} />
                  </div>
                )}
                <div className="min-w-0">
                  <h2 className="font-semibold text-ink-900 truncate group-hover:text-brand-700 transition-colors">
                    {store.name}
                  </h2>
                  <p className="text-xs text-ink-400 truncate">
                    /{store.subdomain}
                  </p>
                </div>
              </div>

              {store.tagline && (
                <p className="text-sm text-ink-500 line-clamp-2 mb-4">
                  {store.tagline}
                </p>
              )}

              <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-ink-400">
                <span className="inline-flex items-center gap-1">
                  <Package size={12} />
                  {store.productCount}{" "}
                  {store.productCount === 1 ? "product" : "products"}
                </span>
                {store.location && (
                  <span className="inline-flex items-center gap-1">
                    <MapPin size={12} />
                    {store.location}
                  </span>
                )}
                {store.category && (
                  <span className="px-2 py-0.5 bg-ink-100 text-ink-600 rounded-full font-medium">
                    {store.category}
                  </span>
                )}
              </div>
            </Link>
          ))}
        </div>
      )}

      {/* Paging */}
      {!loading && lastPage > 0 && (
        <div className="flex items-center justify-center gap-3 mt-10">
          <button
            disabled={page === 0}
            onClick={() => setParam("page", String(page - 1))}
            className="px-4 py-2 border border-ink-200 rounded-lg text-sm font-medium
                       text-ink-600 hover:border-ink-400 disabled:opacity-40
                       disabled:cursor-not-allowed transition-colors"
          >
            Previous
          </button>
          <span className="text-sm text-ink-500">
            Page {page + 1} of {lastPage + 1}
          </span>
          <button
            disabled={page >= lastPage}
            onClick={() => setParam("page", String(page + 1))}
            className="px-4 py-2 border border-ink-200 rounded-lg text-sm font-medium
                       text-ink-600 hover:border-ink-400 disabled:opacity-40
                       disabled:cursor-not-allowed transition-colors"
          >
            Next
          </button>
        </div>
      )}
    </div>
  );
}
