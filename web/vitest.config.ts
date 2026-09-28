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
    // Default 5000ms is fine for hermetic tests but several admin form
    // tests legitimately chain multiple async waits (publishers fetch →
    // namespace Select interaction → mutation). They pass individually
    // but bump up against the 5s ceiling under parallel-suite load —
    // bump to 15s so CI variance doesn't manifest as flakes.
    testTimeout: 15_000,
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
      ],
      // Regression floor: the measured totals rounded down. Raise it when
      // coverage goes up; never lower it to make a change pass.
      thresholds: {
        lines: 84,
        statements: 81,
        functions: 81,
        branches: 75,
      },
    },
  },
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "./src"),
    },
  },
})
