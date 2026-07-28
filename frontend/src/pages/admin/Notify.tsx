import { useState } from "react";
import { gql } from "@apollo/client";
import { useMutation, useQuery } from "@apollo/client/react";
import {
  Mail,
  Send,
  Clock,
  CheckCircle,
  XCircle,
  FileText,
} from "lucide-react";
import toast from "react-hot-toast";
import { AdminNav } from "../../components/AdminNav";

const GET_NOTIFY_DATA = gql`
  query GetNotifyData {
    emailTemplates {
      id
      name
      slug
      subject
      isSystem
    }
    emailCampaigns {
      id
      name
      subject
      status
      recipientCount
      sentCount
      deliveredCount
      openedCount
      clickedCount
      scheduledAt
      completedAt
      createdAt
    }
  }
`;

const CREATE_CAMPAIGN = gql`
  mutation CreateCampaign($input: CreateCampaignInput!) {
    createCampaign(input: $input) {
      id
      status
    }
  }
`;

const SCHEDULE_CAMPAIGN = gql`
  mutation ScheduleCampaign($id: UUID!) {
    scheduleCampaign(id: $id) {
      id
      status
    }
  }
`;

const STATUS_CONFIG: Record<
  string,
  { label: string; color: string; icon: any }
> = {
  draft: { label: "Draft", color: "bg-gray-100 text-gray-600", icon: FileText },
  scheduled: {
    label: "Scheduled",
    color: "bg-yellow-100 text-yellow-700",
    icon: Clock,
  },
  sending: { label: "Sending", color: "bg-blue-100 text-blue-700", icon: Send },
  sent: {
    label: "Sent",
    color: "bg-green-100 text-green-700",
    icon: CheckCircle,
  },
  cancelled: {
    label: "Cancelled",
    color: "bg-gray-100 text-gray-500",
    icon: XCircle,
  },
  failed: { label: "Failed", color: "bg-red-100 text-red-700", icon: XCircle },
};

