import process from "node:process";
import { setTimeout as delay } from "node:timers/promises";

const DEFAULT_TIMEOUT_MS = 20_000;
const PROBE_TIMEOUT_MS = 800;
const RETRY_DELAY_MS = 75;

function option(name, fallback) {
  const index = process.argv.indexOf(name);
  if (index < 0 || index + 1 >= process.argv.length) {
    return fallback;
  }
  return process.argv[index + 1];
}

function probeHost(host) {
  const normalized = host.trim();
  if (normalized === "0.0.0.0") return "127.0.0.1";
  if (normalized === "::" || normalized === "[::]") return "[::1]";
  if (normalized.includes(":") && !normalized.startsWith("[")) {
    return `[${normalized}]`;
  }
  return normalized;
}

async function fetchReady(url) {
  const response = await fetch(url, {
    cache: "no-store",
    signal: AbortSignal.timeout(PROBE_TIMEOUT_MS),
  });
  if (!response.ok) {
    throw new Error(`${url} returned HTTP ${response.status}`);
  }
  return response;
}

async function main() {
  const host = probeHost(option("--host", "127.0.0.1"));
  const port = option("--port", "9245");
  const timeoutMs = Number(option("--timeout-ms", String(DEFAULT_TIMEOUT_MS)));
  const origin = `http://${host}:${port}`;
  const deadline = Date.now() + timeoutMs;
  let lastError;

  while (Date.now() < deadline) {
    try {
      const index = await fetchReady(`${origin}/`);
      const html = await index.text();
      if (!html.includes('/src/main.tsx')) {
        throw new Error("development index is missing the XiaDown entry module");
      }
      // Wait for the same default-Library files listed in Vite's warmup config.
      // Fetching them here joins any in-flight warmup transform and prevents
      // Wails from opening its WebView into a cold source-transform waterfall.
      await Promise.all([
        fetchReady(`${origin}/src/main.tsx`),
        fetchReady(`${origin}/src/index.css`),
        fetchReady(`${origin}/src/App.tsx`),
        fetchReady(`${origin}/src/app/providers/AppProviders.tsx`),
        fetchReady(`${origin}/src/app/main/MainApp.tsx`),
        fetchReady(`${origin}/src/app/library/LibraryWorkspacePage.tsx`),
        fetchReady(`${origin}/appicon_startup.png`),
      ]);
      process.stdout.write(`Frontend ready at ${origin}\n`);
      return;
    } catch (error) {
      lastError = error;
      await delay(RETRY_DELAY_MS);
    }
  }

  throw new Error(
    `Frontend did not become ready at ${origin} within ${timeoutMs}ms`,
    { cause: lastError },
  );
}

await main();
