import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { test as nodeTest } from "node:test";
import { fileURLToPath } from "node:url";

const script = join(dirname(fileURLToPath(import.meta.url)), "verify-macos-bundle.sh");
const test = process.platform === "win32" ? nodeTest.skip : nodeTest;
// Exercise macOS's system Bash 3.2 locally; Linux CI also runs these tests.
const bash = process.platform === "darwin" ? "/bin/bash" : "bash";

function fixture(t, arch = "x86_64") {
  const root = mkdtempSync(join(tmpdir(), "macos-verification-"));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const bin = join(root, "bin");
  const source = join(root, "source");
  const app = join(source, "DEEIX Chat.app");
  const dmg = join(root, "DEEIX Chat.dmg");
  const archive = join(root, "DEEIX Chat.app.tar.gz");
  const log = join(root, "calls.jsonl");
  const temp = join(root, "temp");
  mkdirSync(bin);
  mkdirSync(temp);
  mkdirSync(join(app, "Contents", "MacOS"), { recursive: true });
  writeFileSync(join(app, "Contents", "Info.plist"), JSON.stringify({ LSMinimumSystemVersion: "12.0", CFBundleExecutable: "app" }));
  for (const name of ["app", "deeix-chat-server"]) writeFileSync(join(app, "Contents", "MacOS", name), "fixture");
  writeFileSync(dmg, "fixture");
  execFileSync("tar", ["-czf", archive, "-C", source, "DEEIX Chat.app"]);
  const stub = `#!${process.execPath}
const fs = require('node:fs');
const path = require('node:path');
const tool = path.basename(process.argv[1]);
const args = process.argv.slice(2);
const env = process.env;
const file = args.at(-1);
const kind = file?.endsWith('.dmg') ? 'dmg' : 'app';
fs.appendFileSync(env.TEST_LOG, JSON.stringify({ tool, args }) + '\\n');
switch (tool) {
  case 'codesign':
    if (args.includes('-dv')) {
      // Real-world regression: the DMG has a valid certificate but no
      // TeamIdentifier in its CodeDirectory. Apps still expose that field.
      console.error('TeamIdentifier=' + (kind === 'dmg' ? 'not set' : env.TEST_APP_TEAM));
    } else if (args.includes('--verify')) {
      if (env.TEST_BAD_SIGNATURE === kind) process.exit(1);
      if (args.includes('--test-requirement')) {
        const requirement = args[args.indexOf('--test-requirement') + 1];
        for (const term of ['=anchor apple generic',
          'certificate 1[field.1.2.840.113635.100.6.2.6] exists',
          'certificate leaf[field.1.2.840.113635.100.6.1.13] exists']) {
          if (!requirement.includes(term)) throw new Error('missing Developer ID constraint: ' + term);
        }
        if (!requirement.includes('certificate leaf[subject.OU] = "' + env.TEST_DMG_TEAM + '"')
          || env.TEST_BAD_DMG_CERTIFICATE) process.exit(3);
      }
    } else throw new Error('unexpected codesign operation');
    break;
  case 'plutil': console.log(JSON.parse(fs.readFileSync(file, 'utf8'))[args[args.indexOf('-extract') + 1]]); break;
  case 'lipo': console.log(file.endsWith('/deeix-chat-server') ? env.TEST_SIDECAR_ARCH || env.TEST_ARCH : env.TEST_ARCH); break;
  case 'vtool': console.log('    minos ' + (file.endsWith('/deeix-chat-server') ? env.TEST_SIDECAR_MINIMUM || '12.0' : '12.0')); break;
  case 'xcrun':
    if (env.TEST_MISSING_TICKET === kind) { console.error('missing notarization ticket'); process.exit(65); }
    break;
  case 'spctl':
    if (env.TEST_GATEKEEPER_REJECT === kind) { console.error('rejected'); process.exit(3); }
    console.error('accepted\\nsource=Notarized Developer ID');
    break;
  case 'hdiutil':
    if (args[0] === 'attach') fs.cpSync(env.TEST_SOURCE_APP, path.join(args[args.indexOf('-mountpoint') + 1], 'DEEIX Chat.app'), { recursive: true });
    else if (args[0] === 'detach') fs.rmSync(path.join(file, 'DEEIX Chat.app'), { recursive: true });
    else throw new Error('unexpected disk image operation');
    break;
  default: throw new Error('unexpected tool: ' + tool);
}
`;
  for (const tool of ["codesign", "plutil", "lipo", "vtool", "hdiutil", "xcrun", "spctl"]) {
    writeFileSync(join(bin, tool), stub, { mode: 0o755 });
  }
  const env = { ...process.env, PATH: `${bin}:${process.env.PATH}`, TMPDIR: temp,
    APPLE_TEAM_ID: "ABCDE12345", TEST_APP_TEAM: "ABCDE12345", TEST_DMG_TEAM: "ABCDE12345",
    TEST_ARCH: arch, TEST_SOURCE_APP: app, TEST_LOG: log };
  const run = (extra = {}, artifacts = [dmg, archive]) => spawnSync(bash, [script, "--arch", arch, ...artifacts], {
    env: { ...env, ...extra }, encoding: "utf8",
  });
  const calls = () => existsSync(log) ? readFileSync(log, "utf8").trim().split("\n").map(JSON.parse) : [];
  return { run, calls, dmg, archive };
}

