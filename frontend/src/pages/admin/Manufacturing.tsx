import { gql } from "@apollo/client";
import { useMutation, useQuery } from "@apollo/client/react";
import { Package, Truck, Clock, Zap, ChevronRight } from "lucide-react";
import toast from "react-hot-toast";
import { AdminNav } from "../../components/AdminNav";

const GET_PRODUCTION = gql`
  query GetProduction {
    productionQueue {
      id
      status
      priority
      estimatedCompletionAt
      queuedAt
      shippedAt
      machineId
      order {
        id
        total
        customerEmail
        items {
          productName
          quantity
        }
      }
    }
    productionStats {
      totalInProduction
      totalShipped
      avgTimeHours
      todayQueued
      todayShipped
    }
  }
`;

const ADVANCE_STATUS = gql`
  mutation AdvanceProductionStatus($queueId: UUID!, $status: String!) {
    advanceProductionStatus(queueId: $queueId, status: $status) {
      id
      status
    }
  }
`;

const SIMULATE_PRODUCTION = gql`
  mutation SimulateProduction {
    simulateProduction {
      success
      message
    }
  }
`;

const STAGE_CONFIG: Record<string, { label: string; color: string; next: string }> = {
  queued: { label: "Queued", color: "bg-gray-100 text-gray-600", next: "art_review" },
  art_review: { label: "Art Review", color: "bg-yellow-100 text-yellow-700", next: "printing" },
  printing: { label: "Printing", color: "bg-blue-100 text-blue-700", next: "cutting" },
  cutting: { label: "Cutting", color: "bg-purple-100 text-purple-700", next: "quality_check" },
  quality_check: { label: "QA Check", color: "bg-orange-100 text-orange-700", next: "packaging" },
  packaging: { label: "Packaging", color: "bg-pink-100 text-pink-700", next: "shipped" },
  shipped: { label: "Shipped", color: "bg-green-100 text-green-700", next: "" },
  on_hold: { label: "On Hold", color: "bg-red-100 text-red-700", next: "" },
};

const ACTIVE_STAGES = Object.keys(STAGE_CONFIG).filter((s) => s !== "on_hold");

export function ManufacturingPage() {
  const { data, loading, refetch } = useQuery<{
    productionQueue: any[];
    productionStats: any;
  }>(GET_PRODUCTION, {
    pollInterval: 5000, // Poll every 5 seconds for live production updates
  });

  const [advanceStatus] = useMutation(ADVANCE_STATUS, {
    onCompleted: () => {
      toast.success("Status updated");
      refetch();
    },
    onError: (e) => toast.error(e.message),
  });

  const [simulate, { loading: simulating }] = useMutation<{
    simulateProduction: { success: boolean; message: string };
  }>(SIMULATE_PRODUCTION, {
    onCompleted: (d) => {
      toast.success(d.simulateProduction.message);
      refetch();
    },
    onError: (e) => toast.error(e.message),
  });

  const queue = data?.productionQueue ?? [];
  const stats = data?.productionStats;

  // Group by status for kanban view
  const grouped = queue.reduce((acc: Record<string, any[]>, item: any) => {
    if (!acc[item.status]) acc[item.status] = [];
    acc[item.status].push(item);
    return acc;
  }, {} as Record<string, any[]>);

  const avgTime = stats?.avgTimeHours ?? 0;

  return (
    <div className="min-h-screen bg-gray-50">
      <AdminNav />
      <div className="p-6 max-w-6xl mx-auto">
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-2xl font-bold text-gray-900">Manufacturing</h1>
            <p className="text-sm text-gray-500 mt-1">Live production pipeline</p>
          </div>
          <button
            onClick={() => simulate()}
            disabled={simulating}
            className="flex items-center gap-2 px-4 py-2 bg-purple-500 hover:bg-purple-600
                       text-white text-sm font-medium rounded-lg transition-colors
                       disabled:opacity-50"
          >
            <Zap size={14} />
            {simulating ? "Starting..." : "Simulate production"}
          </button>
        </div>

        {/* Stats */}
        <div className="grid grid-cols-2 lg:grid-cols-5 gap-3 mb-6">
          {[
            { label: "In production", value: stats?.totalInProduction ?? 0, icon: Package },
            { label: "Shipped today", value: stats?.todayShipped ?? 0, icon: Truck },
            { label: "Queued today", value: stats?.todayQueued ?? 0, icon: Clock },
            { label: "Total shipped", value: stats?.totalShipped ?? 0, icon: Truck },
            { label: "Avg time", value: avgTime > 0 ? `${avgTime.toFixed(1)}h` : "—", icon: Clock },
          ].map(({ label, value, icon: Icon }) => (
            <div key={label} className="bg-white rounded-xl border border-gray-100 p-4">
              <Icon size={16} className="text-gray-400 mb-2" />
              <p className="text-xl font-bold text-gray-900">{value}</p>
              <p className="text-xs text-gray-500">{label}</p>
            </div>
          ))}
        </div>

        {/* Pipeline kanban board */}
        {loading && queue.length === 0 ? (
          <div className="flex gap-3">
            {[...Array(4)].map((_, i) => (
              <div key={i} className="w-60 h-32 bg-gray-100 rounded-xl animate-pulse" />
            ))}
          </div>
        ) : (
          <div className="overflow-x-auto">
            <div className="flex gap-3 min-w-max pb-4">
              {ACTIVE_STAGES.map((stage) => {
                const config = STAGE_CONFIG[stage];
                const items = grouped[stage] ?? [];

                return (
                  <div key={stage} className="w-60 flex-shrink-0">
                    {/* Column header */}
                    <div
                      className={`px-3 py-2 rounded-t-xl flex items-center justify-between
                                 ${config.color}`}
                    >
                      <span className="text-xs font-semibold">{config.label}</span>
                      <span className="text-xs bg-white bg-opacity-50 px-1.5 rounded-full">
                        {items.length}
                      </span>
                    </div>

                    {/* Cards */}
                    <div className="bg-gray-50 rounded-b-xl p-2 space-y-2 min-h-24">
                      {items.map((item: any) => (
                        <div
                          key={item.id}
                          className="bg-white rounded-lg border border-gray-100 p-3 text-xs"
                        >
                          <div className="flex items-center justify-between mb-1">
                            <span className="font-mono text-gray-400">
                              #{item.order?.id?.slice(-6)?.toUpperCase() ?? "——"}
                            </span>
                            {item.machineId && (
                              <span className="text-purple-500">{item.machineId}</span>
                            )}
                          </div>
                          <p className="text-gray-700 font-medium truncate">
                            {item.order?.customerEmail ?? "Unknown customer"}
                          </p>
                          <p className="text-gray-400 truncate">
                            {item.order?.items?.[0]?.productName ?? "No items"}
                            {item.order?.items?.length > 1
                              ? ` +${item.order.items.length - 1} more`
                              : ""}
                          </p>

                          {/* Advance button */}
                          {config.next && (
                            <button
                              onClick={() =>
                                advanceStatus({
                                  variables: { queueId: item.id, status: config.next },
                                })
                              }
                              className="mt-2 w-full flex items-center justify-center gap-1
                                         py-1 bg-gray-100 hover:bg-gray-200 rounded text-gray-600
                                         transition-colors"
                            >
                              <ChevronRight size={10} />
                              Move to {STAGE_CONFIG[config.next]?.label}
                            </button>
                          )}
                        </div>
                      ))}
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
