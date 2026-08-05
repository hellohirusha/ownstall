import { gql } from "@apollo/client";
import { useQuery } from "@apollo/client/react";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
  Cell,
} from "recharts";
import { MessageSquare, Clock, CheckCircle, TrendingDown } from "lucide-react";
import { AdminNav } from "../../components/AdminNav";

const GET_SUPPORT_METRICS = gql`
  query GetSupportMetrics {
    supportMetrics {
      openTickets
      avgFirstResponseHours
      avgResolutionHours
      slaBreachRate
      ticketsByDay {
        date
        count
      }
      ticketsByStatus {
        status
        count
      }
      ticketsByPriority {
        priority
        count
      }
    }
  }
`;

export function ReplyMetricsPage() {
  const { data, loading } = useQuery<{ supportMetrics: any }>(GET_SUPPORT_METRICS);
  const metrics = data?.supportMetrics;

  const avgFirstResponse = metrics?.avgFirstResponseHours ?? 0;
  const breachRate = metrics?.slaBreachRate ?? 0;

  const STAT_CARDS = [
    {
      label: "Open tickets",
      value: metrics?.openTickets ?? 0,
      icon: MessageSquare,
      color: "text-blue-600",
      bg: "bg-blue-50",
    },
    {
      label: "Avg first response",
      value: `${avgFirstResponse.toFixed(1)}h`,
      icon: Clock,
      color: avgFirstResponse < 1 ? "text-green-600" : "text-orange-600",
      bg: avgFirstResponse < 1 ? "bg-green-50" : "bg-orange-50",
    },
    {
      label: "Avg resolution",
      value: `${(metrics?.avgResolutionHours ?? 0).toFixed(1)}h`,
      icon: CheckCircle,
      color: "text-green-600",
      bg: "bg-green-50",
    },
    {
      label: "SLA breach rate",
      value: `${(breachRate * 100).toFixed(1)}%`,
      icon: TrendingDown,
      color: breachRate > 0.1 ? "text-red-600" : "text-green-600",
      bg: breachRate > 0.1 ? "bg-red-50" : "bg-green-50",
    },
  ];

  return (
    <div className="min-h-screen bg-gray-50">
      <AdminNav />
      {loading ? (
        <div className="max-w-6xl mx-auto p-6 grid grid-cols-4 gap-4">
          {[...Array(4)].map((_, i) => (
            <div key={i} className="h-28 bg-gray-100 rounded-xl animate-pulse" />
          ))}
        </div>
      ) : (
        <div className="max-w-6xl mx-auto p-6">
          <h1 className="text-2xl font-bold text-gray-900 mb-6">Support Metrics</h1>

          {/* KPI cards */}
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
            {STAT_CARDS.map(({ label, value, icon: Icon, color, bg }) => (
              <div key={label} className="bg-white rounded-xl border border-gray-100 p-4">
                <div
                  className={`w-10 h-10 ${bg} rounded-lg flex items-center justify-center mb-3`}
                >
                  <Icon size={18} className={color} />
                </div>
                <p className="text-2xl font-bold text-gray-900">{value}</p>
                <p className="text-sm text-gray-500 mt-0.5">{label}</p>
              </div>
            ))}
          </div>

          {/* Tickets by day chart */}
          <div className="bg-white rounded-xl border border-gray-100 p-5 mb-6">
            <h2 className="font-semibold text-gray-900 mb-4">Tickets this week</h2>
            <ResponsiveContainer width="100%" height={200}>
              <BarChart data={metrics?.ticketsByDay ?? []}>
                <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
                <XAxis dataKey="date" tick={{ fontSize: 12 }} />
                <YAxis tick={{ fontSize: 12 }} allowDecimals={false} />
                <Tooltip />
                <Bar dataKey="count" fill="#22c55e" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>

          {/* Status + priority breakdown */}
          <div className="grid grid-cols-2 gap-4">
            {[
              { title: "By status", data: metrics?.ticketsByStatus ?? [], key: "status" },
              {
                title: "By priority",
                data: metrics?.ticketsByPriority ?? [],
                key: "priority",
              },
            ].map(({ title, data: chartData, key }) => (
              <div key={title} className="bg-white rounded-xl border border-gray-100 p-5">
                <h2 className="font-semibold text-gray-900 mb-4">{title}</h2>
                <ResponsiveContainer width="100%" height={150}>
                  <BarChart data={chartData} layout="vertical">
                    <XAxis type="number" tick={{ fontSize: 11 }} allowDecimals={false} />
                    <YAxis dataKey={key} type="category" tick={{ fontSize: 11 }} width={70} />
                    <Tooltip />
                    <Bar dataKey="count" radius={[0, 4, 4, 0]}>
                      {chartData.map((_: any, i: number) => (
                        <Cell
                          key={i}
                          fill={["#22c55e", "#3b82f6", "#f59e0b", "#ef4444"][i % 4]}
                        />
                      ))}
                    </Bar>
                  </BarChart>
                </ResponsiveContainer>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
