import { readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const args = process.argv.slice(2);
const checkOnly = args.includes("--check");
const mode = args.find((arg) => !arg.startsWith("--")) ?? "all";

// Low-level manifest synchronization, also called by the API generator.
// `make version` completes it by regenerating Swagger and TypeScript contracts.
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

    // Keep the Rust root package's lockfile entry in sync for --locked builds.
    syncTauriConfig("apps", "desktop", "src-tauri", "tauri.conf.json");
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
  // Git may check text files out with CRLF (Windows, `text=auto`); keep the
  // file's own line endings so only the content is compared.
  const eol = current.includes("\r\n") ? "\r\n" : "\n";
  const next = nextContent.replace(/\r?\n/gu, eol);
  if (current === next) {
    return;
  }
  mismatches.push(filePath);
  if (!checkOnly) {
    writeFileSync(filePath, next);
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

/// Sync the top-level version of a JSON manifest such as tauri.conf.json.
function syncTauriConfig(...pathSegments) {
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
  const packageName = /^name = "([^"]+)"/mu.exec(content.slice(packageSection))?.[1];
  if (packageSection === -1 || !versionLine || !packageName) {
    throw new Error(`Unable to update ${manifestFile} package version`);
  }
  const start = packageSection + versionLine.index;
  const next = `${content.slice(0, start)}version = "${version}"${content.slice(start + versionLine[0].length)}`;
  writeIfChanged(manifestFile, next);
  syncCargoLockVersion(join(dirname(manifestFile), "Cargo.lock"), packageName);
}

function syncCargoLockVersion(lockFile, packageName) {
  const content = readFileSync(lockFile, "utf8");
  let matches = 0;
  const next = content.split(/(?=^\[\[package\]\])/mu).map((entry) => {
    const name = /^name = "([^"]+)"/mu.exec(entry)?.[1];
    // A registry or git dependency may share the root package's name. Its
    // version and every dependency checksum must remain pinned as before.
    if (name !== packageName || /^source\s*=/mu.test(entry)) return entry;
    matches++;
    return replaceOrThrow(entry, /^version = "[^"]+"/mu, `version = "${version}"`, `${lockFile} root package version`);
  }).join("");
  if (matches !== 1) throw new Error(`Expected one local ${packageName} package in ${lockFile}`);
  writeIfChanged(lockFile, next);
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
  const fixCommand = mode === "all" || mode === "backend" ? "pnpm api:generate" : `node scripts/sync-version.mjs ${mode}`;
  console.error(`Run: ${fixCommand}`);
  process.exit(1);
}
