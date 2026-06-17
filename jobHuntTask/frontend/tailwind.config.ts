import type { Config } from "tailwindcss";

const config: Config = {
  darkMode: "class",
  content: ["./app/**/*.{ts,tsx}", "./components/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        background: "hsl(240 6% 6%)",
        foreground: "hsl(0 0% 96%)",
        card: "hsl(240 5% 10%)",
        border: "hsl(240 4% 16%)",
        muted: "hsl(240 4% 46%)",
        accent: "hsl(221 83% 53%)",
        success: "hsl(142 71% 45%)",
        danger: "hsl(0 72% 51%)",
      },
      fontFamily: {
        sans: ["var(--font-geist-sans)", "system-ui", "sans-serif"],
      },
    },
  },
  plugins: [],
};

export default config;
