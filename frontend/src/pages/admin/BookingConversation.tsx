import { useState, useEffect, useRef } from "react";
import { useParams } from "react-router-dom";
import { gql } from "@apollo/client";
import { useMutation, useQuery } from "@apollo/client/react";
import { Send, CheckCircle, Package } from "lucide-react";
import toast from "react-hot-toast";
import { AdminNav } from "../../components/AdminNav";

const GET_BOOKING = gql`
  query GetBooking($id: UUID!) {
    booking(id: $id) {
      id
      title
      status
      agreedPrice
      creatorPayout
      platformFee
      clientEmail
      clientName
      deliveredAt
      service {
        title
      }
      profile {
        displayName
      }
      messages {
        id
        body
        senderName
        senderEmail
        createdAt
      }
    }
  }
`;

const SEND_MESSAGE = gql`
  mutation SendBookingMessage($bookingId: UUID!, $body: String!) {
    sendBookingMessage(bookingId: $bookingId, body: $body) {
      id
      body
      senderName
      createdAt
    }
  }
`;

const DELIVER_BOOKING = gql`
  mutation DeliverBooking($id: UUID!) {
    deliverBooking(id: $id) {
      id
      status
      deliveredAt
    }
  }
`;

const COMPLETE_BOOKING = gql`
  mutation CompleteBooking($id: UUID!) {
    completeBooking(id: $id) {
      id
      status
      completedAt
    }
  }
`;

