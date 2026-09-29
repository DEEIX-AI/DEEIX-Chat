// Writes SHA256SUMS for every asset of a draft release and uploads it.
// Runs after rename-release-assets.mjs, so the file lists final names. The
// format is GNU coreutils' (`sha256sum -c SHA256SUMS`, or on Windows compare
// with `Get-FileHash <file> -Algorithm SHA256`). The assets are downloaded and
// hashed here rather than trusting upload-side values.
//
// Usage: release-checksums.mjs <tag> [--dry-run]   (GH_TOKEN and GH_REPO required)

import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { createReadStream, mkdtempSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { pathToFileURL } from "node:url";

export const CHECKSUMS_FILE = "SHA256SUMS";

const gh = (...args) => execFileSync("gh", args, { encoding: "utf8" });

export function hashFile(path) {
  return new Promise((resolve, reject) => {
    const hash = createHash("sha256");
    createReadStream(path)
      .on("error", reject)
      .on("data", (chunk) => hash.update(chunk))
      .on("end", () => resolve(hash.digest("hex")));
  });
}

/** `<hex>  <name>` lines sorted by name; the checksums file never lists itself. */
export function formatChecksums(entries) {
  const lines = entries
    .filter(({ name }) => name !== CHECKSUMS_FILE)
    .map(({ name, sha256 }) => {
      if (!/^[0-9a-f]{64}$/.test(sha256)) {
        throw new Error(`${name}: not a SHA-256 digest`);
      }
      if (/[\r\n\\]/.test(name)) {
        throw new Error(`${name}: unsupported character in asset name`);
      }
      return { name, line: `${sha256}  ${name}` };
    })
    .sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0));
  return lines.map(({ line }) => `${line}\n`).join("");
}

async function main() {
  const [tag, ...flags] = process.argv.slice(2);
  const dryRun = flags.includes("--dry-run");
  const repo = process.env.GH_REPO;
  if (!tag || !repo) {
    console.error("usage: GH_REPO=owner/name release-checksums.mjs <tag> [--dry-run]");
    process.exit(1);
  }
  const release = JSON.parse(gh("release", "view", tag, "--repo", repo, "--json", "isDraft,assets"));
  if (!release.isDraft && !dryRun) {
    console.error(`${tag} is already published; refusing to change its assets`);
    process.exit(1);
  }
  const names = release.assets.map((asset) => asset.name).filter((name) => name !== CHECKSUMS_FILE);
  if (names.length === 0) {
    console.log("no assets to hash");
    return;
  }

  const work = mkdtempSync(join(tmpdir(), "release-checksums-"));
  try {
    const dir = join(work, "assets");
    const patterns = names.flatMap((name) => ["--pattern", name]);
    gh("release", "download", tag, "--repo", repo, "--dir", dir, ...patterns);
    const downloaded = new Set(readdirSync(dir));
    const missing = names.filter((name) => !downloaded.has(name));
    if (missing.length > 0) {
      throw new Error(`not downloaded: ${missing.join(", ")}`);
    }
    const entries = [];
    for (const name of names) {
      entries.push({ name, sha256: await hashFile(join(dir, name)) });
    }
    const text = formatChecksums(entries);
    process.stdout.write(text);
    if (dryRun) {
      return;
    }
    const file = join(work, CHECKSUMS_FILE);
    writeFileSync(file, text);
    gh("release", "upload", tag, file, "--repo", repo, "--clobber");
    console.log(`${CHECKSUMS_FILE} uploaded (${entries.length} assets)`);
  } finally {
    rmSync(work, { recursive: true, force: true });
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((error) => {
    console.error(error.message);
    process.exit(1);
  });
}
