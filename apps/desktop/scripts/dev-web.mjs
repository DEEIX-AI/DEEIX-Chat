// Tauri `beforeDevCommand` for `tauri dev`: reuse a web dev server that is already
// listening on the dev URL (e.g. started with `pnpm dev:web`), otherwise start one.
// Next.js refuses to start a second dev server for the same app, which would make
// `tauri dev` fail whenever the web app is already running.

import { spawn } from "node:child_process";

const devUrl = "http://localhost:3000";
const probeTimeoutMs = 2000;

async function isServing(url) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), probeTimeoutMs);
  try {
    await fetch(url, { method: "HEAD", signal: controller.signal });
    return true;
  } catch {
    return false;
  } finally {
    clearTimeout(timer);
  }
}

if (await isServing(devUrl)) {
  console.log(`[desktop] Reusing the web dev server already running at ${devUrl}.`);
  // Stay alive like a real dev server would; Tauri stops this process on exit.
  const keepAlive = setInterval(() => {}, 1 << 30);
  for (const signal of ["SIGINT", "SIGTERM"]) {
    process.on(signal, () => {
      clearInterval(keepAlive);
      process.exit(0);
    });
  }
} else {
  const child = spawn("pnpm", ["--filter", "@deeix/web", "dev"], {
    stdio: "inherit",
    shell: process.platform === "win32",
  });
  for (const signal of ["SIGINT", "SIGTERM"]) {
    process.on(signal, () => child.kill(signal));
  }
  child.on("exit", (code, signal) => {
    process.exit(code ?? (signal ? 1 : 0));
  });
}
