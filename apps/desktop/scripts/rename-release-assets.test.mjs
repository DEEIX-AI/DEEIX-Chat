import assert from "node:assert/strict";
import { test } from "node:test";

import { renameAsset, rewriteManifest } from "./rename-release-assets.mjs";

const v = "0.4.4-beta.1";

test("maps Tauri bundle names to the platform-explicit convention", () => {
  const cases = [
    ["DEEIX.Chat_0.4.4-beta.1_aarch64.dmg", "DEEIX-Chat-0.4.4-beta.1-macos-arm64.dmg"],
    ["DEEIX.Chat_0.4.4-beta.1_x64.dmg", "DEEIX-Chat-0.4.4-beta.1-macos-x64.dmg"],
    ["DEEIX.Chat_aarch64.app.tar.gz", "DEEIX-Chat-0.4.4-beta.1-macos-arm64-updater.tar.gz"],
    ["DEEIX.Chat_x64.app.tar.gz.sig", "DEEIX-Chat-0.4.4-beta.1-macos-x64-updater.tar.gz.sig"],
    ["DEEIX.Chat_0.4.4-beta.1_amd64.AppImage", "DEEIX-Chat-0.4.4-beta.1-linux-x64.AppImage"],
    ["DEEIX.Chat_0.4.4-beta.1_amd64.AppImage.sig", "DEEIX-Chat-0.4.4-beta.1-linux-x64.AppImage.sig"],
    ["DEEIX.Chat_0.4.4-beta.1_amd64.deb", "DEEIX-Chat-0.4.4-beta.1-linux-x64.deb"],
    ["DEEIX.Chat_0.4.4-beta.1_amd64.rpm", "DEEIX-Chat-0.4.4-beta.1-linux-x64.rpm"],
    ["DEEIX.Chat_0.4.4-beta.1_x64-setup.exe", "DEEIX-Chat-0.4.4-beta.1-windows-x64-setup.exe"],
    ["DEEIX.Chat_0.4.4-beta.1_x64_en-US.msi", "DEEIX-Chat-0.4.4-beta.1-windows-x64.msi"],
    ["DEEIX.Chat_0.4.4-beta.1_x64_en-US.msi.sig", "DEEIX-Chat-0.4.4-beta.1-windows-x64.msi.sig"],
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
    "DEEIX-Chat-0.4.4-beta.1-macos-arm64-updater.tar.gz.sig",
  ]) {
    assert.equal(renameAsset(name, v), null, name);
  }
});

test("rewrites manifest URLs, raw and URL-encoded", () => {
  const manifest = JSON.stringify({
    platforms: {
      "darwin-aarch64": { url: "https://x/v0.4.4-beta.1/DEEIX.Chat_aarch64.app.tar.gz", signature: "s1" },
      "windows-x86_64": { url: "https://x/v0.4.4-beta.1/DEEIX.Chat_0.4.4-beta.1_x64_en-US.msi", signature: "s2" },
      "linux-x86_64": { url: "https://x/v0.4.4-beta.1/DEEIX.Chat_0.4.4-beta.1_amd64.AppImage", signature: "s3" },
    },
  });
  const names = ["DEEIX.Chat_aarch64.app.tar.gz", "DEEIX.Chat_0.4.4-beta.1_x64_en-US.msi", "DEEIX.Chat_0.4.4-beta.1_amd64.AppImage"];
  const renames = names.map((from) => ({ from, to: renameAsset(from, v) }));
  const out = JSON.parse(rewriteManifest(manifest, renames));
  assert.equal(out.platforms["darwin-aarch64"].url, "https://x/v0.4.4-beta.1/DEEIX-Chat-0.4.4-beta.1-macos-arm64-updater.tar.gz");
  assert.equal(out.platforms["windows-x86_64"].url, "https://x/v0.4.4-beta.1/DEEIX-Chat-0.4.4-beta.1-windows-x64.msi");
  assert.equal(out.platforms["linux-x86_64"].url, "https://x/v0.4.4-beta.1/DEEIX-Chat-0.4.4-beta.1-linux-x64.AppImage");
  assert.equal(out.platforms["darwin-aarch64"].signature, "s1");
});
