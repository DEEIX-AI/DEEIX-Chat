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

function syncFrontend() {
  const packageFile = join(repoRoot, "frontend", "package.json");
  const serviceWorkerFile = join(repoRoot, "frontend", "public", "sw.js");
  const packageJson = JSON.parse(readFileSync(packageFile, "utf8"));
  packageJson.version = version;
  writeIfChanged(packageFile, `${JSON.stringify(packageJson, null, 2)}\n`);

  writeIfChanged(
    serviceWorkerFile,
    replaceOrThrow(
      readFileSync(serviceWorkerFile, "utf8"),
      /const PWA_ASSET_VERSION = "[^"]+";/u,
      `const PWA_ASSET_VERSION = ${JSON.stringify(version)};`,
      "frontend service worker PWA asset version",
    ),
  );
}

function syncBackend() {
  const mainFile = join(repoRoot, "backend", "cmd", "server", "main.go");
  const docsFile = join(repoRoot, "backend", "docs", "docs.go");
  const swaggerJSONFile = join(repoRoot, "backend", "docs", "swagger.json");
  const swaggerYAMLFile = join(repoRoot, "backend", "docs", "swagger.yaml");

  writeIfChanged(
    mainFile,
    replaceOrThrow(
      readFileSync(mainFile, "utf8"),
      /\/\/ @version .+/u,
      `// @version ${version}`,
      "backend swagger annotation version",
    ),
  );

  writeIfChanged(
    docsFile,
    replaceOrThrow(
      readFileSync(docsFile, "utf8"),
      /Version:\s+"[^"]+"/u,
      `Version:          "${version}"`,
      "backend docs.go version",
    ),
  );

  const swaggerJSON = JSON.parse(readFileSync(swaggerJSONFile, "utf8"));
  swaggerJSON.info = swaggerJSON.info ?? {};
  swaggerJSON.info.version = version;
  writeIfChanged(swaggerJSONFile, `${JSON.stringify(swaggerJSON, null, 4)}\n`);

  writeIfChanged(
    swaggerYAMLFile,
    replaceOrThrow(
      readFileSync(swaggerYAMLFile, "utf8"),
      /^  version: .+$/mu,
      `  version: "${version}"`,
      "backend swagger.yaml version",
    ),
  );
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
  console.error(`Run: node scripts/sync-version.mjs ${mode}`);
  process.exit(1);
}
