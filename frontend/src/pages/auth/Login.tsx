import { useState } from "react";
import { useNavigate, useLocation, Link } from "react-router-dom";
import { Input } from "../../components/ui/Input";
import { Logo } from "../../components/Logo";
import { saveSession } from "../../lib/session";

export function Login() {
  const navigate = useNavigate();
  const location = useLocation();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const [form, setForm] = useState({ email: "", password: "" });

  // The guard stashes where the visitor was headed so a session that expired
  // mid-task drops them back on the same page instead of the dashboard root.
  const from =
    (location.state as { from?: string } | null)?.from ?? "/admin/products";

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    setForm((prev) => ({ ...prev, [name]: value }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError("");

    try {
      const res = await fetch(`${process.env.REACT_APP_API_URL}/api/login`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(form),
      });

      const data = await res.json();

      if (!res.ok) {
        setError(data.error || "Sign in failed");
        return;
      }

      saveSession("tenant", data);
      navigate(from, { replace: true });
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
          <div className="flex justify-center">
            <Logo size={40} />
          </div>
          <p className="text-ink-500 mt-3">Sign in to your stall</p>
        </div>

        <div className="bg-white rounded-2xl shadow-sm border border-gray-100 p-8">
          <h2 className="text-xl font-semibold mb-6">Store sign in</h2>

          {error && (
            <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-600">
              {error}
            </div>
          )}

          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <Input
              label="Email"
              name="email"
              type="email"
              autoComplete="email"
              placeholder="you@example.com"
              value={form.email}
              onChange={handleChange}
              required
            />

            <Input
              label="Password"
              name="password"
              type="password"
              autoComplete="current-password"
              placeholder="Your password"
              value={form.password}
              onChange={handleChange}
              required
            />

            <button
              type="submit"
              disabled={loading}
              className="w-full py-2.5 bg-brand-500 hover:bg-brand-600 text-white font-medium
                         rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {loading ? "Signing in..." : "Sign in"}
            </button>
          </form>

          <p className="text-center text-sm text-gray-500 mt-4">
            Don't have a stall yet?{" "}
            <Link
              to="/signup"
              className="text-brand-600 font-medium hover:underline"
            >
              Open one
            </Link>
          </p>
        </div>

        <p className="text-center text-sm text-gray-500 mt-6">
          Shopping instead?{" "}
          <Link
            to="/account/login"
            className="text-brand-600 font-medium hover:underline"
          >
            Buyer sign in
          </Link>
        </p>
      </div>
    </div>
  );
}
