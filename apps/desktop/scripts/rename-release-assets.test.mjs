import assert from "node:assert/strict";
import { test } from "node:test";

import { planRenames, renameAsset, rewriteManifest } from "./rename-release-assets.mjs";

const v = "0.4.4-beta.1";

// Exactly what a tag run of desktop-release.yml uploads today.
const tauriNames = [
  "DEEIX.Chat_0.4.4-beta.1_aarch64.dmg",
  "DEEIX.Chat_0.4.4-beta.1_x64.dmg",
  "DEEIX.Chat_aarch64.app.tar.gz",
  "DEEIX.Chat_aarch64.app.tar.gz.sig",
  "DEEIX.Chat_x64.app.tar.gz",
  "DEEIX.Chat_x64.app.tar.gz.sig",
  "DEEIX.Chat_0.4.4-beta.1_amd64.AppImage",
  "DEEIX.Chat_0.4.4-beta.1_amd64.AppImage.sig",
  "DEEIX.Chat_0.4.4-beta.1_amd64.deb",
  "DEEIX.Chat_0.4.4-beta.1_amd64.deb.sig",
  "DEEIX.Chat_0.4.4-beta.1_x64-setup.exe",
  "DEEIX.Chat_0.4.4-beta.1_x64-setup.exe.sig",
  "DEEIX.Chat_0.4.4-beta.1_x64_en-US.msi",
  "DEEIX.Chat_0.4.4-beta.1_x64_en-US.msi.sig",
  "DEEIX.Chat_0.4.4-beta.1_x64_zh-CN.msi",
  "DEEIX.Chat_0.4.4-beta.1_x64_zh-CN.msi.sig",
  "latest.json",
];

const asAssets = (names) => names.map((name, i) => ({ name, apiUrl: `https://api.github.com/repos/o/r/releases/assets/${i}` }));

test("maps Tauri bundle names to the platform-explicit convention", () => {
  const cases = [
    ["DEEIX.Chat_0.4.4-beta.1_aarch64.dmg", "DEEIX-Chat-0.4.4-beta.1-macos-arm64.dmg"],
    ["DEEIX.Chat_0.4.4-beta.1_x64.dmg", "DEEIX-Chat-0.4.4-beta.1-macos-x64.dmg"],
    ["DEEIX.Chat_aarch64.app.tar.gz", "DEEIX-Chat-0.4.4-beta.1-macos-arm64-updater.tar.gz"],
    ["DEEIX.Chat_x64.app.tar.gz.sig", "DEEIX-Chat-0.4.4-beta.1-macos-x64-updater.tar.gz.sig"],
    ["DEEIX.Chat_0.4.4-beta.1_amd64.AppImage", "DEEIX-Chat-0.4.4-beta.1-linux-x64.AppImage"],
    ["DEEIX.Chat_0.4.4-beta.1_amd64.deb.sig", "DEEIX-Chat-0.4.4-beta.1-linux-x64.deb.sig"],
    ["DEEIX.Chat_0.4.4-beta.1_amd64.rpm", "DEEIX-Chat-0.4.4-beta.1-linux-x64.rpm"],
    ["DEEIX.Chat_0.4.4-beta.1_x64-setup.exe", "DEEIX-Chat-0.4.4-beta.1-windows-x64-setup.exe"],
    ["DEEIX.Chat_0.4.4-beta.1_x64-setup.exe.sig", "DEEIX-Chat-0.4.4-beta.1-windows-x64-setup.exe.sig"],
    ["DEEIX.Chat_0.4.4-beta.1_x64_en-US.msi", "DEEIX-Chat-0.4.4-beta.1-windows-x64-en-US.msi"],
    ["DEEIX.Chat_0.4.4-beta.1_x64_zh-CN.msi", "DEEIX-Chat-0.4.4-beta.1-windows-x64-zh-CN.msi"],
    ["DEEIX.Chat_0.4.4-beta.1_x64_zh-CN.msi.sig", "DEEIX-Chat-0.4.4-beta.1-windows-x64-zh-CN.msi.sig"],
  ];
  for (const [from, to] of cases) {
    assert.equal(renameAsset(from, v), to, from);
  }
});

test("leaves unrelated and already-renamed assets alone", () => {
  for (const name of [
    "latest.json",
    "Source code (zip)",
    "README.txt",
    "DEEIX-Chat-0.4.4-beta.1-macos-arm64.dmg",
    "DEEIX-Chat-0.4.4-beta.1-windows-x64-setup.exe",
    "DEEIX-Chat-0.4.4-beta.1-windows-x64-zh-CN.msi",
    "DEEIX-Chat-0.4.4-beta.1-macos-arm64-updater.tar.gz.sig",
  ]) {
    assert.equal(renameAsset(name, v), null, name);
  }
});

test("a full Tauri upload maps to unique names", () => {
  const renames = planRenames(asAssets(tauriNames), v);
  assert.equal(renames.length, tauriNames.length - 1);
  assert.equal(new Set(renames.map((r) => r.to)).size, renames.length);
});

test("a plan that would produce duplicate names is refused before any change", () => {
  assert.throws(
    () => planRenames(asAssets(["DEEIX.Chat_0.4.4-beta.1_x64.dmg", "DEEIX.Chat_0.4.4-beta.1_x64.DMG"]), v),
    /would both become/,
  );
  assert.throws(
    () => planRenames(asAssets(["DEEIX.Chat_0.4.4-beta.1_x64.dmg", "DEEIX-Chat-0.4.4-beta.1-macos-x64.dmg"]), v),
    /already exists/,
  );
});

test("rerunning after an interrupted run only touches what is left", () => {
  const halfDone = tauriNames.map((name) => (name.includes("aarch64") ? renameAsset(name, v) : name));
  const renames = planRenames(asAssets(halfDone), v);
  assert.ok(renames.length > 0);
  assert.ok(renames.every((r) => !r.from.includes("aarch64")));
});

test("rewrites manifest URLs from the manifest itself and is idempotent", () => {
  const manifest = JSON.stringify({
    version: v,
    platforms: {
      "darwin-aarch64": { signature: "s1", url: "https://x/v0.4.4-beta.1/DEEIX.Chat_aarch64.app.tar.gz" },
      "windows-x86_64": { signature: "s2", url: "https://x/v0.4.4-beta.1/DEEIX.Chat_0.4.4-beta.1_x64_en-US.msi" },
      "linux-x86_64": { signature: "s3", url: "https://x/v0.4.4-beta.1/DEEIX-Chat-0.4.4-beta.1-linux-x64.AppImage" },
    },
  });
  const once = rewriteManifest(manifest, v);
  const out = JSON.parse(once);
  assert.equal(out.platforms["darwin-aarch64"].url, "https://x/v0.4.4-beta.1/DEEIX-Chat-0.4.4-beta.1-macos-arm64-updater.tar.gz");
  assert.equal(out.platforms["windows-x86_64"].url, "https://x/v0.4.4-beta.1/DEEIX-Chat-0.4.4-beta.1-windows-x64-en-US.msi");
  assert.equal(out.platforms["linux-x86_64"].url, "https://x/v0.4.4-beta.1/DEEIX-Chat-0.4.4-beta.1-linux-x64.AppImage");
  assert.equal(out.platforms["windows-x86_64"].signature, "s2");
  assert.equal(out.version, v);
  assert.equal(rewriteManifest(once, v), once);
});