export function BookingConversationPage() {
  const { id } = useParams<{ id: string }>();
  const [messageBody, setMessageBody] = useState("");
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const currentUserEmail = JSON.parse(localStorage.getItem("user") ?? "{}").email ?? "";

  const { data, loading, refetch } = useQuery<{ booking: any }>(GET_BOOKING, {
    variables: { id },
    pollInterval: 10000,
  });

  const [sendMessage, { loading: sending }] = useMutation(SEND_MESSAGE, {
    onCompleted: () => {
      setMessageBody("");
      refetch();
    },
    onError: (e) => toast.error(e.message),
  });

  const [deliverBooking] = useMutation(DELIVER_BOOKING, {
    onCompleted: () => {
      toast.success("Delivery submitted!");
      refetch();
    },
    onError: (e) => toast.error(e.message),
  });

  const [completeBooking] = useMutation(COMPLETE_BOOKING, {
    onCompleted: () => {
      toast.success("Booking completed! Payment released.");
      refetch();
    },
    onError: (e) => toast.error(e.message),
  });

  const booking = data?.booking;

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [booking?.messages]);

  const isClient = currentUserEmail === booking?.clientEmail;
  const isCreator = !!booking && !isClient;

  const STATUS_STEPS = ["pending", "accepted", "in_progress", "delivered", "completed"];
  const currentStep = STATUS_STEPS.indexOf(booking?.status);

  return (
    <div className="h-screen flex flex-col">
      <AdminNav />
      {loading ? (
        <div className="flex-1 flex items-center justify-center">
          <div className="w-8 h-8 border-2 border-green-500 border-t-transparent rounded-full animate-spin" />
        </div>
      ) : !booking ? (
        <div className="flex-1 flex items-center justify-center">
          <p className="text-gray-500">Booking not found</p>
        </div>
      ) : (
        <div className="flex-1 flex flex-col overflow-hidden bg-white">
          {/* Header */}
          <div className="border-b border-gray-100 px-6 py-4 bg-white">
            <div className="flex items-center justify-between">
              <div>
                <h2 className="font-bold text-gray-900">{booking.title}</h2>
                <p className="text-sm text-gray-500">
                  {booking.service?.title ?? "Custom project"} · ${booking.agreedPrice}
                </p>
              </div>

              {/* Status stepper */}
              <div className="flex items-center gap-2">
                {STATUS_STEPS.slice(0, -1).map((step, i) => (
                  <div key={step} className="flex items-center gap-1">
                    <div
                      className={`w-6 h-6 rounded-full flex items-center justify-center text-xs
                        ${i <= currentStep ? "bg-green-500 text-white" : "bg-gray-100 text-gray-400"}`}
                    >
                      {i < currentStep ? <CheckCircle size={12} /> : i + 1}
                    </div>
                    {i < STATUS_STEPS.length - 2 && (
                      <div className={`w-8 h-0.5 ${i < currentStep ? "bg-green-500" : "bg-gray-100"}`} />
                    )}
                  </div>
                ))}
              </div>
            </div>
          </div>

          {/* Messages */}
          <div className="flex-1 overflow-y-auto p-6 space-y-4">
            {booking.messages?.length === 0 && (
              <p className="text-center text-sm text-gray-400 py-8">
                No messages yet — start the conversation below
              </p>
            )}
            {booking.messages?.map((msg: any) => {
              const isMine = msg.senderEmail === currentUserEmail;
              return (
                <div key={msg.id} className={`flex gap-3 ${isMine ? "flex-row-reverse" : ""}`}>
                  <div
                    className={`w-8 h-8 rounded-full flex-shrink-0 flex items-center justify-center
                               text-sm font-medium
                               ${isMine ? "bg-green-100 text-green-600" : "bg-gray-100 text-gray-600"}`}
                  >
                    {(msg.senderName || msg.senderEmail || "?")[0].toUpperCase()}
                  </div>
                  <div className={`max-w-lg ${isMine ? "items-end" : "items-start"} flex flex-col`}>
                    <p className={`text-xs text-gray-400 mb-1 ${isMine ? "text-right" : ""}`}>
                      {msg.senderName || msg.senderEmail} ·{" "}
                      {new Date(msg.createdAt).toLocaleTimeString()}
                    </p>
                    <div
                      className={`px-4 py-2.5 rounded-2xl text-sm
                        ${
                          isMine
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
            <div ref={messagesEndRef} />
          </div>

          {/* Action buttons */}
          {booking.status === "accepted" && isCreator && (
            <div className="px-6 py-3 border-t border-gray-50 bg-gray-50">
              <button
                onClick={() => deliverBooking({ variables: { id } })}
                className="flex items-center gap-2 px-4 py-2 bg-blue-500 text-white
                           text-sm rounded-lg font-medium hover:bg-blue-600"
              >
                <Package size={14} />
                Mark as delivered
              </button>
            </div>
          )}

          {booking.status === "delivered" && isClient && (
            <div className="px-6 py-3 border-t border-gray-50 bg-green-50">
              <div className="flex items-center justify-between">
                <p className="text-sm text-green-700">
                  Creator marked this as delivered. Approve to release payment.
                </p>
                <button
                  onClick={() => completeBooking({ variables: { id } })}
                  className="flex items-center gap-2 px-4 py-2 bg-green-500 text-white
                             text-sm rounded-lg font-medium hover:bg-green-600"
                >
                  <CheckCircle size={14} />
                  Approve & release payment (${booking.agreedPrice})
                </button>
              </div>
            </div>
          )}

          {/* Message input */}
          {!["completed", "cancelled", "declined"].includes(booking.status) && (
            <div className="border-t border-gray-100 p-4">
              <div className="flex gap-2">
                <textarea
                  value={messageBody}
                  onChange={(e) => setMessageBody(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" && (e.metaKey || e.ctrlKey) && messageBody.trim()) {
                      sendMessage({ variables: { bookingId: id, body: messageBody } });
                    }
                  }}
                  placeholder="Send a message..."
                  rows={2}
                  className="flex-1 px-4 py-2.5 border border-gray-200 rounded-xl text-sm
                             focus:outline-none focus:ring-2 focus:ring-green-500 resize-none"
                />
                <button
                  onClick={() => {
                    if (messageBody.trim()) {
                      sendMessage({ variables: { bookingId: id, body: messageBody } });
                    }
                  }}
                  disabled={!messageBody.trim() || sending}
                  className="px-4 py-2 bg-gray-900 text-white rounded-xl hover:bg-gray-800
                             transition-colors disabled:opacity-50"
                >
                  <Send size={16} />
                </button>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
