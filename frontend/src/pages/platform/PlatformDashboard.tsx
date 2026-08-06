import { useState } from "react";
import { gql } from "@apollo/client";
import { useMutation, useQuery } from "@apollo/client/react";
import { useNavigate } from "react-router-dom";
import {
  Ban,
  Check,
  DollarSign,
  ExternalLink,
  Loader2,
  LogOut,
  Package,
  RotateCcw,
  Search,
  ShieldAlert,
  ShoppingBag,
  Store,
  Users,
  X,
} from "lucide-react";
import toast from "react-hot-toast";
import { LogoMark } from "../../components/Logo";
import { clearSession, getSessionUser } from "../../lib/session";
import { apolloClient } from "../../lib/apollo";

const PLATFORM_STATS = gql`
  query PlatformStats {
    platformStats {
      totalStores
      pendingStores
      approvedStores
      suspendedStores
      rejectedStores
      totalProducts
      totalOrders
      paidOrders
      grossRevenue
      totalBuyers
    }
  }
`;

const ADMIN_STORES = gql`
  query AdminStores($status: String, $search: String) {
    adminStores(status: $status, search: $search) {
      id
      name
      subdomain
      tagline
      category
      location
      status
      productCount
      canPublishProducts
      canAcceptOrders
      createdAt
      submittedAt
      reviewNote
    }
  }
`;

const APPROVE = gql`
  mutation ApproveStore($tenantId: UUID!, $note: String) {
    approveStore(tenantId: $tenantId, note: $note) {
      id
      status
    }
  }
`;

const REJECT = gql`
  mutation RejectStore($tenantId: UUID!, $reason: String!) {
    rejectStore(tenantId: $tenantId, reason: $reason) {
      id
      status
    }
  }
`;

const SUSPEND = gql`
  mutation SuspendStore($tenantId: UUID!, $reason: String!) {
    suspendStore(tenantId: $tenantId, reason: $reason) {
      id
      status
    }
  }
`;

const RESTORE = gql`
  mutation RestoreStore($tenantId: UUID!) {
    restoreStore(tenantId: $tenantId) {
      id
      status
    }
  }
`;

const SET_RESTRICTIONS = gql`
  mutation SetStoreRestrictions(
    $tenantId: UUID!
    $canPublishProducts: Boolean!
    $canAcceptOrders: Boolean!
    $reason: String
  ) {
    setStoreRestrictions(
      tenantId: $tenantId
      canPublishProducts: $canPublishProducts
      canAcceptOrders: $canAcceptOrders
      reason: $reason
    ) {
      id
      canPublishProducts
      canAcceptOrders
    }
  }
`;

interface AdminStore {
  id: string;
  name: string;
  subdomain: string;
  tagline: string | null;
  category: string | null;
  location: string | null;
  status: string;
  productCount: number;
  canPublishProducts: boolean;
  canAcceptOrders: boolean;
  createdAt: string;
  submittedAt: string | null;
  reviewNote: string | null;
}

const TABS = [
  { value: "pending", label: "Pending review" },
  { value: "approved", label: "Approved" },
  { value: "suspended", label: "Suspended" },
  { value: "rejected", label: "Rejected" },
  { value: "", label: "All" },
];

const STATUS_STYLE: Record<string, string> = {
  pending: "bg-amber-100 text-amber-800",
  approved: "bg-green-100 text-green-700",
  suspended: "bg-red-100 text-red-700",
  rejected: "bg-ink-200 text-ink-600",
};