export function NotifyPage() {
  const { data, loading } = useQuery<{
    emailTemplates: any[];
    emailCampaigns: any[];
  }>(GET_NOTIFY_DATA, {
    pollInterval: 15000, // refresh so campaign stats populate as the worker sends
  });

  const [createCampaign, { loading: creating }] = useMutation<{
    createCampaign: { id: string; status: string };
  }>(CREATE_CAMPAIGN, { refetchQueries: ["GetNotifyData"] });

  const [scheduleCampaign, { loading: scheduling }] = useMutation<{
    scheduleCampaign: { id: string; status: string };
  }>(SCHEDULE_CAMPAIGN, { refetchQueries: ["GetNotifyData"] });

  const [form, setForm] = useState({ name: "", subject: "", templateId: "" });

  const templates = data?.emailTemplates ?? [];
  const campaigns = data?.emailCampaigns ?? [];

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!form.name || !form.subject || !form.templateId) {
      toast.error("Fill in all fields");
      return;
    }
    try {
      await createCampaign({ variables: { input: form } });
      toast.success("Campaign created as draft");
      setForm({ name: "", subject: "", templateId: "" });
    } catch (err: any) {
      toast.error(err.message || "Failed to create campaign");
    }
  };

  const handleSendNow = async (id: string) => {
    try {
      await scheduleCampaign({ variables: { id } });
      toast.success("Campaign queued — sends within a minute");
    } catch (err: any) {
      toast.error(err.message || "Failed to schedule campaign");
    }
  };

  return (
    <>
      <AdminNav />
      <div className="p-6 max-w-6xl mx-auto">
        <div className="mb-6">
          <h1 className="text-2xl font-bold text-gray-900">Notify</h1>
          <p className="text-sm text-gray-500 mt-1">
            Email templates and campaigns
          </p>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* New campaign form */}
          <div className="bg-white rounded-2xl border border-gray-100 p-5 h-fit">
            <h2 className="font-semibold text-gray-900 mb-4">New campaign</h2>
            <form onSubmit={handleCreate} className="flex flex-col gap-3">
              <div>
                <label className="text-sm font-medium text-gray-700 block mb-1">
                  Name
                </label>
                <input
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  placeholder="July newsletter"
                  className="w-full px-3 py-2 border border-gray-200 rounded-lg text-sm
                             focus:outline-none focus:ring-2 focus:ring-green-500"
                />
              </div>
              <div>
                <label className="text-sm font-medium text-gray-700 block mb-1">
                  Subject
                </label>
                <input
                  value={form.subject}
                  onChange={(e) =>
                    setForm({ ...form, subject: e.target.value })
                  }
                  placeholder="News from {{.StoreName}}"
                  className="w-full px-3 py-2 border border-gray-200 rounded-lg text-sm
                             focus:outline-none focus:ring-2 focus:ring-green-500"
                />
              </div>
              <div>
                <label className="text-sm font-medium text-gray-700 block mb-1">
                  Template
                </label>
                <select
                  value={form.templateId}
                  onChange={(e) =>
                    setForm({ ...form, templateId: e.target.value })
                  }
                  className="w-full px-3 py-2 border border-gray-200 rounded-lg text-sm
                             bg-white focus:outline-none focus:ring-2 focus:ring-green-500"
                >
                  <option value="">Select a template…</option>
                  {templates.map((t: any) => (
                    <option key={t.id} value={t.id}>
                      {t.name}
                    </option>
                  ))}
                </select>
              </div>
              <button
                type="submit"
                disabled={creating}
                className="mt-1 w-full py-2.5 bg-green-500 hover:bg-green-600 text-white
                           font-medium rounded-lg transition-colors text-sm
                           disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {creating ? "Creating..." : "Create draft"}
              </button>
            </form>

            {/* Templates list */}
            <div className="mt-6 pt-5 border-t border-gray-100">
              <h3 className="text-sm font-semibold text-gray-900 mb-3">
                Templates ({templates.length})
              </h3>
              <div className="space-y-2">
                {templates.map((t: any) => (
                  <div key={t.id} className="flex items-center gap-2 text-sm">
                    <Mail size={14} className="text-gray-400 flex-shrink-0" />
                    <span className="text-gray-700 truncate">{t.name}</span>
                    {t.isSystem && (
                      <span className="ml-auto text-xs px-1.5 py-0.5 bg-gray-100 text-gray-500 rounded">
                        system
                      </span>
                    )}
                  </div>
                ))}
              </div>
            </div>
          </div>

          {/* Campaign list */}
          <div className="lg:col-span-2">
            {loading ? (
              <div className="space-y-3">
                {[...Array(3)].map((_, i) => (
                  <div
                    key={i}
                    className="h-24 bg-gray-100 rounded-xl animate-pulse"
                  />
                ))}
              </div>
            ) : campaigns.length === 0 ? (
              <div className="bg-white rounded-2xl border border-gray-100 text-center py-16">
                <Send className="mx-auto h-14 w-14 text-gray-200 mb-4" />
                <h3 className="text-lg font-medium text-gray-900">
                  No campaigns yet
                </h3>
                <p className="text-gray-500 text-sm mt-1">
                  Create one on the left, then hit &ldquo;Send now&rdquo;
                </p>
              </div>
            ) : (
              <div className="space-y-3">
                {campaigns.map((c: any) => {
                  const config = STATUS_CONFIG[c.status] ?? STATUS_CONFIG.draft;
                  const Icon = config.icon;
                  return (
                    <div
                      key={c.id}
                      className="bg-white rounded-2xl border border-gray-100 p-5"
                    >
                      <div className="flex items-center justify-between mb-3">
                        <div className="min-w-0">
                          <h3 className="font-semibold text-gray-900 truncate">
                            {c.name}
                          </h3>
                          <p className="text-sm text-gray-500 truncate">
                            {c.subject}
                          </p>
                        </div>
                        <div className="flex items-center gap-3 flex-shrink-0 ml-4">
                          <span
                            className={`inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium ${config.color}`}
                          >
                            <Icon size={10} />
                            {config.label}
                          </span>
                          {c.status === "draft" && (
                            <button
                              onClick={() => handleSendNow(c.id)}
                              disabled={scheduling}
                              className="flex items-center gap-1.5 px-3 py-1.5 bg-gray-900 text-white
                                         rounded-lg text-xs font-medium hover:bg-gray-800
                                         transition-colors disabled:opacity-50"
                            >
                              <Send size={12} />
                              Send now
                            </button>
                          )}
                        </div>
                      </div>

                      {/* Stats */}
                      <div className="grid grid-cols-5 gap-2 text-center">
                        {[
                          { label: "Recipients", value: c.recipientCount },
                          { label: "Sent", value: c.sentCount },
                          { label: "Delivered", value: c.deliveredCount },
                          { label: "Opened", value: c.openedCount },
                          { label: "Clicked", value: c.clickedCount },
                        ].map((stat) => (
                          <div
                            key={stat.label}
                            className="bg-gray-50 rounded-lg py-2"
                          >
                            <p className="text-lg font-bold text-gray-900">
                              {stat.value}
                            </p>
                            <p className="text-xs text-gray-400">
                              {stat.label}
                            </p>
                          </div>
                        ))}
                      </div>

                      <p className="text-xs text-gray-400 mt-3">
                        Created {new Date(c.createdAt).toLocaleString()}
                        {c.completedAt &&
                          ` · Completed ${new Date(c.completedAt).toLocaleString()}`}
                      </p>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      </div>
    </>
  );
}
