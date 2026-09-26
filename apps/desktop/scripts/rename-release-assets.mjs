// Gives every release asset one platform-explicit name:
//   DEEIX-Chat-<version>-<os>-<arch>[-setup|-updater].<ext>
//   e.g. DEEIX.Chat_0.4.4-beta.1_aarch64.dmg -> DEEIX-Chat-0.4.4-beta.1-macos-arm64.dmg
//
// Tauri names assets after its bundler conventions, and the updater manifest
// references them by file name, so the manifest URLs are rewritten to match.
// Only the artifact bytes are signed (the manifest is fetched over TLS and is
// not itself signed), so renaming does not affect signature verification.
//
// Usage: rename-release-assets.mjs <tag> [--dry-run]   (GH_TOKEN and GH_REPO required)

import { execFileSync } from "node:child_process";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { pathToFileURL } from "node:url";

const PRODUCT = "DEEIX-Chat";

const gh = (...args) => execFileSync("gh", args, { encoding: "utf8" });

function archLabel(raw) {
  switch (raw) {
    case "aarch64":
    case "arm64":
      return "arm64";
    case "x64":
    case "x86_64":
    case "amd64":
      return "x64";
    default:
      return null;
  }
}

/**
 * Map a Tauri asset name to the convention above, or null to leave it alone.
 * Recognised shapes (Tauri 2), each optionally followed by ".sig":
 *   <product>_<version>_<arch>.dmg | .AppImage | .deb | .rpm
 *   <product>_<version>_<arch>-setup.exe
 *   <product>_<version>_<arch>_<locale>.msi
 *   <product>_<arch>.app.tar.gz              (macOS updater payload)
 */
export function renameAsset(name, version) {
  const sig = name.endsWith(".sig") ? ".sig" : "";
  const base = sig ? name.slice(0, -sig.length) : name;
  const stem = `${PRODUCT}-${version}`;

  const macUpdater = base.match(/_([A-Za-z0-9]+)\.app\.tar\.gz$/);
  if (macUpdater) {
    const arch = archLabel(macUpdater[1]);
    return arch ? `${stem}-macos-${arch}-updater.tar.gz${sig}` : null;
  }

  // Longest suffix first so "-setup.exe" wins over ".exe".
  const kinds = [
    ["-setup.exe", "windows", "-setup.exe"],
    [".AppImage", "linux", ".AppImage"],
    [".dmg", "macos", ".dmg"],
    [".deb", "linux", ".deb"],
    [".rpm", "linux", ".rpm"],
    [".msi", "windows", ".msi"],
    [".exe", "windows", "-setup.exe"],
  ];
  const kind = kinds.find(([suffix]) => base.toLowerCase().endsWith(suffix.toLowerCase()));
  if (!kind) {
    return null;
  }
  const [suffix, os, ext] = kind;
  // "<...>_<arch>" or "<...>_<arch>_<locale>" precedes the suffix.
  const head = base.slice(0, -suffix.length).replace(/_[A-Za-z]{2,3}(?:-[A-Za-z0-9]+)?$/, "");
  const arch = archLabel(head.slice(head.lastIndexOf("_") + 1));
  return arch ? `${stem}-${os}-${arch}${ext}${sig}` : null;
}

/** Replace every renamed file name in the manifest text, raw and URL-encoded. */
export function rewriteManifest(manifest, renames) {
  let out = manifest;
  for (const { from, to } of renames) {
    out = out.split(encodeURIComponent(from)).join(encodeURIComponent(to));
    out = out.split(from).join(to);
  }
  return out;
}

function main() {
  const [tag, ...flags] = process.argv.slice(2);
  const dryRun = flags.includes("--dry-run");
  const repo = process.env.GH_REPO;
  if (!tag || !repo) {
    console.error("usage: GH_REPO=owner/name rename-release-assets.mjs <tag> [--dry-run]");
    process.exit(1);
  }

  const release = JSON.parse(gh("release", "view", tag, "--repo", repo, "--json", "isDraft,assets"));
  if (!release.isDraft && !dryRun) {
    console.error(`${tag} is already published; refusing to rewrite its assets`);
    process.exit(1);
  }

  const version = tag.replace(/^v/, "");
  // `id` here is the GraphQL node id; the REST endpoint for an asset is `apiUrl`.
  const renames = release.assets
    .map((asset) => ({ apiUrl: asset.apiUrl, from: asset.name, to: renameAsset(asset.name, version) }))
    .filter(({ from, to }) => to && to !== from);
  if (renames.length === 0) {
    console.log("asset names already follow the convention");
    return;
  }
  for (const { apiUrl, from, to } of renames) {
    console.log(`${from} -> ${to}`);
    if (!dryRun) {
      gh("api", "-X", "PATCH", apiUrl, "-f", `name=${to}`);
    }
  }

  if (!release.assets.some((asset) => asset.name === "latest.json")) {
    console.log("no latest.json in this release; nothing to rewrite");
    return;
  }
  const manifest = gh("release", "download", tag, "--repo", repo, "--pattern", "latest.json", "--output", "-");
  const rewritten = rewriteManifest(manifest, renames);
  if (rewritten === manifest) {
    console.log("latest.json does not reference any renamed asset");
    return;
  }
  if (dryRun) {
    console.log("latest.json would be rewritten");
    return;
  }
  const file = join(mkdtempSync(join(tmpdir(), "release-assets-")), "latest.json");
  writeFileSync(file, rewritten);
  gh("release", "upload", tag, file, "--repo", repo, "--clobber");
  console.log("latest.json rewritten");
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main();
}
