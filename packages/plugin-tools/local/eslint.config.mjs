import { defineConfig } from "eslint/config";
import baseConfig from "./.config/eslint.config.mjs";

export default defineConfig([
  {
    ignores: ["**/node_modules", "**/build", "**/dist"],
  },
  ...baseConfig,
  {
    files: ["src/**/*.{ts,tsx}"],
    rules: {
      "@typescript-eslint/no-deprecated": "warn",
    },
  },
]);
