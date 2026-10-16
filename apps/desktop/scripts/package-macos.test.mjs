import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import { chmodSync, existsSync, mkdirSync, mkdtempSync, readFileSync, readlinkSync, readdirSync, rmSync, symlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { test as nodeTest } from "node:test";
import { fileURLToPath } from "node:url";

const scripts = dirname(fileURLToPath(import.meta.url));
// These shell integration tests run on the Linux desktop CI job and on macOS.
const test = process.platform === "win32" ? nodeTest.skip : nodeTest;
const target = "aarch64-apple-darwin";
const product = "DEEIX Chat";
const version = "1.2.3-beta.4";
const apple = { APPLE_CERTIFICATE: "Y2VydGlmaWNhdGU=", APPLE_CERTIFICATE_PASSWORD: "", APPLE_ID: "test@example.invalid", APPLE_PASSWORD: "test-password" };
const remote = {
  SIGN_AND_NOTARIZE_SCRIPT_URL: "https://example.invalid/sign.sh", SIGN_AND_NOTARIZE_BASE_URL: "https://example.invalid",
  SIGN_AND_NOTARIZE_API_TOKEN: "test-token", SIGN_AND_NOTARIZE_HELPER_URL: "https://example.invalid/result.sh",
};

function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), "macos-packaging-"));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const bin = join(root, "bin");
  const tauri = join(root, "desktop project", "src-tauri");
  const bundle = join(tauri, "target", target, "release", "bundle");
  const app = join(bundle, "macos", `${product}.app`);
  const dmg = join(bundle, "dmg", `${product}_${version}_aarch64.dmg`);
  const archive = `${app}.tar.gz`;
  const image = join(root, "dmg contents");
  const log = join(root, "calls.jsonl");
  const githubEnv = join(root, "github-env");
  mkdirSync(bin);
  mkdirSync(join(app, "Contents", "MacOS"), { recursive: true });
  writeFileSync(join(tauri, "tauri.conf.json"), JSON.stringify({ productName: product, version }));
  writeFileSync(join(app, "Contents", "MacOS", "app"), "unsigned");
  chmodSync(join(app, "Contents", "MacOS", "app"), 0o755);
  symlinkSync("app", join(app, "Contents", "MacOS", "alias"));
  writeFileSync(githubEnv, "");
  const helper = `set -euo pipefail
printf '%s\\n' "$(basename "$1")" >> "$TEST_SIGN_REQUESTS"
if [[ -d "$1" ]]; then
  ditto "$1" "$2"
  printf 'signed-app' > "$2/Contents/MacOS/app"
else
  cp "$1" "$2"
  printf ':signed-dmg' >> "$2"
fi
`;
  const helperPath = join(root, "remote-helper.sh");
  writeFileSync(helperPath, helper);
  const stub = `#!${process.execPath}
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const tool = path.basename(process.argv[1]);
const args = process.argv.slice(2);
const env = process.env;
fs.appendFileSync(env.TEST_LOG, JSON.stringify({ tool, args }) + '\\n');
switch (tool) {
  case 'curl': process.stdout.write(fs.readFileSync(env.TEST_HELPER)); break;
  case 'ditto': fs.cpSync(args[0], args[1], { recursive: true, verbatimSymlinks: true }); break;
  case 'codesign':
    if (args.includes('-dv')) console.error('TeamIdentifier=ABCDE12345');
    if (args.includes('--verify') && env.TEST_BAD_SIGNATURE) process.exit(1);
    if (args.includes('--keychain') && !fs.existsSync(args[args.indexOf('--keychain') + 1])) process.exit(2);
    break;
  case 'hdiutil':
    if (args[0] !== 'create') throw new Error('unexpected extra DMG conversion');
    fs.cpSync(args[args.indexOf('-srcfolder') + 1], env.TEST_IMAGE, { recursive: true, verbatimSymlinks: true });
    fs.writeFileSync(args.at(-1), 'dmg');
    break;
  case 'pnpm':
    if (!env.TAURI_SIGNING_PRIVATE_KEY) throw new Error('updater key missing');
    fs.writeFileSync(args.at(-1) + '.sig', crypto.createHash('sha256').update(fs.readFileSync(args.at(-1))).digest('hex'));
    break;
  case 'openssl':
    if (args[0] === 'rand') console.log('0123456789abcdef');
    else if (args[0] === 'pkcs12') console.log('certificate');
    else console.log('    CN=Developer ID Application: Test (ABCDE12345)\\n    OU=ABCDE12345');
    break;
  case 'security':
    if (args[0] === 'create-keychain') fs.writeFileSync(args.at(-1), 'keychain');
    if (args[0] === 'delete-keychain') fs.rmSync(args.at(-1));
    break;
  case 'xcrun': if (env.TEST_NOTARY_FAILURE) process.exit(1); break;
  default: throw new Error('unexpected tool: ' + tool);
}
`;
  for (const name of ["curl", "ditto", "codesign", "hdiutil", "pnpm", "openssl", "security", "xcrun"]) {
    writeFileSync(join(bin, name), stub, { mode: 0o755 });
  }
  const env = {
    ...process.env,
    ...Object.fromEntries([...Object.keys(apple), ...Object.keys(remote), "APPLE_TEAM_ID", "APPLE_CERTIFICATE_PATH", "TAURI_SIGNING_PRIVATE_KEY"].map((name) => [name, ""])),
    PATH: `${bin}:${process.env.PATH}`, GITHUB_ENV: githubEnv, RUNNER_TEMP: root,
    TEST_HELPER: helperPath, TEST_LOG: log, TEST_IMAGE: image, TEST_SIGN_REQUESTS: join(root, "requests"),
  };
  const configure = (extra = {}) => spawnSync("bash", [join(scripts, "configure-apple-signing.sh")], { env: { ...env, ...extra }, encoding: "utf8" });
  const phase = (name, extra = {}) => spawnSync("bash", [join(scripts, "package-macos.sh"), name, target, tauri], { env: { ...env, ...extra }, encoding: "utf8" });
  const calls = () => existsSync(log) ? readFileSync(log, "utf8").trim().split("\n").map(JSON.parse) : [];
  return { root, app, dmg, archive, bundle, githubEnv, env, image, configure, phase, calls };
}

