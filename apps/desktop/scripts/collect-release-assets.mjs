// Stage one target's verified files under their final release names. Publication
// consumes these artifacts, so no platform uploads or edits latest.json itself.
import { copyFileSync, existsSync, mkdirSync, readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

import { msiVersion } from "./release-config.mjs";

const specifications = {
  "macos-arm64": [
    ["dmg", ".dmg", ".dmg"],
    ["macos", ".app.tar.gz", "-updater.tar.gz", ["darwin-aarch64", "darwin-aarch64-app"]],
  ],
  "macos-x64": [
    ["dmg", ".dmg", ".dmg"],
    ["macos", ".app.tar.gz", "-updater.tar.gz", ["darwin-x86_64", "darwin-x86_64-app"]],
  ],
  "linux-x64": [
    ["deb", ".deb", ".deb", ["linux-x86_64-deb"]],
    ["appimage", ".AppImage", ".AppImage", ["linux-x86_64", "linux-x86_64-appimage"]],
  ],
  "windows-x64": [
    ["nsis", "-setup.exe", "-setup.exe", ["windows-x86_64", "windows-x86_64-nsis"]],
    ["msi", ".msi", ".msi", ["windows-x86_64-msi"]],
    ["portable", "-portable.zip", "-portable.zip"],
    ["portable", "-portable-offline.zip", "-portable-offline.zip", [], true],
  ],
};

export const releasePlatforms = Object.keys(specifications);

export function releaseAssets(platform, version) {
  msiVersion(version);
  const specs = specifications[platform];
  if (!specs) throw new Error(`unsupported release platform: ${platform}`);
  return specs.map(([directory, extension, suffix, updater = [], optional = false]) => ({
    directory, extension, updater, optional,
    name: `DEEIX-Chat-${version}-${platform}${suffix}`,
  }));
}

export function requireFile(path) {
  if (!existsSync(path) || !statSync(path).isFile() || statSync(path).size === 0) {
    throw new Error(`missing or empty release asset: ${path}`);
  }
}

export function collectReleaseAssets({ platform, version, bundle, out }) {
  const plan = [];
  for (const spec of releaseAssets(platform, version)) {
    const directory = join(bundle, spec.directory);
    const matches = existsSync(directory)
      ? readdirSync(directory).filter((name) => name.endsWith(spec.extension)) : [];
    if (spec.optional && matches.length === 0) continue;
    if (matches.length !== 1) throw new Error(`expected one ${spec.extension} in ${directory}, found ${matches.length}`);
    const source = matches[0];
    // The updater archive is the only Tauri filename without a version.
    if (spec.extension !== ".app.tar.gz" && source !== spec.name && !source.includes(`_${version}_`)) {
      throw new Error(`asset ${source} does not match version ${version}`);
    }
    plan.push({ from: join(directory, source), to: join(out, spec.name) });
    if (spec.updater.length) plan.push({ from: join(directory, `${source}.sig`), to: join(out, `${spec.name}.sig`) });
  }
  // Validate the whole target before copying any of it.
  for (const { from, to } of plan) {
    requireFile(from);
    if (existsSync(to)) throw new Error(`release asset already staged: ${to}`);
  }
  mkdirSync(out, { recursive: true });
  for (const { from, to } of plan) copyFileSync(from, to);
  return plan.map(({ to }) => to);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  try {
    const args = process.argv.slice(2);
    const value = (flag) => args[args.indexOf(flag) + 1];
    for (const flag of ["--platform", "--bundle", "--out"]) {
      if (!args.includes(flag) || !value(flag)) throw new Error(`missing ${flag}`);
    }
    const here = dirname(fileURLToPath(import.meta.url));
    const version = readFileSync(resolve(here, "../../../VERSION"), "utf8").trim();
    const files = collectReleaseAssets({ platform: value("--platform"), version, bundle: value("--bundle"), out: value("--out") });
    console.log(files.join("\n"));
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
