import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./app/**/*.{ts,tsx}",
    "./components/**/*.{ts,tsx}",
    "./components/ui/**/*.{ts,tsx}",
  ],
  theme: {
    extend: {},
  },
};

export default config;
