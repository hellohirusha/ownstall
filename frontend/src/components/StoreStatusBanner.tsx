import { gql } from "@apollo/client";
import { useMutation, useQuery } from "@apollo/client/react";
import { AlertTriangle, Clock, Send, XCircle } from "lucide-react";
import toast from "react-hot-toast";

const MY_STORE = gql`
  query MyStoreStatus {
    myStore {
      id
      name
      subdomain
      status
      reviewNote
      canPublishProducts
      canAcceptOrders
    }
  }
`;

const SUBMIT_FOR_REVIEW = gql`
  mutation SubmitStoreForReview {
    submitStoreForReview {
      id
      status
    }
  }
`;

interface MyStore {
  id: string;
  name: string;
  subdomain: string;
  status: string;
  reviewNote: string | null;
  canPublishProducts: boolean;
  canAcceptOrders: boolean;
}

// Tells a seller where their stall stands. Without this a pending stall looks
// identical to a live one from inside the dashboard, and the seller has no way
// to know why nobody can find them.
export function StoreStatusBanner() {
  const { data } = useQuery<{ myStore: MyStore | null }>(MY_STORE);
  const [submit, { loading: submitting }] = useMutation(SUBMIT_FOR_REVIEW, {
    refetchQueries: ["MyStoreStatus"],
  });

  const store = data?.myStore;
  if (!store) return null;

  const restricted = !store.canPublishProducts || !store.canAcceptOrders;

  if (store.status === "approved" && !restricted) return null;

  const handleSubmit = async () => {
    try {
      await submit();
      toast.success("Submitted for review");
    } catch (e: any) {
      toast.error(e?.message ?? "Could not submit");
    }
  };

  if (store.status === "pending") {
    return (
      <Banner tone="amber" icon={Clock}>
        <strong>{store.name} is waiting for review.</strong> Your stall stays
        private and cannot take orders until it is approved. Add your products
        in the meantime — they go live the moment you are approved.
      </Banner>
    );
  }

  if (store.status === "rejected") {
    return (
      <Banner tone="red" icon={XCircle}>
        <strong>Your stall was not approved.</strong>
        {store.reviewNote && <> Reason: {store.reviewNote}</>} Fix the issue and
        resubmit.
        <button
          onClick={handleSubmit}
          disabled={submitting}
          className="ml-3 inline-flex items-center gap-1.5 px-3 py-1 bg-ink-900
                     hover:bg-ink-800 text-white rounded-lg text-xs font-semibold
                     transition-colors disabled:opacity-50"
        >
          <Send size={12} />
          {submitting ? "Submitting…" : "Resubmit for review"}
        </button>
      </Banner>
    );
  }

  if (store.status === "suspended") {
    return (
      <Banner tone="red" icon={AlertTriangle}>
        <strong>Your stall is suspended.</strong>
        {store.reviewNote && <> Reason: {store.reviewNote}</>} It is hidden from
        shoppers and cannot take orders. Contact support to appeal.
      </Banner>
    );
  }

  // Approved but restricted
  return (
    <Banner tone="amber" icon={AlertTriangle}>
      <strong>Your stall has restrictions.</strong>{" "}
      {!store.canPublishProducts && "You cannot publish new products. "}
      {!store.canAcceptOrders && "You cannot accept new orders. "}
      Contact support if you think this is a mistake.
    </Banner>
  );
}

function Banner({
  tone,
  icon: Icon,
  children,
}: {
  tone: "amber" | "red";
  icon: React.ComponentType<{ size?: number; className?: string }>;
  children: React.ReactNode;
}) {
  const styles =
    tone === "red"
      ? "bg-red-50 border-red-200 text-red-800"
      : "bg-amber-50 border-amber-200 text-amber-900";

  return (
    <div className={`border-b ${styles}`}>
      <div className="max-w-6xl mx-auto px-4 py-3 flex items-start gap-2.5 text-sm">
        <Icon size={16} className="mt-0.5 flex-shrink-0" />
        <p>{children}</p>
      </div>
    </div>
  );
}
