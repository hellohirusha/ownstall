import { useEffect, useRef, useState } from "react";
import { gql } from "@apollo/client";
import { useMutation, useQuery } from "@apollo/client/react";
import { Link } from "react-router-dom";
import {
  ExternalLink,
  ImagePlus,
  Loader2,
  Send,
  Store as StoreIcon,
  Trash2,
} from "lucide-react";
import toast from "react-hot-toast";

import { AdminNav } from "../../components/AdminNav";
import { Input } from "../../components/ui/Input";
import { STALL_CATEGORIES } from "../../lib/categories";
import { getAccessToken } from "../../lib/session";

const MY_STORE = gql`
  query MyStallProfile {
    myStore {
      id
      name
      subdomain
      tagline
      description
      category
      location
      logoUrl
      status
      reviewNote
      productCount
      submittedAt
      reviewedAt
    }
  }
`;

const UPDATE_STORE_PROFILE = gql`
  mutation UpdateStoreProfile($input: StoreProfileInput!) {
    updateStoreProfile(input: $input) {
      id
      name
      tagline
      description
      category
      location
      logoUrl
      status
    }
  }
`;

const SUBMIT_FOR_REVIEW = gql`
  mutation SubmitStallForReview {
    submitStoreForReview {
      id
      status
      submittedAt
    }
  }
`;

interface MyStore {
  id: string;
  name: string;
  subdomain: string;
  tagline: string | null;
  description: string | null;
  category: string | null;
  location: string | null;
  logoUrl: string | null;
  status: string;
  reviewNote: string | null;
  productCount: number;
  submittedAt: string | null;
  reviewedAt: string | null;
}

const STATUS_STYLE: Record<string, string> = {
  approved: "bg-green-100 text-green-700",
  pending: "bg-amber-100 text-amber-800",
  rejected: "bg-red-100 text-red-700",
  suspended: "bg-red-100 text-red-700",
};

const EMPTY = {
  name: "",
  tagline: "",
  description: "",
  category: "",
  location: "",
  logoUrl: "",
};

