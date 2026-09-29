import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { dirname, join } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

import { msiVersion, releaseConfig } from "./release-config.mjs";

const script = join(dirname(fileURLToPath(import.meta.url)), "release-config.mjs");

test("maps app versions to MSI ProductVersions", () => {
  const cases = [
    ["0.4.4", "0.4.4"],
    ["0.4.4-beta.2", "0.4.4.2"],
    ["1.2.3-rc.10", "1.2.3.10"],
    ["1.2.3-rc", "1.2.3.0"],
    ["1.2.3-beta.2.hotfix", "1.2.3.2"],
    ["1.2.3+build.7", "1.2.3"],
    ["255.255.65535-beta.65535", "255.255.65535.65535"],
  ];
  for (const [version, expected] of cases) {
    assert.equal(msiVersion(version), expected, version);
  }
});

test("refuses versions Windows Installer cannot represent", () => {
  for (const bad of ["1.2", "v1.2.3", "256.0.0", "0.256.0", "0.0.65536", "1.2.3-beta.65536", "1.2.3-beta 1"]) {
    assert.throws(() => msiVersion(bad), undefined, bad);
  }
});

test("stable builds keep the configured updater endpoint", () => {
  assert.deepEqual(releaseConfig({ version: "0.5.0", channel: "stable", repository: "o/r" }), {
    bundle: { windows: { wix: { version: "0.5.0" } } },
  });
});

test("beta builds poll the rolling desktop-beta manifest", () => {
  const config = releaseConfig({ version: "0.5.0-beta.3", channel: "beta", repository: "DEEIX-AI/DEEIX-Chat" });
  assert.deepEqual(config.plugins.updater.endpoints, [
    "https://github.com/DEEIX-AI/DEEIX-Chat/releases/download/desktop-beta/latest.json",
  ]);
  assert.equal(config.bundle.windows.wix.version, "0.5.0.3");
  assert.throws(() => releaseConfig({ version: "0.5.0-beta.3", channel: "beta" }), /repository/);
  assert.throws(() => releaseConfig({ version: "0.5.0", channel: "nightly" }), /channel/);
});

test("passes the signing certificate thumbprint to the bundler", () => {
  const thumbprint = "0123456789abcdef0123456789abcdef01234567";
  const config = releaseConfig({ version: "1.0.0", channel: "stable", thumbprint });
  assert.equal(config.bundle.windows.certificateThumbprint, thumbprint.toUpperCase());
  assert.equal(config.bundle.windows.wix.version, "1.0.0");
  assert.equal(releaseConfig({ version: "1.0.0", channel: "stable" }).bundle.windows.certificateThumbprint, undefined);
  assert.throws(() => releaseConfig({ version: "1.0.0", channel: "stable", thumbprint: "abc" }), /thumbprint/);
});

test("prints one whitespace-free --config argument", () => {
  const out = execFileSync(
    process.execPath,
    [script, "--channel", "beta", "--repository", "o/r", "--version", "1.0.0-beta.1"],
    { encoding: "utf8" },
  ).trim();
  const [flag, json, ...rest] = out.split(/\s+/);
  assert.equal(flag, "--config");
  assert.deepEqual(rest, []);
  assert.equal(JSON.parse(json).bundle.windows.wix.version, "1.0.0.1");
});
