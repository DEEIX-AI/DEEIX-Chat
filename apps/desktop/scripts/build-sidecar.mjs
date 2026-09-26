// Builds the Go server as a Tauri sidecar.
//
// Tauri's `bundle.externalBin` expects `<name>-<rust-target-triple>` next to
// the config (e.g. binaries/deeix-chat-server-aarch64-apple-darwin) and strips
// the suffix when bundling. The server links SQLite through cgo, so it is
// always built natively on the platform that bundles it; CI runs this once per
// matrix leg. Pass --target <triple> when cross-building on macOS (x86_64 on an
// arm64 runner works because Apple ships both toolchains).
//
// Usage: node scripts/build-sidecar.mjs [--target <rust-triple>]

import { execFileSync } from "node:child_process";
import { mkdirSync, readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const desktopDir = resolve(here, "..");
const repoRoot = resolve(desktopDir, "..", "..");
const backendDir = join(repoRoot, "backend");
const outDir = join(desktopDir, "src-tauri", "binaries");

const args = process.argv.slice(2);
const targetIndex = args.indexOf("--target");
const triple = targetIndex >= 0 ? args[targetIndex + 1] : hostTriple();
if (!triple) {
  console.error("Unable to determine the target triple; pass --target <rust-triple>.");
  process.exit(1);
}

const { goos, goarch } = tripleToGo(triple);
const version = readFileSync(join(repoRoot, "VERSION"), "utf8").trim();
const commit = safeGit(["rev-parse", "--short", "HEAD"]) || "unknown";
const buildTime = new Date().toISOString();

const ext = goos === "windows" ? ".exe" : "";
const output = join(outDir, `deeix-chat-server-${triple}${ext}`);
mkdirSync(outDir, { recursive: true });

const module = "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/buildinfo";
const ldflags = [
  "-s",
  "-w",
  `-X ${module}.Version=${version}`,
  `-X ${module}.Commit=${commit}`,
  `-X ${module}.BuildTime=${buildTime}`,
].join(" ");

console.log(`Building sidecar ${triple} (GOOS=${goos} GOARCH=${goarch}) → ${output}`);

// Local mode only uses SQLite, the memory cache and local storage; the other
// drivers, the Swagger UI and gin's msgpack binding are compiled out.
const tags = "nopostgres,noredis,nos3,noswagger,nomsgpack";

execFileSync("go", ["build", "-trimpath", "-tags", tags, "-ldflags", ldflags, "-o", output, "./cmd/server"], {
  cwd: backendDir,
  stdio: "inherit",
  env: { ...process.env, GOOS: goos, GOARCH: goarch, CGO_ENABLED: "1" },
});

function hostTriple() {
  try {
    const out = execFileSync("rustc", ["-vV"], { encoding: "utf8" });
    return out.match(/^host:\s*(\S+)/m)?.[1] ?? "";
  } catch {
    return "";
  }
}

function tripleToGo(t) {
  const arch = t.startsWith("aarch64") ? "arm64" : t.startsWith("x86_64") ? "amd64" : null;
  const os = t.includes("apple-darwin") ? "darwin" : t.includes("windows") ? "windows" : t.includes("linux") ? "linux" : null;
  if (!arch || !os) {
    console.error(`Unsupported target triple: ${t}`);
    process.exit(1);
  }
  return { goos: os, goarch: arch };
}

function safeGit(gitArgs) {
  try {
    return execFileSync("git", gitArgs, { cwd: repoRoot, encoding: "utf8" }).trim();
  } catch {
    return "";
  }
}