function passed(result) {
  assert.equal(result.status, 0, result.stderr + result.stdout);
}

test("signing configuration selects one provider and rejects partial credentials", (t) => {
  const f = fixture(t);
  passed(f.configure());
  assert.match(readFileSync(f.githubEnv, "utf8"), /DESKTOP_SIGNING_MODE=adhoc/);
  writeFileSync(f.githubEnv, "");
  passed(f.configure({ ...apple, ...remote }));
  assert.equal(readFileSync(f.githubEnv, "utf8"), "DESKTOP_SIGNING_MODE=external\nAPPLE_SIGNING_IDENTITY=-\n");
  assert.equal(f.calls().length, 0, "external signing must not import or inspect the Apple certificate");
  assert.notEqual(f.configure({ SIGN_AND_NOTARIZE_SCRIPT_URL: remote.SIGN_AND_NOTARIZE_SCRIPT_URL }).status, 0);
  assert.notEqual(f.configure({ APPLE_ID: apple.APPLE_ID }).status, 0);
  writeFileSync(f.githubEnv, "");
  passed(f.configure(apple));
  const exported = readFileSync(f.githubEnv, "utf8");
  assert.match(exported, /DESKTOP_SIGNING_MODE=apple/);
  assert.match(exported, /APPLE_SIGNING_IDENTITY=Developer ID Application: Test \(ABCDE12345\)/);
  assert.match(exported, /APPLE_TEAM_ID=ABCDE12345/);
});

test("DMG and updater contain the same final signed app with its executable mode and symlink", (t) => {
  const f = fixture(t);
  const env = { ...remote, DESKTOP_SIGNING_MODE: "external", TAURI_SIGNING_PRIVATE_KEY: "test-updater-key" };
  for (const phase of ["sign-app", "dmg", "sign-dmg", "updater"]) passed(f.phase(phase, env));
  const unpacked = join(f.root, "updater contents");
  mkdirSync(unpacked);
  execFileSync("tar", ["-xzf", f.archive, "-C", unpacked]);
  for (const app of [f.app, join(f.image, `${product}.app`), join(unpacked, `${product}.app`)]) {
    assert.equal(readFileSync(join(app, "Contents", "MacOS", "app"), "utf8"), "signed-app");
    assert.equal(readlinkSync(join(app, "Contents", "MacOS", "alias")), "app");
    execFileSync("test", ["-x", join(app, "Contents", "MacOS", "app")]);
  }
  assert.equal(readlinkSync(join(f.image, "Applications")), "/Applications");
  assert.equal(readFileSync(f.dmg, "utf8"), "dmg:signed-dmg");
  assert.equal(existsSync(`${f.archive}.sig`), true);
  assert.match(readFileSync(f.githubEnv, "utf8"), /APPLE_TEAM_ID=ABCDE12345/);
  assert.equal(f.calls().filter(({ tool }) => tool === "curl").length, 1);
  const images = f.calls().filter(({ tool }) => tool === "hdiutil");
  assert.equal(images.length, 1);
  assert.equal(images[0].args[images[0].args.indexOf("-format") + 1], "ULFO");
  const signer = f.calls().find(({ tool }) => tool === "pnpm");
  assert.deepEqual(signer.args.slice(2), ["exec", "tauri", "signer", "sign", "--app-version", version, f.archive]);
  assert.deepEqual(readFileSync(f.env.TEST_SIGN_REQUESTS, "utf8").trim().split("\n"), [
    `DEEIX-Chat-${version}-macos-arm64.app`, `DEEIX-Chat-${version}-macos-arm64.dmg`,
  ]);
  assert.equal(readdirSync(f.bundle).some((name) => name.startsWith(".package-macos.")), false);
});

test("an invalid service signature never replaces the app and a missing updater key fails", (t) => {
  const f = fixture(t);
  assert.notEqual(f.phase("sign-app", { ...remote, DESKTOP_SIGNING_MODE: "external", TEST_BAD_SIGNATURE: "1" }).status, 0);
  assert.equal(readFileSync(join(f.app, "Contents", "MacOS", "app"), "utf8"), "unsigned");
  assert.notEqual(f.phase("updater").status, 0);
  assert.equal(existsSync(f.archive), false);
});

test("local Apple DMG signing imports a fresh keychain and cleans it up on notarization failure", (t) => {
  const f = fixture(t);
  passed(f.phase("dmg"));
  const result = f.phase("sign-dmg", {
    ...apple, DESKTOP_SIGNING_MODE: "apple", APPLE_SIGNING_IDENTITY: "Developer ID Application: Test (ABCDE12345)",
    APPLE_TEAM_ID: "ABCDE12345", TEST_NOTARY_FAILURE: "1",
  });
  assert.notEqual(result.status, 0);
  const calls = f.calls();
  const sign = calls.find(({ tool, args }) => tool === "codesign" && args.includes("--keychain"));
  assert.ok(sign);
  const keychain = sign.args[sign.args.indexOf("--keychain") + 1];
  assert.ok(calls.some(({ tool, args }) => tool === "security" && args[0] === "delete-keychain" && args[1] === keychain));
  assert.equal(existsSync(keychain), false);
  assert.equal(readdirSync(f.bundle).some((name) => name.startsWith(".package-macos.")), false);
});
