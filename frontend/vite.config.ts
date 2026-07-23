import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import wails from "@wailsio/runtime/plugins/vite";
import { fileURLToPath, URL } from "node:url";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), wails("./bindings")],
  server: {
    // Pre-transform the default Library startup graph before Wails opens its
    // WebView. The dev launcher probes the same URLs, so a successful probe
    // means the expensive source transforms are already in Vite's cache.
    warmup: {
      clientFiles: [
        "./index.html",
        "./src/main.tsx",
        "./src/index.css",
        "./src/App.tsx",
        "./src/app/providers/AppProviders.tsx",
        "./src/app/main/MainApp.tsx",
        "./src/app/library/LibraryWorkspacePage.tsx",
      ],
    },
  },
  build: {
    // Keep the warning meaningful. Window-specific application roots are lazy
    // imports, so Rollup can now split their dependency graphs naturally
    // instead of forcing every dependency into one startup vendor chunk.
    chunkSizeWarningLimit: 750,
    rollupOptions: {
      output: {
        manualChunks(id) {
          const normalized = id.replace(/\\/g, "/");
          // Keep Vite's tiny dynamic-import helper out of whichever lazy
          // dependency Rollup happens to visit first. If it is absorbed by the
          // media chunk, the entry module statically imports and preloads the
          // entire player stack for every window.
          if (normalized.includes("vite/preload-helper")) return "runtime-preload";
          if (!normalized.includes("/node_modules/")) return undefined;
          if (/\/node_modules\/(react|react-dom|scheduler)\//.test(normalized)) return "framework-react";
          if (normalized.includes("/node_modules/@radix-ui/")) return "framework-radix";
          if (normalized.includes("/node_modules/lucide-react/")) return "vendor-icons";
          if (
            normalized.includes("/node_modules/@tanstack/") ||
            normalized.includes("/node_modules/zustand/")
          ) return "vendor-state";
          if (
            normalized.includes("/node_modules/react-markdown/") ||
            normalized.includes("/node_modules/remark-") ||
            normalized.includes("/node_modules/unified/") ||
            normalized.includes("/node_modules/rehype-") ||
            normalized.includes("/node_modules/mdast-") ||
            normalized.includes("/node_modules/hast-")
          ) return "vendor-markdown";
          if (
            normalized.includes("/node_modules/@vidstack/") ||
            normalized.includes("/node_modules/flv.js/")
          ) return "vendor-media";
          if (normalized.includes("/node_modules/@wailsio/")) return "vendor-runtime";
          if (normalized.includes("/node_modules/opencc-js/")) return "vendor-opencc";
          return undefined;
        },
      },
    },
  },
  worker: {
    format: "es",
  },
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
});
