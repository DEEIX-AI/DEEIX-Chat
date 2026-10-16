import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

import { collectReleaseAssets } from "./collect-release-assets.mjs";
import { assembleRelease, existingDraft, releaseManifest } from "./publish-desktop-release.mjs";

const script = join(dirname(fileURLToPath(import.meta.url)), "publish-desktop-release.mjs");
const version = "1.2.3-beta.4";
const tag = `v${version}`;
const repository = "DEEIX-AI/DEEIX-Chat";
const sha = "a".repeat(40);
const prefix = `DEEIX-Chat-${version}`;

// Examples of the actual Tauri/portable filenames, independent of the staging
// implementation. The manifest must retain the keys used by existing installs.
const bundles = {
  "macos-arm64": [
    [`dmg/DEEIX Chat_${version}_aarch64.dmg`, "macos-arm64.dmg"],
    ["macos/DEEIX Chat.app.tar.gz", "macos-arm64-updater.tar.gz", ["darwin-aarch64", "darwin-aarch64-app"]],
  ],
  "macos-x64": [
    [`dmg/DEEIX Chat_${version}_x64.dmg`, "macos-x64.dmg"],
    ["macos/DEEIX Chat.app.tar.gz", "macos-x64-updater.tar.gz", ["darwin-x86_64", "darwin-x86_64-app"]],
  ],
  "linux-x64": [
    [`deb/DEEIX Chat_${version}_amd64.deb`, "linux-x64.deb", ["linux-x86_64-deb"]],
    [`appimage/DEEIX Chat_${version}_amd64.AppImage`, "linux-x64.AppImage", ["linux-x86_64", "linux-x86_64-appimage"]],
  ],
  "windows-x64": [
    [`nsis/DEEIX Chat_${version}_x64-setup.exe`, "windows-x64-setup.exe", ["windows-x86_64", "windows-x86_64-nsis"]],
    [`msi/DEEIX Chat_${version}_x64_en-US.msi`, "windows-x64.msi", ["windows-x86_64-msi"]],
    [`portable/${prefix}-windows-x64-portable.zip`, "windows-x64-portable.zip"],
  ],
};

function fixture(t, { offline = false } = {}) {
  const root = mkdtempSync(join(tmpdir(), "desktop-assets-"));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const out = join(root, "release");
  const write = (path, bytes) => {
    mkdirSync(dirname(path), { recursive: true });
    writeFileSync(path, bytes);
  };
  for (const [platform, entries] of Object.entries(bundles)) {
    for (const [source, final, updater] of entries) {
      write(join(root, platform, source), `bytes:${final}`);
      if (updater) write(join(root, platform, `${source}.sig`), `signature:${final}\n`);
    }
  }
  if (offline) write(join(root, "windows-x64", "portable", `${prefix}-windows-x64-portable-offline.zip`), "offline runtime");
  const collect = (platform) => collectReleaseAssets({ platform, version, bundle: join(root, platform), out });
  const collectAll = () => Object.keys(bundles).forEach(collect);
  return { root, out, write, collect, collectAll, options: { version, tag, repository, directory: out } };
}

test("stages all targets without changing signed bytes and preserves the installed updater keys", (t) => {
  const f = fixture(t);
  f.collectAll();
  const { manifest, files } = releaseManifest(f.options);
  const expectedKeys = [];
  for (const entries of Object.values(bundles)) {
    for (const [, final, updater] of entries) {
      const name = `${prefix}-${final}`;
      assert.equal(readFileSync(join(f.out, name), "utf8"), `bytes:${final}`);
      for (const key of updater ?? []) {
        expectedKeys.push(key);
        assert.deepEqual(manifest.platforms[key], {
          signature: `signature:${final}`,
          url: `https://github.com/${repository}/releases/download/${tag}/${name}`,
        });
      }
    }
  }
  assert.deepEqual(Object.keys(manifest.platforms).sort(), expectedKeys.sort());
  assert.equal(manifest.platforms["windows-x86_64"].url.endsWith("-setup.exe"), true);
  assert.equal(manifest.version, version);
  assert.equal(files.length, 15);
});

