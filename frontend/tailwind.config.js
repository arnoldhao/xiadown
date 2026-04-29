/** @type {import('tailwindcss').Config} */
import animate from "tailwindcss-animate";

export default {
  darkMode: ["class"],
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
    "./node_modules/streamdown/dist/**/*.{js,mjs}",
    "./node_modules/@assistant-ui/react-streamdown/dist/**/*.{js,mjs}",
  ],
  theme: {
    fontSize: {
      "2xs": ["var(--app-font-size-2xs)", { lineHeight: "1.4" }],
      xs: ["var(--app-font-size-xs)", { lineHeight: "1.4" }],
      sm: ["var(--app-font-size-sm)", { lineHeight: "1.4" }],
      base: ["var(--app-font-size)", { lineHeight: "1.5" }],
      lg: ["calc(var(--app-font-size) + 2px)", { lineHeight: "1.4" }],
      xl: ["calc(var(--app-font-size) + 4px)", { lineHeight: "1.35" }],
      "2xl": ["calc(var(--app-font-size) + 9px)", { lineHeight: "1.25" }],
      "3xl": ["calc(var(--app-font-size) + 15px)", { lineHeight: "1.15" }],
      "4xl": ["calc(var(--app-font-size) + 21px)", { lineHeight: "1.1" }],
      "5xl": ["calc(var(--app-font-size) + 33px)", { lineHeight: "1" }],
      "6xl": ["calc(var(--app-font-size) + 45px)", { lineHeight: "1" }],
      "7xl": ["calc(var(--app-font-size) + 57px)", { lineHeight: "1" }],
      "8xl": ["calc(var(--app-font-size) + 81px)", { lineHeight: "1" }],
      "9xl": ["calc(var(--app-font-size) + 113px)", { lineHeight: "1" }],
    },
    extend: {
      fontFamily: {
        display: ["var(--app-font-display)"],
        body: ["var(--app-font-body)"],
        sans: ["var(--app-font-body)"],
        serif: ["var(--app-font-display)"],
      },

      colors: {
        border: "hsl(var(--border))",
        input: "hsl(var(--input))",
        ring: "hsl(var(--ring))",
        background: "hsl(var(--background))",
        foreground: "hsl(var(--foreground))",
        primary: {
          DEFAULT: "hsl(var(--primary))",
          foreground: "hsl(var(--primary-foreground))",
        },
        secondary: {
          DEFAULT: "hsl(var(--secondary))",
          foreground: "hsl(var(--secondary-foreground))",
        },
        destructive: {
          DEFAULT: "hsl(var(--destructive))",
          foreground: "hsl(var(--destructive-foreground))",
        },
        muted: {
          DEFAULT: "hsl(var(--muted))",
          foreground: "hsl(var(--muted-foreground))",
        },
        accent: {
          DEFAULT: "hsl(var(--accent))",
          foreground: "hsl(var(--accent-foreground))",
        },
        popover: {
          DEFAULT: "hsl(var(--popover))",
          foreground: "hsl(var(--popover-foreground))",
        },
        card: {
          DEFAULT: "hsl(var(--card))",
          foreground: "hsl(var(--card-foreground))",
        },
        sidebar: {
          DEFAULT: "hsl(var(--sidebar-background))",
          foreground: "hsl(var(--sidebar-foreground))",
          primary: "hsl(var(--sidebar-primary))",
          "primary-foreground": "hsl(var(--sidebar-primary-foreground))",
          accent: "hsl(var(--sidebar-accent))",
          "accent-foreground": "hsl(var(--sidebar-accent-foreground))",
          border: "hsl(var(--sidebar-border))",
          ring: "hsl(var(--sidebar-ring))",
        },
        light: {
          bg: "#F9FAFB",
          card: "#FFFFFF",
          border: "#E5E7EB",
          text: "#1F2937",
          muted: "#6B7280",
          accent: "#4F46E5",
        },
        dark: {
          bg: "#0F0F10",
          card: "#18181B",
          border: "#27272A",
          text: "#FAFAF9",
          muted: "#A1A1AA",
          accent: "#818CF8",
        },
      },
      borderRadius: {
        xl: "calc(var(--radius) + 4px)",
        lg: "var(--radius)",
        md: "calc(var(--radius) - 2px)",
        sm: "calc(var(--radius) - 4px)",
      },
      boxShadow: {
        soft: "0 2px 8px rgba(0, 0, 0, 0.04)",
        elevated: "0 4px 24px rgba(0, 0, 0, 0.08)",
      },
    },
  },
  plugins: [animate],
};
