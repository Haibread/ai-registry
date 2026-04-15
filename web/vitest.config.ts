import { defineConfig } from "vitest/config"
import react from "@vitejs/plugin-react"
import path from "path"

export default defineConfig({
  plugins: [react()],
  test: {
    // Use jsdom for component tests; override per-file with @vitest-environment
    environment: "jsdom",
    environmentOptions: {
      jsdom: {
        url: "http://localhost:3000",
      },
    },
    globals: true,
    setupFiles: ["./src/test/setup.ts"],
    include: ["src/**/*.test.{ts,tsx}", "*.test.ts"],
    coverage: {
      provider: "v8",
      reporter: ["text", "lcov"],
      include: ["src/**/*.{ts,tsx}"],
      exclude: [
        "src/test/**",
        "src/**/*.d.ts",
        "src/lib/schema.d.ts",
        // Public user pages are covered by Playwright e2e, not vitest.
        // Admin pages DO get unit tests (detail.test.tsx, list.test.tsx,
        // new.test.tsx, ...) and must be measured against the v0.2.2 DoD
        // floor of 80 % statements per admin page — hence the explicit
        // negation below.
        "src/pages/!(admin)/**",
        "src/pages/*.tsx",
      ],
    },
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
})