export function StallProfilePage() {
  const { data, loading } = useQuery<{ myStore: MyStore | null }>(MY_STORE);
  const [form, setForm] = useState(EMPTY);
  const [uploading, setUploading] = useState(false);
  const fileInput = useRef<HTMLInputElement>(null);

  const [save, { loading: saving }] = useMutation(UPDATE_STORE_PROFILE, {
    refetchQueries: ["MyStallProfile", "MyStoreStatus"],
  });
  const [submitForReview, { loading: submitting }] = useMutation(
    SUBMIT_FOR_REVIEW,
    { refetchQueries: ["MyStallProfile", "MyStoreStatus"] },
  );

  const store = data?.myStore ?? null;

  // Seed the form once the stall arrives. Nulls become empty strings so the
  // inputs stay controlled, and an empty string is exactly what the API
  // reads as "clear this field".
  useEffect(() => {
    if (!store) return;
    setForm({
      name: store.name ?? "",
      tagline: store.tagline ?? "",
      description: store.description ?? "",
      category: store.category ?? "",
      location: store.location ?? "",
      logoUrl: store.logoUrl ?? "",
    });
  }, [store]);

  const handleChange = (
    e: React.ChangeEvent<
      HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement
    >,
  ) => {
    const { name, value } = e.target;
    setForm((prev) => ({ ...prev, [name]: value }));
  };

  // Reuses the product-image endpoint — it is a generic authenticated
  // Cloudinary upload, not something product-specific.
  const handleLogoUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    setUploading(true);
    const body = new FormData();
    body.append("image", file);

    try {
      const token = getAccessToken("tenant");
      const res = await fetch(
        `${process.env.REACT_APP_API_URL}/api/upload/product-image`,
        {
          method: "POST",
          headers: token ? { Authorization: `Bearer ${token}` } : undefined,
          body,
        },
      );
      const result = await res.json();
      if (result.url) {
        setForm((prev) => ({ ...prev, logoUrl: result.url }));
        toast.success("Logo uploaded — remember to save");
      } else {
        toast.error(result.error || "Logo upload failed");
      }
    } catch {
      toast.error("Logo upload failed");
    } finally {
      setUploading(false);
      // Let the same file be picked again after a failure
      if (fileInput.current) fileInput.current.value = "";
    }
  };

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!form.name.trim()) {
      toast.error("Your stall needs a name");
      return;
    }

    try {
      await save({ variables: { input: form } });
      toast.success("Stall profile saved");
    } catch (err: any) {
      toast.error(err?.message ?? "Could not save");
    }
  };

  const handleSubmitForReview = async () => {
    try {
      await submitForReview();
      toast.success("Sent for review");
    } catch (err: any) {
      toast.error(err?.message ?? "Could not submit");
    }
  };

  if (loading && !store) {
    return (
      <>
        <AdminNav />
        <div className="flex justify-center py-24">
          <div className="w-8 h-8 border-2 border-brand-600 border-t-transparent rounded-full animate-spin" />
        </div>
      </>
    );
  }

  if (!store) {
    return (
      <>
        <AdminNav />
        <div className="max-w-2xl mx-auto px-4 py-20 text-center">
          <StoreIcon className="mx-auto h-12 w-12 text-ink-200 mb-4" />
          <h1 className="text-xl font-bold text-ink-900 mb-2">
            No stall on this account
          </h1>
          <p className="text-ink-500">
            Sign in with the account that opened the stall.
          </p>
        </div>
      </>
    );
  }

  const canSubmit = store.status === "pending" || store.status === "rejected";

  return (
    <>
      <AdminNav />

      <div className="max-w-3xl mx-auto px-4 py-8">
        <header className="mb-8">
          <h1 className="text-2xl font-bold text-ink-900 mb-1">
            Stall profile
          </h1>
          <p className="text-sm text-ink-500">
            This is what shoppers see in the stall directory. A category and a
            tagline are what make your stall findable.
          </p>
        </header>

        {/* Status */}
        <div className="rounded-2xl border border-ink-200 bg-white p-5 shadow-card mb-6">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex items-center gap-3">
              <span
                className={`px-2.5 py-1 rounded-full text-xs font-semibold ${
                  STATUS_STYLE[store.status] ?? "bg-ink-100 text-ink-600"
                }`}
              >
                {store.status}
              </span>
              <span className="text-sm text-ink-500">
                {store.productCount}{" "}
                {store.productCount === 1 ? "product" : "products"}
              </span>
            </div>

            <div className="flex items-center gap-2">
              {store.status === "approved" && (
                <Link
                  to={`/store?store=${store.subdomain}`}
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium
                             text-ink-600 hover:text-ink-900 transition-colors"
                >
                  <ExternalLink size={14} />
                  View stall
                </Link>
              )}
              {canSubmit && (
                <button
                  onClick={handleSubmitForReview}
                  disabled={submitting}
                  className="inline-flex items-center gap-1.5 px-3.5 py-1.5 bg-ink-900
                             hover:bg-ink-800 text-white rounded-lg text-sm font-semibold
                             transition-colors disabled:opacity-50"
                >
                  <Send size={14} />
                  {submitting ? "Sending…" : "Submit for review"}
                </button>
              )}
            </div>
          </div>

          {store.reviewNote && (
            <p className="mt-3 text-sm text-ink-600">
              <span className="font-medium">Reviewer note:</span>{" "}
              {store.reviewNote}
            </p>
          )}
        </div>

        <form onSubmit={handleSave} className="space-y-6">
          {/* Identity */}
          <section className="rounded-2xl border border-ink-200 bg-white p-6 shadow-card">
            <h2 className="font-semibold text-ink-900 mb-4">Identity</h2>

            <div className="space-y-4">
              <Input
                label="Stall name"
                name="name"
                value={form.name}
                onChange={handleChange}
                required
              />

              <div>
                <label className="text-sm font-medium text-ink-700">
                  Stall address
                </label>
                <p className="mt-1 px-3 py-2 bg-ink-50 border border-ink-200 rounded-lg text-sm text-ink-500">
                  /{store.subdomain}
                </p>
                <p className="mt-1 text-xs text-ink-400">
                  Fixed when the stall was opened — contact support to change
                  it, since existing links depend on it.
                </p>
              </div>

              <div>
                <Input
                  label="Tagline"
                  name="tagline"
                  value={form.tagline}
                  onChange={handleChange}
                  maxLength={120}
                  placeholder="Hand-thrown stoneware, made in Kandy"
                />
                <p className="mt-1 text-xs text-ink-400">
                  One line, shown on your card in the directory.{" "}
                  {120 - form.tagline.length} characters left.
                </p>
              </div>

              <div className="flex flex-col gap-1">
                <label
                  htmlFor="description"
                  className="text-sm font-medium text-ink-700"
                >
                  About your stall
                </label>
                <textarea
                  id="description"
                  name="description"
                  value={form.description}
                  onChange={handleChange}
                  rows={4}
                  placeholder="What you make, how you make it, anything a shopper should know."
                  className="w-full px-3 py-2 border border-ink-300 rounded-lg text-sm
                             focus:outline-none focus:ring-2 focus:ring-brand-500
                             focus:border-transparent resize-none"
                />
              </div>
            </div>
          </section>

          {/* Listing */}
          <section className="rounded-2xl border border-ink-200 bg-white p-6 shadow-card">
            <h2 className="font-semibold text-ink-900 mb-1">
              Directory listing
            </h2>
            <p className="text-sm text-ink-500 mb-4">
              Shoppers filter by category, so a stall without one is much
              harder to find.
            </p>

            <div className="space-y-4">
              <div className="flex flex-col gap-1">
                <label
                  htmlFor="category"
                  className="text-sm font-medium text-ink-700"
                >
                  Category
                </label>
                <select
                  id="category"
                  name="category"
                  value={form.category}
                  onChange={handleChange}
                  className="w-full px-3 py-2 border border-ink-300 rounded-lg text-sm bg-white
                             focus:outline-none focus:ring-2 focus:ring-brand-500
                             focus:border-transparent"
                >
                  <option value="">No category</option>
                  {STALL_CATEGORIES.map((category) => (
                    <option key={category} value={category}>
                      {category}
                    </option>
                  ))}
                  {/* Keep a category set before this list existed selectable */}
                  {form.category &&
                    !STALL_CATEGORIES.includes(form.category as any) && (
                      <option value={form.category}>{form.category}</option>
                    )}
                </select>
              </div>

              <Input
                label="Location"
                name="location"
                value={form.location}
                onChange={handleChange}
                placeholder="Colombo, Sri Lanka"
              />

              {/* Logo */}
              <div>
                <label className="text-sm font-medium text-ink-700 block mb-2">
                  Stall logo
                </label>
                <div className="flex items-center gap-4">
                  {form.logoUrl ? (
                    <img
                      src={form.logoUrl}
                      alt="Stall logo"
                      className="w-16 h-16 rounded-xl object-cover border border-ink-200"
                    />
                  ) : (
                    <div
                      className="w-16 h-16 rounded-xl bg-brand-50 text-brand-700
                                 flex items-center justify-center"
                    >
                      <StoreIcon size={22} />
                    </div>
                  )}

                  <div className="flex items-center gap-2">
                    <input
                      ref={fileInput}
                      type="file"
                      accept="image/*"
                      onChange={handleLogoUpload}
                      className="hidden"
                      id="logo-upload"
                    />
                    <label
                      htmlFor="logo-upload"
                      className="inline-flex items-center gap-1.5 px-3.5 py-2 border border-ink-300
                                 rounded-lg text-sm font-medium text-ink-700 hover:border-ink-400
                                 transition-colors cursor-pointer"
                    >
                      {uploading ? (
                        <>
                          <Loader2 size={15} className="animate-spin" />
                          Uploading…
                        </>
                      ) : (
                        <>
                          <ImagePlus size={15} />
                          {form.logoUrl ? "Replace" : "Upload"}
                        </>
                      )}
                    </label>

                    {form.logoUrl && (
                      <button
                        type="button"
                        onClick={() =>
                          setForm((prev) => ({ ...prev, logoUrl: "" }))
                        }
                        className="inline-flex items-center gap-1.5 px-3 py-2 text-sm
                                   text-ink-500 hover:text-red-600 transition-colors"
                      >
                        <Trash2 size={15} />
                        Remove
                      </button>
                    )}
                  </div>
                </div>
              </div>
            </div>
          </section>

          <div className="flex items-center gap-3">
            <button
              type="submit"
              disabled={saving}
              className="px-5 py-2.5 bg-brand-600 hover:bg-brand-700 text-white text-sm
                         font-semibold rounded-lg transition-colors disabled:opacity-50
                         disabled:cursor-not-allowed"
            >
              {saving ? "Saving…" : "Save changes"}
            </button>
            <Link
              to="/admin/products"
              className="text-sm text-ink-500 hover:text-ink-900 transition-colors"
            >
              Back to products
            </Link>
          </div>
        </form>
      </div>
    </>
  );
}
