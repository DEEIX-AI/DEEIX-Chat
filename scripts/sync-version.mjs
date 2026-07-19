import { readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const args = process.argv.slice(2);
const checkOnly = args.includes("--check");
const mode = args.find((arg) => !arg.startsWith("--")) ?? "all";
const validModes = new Set(["all", "frontend", "backend"]);

if (!validModes.has(mode)) {
  throw new Error(`Invalid sync-version mode: ${mode}`);
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

function syncFrontend() {
  syncPackageVersion("frontend");
}

function syncBackend() {
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
}

function syncWorkspace() {
  syncPackageVersion();
  syncPackageVersion("packages", "api-contract");
}

if (mode === "all") {
  syncWorkspace();
}

if (mode === "all" || mode === "frontend") {
  syncFrontend();
}

if (mode === "all" || mode === "backend") {
  syncBackend();
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
