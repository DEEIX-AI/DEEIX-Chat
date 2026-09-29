import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, readFileSync, rmSync, utimesSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { crc32, inflateRawSync } from "node:zlib";

import {
  APP_EXE,
  MARKER_TEXT,
  archFromTriple,
  cargoBinaryName,
  findBuildOutputs,
  parseArgs,
  planEntries,
  portableAssetName,
  readmeText,
  resolveWebview2Dir,
  writeZip,
} from "./package-portable.mjs";

const scratch = () => mkdtempSync(join(tmpdir(), "package-portable-"));

function touch(path, content = "x") {
  mkdirSync(join(path, ".."), { recursive: true });
  writeFileSync(path, content);
  return path;
}

/** Minimal reader: central directory -> [{ name, data, isDir }], verifying CRCs and local headers. */
function readZip(file) {
  const buf = readFileSync(file);
  const eocd = buf.lastIndexOf(Buffer.from([0x50, 0x4b, 0x05, 0x06]));
  assert.ok(eocd >= 0, "end of central directory");
  const count = buf.readUInt16LE(eocd + 10);
  let p = buf.readUInt32LE(eocd + 16);
  const out = [];
  for (let i = 0; i < count; i++) {
    assert.equal(buf.readUInt32LE(p), 0x02014b50);
    const method = buf.readUInt16LE(p + 10);
    const crc = buf.readUInt32LE(p + 16);
    const size = buf.readUInt32LE(p + 20);
    const nameLength = buf.readUInt16LE(p + 28);
    const extraLength = buf.readUInt16LE(p + 30);
    const commentLength = buf.readUInt16LE(p + 32);
    const attrs = buf.readUInt32LE(p + 38);
    const localOffset = buf.readUInt32LE(p + 42);
    const name = buf.subarray(p + 46, p + 46 + nameLength).toString("utf8");
    assert.equal(buf.readUInt32LE(localOffset), 0x04034b50);
    const localName = buf.subarray(localOffset + 30, localOffset + 30 + buf.readUInt16LE(localOffset + 26)).toString("utf8");
    assert.equal(localName, name);
    const start = localOffset + 30 + buf.readUInt16LE(localOffset + 26) + buf.readUInt16LE(localOffset + 28);
    const body = buf.subarray(start, start + size);
    const data = method === 8 ? inflateRawSync(body) : Buffer.from(body);
    assert.equal(crc32(data), crc, `crc of ${name}`);
    out.push({ name, data, isDir: name.endsWith("/"), dirAttr: (attrs & 0x10) !== 0 });
    p += 46 + nameLength + extraLength + commentLength;
  }
  return out;
}

test("asset names follow the release convention", () => {
  assert.equal(portableAssetName("0.4.4"), "DEEIX-Chat-0.4.4-windows-x64-portable.zip");
  assert.equal(
    portableAssetName("0.4.4-beta.2", { offline: true }),
    "DEEIX-Chat-0.4.4-beta.2-windows-x64-portable-offline.zip",
  );
  assert.equal(portableAssetName("1.0.0", { arch: "arm64" }), "DEEIX-Chat-1.0.0-windows-arm64-portable.zip");
  assert.equal(archFromTriple("x86_64-pc-windows-msvc"), "x64");
  assert.equal(archFromTriple("aarch64-pc-windows-msvc"), "arm64");
  assert.throws(() => archFromTriple("riscv64gc-unknown-linux-gnu"));
});

test("reads the cargo binary name from [package]", () => {
  const toml = `[package]\nname = "deeix-chat-desktop"\nversion = "1"\n\n[lib]\nname = "deeix_chat_desktop_lib"\n`;
  assert.equal(cargoBinaryName(toml), "deeix-chat-desktop");
  assert.equal(cargoBinaryName(readFileSync(new URL("../src-tauri/Cargo.toml", import.meta.url), "utf8")), "deeix-chat-desktop");
});

