import { useState } from "react";
import { Link } from "react-router-dom";
import { LifeBuoy, Mail, MapPin, Store } from "lucide-react";

const SUPPORT_EMAIL = "hello@ownstall.app";
const SELLER_EMAIL = "sellers@ownstall.app";

const CHANNELS = [
  {
    icon: LifeBuoy,
    title: "Help with an order",
    desc: "Order not arrived, wrong item, refund question — start here and include your order reference.",
    action: SUPPORT_EMAIL,
  },
  {
    icon: Store,
    title: "Selling on Ownstall",
    desc: "Questions about opening a stall, the approval review, payouts or account restrictions.",
    action: SELLER_EMAIL,
  },
];

export function ContactPage() {
  const [form, setForm] = useState({ name: "", subject: "", message: "" });

  const handleChange = (
    e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>,
  ) => {
    const { name, value } = e.target;
    setForm((prev) => ({ ...prev, [name]: value }));
  };

  // There is no public inbound-message endpoint, and a form that quietly
  // discards what you typed is worse than no form — so this composes the
  // message in your own mail client instead of pretending to send it.
  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const subject = encodeURIComponent(form.subject || "Ownstall enquiry");
    const body = encodeURIComponent(
      `${form.message}\n\n— ${form.name || "Ownstall visitor"}`,
    );
    window.location.href = `mailto:${SUPPORT_EMAIL}?subject=${subject}&body=${body}`;
  };

  return (
    <>
      <section className="border-b border-ink-200 bg-ink-50">
        <div className="max-w-3xl mx-auto px-4 py-16">
          <p className="text-sm font-semibold text-brand-700 mb-2">Contact</p>
          <h1 className="text-4xl font-bold text-ink-900 tracking-tight mb-4">
            Get in touch.
          </h1>
          <p className="text-lg text-ink-500">
            Whether you're buying, selling or just curious — here's how to reach
            us.
          </p>
        </div>
      </section>

      <section className="max-w-3xl mx-auto px-4 py-16">
        <div className="grid gap-4 sm:grid-cols-2 mb-12">
          {CHANNELS.map((channel) => {
            const Icon = channel.icon;
            return (
              <div
                key={channel.title}
                className="rounded-2xl border border-ink-200 bg-white p-6 shadow-card"
              >
                <div
                  className="w-10 h-10 bg-brand-50 text-brand-700 rounded-xl
                             flex items-center justify-center mb-4"
                >
                  <Icon size={20} />
                </div>
                <h2 className="font-semibold text-ink-900 mb-1">
                  {channel.title}
                </h2>
                <p className="text-sm text-ink-500 mb-3">{channel.desc}</p>
                <a
                  href={`mailto:${channel.action}`}
                  className="text-sm font-medium text-brand-700 hover:underline"
                >
                  {channel.action}
                </a>
              </div>
            );
          })}
        </div>

        {/* Compose form */}
        <div className="rounded-2xl border border-ink-200 bg-white p-8 shadow-card">
          <h2 className="text-xl font-bold text-ink-900 mb-1">
            Send us a message
          </h2>
          <p className="text-sm text-ink-500 mb-6">
            This opens your email app with the message ready to send.
          </p>

          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <div className="flex flex-col gap-1">
              <label
                htmlFor="contact-name"
                className="text-sm font-medium text-ink-700"
              >
                Your name
              </label>
              <input
                id="contact-name"
                name="name"
                value={form.name}
                onChange={handleChange}
                className="w-full px-3 py-2 border border-ink-300 rounded-lg text-sm
                           focus:outline-none focus:ring-2 focus:ring-brand-500
                           focus:border-transparent"
              />
            </div>

            <div className="flex flex-col gap-1">
              <label
                htmlFor="contact-subject"
                className="text-sm font-medium text-ink-700"
              >
                Subject
              </label>
              <input
                id="contact-subject"
                name="subject"
                value={form.subject}
                onChange={handleChange}
                placeholder="What is this about?"
                className="w-full px-3 py-2 border border-ink-300 rounded-lg text-sm
                           focus:outline-none focus:ring-2 focus:ring-brand-500
                           focus:border-transparent"
              />
            </div>

            <div className="flex flex-col gap-1">
              <label
                htmlFor="contact-message"
                className="text-sm font-medium text-ink-700"
              >
                Message
              </label>
              <textarea
                id="contact-message"
                name="message"
                value={form.message}
                onChange={handleChange}
                rows={5}
                required
                className="w-full px-3 py-2 border border-ink-300 rounded-lg text-sm
                           focus:outline-none focus:ring-2 focus:ring-brand-500
                           focus:border-transparent resize-none"
              />
            </div>

            <button
              type="submit"
              className="self-start inline-flex items-center gap-2 px-5 py-2.5 bg-brand-600
                         hover:bg-brand-700 text-white text-sm font-semibold rounded-lg
                         transition-colors"
            >
              <Mail size={16} />
              Compose email
            </button>
          </form>
        </div>

        <div className="mt-10 flex items-start gap-2 text-sm text-ink-500">
          <MapPin size={16} className="mt-0.5 flex-shrink-0" />
          <p>
            Ownstall — Colombo, Sri Lanka.
            <br />
            Looking for the seller terms? They're on the{" "}
            <Link to="/terms" className="text-brand-700 hover:underline">
              terms page
            </Link>
            .
          </p>
        </div>
      </section>
    </>
  );
}
