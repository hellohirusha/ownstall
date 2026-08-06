import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Input } from "../../components/ui/Input";
import { Logo } from "../../components/Logo";
import { saveSession } from "../../lib/session";

export function BuyerSignup() {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [form, setForm] = useState({
    first_name: "",
    last_name: "",
    email: "",
    password: "",
  });

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    setForm((prev) => ({ ...prev, [name]: value }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (form.password.length < 8) {
      setError("Password must be at least 8 characters");
      return;
    }

    setLoading(true);
    setError("");

    try {
      const res = await fetch(
        `${process.env.REACT_APP_API_URL}/api/buyer/signup`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(form),
        },
      );
      const data = await res.json();

      if (!res.ok) {
        setError(data.error || "Could not create your account");
        return;
      }

      saveSession("buyer", data);
      navigate("/account", { replace: true });
    } catch {
      setError("Network error — please try again");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-ink-50 flex items-center justify-center p-4">
      <div className="w-full max-w-md">
        <div className="text-center mb-8">
          <div className="flex justify-center">
            <Logo size={40} />
          </div>
          <p className="text-ink-500 mt-3">Create a shopper account</p>
        </div>

        <div className="bg-white rounded-2xl shadow-card border border-ink-200 p-8">
          <h1 className="text-xl font-semibold mb-1">Create your account</h1>
          <p className="text-sm text-ink-500 mb-6">
            Keeps your orders from every stall in one place.
          </p>

          {error && (
            <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-600">
              {error}
            </div>
          )}

          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <div className="grid grid-cols-2 gap-3">
              <Input
                label="First name"
                name="first_name"
                autoComplete="given-name"
                value={form.first_name}
                onChange={handleChange}
              />
              <Input
                label="Last name"
                name="last_name"
                autoComplete="family-name"
                value={form.last_name}
                onChange={handleChange}
              />
            </div>

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
              autoComplete="new-password"
              placeholder="8+ characters"
              value={form.password}
              onChange={handleChange}
              required
            />

            <p className="text-xs text-ink-400">
              By creating an account you agree to our{" "}
              <Link to="/terms" className="text-brand-700 hover:underline">
                terms of service
              </Link>{" "}
              and{" "}
              <Link to="/privacy" className="text-brand-700 hover:underline">
                privacy policy
              </Link>
              .
            </p>

            <button
              type="submit"
              disabled={loading}
              className="w-full py-2.5 bg-brand-600 hover:bg-brand-700 text-white font-medium
                         rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {loading ? "Creating account..." : "Create account"}
            </button>
          </form>

          <p className="text-center text-sm text-ink-500 mt-4">
            Already have one?{" "}
            <Link
              to="/account/login"
              className="text-brand-700 font-medium hover:underline"
            >
              Sign in
            </Link>
          </p>
        </div>

        <p className="text-center text-sm text-ink-500 mt-6">
          Want to sell instead?{" "}
          <Link to="/signup" className="text-brand-700 font-medium hover:underline">
            Open a stall
          </Link>
        </p>
      </div>
    </div>
  );
}
