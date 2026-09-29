import assert from "node:assert/strict";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";

import { CHECKSUMS_FILE, formatChecksums, hashFile } from "./release-checksums.mjs";

test("hashes files as sha256sum does", async () => {
  const dir = mkdtempSync(join(tmpdir(), "release-checksums-"));
  try {
    const file = join(dir, "a.bin");
    writeFileSync(file, "abc");
    // FIPS 180-2 test vector.
    assert.equal(await hashFile(file), "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad");
    writeFileSync(file, "");
    assert.equal(await hashFile(file), "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855");
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("formats sorted coreutils lines and never lists itself", () => {
  const a = "a".repeat(64);
  const b = "b".repeat(64);
  const text = formatChecksums([
    { name: "latest.json", sha256: a },
    { name: CHECKSUMS_FILE, sha256: b },
    { name: "DEEIX-Chat-1.0.0-windows-x64.msi", sha256: b },
    { name: "DEEIX-Chat-1.0.0-windows-x64-portable.zip", sha256: a },
  ]);
  assert.equal(
    text,
    `${a}  DEEIX-Chat-1.0.0-windows-x64-portable.zip\n${b}  DEEIX-Chat-1.0.0-windows-x64.msi\n${a}  latest.json\n`,
  );
});

test("refuses malformed digests and names", () => {
  assert.throws(() => formatChecksums([{ name: "x", sha256: "ABC" }]), /SHA-256/);
  assert.throws(() => formatChecksums([{ name: "x\ny", sha256: "a".repeat(64) }]), /unsupported/);
});