test("hashes every final download, signature and manifest, including the optional offline portable", async (t) => {
  const f = fixture(t, { offline: true });
  f.collectAll();
  const files = await assembleRelease(f.options);
  assert.equal(files.length, 18);
  const lines = readFileSync(join(f.out, "SHA256SUMS"), "utf8").trim().split("\n");
  const hashes = new Map(lines.map((line) => [line.slice(66), line.slice(0, 64)]));
  assert.deepEqual([...hashes.keys()].sort(), files.filter((name) => name !== "SHA256SUMS").sort());
  for (const [name, hash] of hashes) {
    assert.equal(hash, createHash("sha256").update(readFileSync(join(f.out, name))).digest("hex"));
  }
  // A retry can rebuild generated metadata without treating it as an extra asset.
  assert.deepEqual(await assembleRelease(f.options), files);
});

test("a missing updater signature prevents staging any files for that platform", (t) => {
  const f = fixture(t);
  rmSync(join(f.root, "linux-x64", `${bundles["linux-x64"][1][0]}.sig`));
  assert.throws(() => f.collect("linux-x64"), /missing or empty/);
  assert.equal(existsSync(f.out), false);
});

test("rejects stale versions and multiple bundles instead of guessing which installer to ship", (t) => {
  const f = fixture(t);
  const directory = join(f.root, "macos-arm64", "dmg");
  f.write(join(directory, "DEEIX Chat_0.0.1_aarch64.dmg"), "stale");
  assert.throws(() => f.collect("macos-arm64"), /expected one/);
  rmSync(join(f.root, "macos-arm64", bundles["macos-arm64"][0][0]));
  assert.throws(() => f.collect("macos-arm64"), /does not match version/);
  assert.equal(existsSync(f.out), false);
});

test("incomplete or mixed releases cannot produce a manifest", (t) => {
  const f = fixture(t);
  f.collect("macos-arm64");
  assert.throws(() => releaseManifest(f.options), /missing or empty/);
  for (const platform of ["macos-x64", "linux-x64", "windows-x64"]) f.collect(platform);
  assert.throws(() => releaseManifest({ ...f.options, tag: "v9.0.0" }), /does not match/);
  f.write(join(f.out, "stale.exe"), "other version");
  assert.throws(() => releaseManifest(f.options), /unexpected release asset/);
  rmSync(join(f.out, "stale.exe"));
  f.write(join(f.out, `${prefix}-linux-x64.deb.sig`), " \n");
  assert.throws(() => releaseManifest(f.options), /empty updater signature/);
  assert.equal(existsSync(join(f.out, "latest.json")), false);
});

test("publication only resumes a single draft and never replaces a published release", () => {
  const draft = { id: 123, tag_name: tag, draft: true };
  assert.equal(existingDraft([draft, { tag_name: "v0.1.0", draft: false }], tag), draft);
  assert.equal(existingDraft([], tag), undefined);
  assert.throws(() => existingDraft([draft, { ...draft, id: 456 }], tag), /multiple releases/);
  assert.throws(() => existingDraft([{ ...draft, draft: false }], tag), /already published/);
});

test("dry-run publication assembles metadata without invoking GitHub", (t) => {
  const f = fixture(t);
  f.collectAll();
  f.write(join(f.root, "VERSION"), version);
  const output = execFileSync(process.execPath, [script, "--tag", tag, "--sha", sha, "--dir", f.out, "--dry-run"], {
    cwd: f.root, encoding: "utf8", env: { ...process.env, GH_REPO: repository, GH_TOKEN: "", PATH: "" },
  });
  assert.equal(output.trim().split("\n").length, 17);
  assert.equal(readdirSync(f.out).length, 17);
  assert.equal(JSON.parse(readFileSync(join(f.out, "latest.json"), "utf8")).version, version);
});

