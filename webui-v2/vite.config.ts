import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";

// Isolated dev/preview ports so this never collides with the live daemon (:47274)
// or the legacy dashboard dev server.
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
  },
  server: {
    port: Number(process.env.AG_DEV_PORT) || 47280,
    strictPort: true,
    proxy: {
      // Proxy /api/* to the Grafel daemon during dev so the SPA can talk to a real dataset.
      // AG_API_TARGET lets you point at an isolated daemon (never the live :47274) for verification.
      "/api": { target: process.env.AG_API_TARGET || "http://localhost:47274", changeOrigin: false },
    },
  },
  preview: { port: 47281, strictPort: true },
});
