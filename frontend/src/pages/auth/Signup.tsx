import { useState } from "react";
import { useNavigate, Link } from "react-router-dom";
import { Input } from "../../components/ui/Input";
import { saveSession } from "../../lib/session";
import { TERMS_VERSION } from "../legal/legalContent";

export function Signup() {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [agreed, setAgreed] = useState(false);

  const [form, setForm] = useState({
    store_name: "",
    subdomain: "",
    email: "",
    password: "",
  });

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    setForm((prev) => ({ ...prev, [name]: value }));
    // Auto-generate subdomain from store name
    if (name === "store_name") {
      setForm((prev) => ({
        ...prev,
        subdomain: value
          .toLowerCase()
          .replace(/[^a-z0-9]/g, "")
          .slice(0, 30),
      }));
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!agreed) {
      setError("You must accept the seller terms to open a stall");
      return;
    }

    setLoading(true);
    setError("");

    try {
      const res = await fetch(`${process.env.REACT_APP_API_URL}/api/signup`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(form),
      });

      const data = await res.json();

      if (!res.ok) {
        setError(data.error || "Signup failed");
        return;
      }

      saveSession("tenant", data);

      // Record the acceptance against the stall. The checkbox is the consent;
      // this is the durable evidence of which version was agreed to.
      try {
        await fetch(`${process.env.REACT_APP_GRAPHQL_URL}`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${data.access_token}`,
          },
          body: JSON.stringify({
            query: `mutation AcceptTerms($version: String!) {
              acceptTerms(version: $version)
            }`,
            variables: { version: TERMS_VERSION },
          }),
        });
      } catch {
        // The stall exists either way; a failed audit write must not strand
        // the seller on a form that already succeeded.
      }

      navigate("/admin/products");
    } catch {
      setError("Network error — please try again");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-gray-50 flex items-center justify-center p-4">
      <div className="w-full max-w-md">
        <div className="text-center mb-8">
          <h1 className="text-3xl font-bold text-gray-900">Ownstall</h1>
          <p className="text-gray-500 mt-2">Launch your store in minutes</p>
        </div>

        <div className="bg-white rounded-2xl shadow-sm border border-gray-100 p-8">
          <h2 className="text-xl font-semibold mb-6">Create your store</h2>

          {error && (
            <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-600">
              {error}
            </div>
          )}

          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <Input
              label="Store name"
              name="store_name"
              placeholder="Acme Stickers"
              value={form.store_name}
              onChange={handleChange}
              required
            />

            <div>
              <Input
                label="Store URL"
                name="subdomain"
                placeholder="acmestickers"
                value={form.subdomain}
                onChange={handleChange}
                required
              />
              <p className="mt-1 text-xs text-gray-400">
                Your store:{" "}
                <span className="font-medium">
                  {form.subdomain || "yourstore"}.ownstall.app
                </span>
              </p>
            </div>

            <Input
              label="Email"
              name="email"
              type="email"
              placeholder="you@example.com"
              value={form.email}
              onChange={handleChange}
              required
            />

            <Input
              label="Password"
              name="password"
              type="password"
              placeholder="8+ characters"
              value={form.password}
              onChange={handleChange}
              required
            />

            <label className="flex items-start gap-2.5 cursor-pointer">
              <input
                type="checkbox"
                checked={agreed}
                onChange={(e) => setAgreed(e.target.checked)}
                className="mt-0.5 w-4 h-4 rounded border-ink-300 text-brand-600
                           focus:ring-brand-500 cursor-pointer"
              />
              <span className="text-xs text-ink-500 leading-relaxed">
                I have read and agree to the{" "}
                <Link
                  to="/terms"
                  target="_blank"
                  className="text-brand-700 font-medium hover:underline"
                >
                  seller terms of service
                </Link>{" "}
                and the{" "}
                <Link
                  to="/privacy"
                  target="_blank"
                  className="text-brand-700 font-medium hover:underline"
                >
                  privacy policy
                </Link>
                . I understand my stall is reviewed before it goes public.
              </span>
            </label>

            <button
              type="submit"
              disabled={loading || !agreed}
              className="w-full py-2.5 bg-brand-600 hover:bg-brand-700 text-white font-medium
                         rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {loading ? "Creating your stall..." : "Open my stall"}
            </button>
          </form>

          <p className="text-center text-sm text-gray-500 mt-4">
            Already have a store?{" "}
            <Link
              to="/login"
              className="text-brand-600 font-medium hover:underline"
            >
              Sign in
            </Link>
          </p>
        </div>
      </div>
    </div>
  );
}
