import { useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { ShieldCheck } from "lucide-react";
import { Input } from "../../components/ui/Input";
import { LogoMark } from "../../components/Logo";
import { saveSession } from "../../lib/session";

export function PlatformLogin() {
  const navigate = useNavigate();
  const location = useLocation();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [form, setForm] = useState({ email: "", password: "" });

  const from = (location.state as { from?: string } | null)?.from ?? "/platform";

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    setForm((prev) => ({ ...prev, [name]: value }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError("");

    try {
      const res = await fetch(
        `${process.env.REACT_APP_API_URL}/api/admin/login`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(form),
        },
      );
      const data = await res.json();

      if (!res.ok) {
        setError(data.error || "Sign in failed");
        return;
      }

      saveSession("admin", data);
      navigate(from, { replace: true });
    } catch {
      setError("Network error — please try again");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-ink-900 flex items-center justify-center p-4">
      <div className="w-full max-w-md">
        <div className="text-center mb-8">
          <div className="flex justify-center mb-3">
            <LogoMark size={44} />
          </div>
          <h1 className="text-xl font-bold text-white">Operator console</h1>
          <p className="text-ink-400 text-sm mt-1">
            Ownstall platform administration
          </p>
        </div>

        <div className="bg-white rounded-2xl shadow-lift p-8">
          <div className="flex items-center gap-2 mb-6 text-ink-500">
            <ShieldCheck size={18} className="text-brand-600" />
            <span className="text-sm font-medium">Restricted access</span>
          </div>

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
              value={form.email}
              onChange={handleChange}
              required
            />
            <Input
              label="Password"
              name="password"
              type="password"
              autoComplete="current-password"
              value={form.password}
              onChange={handleChange}
              required
            />
            <button
              type="submit"
              disabled={loading}
              className="w-full py-2.5 bg-ink-900 hover:bg-ink-800 text-white font-medium
                         rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {loading ? "Signing in..." : "Sign in"}
            </button>
          </form>
        </div>

        {/* Operator accounts are provisioned, never self-served — so there is
            deliberately no sign-up link here. */}
        <p className="text-center text-xs text-ink-500 mt-6">
          Operator accounts are created by an existing operator.
        </p>
      </div>
    </div>
  );
}
