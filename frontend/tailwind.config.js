// Ownstall palette
//
// A stall is a market pitch: the teal is the canopy, the amber is the awning
// stripe. Teal carries every primary action, amber is reserved for highlights
// and "open for business" moments so it never competes with the primary.
//
// `gray` is deliberately overridden rather than extended: the app already
// styles almost everything with gray-*, so redefining the scale re-themes the
// existing pages to one cool neutral instead of leaving them on Tailwind's
// default warm gray next to a cool brand colour.
//
// Keep this file and mobile/src/theme.ts in step — they are the same palette
// for two runtimes.

const brand = {
  50: "#f0fdfa",
  100: "#ccfbf1",
  200: "#99f6e4",
  300: "#5eead4",
  400: "#2dd4bf",
  500: "#14b8a6",
  600: "#0d9488",
  700: "#0f766e",
  800: "#115e59",
  900: "#134e4a",
  950: "#042f2e",
};

const accent = {
  50: "#fffbeb",
  100: "#fef3c7",
  200: "#fde68a",
  300: "#fcd34d",
  400: "#fbbf24",
  500: "#f59e0b",
  600: "#d97706",
  700: "#b45309",
  800: "#92400e",
  900: "#78350f",
};

const ink = {
  50: "#f8fafc",
  100: "#f1f5f9",
  200: "#e2e8f0",
  300: "#cbd5e1",
  400: "#94a3b8",
  500: "#64748b",
  600: "#475569",
  700: "#334155",
  800: "#1e293b",
  900: "#0f172a",
  950: "#020617",
};

module.exports = {
  content: ["./src/**/*.{js,jsx,ts,tsx}"],
  theme: {
    extend: {
      colors: {
        brand,
        accent,
        ink,
        gray: ink,
      },
      fontFamily: {
        sans: [
          "Inter",
          "-apple-system",
          "BlinkMacSystemFont",
          "Segoe UI",
          "Roboto",
          "Helvetica Neue",
          "sans-serif",
        ],
      },
      boxShadow: {
        card: "0 1px 2px 0 rgb(15 23 42 / 0.04), 0 1px 3px 0 rgb(15 23 42 / 0.06)",
        lift: "0 10px 30px -12px rgb(15 23 42 / 0.18)",
      },
      backgroundImage: {
        "brand-gradient": "linear-gradient(135deg, #0f766e 0%, #14b8a6 100%)",
      },
    },
  },
  plugins: [],
};