test("finds the build outputs, preferring the bundler-signed sidecar", () => {
  const dir = scratch();
  try {
    writeFileSync(join(dir, "Cargo.toml"), `[package]\nname = "app-bin"\n`);
    assert.throws(() => findBuildOutputs(dir), /app-bin\.exe/);
    const plain = touch(join(dir, "target", "release", "app-bin.exe"));
    const copied = touch(join(dir, "target", "release", "deeix-chat-server.exe"));
    assert.deepEqual(findBuildOutputs(dir), { exe: plain, sidecar: copied });
    const signed = touch(join(dir, "binaries", "deeix-chat-server-x86_64-pc-windows-msvc.exe"));
    assert.equal(findBuildOutputs(dir).sidecar, signed);
    // With both target layouts present, the newer build wins.
    const triple = touch(join(dir, "target", "x86_64-pc-windows-msvc", "release", "app-bin.exe"));
    utimesSync(plain, new Date(1_000_000), new Date(1_000_000));
    assert.equal(findBuildOutputs(dir).exe, triple);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("accepts a WebView2 runtime folder or the folder the .cab expands into", () => {
  const dir = scratch();
  try {
    assert.throws(() => resolveWebview2Dir(dir), /msedgewebview2\.exe/);
    const nested = join(dir, "Microsoft.WebView2.FixedVersionRuntime.140.0.1.0.x64");
    touch(join(nested, "msedgewebview2.exe"));
    assert.equal(resolveWebview2Dir(dir), nested);
    assert.equal(resolveWebview2Dir(nested), nested);
    touch(join(dir, "other", "file.txt"));
    assert.throws(() => resolveWebview2Dir(dir), /does not contain/);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("packs the portable layout under one folder with a space in its name", () => {
  const dir = scratch();
  try {
    const exe = touch(join(dir, "deeix-chat-desktop.exe"), "MZ app ".repeat(1000));
    const sidecar = touch(join(dir, "server.exe"), "MZ server");
    const out = writeZip(planEntries({ exe, sidecar, version: "1.2.3" }), join(dir, "out", portableAssetName("1.2.3")), {
      mtime: new Date(Date.UTC(2026, 0, 2, 3, 4, 6)),
    });
    const entries = readZip(out);
    assert.deepEqual(
      entries.map((e) => e.name),
      ["DEEIX Chat/", `DEEIX Chat/${APP_EXE}`, "DEEIX Chat/deeix-chat-server.exe", "DEEIX Chat/portable", "DEEIX Chat/README.txt"],
    );
    const byName = Object.fromEntries(entries.map((e) => [e.name, e]));
    assert.ok(byName["DEEIX Chat/"].dirAttr);
    assert.equal(byName[`DEEIX Chat/${APP_EXE}`].data.toString(), "MZ app ".repeat(1000));
    assert.equal(byName["DEEIX Chat/portable"].data.toString(), MARKER_TEXT);
    const readme = byName["DEEIX Chat/README.txt"].data.toString("utf8");
    assert.match(readme, /DEEIX Chat 1\.2\.3/);
    assert.ok(!readme.includes("webview2\" folder"), "online package does not claim a bundled runtime");

    // Same inputs and timestamp: byte-identical archive.
    const again = writeZip(planEntries({ exe, sidecar, version: "1.2.3" }), join(dir, "again.zip"), {
      mtime: new Date(Date.UTC(2026, 0, 2, 3, 4, 6)),
    });
    assert.deepEqual(readFileSync(again), readFileSync(out));

    // Cross-check with the system unzip where available.
    if (spawnSync("unzip", ["-v"]).status === 0) {
      const listing = execFileSync("unzip", ["-l", out], { encoding: "utf8" });
      assert.match(listing, /DEEIX Chat\/DEEIX Chat\.exe/);
      assert.equal(spawnSync("unzip", ["-tq", out]).status, 0);
    }
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("the offline variant carries the flattened WebView2 runtime", () => {
  const dir = scratch();
  try {
    const exe = touch(join(dir, "app.exe"));
    const sidecar = touch(join(dir, "server.exe"));
    const cab = join(dir, "expanded", "Microsoft.WebView2.FixedVersionRuntime.140.0.1.0.x64");
    touch(join(cab, "msedgewebview2.exe"), "runtime");
    touch(join(cab, "Locales", "zh-CN.pak"), "pak");
    const entries = planEntries({ exe, sidecar, version: "1.2.3", webview2: join(dir, "expanded") });
    const names = entries.map((e) => e.name);
    assert.ok(names.includes("DEEIX Chat/webview2/"));
    assert.ok(names.includes("DEEIX Chat/webview2/msedgewebview2.exe"));
    assert.ok(names.includes("DEEIX Chat/webview2/Locales/"));
    assert.ok(names.includes("DEEIX Chat/webview2/Locales/zh-CN.pak"));
    assert.ok(names.every((n) => !n.includes("\\") && !n.includes("FixedVersionRuntime")));
    const readme = entries.find((e) => e.name.endsWith("README.txt")).content;
    assert.match(readme, /private copy is included/);
    const out = writeZip(entries, join(dir, portableAssetName("1.2.3", { offline: true })));
    assert.equal(readZip(out).find((e) => e.name.endsWith("zh-CN.pak")).data.toString(), "pak");
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("README covers requirements, data, credentials, updates and removal in both languages", () => {
  const readme = readmeText("9.9.9");
  assert.ok(readme.includes("\r\n") && !/[^\r]\n/.test(readme), "CRLF line endings");
  for (const needle of [
    "WebView2",
    "https://developer.microsoft.com/microsoft-edge/webview2/",
    "\"data\" folder",
    "Credential Manager",
    "never updates itself",
    "sign out of every server",
    "com.deeix.chat.desktop.portable-",
    "系统要求",
    "凭据管理器",
    "彻底删除",
  ]) {
    assert.ok(readme.includes(needle), needle);
  }
});

test("rejects unknown or incomplete arguments", () => {
  assert.deepEqual(parseArgs(["--version", "1.0.0", "--out-dir", "x"]), { version: "1.0.0", outDir: "x" });
  assert.throws(() => parseArgs(["--webview2"]), /usage/);
  assert.throws(() => parseArgs(["--nope", "1"]), /usage/);
  assert.throws(() => parseArgs(["--version", "--exe"]), /usage/);
});
