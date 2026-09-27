import { tanstackStart } from "@tanstack/react-start/plugin/vite";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig, loadEnv } from "vite";

export default defineConfig(({ mode }) => {
  const env = { ...loadEnv(mode, process.cwd(), ""), ...process.env };
  return {
    cacheDir: env.GRIDOS_VITE_CACHE ?? "node_modules/.vite",
    define: {
      "import.meta.env.VITE_GRIDOS_AUTH_MODE": JSON.stringify(
        env.GRIDOS_AUTH_MODE ?? "clerk",
      ),
    },
    server: {
      proxy: {
        ...(env.GRIDOS_AUTH_MODE === "local"
          ? {
              "/local/step-up": {
                target: env.GRIDOS_IDENTITY_URL ?? "http://127.0.0.1:8080",
                changeOrigin: true,
                timeout: 10000,
                proxyTimeout: 10000,
              },
            }
          : {}),
        "/geo": {
          target: env.GRIDOS_API_URL ?? "http://127.0.0.1:8080",
          changeOrigin: true,
        },
        "/rpc": {
          target: env.GRIDOS_API_URL ?? "http://127.0.0.1:8080",
          changeOrigin: true,
          rewrite: (path) => path.replace(/^\/rpc/, ""),
        },
      },
    },
    plugins: [tailwindcss(), tanstackStart(), react()],
  };
});
