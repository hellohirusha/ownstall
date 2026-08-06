import { useState } from "react";
import { useParams } from "react-router-dom";
import { gql } from "@apollo/client";
import { useMutation, useQuery } from "@apollo/client/react";
import { User, Clock, Tag, Send, Lock, ChevronDown, CheckCircle, Zap } from "lucide-react";
import toast from "react-hot-toast";
import { AdminNav } from "../../components/AdminNav";

const GET_TICKET = gql`
  query GetTicket($id: UUID!) {
    ticket(id: $id) {
      id
      number
      subject
      status
      priority
      customerEmail
      customerName
      assigneeId
      assigneeName
      slaStatus
      slaFirstResponseAt
      firstResponseAt
      aiDraftBody
      aiDraftConfidence
      createdAt
      messages {
        id
        authorType
        authorName
        authorEmail
        body
        isInternal
        createdAt
      }
    }
    cannedResponses {
      id
      name
      shortcut
      body
    }
  }
`;

const REPLY_TICKET = gql`
  mutation ReplyTicket($ticketId: UUID!, $body: String!, $isInternal: Boolean!) {
    replyToTicket(ticketId: $ticketId, body: $body, isInternal: $isInternal) {
      id
      body
      authorType
      authorName
      createdAt
    }
  }
`;

const UPDATE_STATUS = gql`
  mutation UpdateTicketStatus($ticketId: UUID!, $status: String!) {
    updateTicketStatus(ticketId: $ticketId, status: $status) {
      id
      status
    }
  }
`;

const SLA_STATUS_CONFIG = {
  breached: { color: "text-red-600", label: "SLA breached", bg: "bg-red-50" },
  at_risk: { color: "text-orange-600", label: "SLA at risk", bg: "bg-orange-50" },
  ok: { color: "text-green-600", label: "Within SLA", bg: "bg-green-50" },
  met: { color: "text-gray-500", label: "SLA met", bg: "bg-gray-50" },
  none: { color: "text-gray-400", label: "", bg: "" },
};