export function PlatformDashboard() {
  const navigate = useNavigate();
  const admin = getSessionUser("admin");

  const [tab, setTab] = useState("pending");
  const [search, setSearch] = useState("");
  const [draft, setDraft] = useState("");

  const { data: statsData } = useQuery<{ platformStats: any }>(PLATFORM_STATS);
  const { data, loading, refetch } = useQuery<{ adminStores: AdminStore[] }>(
    ADMIN_STORES,
    { variables: { status: tab || null, search: search || null } },
  );

  const refresh = { refetchQueries: ["AdminStores", "PlatformStats"] };
  const [approve, { loading: approving }] = useMutation(APPROVE, refresh);
  const [reject, { loading: rejecting }] = useMutation(REJECT, refresh);
  const [suspend, { loading: suspending }] = useMutation(SUSPEND, refresh);
  const [restore, { loading: restoring }] = useMutation(RESTORE, refresh);
  const [setRestrictions] = useMutation(SET_RESTRICTIONS, refresh);

  const busy = approving || rejecting || suspending || restoring;
  const stats = statsData?.platformStats;
  const stores = data?.adminStores ?? [];

  const handleSignOut = () => {
    clearSession("admin");
    void apolloClient.clearStore();
    navigate("/platform/login", { replace: true });
  };

  // Rejecting and suspending both need a reason on the record — the audit
  // trail is worthless if half the entries say nothing.
  const withReason = async (
    action: (reason: string) => Promise<unknown>,
    prompt_: string,
  ) => {
    const reason = window.prompt(prompt_);
    if (reason === null) return;
    if (!reason.trim()) {
      toast.error("A reason is required");
      return;
    }
    try {
      await action(reason.trim());
      toast.success("Done");
    } catch (e: any) {
      toast.error(e?.message ?? "Action failed");
    }
  };

  const statCards = stats
    ? [
        { label: "Stalls", value: stats.totalStores, icon: Store },
        { label: "Awaiting review", value: stats.pendingStores, icon: ShieldAlert },
        { label: "Products", value: stats.totalProducts, icon: Package },
        { label: "Orders", value: stats.totalOrders, icon: ShoppingBag },
        { label: "Buyers", value: stats.totalBuyers, icon: Users },
        {
          label: "Gross revenue",
          value: `$${Number(stats.grossRevenue).toFixed(2)}`,
          icon: DollarSign,
        },
      ]
    : [];

  return (
    <div className="min-h-screen bg-ink-50">
      {/* Console header — deliberately dark so an operator never confuses
          this surface with a seller dashboard. */}
      <header className="bg-ink-900 text-white">
        <div className="max-w-6xl mx-auto px-4 h-16 flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <LogoMark size={28} />
            <div>
              <p className="font-bold leading-tight">Ownstall</p>
              <p className="text-[11px] text-ink-400 leading-tight">
                Operator console
              </p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            {admin?.email && (
              <span className="hidden sm:block text-sm text-ink-400">
                {admin.email}
              </span>
            )}
            <button
              onClick={handleSignOut}
              className="inline-flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm
                         font-medium text-ink-300 hover:text-white hover:bg-white/10
                         transition-colors"
            >
              <LogOut size={16} />
              Sign out
            </button>
          </div>
        </div>
      </header>

      <main className="max-w-6xl mx-auto px-4 py-8">
        {/* Stats */}
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-3 mb-8">
          {statCards.map((card) => {
            const Icon = card.icon;
            return (
              <div
                key={card.label}
                className="bg-white rounded-2xl border border-ink-200 p-4 shadow-card"
              >
                <Icon size={16} className="text-brand-600 mb-2" />
                <p className="text-xl font-bold text-ink-900">{card.value}</p>
                <p className="text-xs text-ink-500">{card.label}</p>
              </div>
            );
          })}
        </div>

        {/* Filters */}
        <div className="flex flex-wrap items-center gap-2 mb-4">
          {TABS.map((t) => (
            <button
              key={t.value || "all"}
              onClick={() => setTab(t.value)}
              className={`px-3.5 py-2 rounded-lg text-sm font-medium transition-colors ${
                tab === t.value
                  ? "bg-ink-900 text-white"
                  : "bg-white border border-ink-200 text-ink-600 hover:border-ink-400"
              }`}
            >
              {t.label}
              {t.value === "pending" && stats?.pendingStores > 0 && (
                <span className="ml-1.5 px-1.5 py-0.5 bg-amber-400 text-ink-900 rounded-full text-[11px] font-bold">
                  {stats.pendingStores}
                </span>
              )}
            </button>
          ))}

          <form
            onSubmit={(e) => {
              e.preventDefault();
              setSearch(draft.trim());
            }}
            className="relative ml-auto"
          >
            <Search
              size={15}
              className="absolute left-3 top-1/2 -translate-y-1/2 text-ink-400 pointer-events-none"
            />
            <input
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              placeholder="Search stalls"
              aria-label="Search stalls"
              className="pl-9 pr-3 py-2 border border-ink-200 rounded-lg text-sm bg-white
                         focus:outline-none focus:ring-2 focus:ring-brand-500"
            />
          </form>
        </div>

        {/* Stalls */}
        {loading ? (
          <div className="space-y-3">
            {[...Array(4)].map((_, i) => (
              <div
                key={i}
                className="animate-pulse bg-white rounded-2xl border border-ink-200 p-5"
              >
                <div className="h-4 bg-ink-100 rounded w-1/4 mb-3" />
                <div className="h-3 bg-ink-100 rounded w-1/2" />
              </div>
            ))}
          </div>
        ) : stores.length === 0 ? (
          <div className="text-center py-16 bg-white border border-dashed border-ink-200 rounded-2xl">
            <Check className="mx-auto h-10 w-10 text-ink-200 mb-3" />
            <p className="text-ink-500">
              {tab === "pending"
                ? "Nothing waiting for review."
                : "No stalls here."}
            </p>
          </div>
        ) : (
          <div className="space-y-3">
            {stores.map((store) => (
              <div
                key={store.id}
                className="bg-white rounded-2xl border border-ink-200 p-5 shadow-card"
              >
                <div className="flex flex-wrap items-start justify-between gap-4">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2 flex-wrap mb-1">
                      <h2 className="font-semibold text-ink-900">
                        {store.name}
                      </h2>
                      <span
                        className={`px-2 py-0.5 rounded-full text-[11px] font-semibold ${
                          STATUS_STYLE[store.status] ?? "bg-ink-100 text-ink-600"
                        }`}
                      >
                        {store.status}
                      </span>
                      {!store.canPublishProducts && (
                        <span className="px-2 py-0.5 rounded-full text-[11px] font-semibold bg-amber-100 text-amber-800">
                          publishing blocked
                        </span>
                      )}
                      {!store.canAcceptOrders && (
                        <span className="px-2 py-0.5 rounded-full text-[11px] font-semibold bg-amber-100 text-amber-800">
                          orders blocked
                        </span>
                      )}
                    </div>

                    <p className="text-xs text-ink-400 mb-2">
                      /{store.subdomain} · {store.productCount} products
                      {store.category && ` · ${store.category}`}
                      {store.location && ` · ${store.location}`}
                    </p>

                    {store.tagline && (
                      <p className="text-sm text-ink-500">{store.tagline}</p>
                    )}
                    {store.reviewNote && (
                      <p className="text-xs text-ink-400 mt-1.5 italic">
                        Note: {store.reviewNote}
                      </p>
                    )}
                  </div>

                  <div className="flex flex-wrap items-center gap-2">
                    <a
                      href={`/store?store=${store.subdomain}`}
                      target="_blank"
                      rel="noreferrer"
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 border border-ink-200
                                 rounded-lg text-xs font-medium text-ink-600 hover:border-ink-400
                                 transition-colors"
                    >
                      <ExternalLink size={13} />
                      View
                    </a>

                    {store.status === "pending" && (
                      <>
                        <button
                          disabled={busy}
                          onClick={async () => {
                            await approve({ variables: { tenantId: store.id } });
                            toast.success(`${store.name} approved`);
                          }}
                          className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-brand-600
                                     hover:bg-brand-700 text-white rounded-lg text-xs font-semibold
                                     transition-colors disabled:opacity-50"
                        >
                          {busy ? (
                            <Loader2 size={13} className="animate-spin" />
                          ) : (
                            <Check size={13} />
                          )}
                          Approve
                        </button>
                        <button
                          disabled={busy}
                          onClick={() =>
                            withReason(
                              (reason) =>
                                reject({
                                  variables: { tenantId: store.id, reason },
                                }),
                              `Why is "${store.name}" being rejected?`,
                            )
                          }
                          className="inline-flex items-center gap-1.5 px-3 py-1.5 border border-ink-200
                                     rounded-lg text-xs font-medium text-ink-600 hover:border-red-400
                                     hover:text-red-600 transition-colors disabled:opacity-50"
                        >
                          <X size={13} />
                          Reject
                        </button>
                      </>
                    )}

                    {store.status === "approved" && (
                      <>
                        <button
                          disabled={busy}
                          onClick={() =>
                            setRestrictions({
                              variables: {
                                tenantId: store.id,
                                canPublishProducts: !store.canPublishProducts,
                                canAcceptOrders: store.canAcceptOrders,
                              },
                            })
                          }
                          className="px-3 py-1.5 border border-ink-200 rounded-lg text-xs
                                     font-medium text-ink-600 hover:border-ink-400 transition-colors"
                        >
                          {store.canPublishProducts ? "Block" : "Allow"}{" "}
                          publishing
                        </button>
                        <button
                          disabled={busy}
                          onClick={() =>
                            setRestrictions({
                              variables: {
                                tenantId: store.id,
                                canPublishProducts: store.canPublishProducts,
                                canAcceptOrders: !store.canAcceptOrders,
                              },
                            })
                          }
                          className="px-3 py-1.5 border border-ink-200 rounded-lg text-xs
                                     font-medium text-ink-600 hover:border-ink-400 transition-colors"
                        >
                          {store.canAcceptOrders ? "Block" : "Allow"} orders
                        </button>
                        <button
                          disabled={busy}
                          onClick={() =>
                            withReason(
                              (reason) =>
                                suspend({
                                  variables: { tenantId: store.id, reason },
                                }),
                              `Why is "${store.name}" being suspended?`,
                            )
                          }
                          className="inline-flex items-center gap-1.5 px-3 py-1.5 border border-red-200
                                     rounded-lg text-xs font-semibold text-red-600 hover:bg-red-50
                                     transition-colors disabled:opacity-50"
                        >
                          <Ban size={13} />
                          Suspend
                        </button>
                      </>
                    )}

                    {(store.status === "suspended" ||
                      store.status === "rejected") && (
                      <button
                        disabled={busy}
                        onClick={async () => {
                          await restore({ variables: { tenantId: store.id } });
                          toast.success(`${store.name} restored`);
                        }}
                        className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-brand-600
                                   hover:bg-brand-700 text-white rounded-lg text-xs font-semibold
                                   transition-colors disabled:opacity-50"
                      >
                        <RotateCcw size={13} />
                        Restore
                      </button>
                    )}
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}

        <button
          onClick={() => refetch()}
          className="mt-6 text-sm text-ink-500 hover:text-ink-900 transition-colors"
        >
          Refresh
        </button>
      </main>
    </div>
  );
}