test("publication uses a verified tag without workflow-write permission and blocks unsafe retries", { skip: process.platform === "win32" }, (t) => {
  const f = fixture(t);
  f.collectAll();
  f.write(join(f.root, "VERSION"), version);
  const bin = join(f.root, "bin");
  const log = join(f.root, "github-calls.jsonl");
  mkdirSync(bin);
  writeFileSync(join(bin, "gh"), `#!${process.execPath}
const fs = require('node:fs');
const args = process.argv.slice(2);
const input = args.includes('--input') ? JSON.parse(fs.readFileSync(0, 'utf8')) : null;
fs.appendFileSync(process.env.TEST_LOG, JSON.stringify({ args, input }) + '\\n');
if (args[0] === 'release') process.exit(0);
if (args[1].includes('/commits/')) {
  // A same-named branch can still resolve when the actual tag is missing.
  if (args[1].includes('/commits/refs%2Ftags%2F') && process.env.TEST_MISSING_TAG === '1') {
    console.error('gh: Not Found (HTTP 404)');
    process.exit(1);
  }
  console.log(JSON.stringify({ sha: process.env.TEST_COMMIT }));
} else if (args.includes('POST')) {
  // GitHub checks workflow-write permission for an explicit target commit
  // with workflow changes, even when contents: write has been granted.
  if (input.target_commitish) {
    console.error('gh: Resource not accessible by integration (HTTP 403)');
    process.exit(1);
  }
  console.log(JSON.stringify({ id: 123, html_url: 'https://example.invalid/draft', ...input }));
}
else console.log(JSON.stringify([JSON.parse(process.env.TEST_RELEASES)]));
`, { mode: 0o755 });
  const run = (commit, releases, extraEnv = {}) => {
    writeFileSync(log, "");
    const result = spawnSync(process.execPath, [script, "--tag", tag, "--sha", sha, "--dir", f.out], {
      cwd: f.root, encoding: "utf8",
      env: { ...process.env, GH_REPO: repository, GITHUB_STEP_SUMMARY: "", PATH: `${bin}:${process.env.PATH}`,
        TEST_COMMIT: commit, TEST_RELEASES: JSON.stringify(releases), TEST_LOG: log, TEST_MISSING_TAG: "", ...extraEnv },
    });
    const calls = readFileSync(log, "utf8").trim().split("\n").map(JSON.parse);
    return { result, calls, writes: calls.filter(({ args }) => args[0] === "release" || args.includes("POST")) };
  };
  const draft = { tag_name: tag, draft: true, assets: [] };
  for (const [commit, releases, message] of [
    ["b".repeat(40), [], /moved/],
    [sha, [{ ...draft, draft: false }], /already published/],
    [sha, [{ ...draft, assets: [{ name: "old-installer.exe" }] }], /outside this build/],
  ]) {
    const { result, writes } = run(commit, releases);
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, message);
    assert.deepEqual(writes, []);
  }
  const { result, calls, writes } = run(sha, []);
  assert.equal(result.status, 0, result.stderr);
  assert.deepEqual(calls[0].args, ["api", `repos/${repository}/commits/${encodeURIComponent(`refs/tags/${tag}`)}`]);
  assert.equal(writes.length, 2, "one draft creation and one asset upload");
  assert.equal(writes[0].input.tag_name, tag);
  assert.equal(Object.hasOwn(writes[0].input, "target_commitish"), false);
  assert.equal(writes[0].input.draft, true);
  assert.equal(writes[0].input.prerelease, true);
  assert.ok(writes[1].args.includes(join(f.out, "SHA256SUMS")));
  assert.ok(writes[1].args.includes(join(f.out, "latest.json")));
  const retry = run(sha, [draft]);
  assert.equal(retry.result.status, 0, retry.result.stderr);
  assert.equal(retry.writes.length, 1, "retry uploads to the existing draft without creating another one");
  const missing = run(sha, [], { TEST_MISSING_TAG: "1" });
  assert.notEqual(missing.result.status, 0);
  assert.match(missing.result.stderr, /Not Found/);
  assert.deepEqual(missing.writes, [], "a missing tag must never create a release or a new tag");
  assert.equal(missing.calls.length, 1);
});