test("a Developer ID DMG without TeamIdentifier verifies together with its updater on both architectures", (t) => {
  for (const arch of ["x86_64", "arm64"]) {
    const f = fixture(t, arch);
    const result = f.run();
    assert.equal(result.status, 0, result.stderr + result.stdout);
    assert.match(result.stdout, /verified 2 macOS artifact\(s\)/);
    const calls = f.calls();
    assert.equal(calls.filter(({ tool }) => tool === "spctl").length, 3, "assess the DMG and both extracted apps");
    assert.equal(calls.filter(({ tool }) => tool === "xcrun").length, 3, "validate all three stapled tickets");
    assert.equal(calls.filter(({ tool, args }) => tool === "codesign" && args.includes("-dv") && args.at(-1) === f.dmg).length, 0);
    assert.ok(calls.some(({ tool, args }) => tool === "codesign" && args.includes("--test-requirement") && args.at(-1) === f.dmg));
    assert.ok(calls.some(({ tool, args }) => tool === "hdiutil" && args[0] === "detach"));
  }
});

test("wrong-team, ad-hoc and non-Developer-ID DMG certificates fail before mounting", (t) => {
  for (const extra of [
    { TEST_DMG_TEAM: "OTHER12345" },
    { TEST_BAD_DMG_CERTIFICATE: "adhoc" },
    { TEST_BAD_DMG_CERTIFICATE: "development" },
  ]) {
    const f = fixture(t);
    const result = f.run(extra);
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /Developer ID Application certificate for team ABCDE12345/);
    assert.equal(f.calls().some(({ tool }) => tool === "hdiutil"), false);
  }
});

test("the expected Team ID cannot inject a different code requirement", (t) => {
  const f = fixture(t);
  const result = f.run({ APPLE_TEAM_ID: 'ABCDE12345" or always' });
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /APPLE_TEAM_ID must contain 10 uppercase letters or digits/);
  assert.equal(f.calls().length, 0);
});

test("signature integrity, app team, stapled tickets and Gatekeeper failures still block release", (t) => {
  for (const [extra, expected] of [
    [{ TEST_BAD_SIGNATURE: "dmg" }, /Developer ID Application certificate/],
    [{ TEST_BAD_SIGNATURE: "app" }, undefined],
    [{ TEST_APP_TEAM: "OTHER12345" }, /signed by team 'OTHER12345'/],
    [{ TEST_MISSING_TICKET: "dmg" }, /missing notarization ticket/],
    [{ TEST_MISSING_TICKET: "app" }, /missing notarization ticket/],
    [{ TEST_GATEKEEPER_REJECT: "dmg" }, /Gatekeeper rejected/],
    [{ TEST_GATEKEEPER_REJECT: "app" }, /Gatekeeper rejected/],
  ]) {
    const f = fixture(t);
    const result = f.run(extra);
    assert.notEqual(result.status, 0, JSON.stringify(extra));
    if (expected) assert.match(result.stderr, expected);
  }
});

test("the shipped sidecar must still match the architecture and declared minimum macOS", (t) => {
  for (const [extra, expected] of [
    [{ TEST_SIDECAR_ARCH: "arm64" }, /expected x86_64/],
    [{ TEST_SIDECAR_MINIMUM: "13.0" }, /requires macOS 13.0, but the app declares 12.0/],
  ]) {
    const f = fixture(t);
    const result = f.run(extra);
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, expected);
  }
});

test("ad-hoc builds still check the shipped apps without requiring Developer ID credentials", (t) => {
  const f = fixture(t);
  const result = f.run({ APPLE_TEAM_ID: "" });
  assert.equal(result.status, 0, result.stderr + result.stdout);
  const calls = f.calls();
  assert.equal(calls.some(({ tool }) => ["xcrun", "spctl"].includes(tool)), false);
  assert.equal(calls.filter(({ tool, args }) => tool === "codesign" && args.includes("--verify")).length, 2);
});
