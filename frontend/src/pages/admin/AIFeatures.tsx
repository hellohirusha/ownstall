import { useState } from "react";
import { gql } from "@apollo/client";
import { useMutation, useQuery } from "@apollo/client/react";
import {
  Sparkles,
  Zap,
  Image as ImageIcon,
  TrendingUp,
  RefreshCw,
  CheckCircle,
  AlertCircle,
  DollarSign,
} from "lucide-react";
import toast from "react-hot-toast";
import { AdminNav } from "../../components/AdminNav";

const GET_AI_OVERVIEW = gql`
  query GetAIOverview {
    aiStats {
      enabled
      model
      totalRequests
      costUsd
      costLimitUsd
      circuitBreakerOpen
      copyGen {
        generated
        autoPublished
        avgQualityScore
      }
      autoReply {
        drafted
        autoSent
        avgConfidence
        ticketsMissingDraft
        deflectionRate
      }
      recommendations {
        indexed
        missing
      }
    }
    productsMissingCopy {
      id
      name
    }
  }
`;

const GENERATE_COPY = gql`
  mutation GenerateProductCopy($productId: UUID!) {
    generateProductCopy(productId: $productId) {
      body
      qualityScore
      toneLabel
      wordCount
      willAutoPublish
    }
  }
`;

const INDEX_ALL_PRODUCTS = gql`
  mutation IndexAllProducts {
    indexAllProducts {
      success
      count
    }
  }
`;

const PROCESS_TICKETS = gql`
  mutation ProcessNewTickets {
    processNewTickets {
      processed
    }
  }
`;

type GeneratedCopy = {
  body: string;
  qualityScore: number;
  toneLabel: string;
  wordCount: number;
  willAutoPublish: boolean;
};

type AIStats = {
  enabled: boolean;
  model: string;
  totalRequests: number;
  costUsd: number;
  costLimitUsd: number;
  circuitBreakerOpen: boolean;
  copyGen: { generated: number; autoPublished: number; avgQualityScore: number };
  autoReply: {
    drafted: number;
    autoSent: number;
    avgConfidence: number;
    ticketsMissingDraft: number;
    deflectionRate: number;
  };
  recommendations: { indexed: number; missing: number };
};

// Tailwind purges classes it cannot see as complete strings, so the
// card colours are spelled out rather than built as `bg-${color}-50`.
const CARD_STYLES = {
  purple: { wrap: "bg-purple-50", icon: "text-purple-500" },
  blue: { wrap: "bg-blue-50", icon: "text-blue-500" },
  green: { wrap: "bg-green-50", icon: "text-green-500" },
  orange: { wrap: "bg-orange-50", icon: "text-orange-500" },
} as const;

function scoreBarColor(score: number) {
  if (score >= 0.85) return "bg-green-500";
  if (score >= 0.7) return "bg-yellow-500";
  return "bg-red-400";
}

