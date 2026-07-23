import { rm, writeFile } from "node:fs/promises";

// Vite's optimised dependency chunks are tied to the exact frontend lockfile.
// Clear only generated caches after dependency installation so a Wails dev
// window cannot load stale chunk URLs while Vite is rebuilding them.
await Promise.all(
  ["node_modules/.vite", "node_modules/.vite-temp"].map((path) =>
    rm(path, { recursive: true, force: true })
  )
);
await writeFile("node_modules/.wails-deps.stamp", "", "utf8");
