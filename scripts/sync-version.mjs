import { readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const args = process.argv.slice(2);
const checkOnly = args.includes("--check");
const mode = args.find((arg) => !arg.startsWith("--")) ?? "all";

// Every versioned target in the workspace. Adding a new app (desktop, mobile)
// means adding one entry here; nothing else in the release pipeline changes.
const targets = {
  workspace: () => {
    syncPackageVersion();
    syncPackageVersion("packages", "api-contract");
    syncPackageVersion("packages", "core");
  },
  web: () => {
    syncPackageVersion("apps", "web");
  },
  desktop: () => {
    syncPackageVersion("apps", "desktop");

    // The Tauri manifest and the Rust crate carry their own version fields.
    syncJsonVersion("apps", "desktop", "src-tauri", "tauri.conf.json");
    syncCargoVersion("apps", "desktop", "src-tauri", "Cargo.toml");
  },
  backend: () => {
    const mainFile = join(repoRoot, "backend", "cmd", "server", "main.go");

    syncPackageVersion("backend");

    writeIfChanged(
      mainFile,
      replaceOrThrow(
        readFileSync(mainFile, "utf8"),
        /\/\/ @version .+/u,
        `// @version ${version}`,
        "backend swagger annotation version",
      ),
    );
  },
};

const validModes = new Set(["all", ...Object.keys(targets)]);

if (!validModes.has(mode)) {
  throw new Error(`Invalid sync-version mode: ${mode}. Expected one of: ${[...validModes].join(", ")}`);
}

const version = readFileSync(join(repoRoot, "VERSION"), "utf8").trim();

if (!/^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$/u.test(version)) {
  throw new Error(`Invalid VERSION value: ${version}`);
}

const mismatches = [];

function writeIfChanged(filePath, nextContent) {
  const current = readFileSync(filePath, "utf8");
  if (current === nextContent) {
    return;
  }
  mismatches.push(filePath);
  if (!checkOnly) {
    writeFileSync(filePath, nextContent);
  }
}

function replaceOrThrow(content, pattern, replacement, label) {
  if (!pattern.test(content)) {
    throw new Error(`Unable to update ${label}`);
  }
  return content.replace(pattern, replacement);
}

function syncPackageVersion(...pathSegments) {
  const packageFile = join(repoRoot, ...pathSegments, "package.json");
  const packageJson = JSON.parse(readFileSync(packageFile, "utf8"));
  packageJson.version = version;
  writeIfChanged(packageFile, `${JSON.stringify(packageJson, null, 2)}\n`);
}

/// 同步任意 JSON 清单里的顶层 version 字段（如 tauri.conf.json）。
function syncJsonVersion(...pathSegments) {
  const manifestFile = join(repoRoot, ...pathSegments);
  const manifest = JSON.parse(readFileSync(manifestFile, "utf8"));
  manifest.version = version;
  writeIfChanged(manifestFile, `${JSON.stringify(manifest, null, 2)}\n`);
}

/// 同步 Cargo.toml 的 [package] version 行；只替换该节内的第一处。
function syncCargoVersion(...pathSegments) {
  const manifestFile = join(repoRoot, ...pathSegments);
  const content = readFileSync(manifestFile, "utf8");
  const packageSection = content.indexOf("[package]");
  const versionLine = /^version = ".*"/mu.exec(content.slice(packageSection));
  if (packageSection === -1 || !versionLine) {
    throw new Error(`Unable to update ${manifestFile} package version`);
  }
  const start = packageSection + versionLine.index;
  const next = `${content.slice(0, start)}version = "${version}"${content.slice(start + versionLine[0].length)}`;
  writeIfChanged(manifestFile, next);
}

for (const [name, sync] of Object.entries(targets)) {
  if (mode === "all" || mode === name) {
    sync();
  }
}

if (checkOnly && mismatches.length > 0) {
  console.error(`VERSION is not synchronized with ${mismatches.length} file(s):`);
  for (const filePath of mismatches) {
    console.error(`- ${filePath}`);
  }
  const fixCommand = mode === "all" ? "pnpm api:generate" : `node scripts/sync-version.mjs ${mode}`;
  console.error(`Run: ${fixCommand}`);
  process.exit(1);
}