export function TicketDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [replyBody, setReplyBody] = useState("");
  const [isInternal, setIsInternal] = useState(false);
  const [showCanned, setShowCanned] = useState(false);

  const { data, loading, refetch } = useQuery<{
    ticket: any;
    cannedResponses: any[];
  }>(GET_TICKET, {
    variables: { id },
    pollInterval: 10000,
  });

  const [replyTicket, { loading: replying }] = useMutation(REPLY_TICKET, {
    onCompleted: () => {
      setReplyBody("");
      refetch();
      toast.success(isInternal ? "Note added" : "Reply sent");
    },
    onError: (e) => toast.error(e.message),
    refetchQueries: ["GetTickets"],
  });

  const [updateStatus] = useMutation(UPDATE_STATUS, {
    onCompleted: () => refetch(),
    refetchQueries: ["GetTickets"],
  });

  const ticket = data?.ticket;
  const cannedResponses = data?.cannedResponses ?? [];

  const handleSubmit = () => {
    if (!replyBody.trim()) return;
    replyTicket({ variables: { ticketId: id, body: replyBody, isInternal } });
  };

  const handleUseAIDraft = () => {
    if (ticket?.aiDraftBody) {
      setReplyBody(ticket.aiDraftBody);
      setIsInternal(false);
    }
  };

  const slaConfig =
    SLA_STATUS_CONFIG[ticket?.slaStatus as keyof typeof SLA_STATUS_CONFIG] ??
    SLA_STATUS_CONFIG.none;

  return (
    <div className="h-screen flex flex-col">
      <AdminNav />
      {loading ? (
        <div className="flex-1 flex items-center justify-center">
          <div className="w-8 h-8 border-2 border-brand-600 border-t-transparent rounded-full animate-spin" />
        </div>
      ) : !ticket ? (
        <div className="flex-1 flex items-center justify-center">
          <p className="text-gray-400">Ticket not found</p>
        </div>
      ) : (
        <div className="flex-1 flex flex-col overflow-hidden bg-white">
          {/* Ticket header */}
          <div className="border-b border-gray-100 p-4">
            <div className="flex items-start justify-between gap-4">
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 mb-1">
                  <span className="text-xs font-mono text-gray-400">
                    #{ticket.number}
                  </span>
                  {slaConfig.label && (
                    <span
                      className={`text-xs px-2 py-0.5 rounded-full font-medium
                                 ${slaConfig.color} ${slaConfig.bg}`}
                    >
                      {slaConfig.label}
                    </span>
                  )}
                </div>
                <h2 className="text-lg font-semibold text-gray-900 truncate">
                  {ticket.subject}
                </h2>
                <div className="flex items-center gap-3 mt-1 text-xs text-gray-500">
                  <span className="flex items-center gap-1">
                    <User size={11} />
                    {ticket.customerName || ticket.customerEmail}
                  </span>
                  <span className="flex items-center gap-1">
                    <Clock size={11} />
                    {new Date(ticket.createdAt).toLocaleString()}
                  </span>
                </div>
              </div>

              {/* Actions */}
              <div className="flex items-center gap-2 flex-shrink-0">
                {ticket.status !== "resolved" && (
                  <button
                    onClick={() =>
                      updateStatus({ variables: { ticketId: id, status: "resolved" } })
                    }
                    className="flex items-center gap-1.5 px-3 py-1.5 bg-brand-600 hover:bg-brand-700
                               text-white text-xs rounded-lg font-medium transition-colors"
                  >
                    <CheckCircle size={12} />
                    Resolve
                  </button>
                )}

                {ticket.status === "resolved" && (
                  <button
                    onClick={() =>
                      updateStatus({ variables: { ticketId: id, status: "open" } })
                    }
                    className="px-3 py-1.5 border border-gray-200 text-gray-600 text-xs
                               rounded-lg hover:bg-gray-50 transition-colors"
                  >
                    Reopen
                  </button>
                )}
              </div>
            </div>
          </div>

          {/* Message thread */}
          <div className="flex-1 overflow-y-auto p-4 space-y-4">
            {ticket.messages.map((msg: any) => {
              const isCustomer = msg.authorType === "customer";
              const isStaff = msg.authorType === "staff";

              return (
                <div
                  key={msg.id}
                  className={`flex gap-3 ${isStaff ? "flex-row-reverse" : ""}`}
                >
                  {/* Avatar */}
                  <div
                    className={`w-8 h-8 rounded-full flex-shrink-0 flex items-center justify-center
                               text-sm font-medium
                               ${
                                 isCustomer
                                   ? "bg-blue-100 text-blue-600"
                                   : "bg-gray-100 text-gray-600"
                               }`}
                  >
                    {(msg.authorName || msg.authorEmail || "?")[0].toUpperCase()}
                  </div>

                  {/* Message bubble */}
                  <div
                    className={`max-w-xl ${isStaff ? "items-end" : "items-start"} flex flex-col`}
                  >
                    <div className="flex items-center gap-2 mb-1">
                      <span className="text-xs font-medium text-gray-700">
                        {msg.authorName || msg.authorEmail || "Unknown"}
                      </span>
                      {msg.isInternal && (
                        <span className="flex items-center gap-0.5 text-xs text-orange-500">
                          <Lock size={10} />
                          Internal note
                        </span>
                      )}
                      <span className="text-xs text-gray-400">
                        {new Date(msg.createdAt).toLocaleTimeString()}
                      </span>
                    </div>

                    <div
                      className={`px-4 py-3 rounded-2xl text-sm leading-relaxed
                        ${
                          msg.isInternal
                            ? "bg-orange-50 border border-orange-200 text-orange-900"
                            : isStaff
                            ? "bg-gray-900 text-white rounded-tr-sm"
                            : "bg-gray-100 text-gray-900 rounded-tl-sm"
                        }`}
                    >
                      <p className="whitespace-pre-wrap">{msg.body}</p>
                    </div>
                  </div>
                </div>
              );
            })}

            {/* AI Draft suggestion (shown when available) */}
            {ticket.aiDraftBody && !ticket.firstResponseAt && (
              <div className="border border-purple-200 bg-purple-50 rounded-xl p-4">
                <div className="flex items-center justify-between mb-2">
                  <div className="flex items-center gap-2">
                    <Zap size={14} className="text-purple-500" />
                    <span className="text-sm font-medium text-purple-700">
                      AI suggested reply
                    </span>
                    <span className="text-xs text-purple-400">
                      {Math.round((ticket.aiDraftConfidence ?? 0) * 100)}% confidence
                    </span>
                  </div>
                  <button
                    onClick={handleUseAIDraft}
                    className="text-xs text-purple-600 hover:text-purple-800 font-medium"
                  >
                    Use this draft →
                  </button>
                </div>
                <p className="text-sm text-purple-800 whitespace-pre-wrap line-clamp-4">
                  {ticket.aiDraftBody}
                </p>
              </div>
            )}
          </div>

          {/* Reply box */}
          <div className="border-t border-gray-100 p-4">
            {/* Toggle: Reply vs Internal Note */}
            <div className="flex gap-1 mb-3 bg-gray-100 p-1 rounded-lg w-fit">
              <button
                onClick={() => setIsInternal(false)}
                className={`px-3 py-1 rounded-md text-xs font-medium transition-colors
                  ${!isInternal ? "bg-white text-gray-900 shadow-sm" : "text-gray-500"}`}
              >
                Reply to customer
              </button>
              <button
                onClick={() => setIsInternal(true)}
                className={`px-3 py-1 rounded-md text-xs font-medium transition-colors flex items-center gap-1
                  ${isInternal ? "bg-white text-gray-900 shadow-sm" : "text-gray-500"}`}
              >
                <Lock size={10} />
                Internal note
              </button>
            </div>

            {/* Canned responses picker */}
            <div className="relative mb-2">
              <button
                onClick={() => setShowCanned(!showCanned)}
                className="flex items-center gap-1 text-xs text-gray-400 hover:text-gray-600 transition-colors"
              >
                <Tag size={12} />
                Canned responses
                <ChevronDown size={10} />
              </button>
              {showCanned && (
                <div
                  className="absolute bottom-full left-0 mb-2 w-72 bg-white rounded-xl
                             border border-gray-100 shadow-lg z-10 overflow-hidden"
                >
                  <div className="p-2 border-b border-gray-50">
                    <p className="text-xs font-medium text-gray-500 px-2">Quick replies</p>
                  </div>
                  <div className="max-h-48 overflow-y-auto">
                    {cannedResponses.length === 0 ? (
                      <p className="text-xs text-gray-400 px-3 py-2.5">
                        No canned responses yet
                      </p>
                    ) : (
                      cannedResponses.map((cr: any) => (
                        <button
                          key={cr.id}
                          onClick={() => {
                            setReplyBody(cr.body);
                            setShowCanned(false);
                          }}
                          className="w-full text-left px-3 py-2.5 hover:bg-gray-50 transition-colors"
                        >
                          <p className="text-sm font-medium text-gray-900">{cr.name}</p>
                          <p className="text-xs text-gray-400 truncate">{cr.body}</p>
                        </button>
                      ))
                    )}
                  </div>
                </div>
              )}
            </div>

            <div
              className={`relative rounded-xl border transition-colors
              ${isInternal ? "border-orange-200 bg-orange-50" : "border-gray-200"}`}
            >
              <textarea
                value={replyBody}
                onChange={(e) => setReplyBody(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) handleSubmit();
                }}
                placeholder={isInternal ? "Add an internal note..." : "Write a reply..."}
                rows={4}
                className="w-full px-4 py-3 text-sm bg-transparent resize-none
                           focus:outline-none placeholder-gray-400"
              />
              <div className="flex items-center justify-between px-4 pb-3">
                <p className="text-xs text-gray-400">⌘↵ to send</p>
                <button
                  onClick={handleSubmit}
                  disabled={!replyBody.trim() || replying}
                  className={`flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium
                             transition-colors disabled:opacity-50
                             ${
                               isInternal
                                 ? "bg-orange-500 hover:bg-orange-600 text-white"
                                 : "bg-gray-900 hover:bg-gray-800 text-white"
                             }`}
                >
                  <Send size={14} />
                  {replying ? "Sending..." : isInternal ? "Add note" : "Send reply"}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
