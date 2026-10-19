// Build-time Tauri config for a release. CI writes it with --output; local
// callers can still use the printed `--config <json>` argument:
//
//   - beta builds poll the rolling desktop-beta manifest instead of
//     releases/latest (see desktop-release.yml);
//   - Windows builds sign with the imported certificate. tauri-cli reads the
//     thumbprint only from bundle.windows.certificateThumbprint (there is no
//     environment variable for it), so it is passed here;
//   - the MSI gets a numeric ProductVersion. Windows Installer only accepts
//     major.minor.build[.revision] with major/minor <= 255 and build/revision
//     <= 65535, so "0.4.4-beta.2" becomes 0.4.4.2 (the bundler itself rejects
//     a non-numeric prerelease). Windows Installer ignores the fourth field
//     when comparing versions and Tauri's MSI allows downgrades, so every MSI
//     replaces whatever is installed; the field only makes builds tell apart.
//
// bundle.windows.wix.upgradeCode in tauri.conf.json is the MSI product family.
// It equals Tauri's default for "DEEIX Chat" (the one 0.4.4-beta.1 shipped
// with) and must NEVER change: a different code installs side by side instead
// of upgrading, and breaks every GPO/Intune/SCCM assignment keyed on it.
//
// Usage: release-config.mjs --channel stable|beta [--repository owner/name]
//                           [--version x.y.z] [--thumbprint <sha1 hex>]
//                           [--prebuilt-frontend] [--defer-updater] [--output path]

import { readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const tauriConfigPath = join(here, "..", "src-tauri", "tauri.conf.json");

/** Windows Installer ProductVersion for an app version. */
export function msiVersion(version) {
  const match = /^(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?(?:\+[0-9A-Za-z.-]+)?$/.exec(version);
  if (!match) {
    throw new Error(`not a semantic version: ${version}`);
  }
  const [major, minor, patch] = match.slice(1, 4).map(Number);
  if (major > 255 || minor > 255 || patch > 65535) {
    throw new Error(`${version} exceeds the MSI limits (255.255.65535)`);
  }
  const core = `${major}.${minor}.${patch}`;
  if (!match[4]) {
    return core;
  }
  // Last numeric identifier of the prerelease: beta.2 -> 2, rc -> 0.
  const numbers = match[4].split(".").filter((part) => /^\d+$/.test(part));
  const revision = numbers.length > 0 ? Number(numbers[numbers.length - 1]) : 0;
  if (revision > 65535) {
    throw new Error(`${version}: prerelease number exceeds 65535`);
  }
  return `${core}.${revision}`;
}

/** The config object merged over tauri.conf.json for a release build. */
export function releaseConfig({ version, channel, repository, thumbprint, prebuiltFrontend = false, deferUpdater = false }) {
  if (channel !== "stable" && channel !== "beta") {
    throw new Error(`unknown channel: ${channel}`);
  }
  const config = { bundle: { windows: { wix: { version: msiVersion(version) } } } };
  if (prebuiltFrontend) {
    config.build = { beforeBuildCommand: "" };
  }
  // macOS archives must contain the final signed/notarized app. CI creates
  // and signs that archive after the external signing service has returned.
  if (deferUpdater) {
    config.bundle.createUpdaterArtifacts = false;
  }
  if (thumbprint) {
    if (!/^[0-9A-Fa-f]{40}$/.test(thumbprint)) {
      throw new Error("--thumbprint must be a 40-digit SHA-1 certificate thumbprint");
    }
    config.bundle.windows.certificateThumbprint = thumbprint.toUpperCase();
  }
  if (channel === "beta") {
    if (!/^[\w.-]+\/[\w.-]+$/.test(repository ?? "")) {
      throw new Error("beta builds need --repository owner/name");
    }
    config.plugins = {
      updater: { endpoints: [`https://github.com/${repository}/releases/download/desktop-beta/latest.json`] },
    };
  }
  return config;
}

function main() {
  const args = process.argv.slice(2);
  const value = (flag) => {
    const index = args.indexOf(flag);
    return index >= 0 ? args[index + 1] : undefined;
  };
  const version = value("--version") ?? JSON.parse(readFileSync(tauriConfigPath, "utf8")).version;
  const config = releaseConfig({
    version,
    channel: value("--channel"),
    repository: value("--repository"),
    thumbprint: value("--thumbprint"),
    prebuiltFrontend: args.includes("--prebuilt-frontend"),
    deferUpdater: args.includes("--defer-updater"),
  });
  const output = value("--output");
  if (output) {
    writeFileSync(output, `${JSON.stringify(config)}\n`);
  } else {
    process.stdout.write(`--config ${JSON.stringify(config)}\n`);
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  try {
    main();
  } catch (error) {
    console.error(error.message);
    process.exit(1);
  }
}
