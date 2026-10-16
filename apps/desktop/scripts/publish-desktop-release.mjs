// Assemble all four verified targets, then upload once into one draft. The
// updater keys and public filenames match the previous tauri-action releases.
import { execFileSync } from "node:child_process";
import { appendFileSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { pathToFileURL } from "node:url";

import { releaseAssets, releasePlatforms, requireFile } from "./collect-release-assets.mjs";
import { formatChecksums, hashFile } from "./release-checksums.mjs";

export function releaseManifest({ version, tag, repository, directory, date = new Date().toISOString() }) {
  if (tag !== `v${version}`) throw new Error(`Tag ${tag} does not match VERSION ${version}`);
  if (!/^[\w.-]+\/[\w.-]+$/.test(repository ?? "")) throw new Error("expected repository owner/name");
  const platforms = {};
  const files = [];
  const present = new Set(readdirSync(directory));
  for (const platform of releasePlatforms) {
    for (const spec of releaseAssets(platform, version)) {
      if (spec.optional && !present.has(spec.name)) continue;
      requireFile(join(directory, spec.name));
      files.push(spec.name);
      if (!spec.updater.length) continue;
      const signatureName = `${spec.name}.sig`;
      requireFile(join(directory, signatureName));
      const signature = readFileSync(join(directory, signatureName), "utf8").trim();
      if (!signature) throw new Error(`empty updater signature: ${signatureName}`);
      files.push(signatureName);
      const entry = {
        signature,
        url: `https://github.com/${repository}/releases/download/${encodeURIComponent(tag)}/${encodeURIComponent(spec.name)}`,
      };
      for (const key of spec.updater) platforms[key] = entry;
    }
  }
  const allowed = new Set([...files, "latest.json", "SHA256SUMS"]);
  for (const name of present) {
    if (!allowed.has(name)) throw new Error(`unexpected release asset: ${name}`);
  }
  return { manifest: { version, notes: "", pub_date: date, platforms }, files: files.sort() };
}

export async function assembleRelease(options) {
  const { manifest, files } = releaseManifest(options);
  writeFileSync(join(options.directory, "latest.json"), `${JSON.stringify(manifest, null, 2)}\n`);
  files.push("latest.json");
  const entries = await Promise.all(files.map(async (name) => ({ name, sha256: await hashFile(join(options.directory, name)) })));
  writeFileSync(join(options.directory, "SHA256SUMS"), formatChecksums(entries));
  return [...files, "SHA256SUMS"];
}

export function existingDraft(releases, tag) {
  const matches = releases.filter((release) => release.tag_name === tag);
  if (matches.length > 1) throw new Error(`${tag} has multiple releases; remove the duplicate drafts before retrying`);
  if (matches[0] && !matches[0].draft) throw new Error(`${tag} is already published; refusing to replace its assets`);
  return matches[0];
}

function releaseNotes(version) {
  const stem = `DEEIX-Chat-${version}`;
  return `### ⏬ Download

| Platform | Download |
| --- | --- |
| macOS · Apple silicon | \`${stem}-macos-arm64.dmg\` |
| macOS · Intel | \`${stem}-macos-x64.dmg\` |
| Linux · x86_64 | \`${stem}-linux-x64.AppImage\` or \`.deb\` |
| Windows · x86_64 | \`${stem}-windows-x64-setup.exe\` |
| Windows · x86_64 · MSI | \`${stem}-windows-x64.msi\` |
| Windows · x86_64 · Portable | \`${stem}-windows-x64-portable.zip\` |

Checksums are in \`SHA256SUMS\`. \`-updater.tar.gz\`, \`.sig\` and \`latest.json\` belong to the built-in updater.
`;
}

async function main() {
  const args = process.argv.slice(2);
  const value = (flag) => args[args.indexOf(flag) + 1];
  for (const flag of ["--tag", "--sha", "--dir"]) {
    if (!args.includes(flag) || !value(flag)) throw new Error(`missing ${flag}`);
  }
  const tag = value("--tag");
  const sha = value("--sha");
  if (!/^[0-9a-f]{40}$/.test(sha)) throw new Error("--sha must be an immutable Git commit");
  const repository = process.env.GH_REPO;
  const directory = value("--dir");
  const version = readFileSync("VERSION", "utf8").trim();
  const files = await assembleRelease({ version, tag, repository, directory });
  // A dry run exercises manifest/checksum generation without a GitHub write.
  if (args.includes("--dry-run")) {
    console.log(files.join("\n"));
    return;
  }
  const gh = (args, input) => execFileSync("gh", args, { encoding: "utf8", input, maxBuffer: 16 * 1024 * 1024 });
  // Resolve the actual tag, including annotated tags, never a same-named branch.
  const commit = JSON.parse(gh(["api", `repos/${repository}/commits/${encodeURIComponent(`refs/tags/${tag}`)}`]));
  if (commit.sha !== sha) throw new Error(`${tag} moved since the build started (${sha} -> ${commit.sha})`);
  const pages = JSON.parse(gh(["api", `repos/${repository}/releases?per_page=100`, "--paginate", "--slurp"]));
  let release = existingDraft(pages.flat(), tag);
  const unexpected = (release?.assets ?? []).filter(({ name }) => !files.includes(name));
  if (unexpected.length) {
    throw new Error(`draft contains assets outside this build; remove or reconcile them before retrying: ${unexpected.map(({ name }) => name).join(", ")}`);
  }
  if (!release) {
    // The tag already exists and was verified above. An explicit target_commitish
    // can require workflow-write permission, which GITHUB_TOKEN cannot grant.
    release = JSON.parse(gh(["api", "--method", "POST", `repos/${repository}/releases`, "--input", "-"], JSON.stringify({
      tag_name: tag, name: tag, draft: true,
      prerelease: version.includes("-"), body: releaseNotes(version),
    })));
  }
  gh(["release", "upload", tag, ...files.map((name) => join(directory, name)), "--repo", repository, "--clobber"]);
  console.log(`Draft release ready: ${release.html_url}`);
  if (process.env.GITHUB_STEP_SUMMARY) {
    appendFileSync(process.env.GITHUB_STEP_SUMMARY, `Draft release: ${release.html_url}\n\nBuilt from ${sha}; uploaded ${files.length} files.\n`);
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((error) => {
    console.error(error.message);
    process.exitCode = 1;
  });
}
