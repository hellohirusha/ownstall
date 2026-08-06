// Ownstall palette for React Native.
//
// This is the same palette as frontend/tailwind.config.js — teal canopy,
// amber awning, cool slate ink. Keep the two in step; the web config is the
// source of truth and this file is its hand-mirrored copy because Tailwind
// does not run in this runtime.

export const colors = {
  brand: {
    50: "#f0fdfa",
    100: "#ccfbf1",
    300: "#5eead4",
    400: "#2dd4bf",
    500: "#14b8a6",
    600: "#0d9488",
    700: "#0f766e",
    900: "#134e4a",
  },
  accent: {
    100: "#fef3c7",
    300: "#fcd34d",
    400: "#fbbf24",
    500: "#f59e0b",
    600: "#d97706",
    900: "#78350f",
  },
  ink: {
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
  },
  white: "#ffffff",

  // Semantic colours stay independent of the brand: a "paid" badge must not
  // become brand-coloured just because the brand happens to be green-ish.
  success: {
    bg: "#dcfce7",
    fg: "#15803d",
    solid: "#16a34a",
  },
  warning: {
    bg: "#fef3c7",
    fg: "#92400e",
    solid: "#f59e0b",
  },
  danger: {
    bg: "#fee2e2",
    fg: "#b91c1c",
    solid: "#dc2626",
  },
  info: {
    bg: "#dbeafe",
    fg: "#1d4ed8",
    solid: "#2563eb",
  },
} as const;

export const radius = {
  sm: 8,
  md: 12,
  lg: 16,
  pill: 999,
} as const;

export const spacing = {
  xs: 4,
  sm: 8,
  md: 16,
  lg: 24,
  xl: 32,
} as const;
