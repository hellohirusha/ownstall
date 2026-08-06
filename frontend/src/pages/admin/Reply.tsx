import { useState } from "react";
import type { ReactNode } from "react";
import { gql } from "@apollo/client";
import { useQuery } from "@apollo/client/react";
import { Link, useNavigate } from "react-router-dom";
import { BarChart2, MessageSquare, Search, AlertTriangle, Clock, CheckCircle } from "lucide-react";
import { AdminNav } from "../../components/AdminNav";

const GET_TICKETS = gql`
  query GetTickets($status: String, $priority: String, $search: String) {
    tickets(status: $status, priority: $priority, search: $search) {
      id
      number
      subject
      status
      priority
      customerEmail
      customerName
      assigneeName
      latestMessage
      unreadCount
      slaStatus
      createdAt
      updatedAt
    }
  }
`;

const STATUS_TABS = [
  { value: "", label: "All" },
  { value: "open", label: "Open" },
  { value: "pending", label: "Pending" },
  { value: "resolved", label: "Resolved" },
];

const PRIORITY_COLORS: Record<string, string> = {
  urgent: "bg-red-100 text-red-700 border-red-200",
  high: "bg-orange-100 text-orange-700 border-orange-200",
  normal: "bg-gray-100 text-gray-600 border-gray-200",
  low: "bg-blue-50 text-blue-600 border-blue-100",
};

const SLA_ICONS: Record<string, ReactNode> = {
  breached: <AlertTriangle size={12} className="text-red-500" />,
  at_risk: <Clock size={12} className="text-orange-500" />,
  ok: <CheckCircle size={12} className="text-brand-600" />,
};

export function ReplyPage() {
  const navigate = useNavigate();
  const [activeStatus, setActiveStatus] = useState("");
  const [search, setSearch] = useState("");
  const [searchInput, setSearchInput] = useState("");

  const { data, loading } = useQuery<{ tickets: any[] }>(GET_TICKETS, {
    variables: { status: activeStatus, search },
    pollInterval: 15000, // Auto-refresh every 15s
  });

  const tickets = data?.tickets ?? [];
  const openCount = tickets.filter((t: any) => t.status === "open").length;

  const formatTimeAgo = (dateStr: string) => {
    const diff = Date.now() - new Date(dateStr).getTime();
    const minutes = Math.floor(diff / 60000);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);
    if (days > 0) return `${days}d ago`;
    if (hours > 0) return `${hours}h ago`;
    return `${minutes}m ago`;
  };

  return (
    <div className="h-screen flex flex-col">
      <AdminNav />
      <div className="flex-1 flex overflow-hidden">
        {/* Sidebar */}
        <div className="w-72 border-r border-gray-100 bg-white flex flex-col">
          {/* Header */}
          <div className="p-4 border-b border-gray-100">
            <div className="flex items-center justify-between mb-3">
              <h1 className="font-bold text-gray-900">Reply</h1>
              <div className="flex items-center gap-2">
                {openCount > 0 && (
                  <span
                    className="text-xs bg-brand-600 text-white font-medium
                               px-2 py-0.5 rounded-full"
                  >
                    {openCount} open
                  </span>
                )}
                <Link
                  to="/admin/reply/metrics"
                  title="Support metrics"
                  className="text-gray-400 hover:text-gray-600 transition-colors"
                >
                  <BarChart2 size={16} />
                </Link>
              </div>
            </div>
            {/* Search */}
            <div className="relative">
              <Search
                size={14}
                className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400"
              />
              <input
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && setSearch(searchInput)}
                placeholder="Search tickets..."
                className="w-full pl-8 pr-3 py-2 bg-gray-50 border border-gray-100 rounded-lg
                           text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              />
            </div>
          </div>

          {/* Status filter tabs */}
          <div className="p-2">
            {STATUS_TABS.map((tab) => (
              <button
                key={tab.value}
                onClick={() => setActiveStatus(tab.value)}
                className={`w-full text-left px-3 py-2 rounded-lg text-sm transition-colors mb-0.5
                  ${
                    activeStatus === tab.value
                      ? "bg-green-50 text-green-700 font-medium"
                      : "text-gray-600 hover:bg-gray-50"
                  }`}
              >
                {tab.label}
              </button>
            ))}
          </div>

          {/* Ticket list */}
          <div className="flex-1 overflow-y-auto">
            {loading ? (
              <div className="p-4 space-y-3">
                {[...Array(5)].map((_, i) => (
                  <div key={i} className="h-16 bg-gray-50 rounded-lg animate-pulse" />
                ))}
              </div>
            ) : tickets.length === 0 ? (
              <div className="text-center py-12 px-4">
                <MessageSquare className="mx-auto h-10 w-10 text-gray-200 mb-2" />
                <p className="text-sm text-gray-400">No tickets</p>
              </div>
            ) : (
              tickets.map((ticket: any) => (
                <button
                  key={ticket.id}
                  onClick={() => navigate(`/admin/reply/${ticket.id}`)}
                  className="w-full text-left p-4 border-b border-gray-50
                             hover:bg-gray-50 transition-colors"
                >
                  <div className="flex items-start justify-between gap-2 mb-1">
                    <div className="flex items-center gap-1.5">
                      {/* SLA indicator */}
                      {SLA_ICONS[ticket.slaStatus]}
                      <span className="text-xs font-mono text-gray-400">
                        #{ticket.number}
                      </span>
                    </div>
                    <div className="flex items-center gap-1.5">
                      {ticket.unreadCount > 0 && (
                        <span className="w-2 h-2 bg-brand-600 rounded-full flex-shrink-0" />
                      )}
                      <span className="text-xs text-gray-400">
                        {formatTimeAgo(ticket.updatedAt)}
                      </span>
                    </div>
                  </div>

                  <p className="text-sm font-medium text-gray-900 truncate mb-1">
                    {ticket.subject}
                  </p>
                  <p className="text-xs text-gray-500 truncate">
                    {ticket.customerName || ticket.customerEmail}
                  </p>
                  {ticket.latestMessage && (
                    <p className="text-xs text-gray-400 truncate mt-1">
                      {ticket.latestMessage}
                    </p>
                  )}

                  <div className="flex items-center gap-1.5 mt-2">
                    <span
                      className={`text-xs px-1.5 py-0.5 rounded border font-medium
                                 ${PRIORITY_COLORS[ticket.priority]}`}
                    >
                      {ticket.priority}
                    </span>
                    {ticket.assigneeName && (
                      <span className="text-xs text-gray-400">
                        → {ticket.assigneeName}
                      </span>
                    )}
                  </div>
                </button>
              ))
            )}
          </div>
        </div>

        {/* Main area — empty state if no ticket selected */}
        <div className="flex-1 flex items-center justify-center bg-gray-50">
          <div className="text-center">
            <MessageSquare className="mx-auto h-16 w-16 text-gray-200 mb-3" />
            <p className="text-gray-400">Select a ticket to view</p>
          </div>
        </div>
      </div>
    </div>
  );
}