export function AIFeaturesPage() {
  const [selectedProductId, setSelectedProductId] = useState("");
  const [generatedCopies, setGeneratedCopies] = useState<GeneratedCopy[]>([]);

  const { data, loading, refetch } = useQuery<{
    aiStats: AIStats;
    productsMissingCopy: { id: string; name: string }[];
  }>(GET_AI_OVERVIEW);

  const [generateCopy, { loading: generating }] = useMutation<{
    generateProductCopy: GeneratedCopy[];
  }>(GENERATE_COPY, {
    onCompleted: (d) => {
      setGeneratedCopies(d.generateProductCopy);
      const published = d.generateProductCopy.filter((c) => c.willAutoPublish).length;
      toast.success(
        published > 0
          ? `Generated 3 variants — best one auto-published`
          : `Generated 3 variants — none cleared the auto-publish bar`
      );
      refetch();
    },
    onError: (e) => toast.error(e.message),
  });

  const [indexAll, { loading: indexing }] = useMutation<{
    indexAllProducts: { success: boolean; count: number };
  }>(INDEX_ALL_PRODUCTS, {
    onCompleted: (d) => {
      toast.success(`Indexed ${d.indexAllProducts.count} products`);
      refetch();
    },
    onError: (e) => toast.error(e.message),
  });

  const [processTickets, { loading: processing }] = useMutation<{
    processNewTickets: { processed: number };
  }>(PROCESS_TICKETS, {
    onCompleted: (d) => {
      toast.success(`Drafted ${d.processNewTickets.processed} ticket(s)`);
      refetch();
    },
    onError: (e) => toast.error(e.message),
  });

  const stats = data?.aiStats;
  const circuitOpen = stats?.circuitBreakerOpen ?? false;
  const aiDisabled = stats ? !stats.enabled : false;
  const blocked = circuitOpen || aiDisabled;

  const cards = [
    {
      icon: Sparkles,
      title: "Copy Generator",
      metric: `${stats?.copyGen.autoPublished ?? 0} auto-published`,
      sub: `${stats?.copyGen.generated ?? 0} generated · avg ${(
        (stats?.copyGen.avgQualityScore ?? 0) * 100
      ).toFixed(0)}%`,
      style: CARD_STYLES.purple,
    },
    {
      icon: Zap,
      title: "Auto-Reply",
      metric: `${stats?.autoReply.autoSent ?? 0} auto-sent`,
      sub: `${((stats?.autoReply.deflectionRate ?? 0) * 100).toFixed(0)}% deflection`,
      style: CARD_STYLES.blue,
    },
    {
      icon: TrendingUp,
      title: "Recommendations",
      metric: `${stats?.recommendations.indexed ?? 0} indexed`,
      sub: `${stats?.recommendations.missing ?? 0} awaiting index`,
      style: CARD_STYLES.green,
    },
    {
      icon: ImageIcon,
      title: "Image QA",
      metric: "On demand",
      sub: "Vision model check",
      style: CARD_STYLES.orange,
    },
  ];

  return (
    <div className="min-h-screen bg-gray-50">
      <AdminNav />

      <div className="max-w-5xl mx-auto px-4 py-6">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-2xl font-bold text-gray-900 flex items-center gap-2">
              <Sparkles size={24} className="text-purple-500" />
              AI Features
            </h1>
            <p className="text-sm text-gray-500 mt-1">
              {stats?.model ? `Model: ${stats.model}` : "Automated content and support tools"}
            </p>
          </div>

          {/* Month-to-date spend against the budget */}
          <div
            className={`flex items-center gap-2 px-4 py-2 rounded-xl border ${
              circuitOpen ? "bg-red-50 border-red-200" : "bg-green-50 border-green-200"
            }`}
          >
            <DollarSign
              size={14}
              className={circuitOpen ? "text-red-500" : "text-brand-600"}
            />
            <div>
              <p className="text-xs font-semibold text-gray-900">
                ${(stats?.costUsd ?? 0).toFixed(4)}
                <span className="text-gray-400"> / ${stats?.costLimitUsd ?? 0}</span>
              </p>
              <p className={`text-xs ${circuitOpen ? "text-red-500" : "text-gray-400"}`}>
                {circuitOpen
                  ? "Circuit breaker open"
                  : `${stats?.totalRequests ?? 0} calls this month`}
              </p>
            </div>
          </div>
        </div>

        {aiDisabled && (
          <div className="mb-6 p-4 bg-amber-50 border border-amber-200 rounded-xl flex items-start gap-3">
            <AlertCircle size={18} className="text-amber-500 flex-shrink-0 mt-0.5" />
            <div>
              <p className="font-medium text-amber-800">No AI provider configured</p>
              <p className="text-sm text-amber-600 mt-0.5">
                Set GROQ_API_KEY (or OPENAI_API_KEY) on the API to enable these features.
              </p>
            </div>
          </div>
        )}

        {circuitOpen && (
          <div className="mb-6 p-4 bg-red-50 border border-red-200 rounded-xl flex items-start gap-3">
            <AlertCircle size={18} className="text-red-500 flex-shrink-0 mt-0.5" />
            <div>
              <p className="font-medium text-red-800">AI cost limit reached</p>
              <p className="text-sm text-red-600 mt-0.5">
                Calls are paused until the monthly budget resets, or until
                AI_MONTHLY_COST_LIMIT_USD is raised.
              </p>
            </div>
          </div>
        )}

        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
          {cards.map(({ icon: Icon, title, metric, sub, style }) => (
            <div key={title} className="bg-white rounded-xl border border-gray-100 p-4">
              <div
                className={`w-9 h-9 rounded-lg mb-3 flex items-center justify-center ${style.wrap}`}
              >
                <Icon size={16} className={style.icon} />
              </div>
              <p className="text-sm font-semibold text-gray-900">{title}</p>
              <p className="text-lg font-bold text-gray-900 mt-1">{metric}</p>
              <p className="text-xs text-gray-400 mt-0.5">{sub}</p>
            </div>
          ))}
        </div>

        {/* ── Product copy generator ───────────────────────── */}
        <div className="bg-white rounded-xl border border-gray-100 p-6 mb-6">
          <h2 className="font-semibold text-gray-900 mb-1 flex items-center gap-2">
            <Sparkles size={16} className="text-purple-500" />
            Product copy generator
          </h2>
          <p className="text-sm text-gray-500 mb-4">
            Writes three tones, scores them, and publishes the best automatically at 85%+.
          </p>

          <div className="flex gap-3 mb-4">
            <select
              value={selectedProductId}
              onChange={(e) => setSelectedProductId(e.target.value)}
              className="flex-1 px-3 py-2 border border-gray-200 rounded-lg text-sm
                         focus:outline-none focus:ring-2 focus:ring-purple-500"
            >
              <option value="">
                {loading ? "Loading products…" : "Select a product"}
              </option>
              {data?.productsMissingCopy?.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </select>
            <button
              onClick={() =>
                selectedProductId &&
                generateCopy({ variables: { productId: selectedProductId } })
              }
              disabled={generating || !selectedProductId || blocked}
              className="flex items-center gap-2 px-4 py-2 bg-purple-500 hover:bg-purple-600
                         text-white text-sm rounded-lg font-medium disabled:opacity-50"
            >
              {generating ? (
                <RefreshCw size={14} className="animate-spin" />
              ) : (
                <Sparkles size={14} />
              )}
              Generate 3 variants
            </button>
          </div>

          {generatedCopies.length > 0 && (
            <div className="space-y-3">
              {generatedCopies.map((copy, i) => (
                <div
                  key={i}
                  className={`p-4 rounded-xl border-2 ${
                    copy.willAutoPublish
                      ? "border-green-300 bg-green-50"
                      : "border-gray-100"
                  }`}
                >
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2">
                      <span className="text-xs font-medium text-gray-500 uppercase tracking-wide">
                        {copy.toneLabel}
                      </span>
                      <span className="text-xs text-gray-400">{copy.wordCount} words</span>
                      {copy.willAutoPublish && (
                        <span
                          className="flex items-center gap-1 text-xs text-green-600
                                     bg-green-100 px-2 py-0.5 rounded-full"
                        >
                          <CheckCircle size={10} />
                          Auto-published
                        </span>
                      )}
                    </div>
                    <div className="flex items-center gap-2">
                      <div className="w-20 h-1.5 bg-gray-100 rounded-full overflow-hidden">
                        <div
                          className={`h-full rounded-full ${scoreBarColor(copy.qualityScore)}`}
                          style={{ width: `${copy.qualityScore * 100}%` }}
                        />
                      </div>
                      <span className="text-xs font-mono text-gray-500">
                        {(copy.qualityScore * 100).toFixed(0)}
                      </span>
                    </div>
                  </div>
                  <p className="text-sm text-gray-700 leading-relaxed">{copy.body}</p>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* ── Recommendations ──────────────────────────────── */}
        <div className="bg-white rounded-xl border border-gray-100 p-6 mb-6">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="font-semibold text-gray-900 flex items-center gap-2">
                <TrendingUp size={16} className="text-brand-600" />
                Product recommendations
              </h2>
              <p className="text-sm text-gray-500 mt-1">
                {stats?.recommendations.missing ?? 0} product(s) need indexing
              </p>
            </div>
            <button
              onClick={() => indexAll()}
              disabled={indexing}
              className="flex items-center gap-2 px-4 py-2 bg-brand-600 hover:bg-brand-700
                         text-white text-sm rounded-lg font-medium disabled:opacity-50"
            >
              {indexing ? (
                <RefreshCw size={14} className="animate-spin" />
              ) : (
                <TrendingUp size={14} />
              )}
              Index all products
            </button>
          </div>
        </div>

        {/* ── Auto-reply ───────────────────────────────────── */}
        <div className="bg-white rounded-xl border border-gray-100 p-6">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="font-semibold text-gray-900 flex items-center gap-2">
                <Zap size={16} className="text-blue-500" />
                Auto-reply drafts
              </h2>
              <p className="text-sm text-gray-500 mt-1">
                {stats?.autoReply.ticketsMissingDraft ?? 0} open ticket(s) without a draft
              </p>
            </div>
            <button
              onClick={() => processTickets()}
              disabled={processing || blocked}
              className="flex items-center gap-2 px-4 py-2 bg-blue-500 hover:bg-blue-600
                         text-white text-sm rounded-lg font-medium disabled:opacity-50"
            >
              {processing ? (
                <RefreshCw size={14} className="animate-spin" />
              ) : (
                <Zap size={14} />
              )}
              Generate drafts now
            </button>
          </div>

          <div className="mt-4 pt-4 border-t border-gray-100 text-sm text-gray-600">
            <p>
              Auto-send threshold: <strong>90% confidence</strong>
              <span className="text-gray-400 ml-2 text-xs">
                replies at or above it are sent and the ticket is resolved, without review
              </span>
            </p>
            <p className="mt-1 text-xs text-gray-400">
              {stats?.autoReply.drafted ?? 0} drafted · avg confidence{" "}
              {((stats?.autoReply.avgConfidence ?? 0) * 100).toFixed(0)}%
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
