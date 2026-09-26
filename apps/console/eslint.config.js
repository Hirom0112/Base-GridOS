import js from "@eslint/js";
import tseslint from "typescript-eslint";
import globals from "globals";

export default tseslint.config(
  {
    ignores: [
      "dist/**",
      ".tanstack/**",
      "src/api/gen/**",
      "src/routeTree.gen.ts",
      "test-results/**",
      "playwright-report/**",
    ],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    languageOptions: { globals: { ...globals.browser, ...globals.node } },
    rules: {
      "@typescript-eslint/no-explicit-any": "error",
      complexity: ["error", 18],
      "max-depth": ["error", 4],
      "max-lines": ["error", 500],
      "max-lines-per-function": ["error", 150],
    },
  },
);
