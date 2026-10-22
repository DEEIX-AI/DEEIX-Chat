import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import { copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { test as nodeTest } from "node:test";
import { fileURLToPath } from "node:url";
import { runInNewContext } from "node:vm";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "../../..");
const workflow = (name) => readFileSync(join(root, ".github", "workflows", `${name}.yml`), "utf8");
const release = workflow("desktop-release");
const prepare = workflow("desktop-prepare");
const shellTest = process.platform === "win32" ? nodeTest.skip : nodeTest;

// Read the actual workflow blocks, so the integration tests exercise the shell
// steps and routing conditions that Actions runs without duplicating them here.
function block(text, marker) {
  const lines = text.split("\n");
  const start = lines.findIndex((line) => line.trim() === marker);
  assert.notEqual(start, -1, `missing workflow block: ${marker}`);
  const indent = lines[start].search(/\S/);
  let end = start + 1;
  while (end < lines.length && (!lines[end].trim() || lines[end].search(/\S/) > indent)) end++;
  return lines.slice(start + 1, end);
}

function shellStep(text, name) {
  return block(block(text, `- name: ${name}`).join("\n"), "run: |").map((line) => line.slice(10)).join("\n");
}

function evaluate(expression, context) {
  const source = expression.replace(/^\$\{\{\s*|\s*\}\}$/g, "")
    .replace(/\.([\w]+-[\w-]+)/g, (_, name) => `[${JSON.stringify(name)}]`);
  return runInNewContext(source, context, { timeout: 1000 });
}

function jobCondition(name, context) {
  const line = block(release, `${name}:`).find((value) => value.trim().startsWith("if:"));
  assert.ok(line, `missing ${name} condition`);
  return evaluate(line.trim().slice(3).trim(), context);
}

nodeTest("tag releases build in their own context; branch requests dispatch without building", () => {
  for (const [refType, refName, input, dispatch, build, tag] of [
    ["tag", "v1.2.3", "", false, true, "v1.2.3"],
    ["tag", "v1.2.3-beta.4", "", false, true, "v1.2.3-beta.4"],
    ["tag", "v1.2.3", "v1.2.3", false, true, "v1.2.3"],
    ["branch", "dev", "", false, true, ""],
    ["branch", "feature/desktop", "", false, true, ""],
    ["branch", "dev", "v1.2.3", true, false, "v1.2.3"],
    ["branch", "main", "v1.2.3-beta.4", true, false, "v1.2.3-beta.4"],
  ]) {
    const context = { github: { ref_type: refType, ref_name: refName }, inputs: { release_tag: input } };
    assert.equal(jobCondition("dispatch", context), dispatch, `${refType} ${refName} ${input}`);
    assert.equal(jobCondition("prepare", context), build, `${refType} ${refName} ${input}`);
    const expression = block(release, "prepare:").find((line) => line.trim().startsWith("release-tag:")).trim().slice("release-tag:".length).trim();
    assert.equal(evaluate(expression, context), tag);
    const needs = { prepare: { outputs: { "release-tag": build ? tag : "" } } };
    assert.equal(jobCondition("publish", { needs }), build && tag !== "");
  }
});

nodeTest("every desktop checkout uses the event commit, never a supplied tag or job output", () => {
  for (const name of ["desktop-release", "desktop-prepare", "desktop-macos", "desktop-linux", "desktop-windows"]) {
    const text = workflow(name);
    const checkouts = text.split("\n").filter((line) => line.trim().startsWith("- uses: actions/checkout@"));
    assert.ok(checkouts.length > 0, name);
    const refs = [...text.matchAll(/- uses: actions\/checkout@[^\n]+\n\s+with:\n\s+ref: ([^\n]+)/g)].map((match) => match[1]);
    assert.equal(refs.length, checkouts.length, name);
    assert.ok(refs.every((ref) => ref === "${{ github.sha }}"), name);
  }
});

function fixture(t) {
  const directory = mkdtempSync(join(tmpdir(), "desktop-workflow-"));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const write = (path, content, options) => {
    mkdirSync(dirname(join(directory, path)), { recursive: true });
    writeFileSync(join(directory, path), content, options);
  };
  write("VERSION", "1.2.3-beta.4\n");
  write(".github/actions/desktop-setup/action.yml", "name: fixture\n");
  write("apps/desktop/scripts/collect-release-assets.mjs", "");
  copyFileSync(join(root, "apps/desktop/scripts/desktop-source.mjs"), join(directory, "apps/desktop/scripts/desktop-source.mjs"));
  write("scripts/sync-version.mjs", "import { writeFileSync } from 'node:fs'; writeFileSync('sync-ran', 'yes');\n");
  const git = (args) => execFileSync("git", args, { cwd: directory, encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] }).trim();
  git(["init", "-q"]);
  git(["add", "."]);
  git(["-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "fixture"]);
  const sha = git(["rev-parse", "HEAD"]);
  const env = { ...process.env, GITHUB_SHA: sha, GITHUB_REF: "refs/tags/v1.2.3-beta.4", RELEASE_TAG: "",
    GITHUB_OUTPUT: join(directory, "outputs"), GITHUB_STEP_SUMMARY: join(directory, "summary") };
  delete env.SOURCE_SHA;
  const run = (script, extra = {}) => spawnSync("bash", ["-e", "-o", "pipefail", "-c", script], {
    cwd: directory, env: { ...env, ...extra }, encoding: "utf8",
  });
  return { directory, sha, env, write, run };
}

shellTest("source verification resolves stable, beta and artifact-only runs from their actual ref", (t) => {
  for (const [version, ref, requested, tag, channel] of [
    ["1.2.3", "refs/tags/v1.2.3", "", "v1.2.3", "stable"],
    ["1.2.3-beta.4", "refs/tags/v1.2.3-beta.4", "v1.2.3-beta.4", "v1.2.3-beta.4", "beta"],
    ["1.2.3-beta.4", "refs/heads/dev", "", "", "beta"],
  ]) {
    const f = fixture(t);
    f.write("VERSION", `${version}\n`);
    const result = f.run(shellStep(prepare, "Verify source"), { GITHUB_REF: ref, RELEASE_TAG: requested });
    assert.equal(result.status, 0, result.stderr);
    assert.equal(existsSync(join(f.directory, "sync-ran")), true);
    assert.equal(readFileSync(f.env.GITHUB_OUTPUT, "utf8"), `sha=${f.sha}\nrelease-tag=${tag}\nchannel=${channel}\n`);
  }
});

shellTest("cross-ref inputs and source commits fail before running selected-source scripts", (t) => {
  for (const extra of [
    { GITHUB_REF: "refs/heads/dev", RELEASE_TAG: "v1.2.3-beta.4" },
    { RELEASE_TAG: "v9.9.9" },
    { RELEASE_TAG: "v1.2.3-beta.4%2Fother" },
    { GITHUB_SHA: "b".repeat(40) },
    { SOURCE_SHA: "b".repeat(40) },
  ]) {
    const f = fixture(t);
    const result = f.run(shellStep(prepare, "Verify source"), extra);
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /does not match the workflow/);
    assert.equal(existsSync(join(f.directory, "sync-ran")), false);
    assert.equal(existsSync(f.env.GITHUB_OUTPUT), false);
  }
});

shellTest("native setup and publication reject a mismatched source before consuming it", (t) => {
  const f = fixture(t);
  const action = readFileSync(join(root, ".github/actions/desktop-setup/action.yml"), "utf8");
  for (const [text, name] of [[action, "Verify source commit"], [release, "Verify publication source"]]) {
    const command = block(text, `- name: ${name}`).find((line) => line.trim().startsWith("run:")).trim().slice(4).trim();
    assert.equal(f.run(command, { SOURCE_SHA: f.sha }).status, 0);
    const result = f.run(command, { SOURCE_SHA: "b".repeat(40) });
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /requested source does not match/);
  }
});

shellTest("both dispatchers target the tag without forwarding an input that could redispatch", (t) => {
  const f = fixture(t);
  f.write("bin/gh", `#!${process.execPath}\nimport('node:fs').then(fs => {\n  const args = process.argv.slice(2);\n  fs.appendFileSync(process.env.TEST_GH_CALLS, JSON.stringify(args) + '\\n');\n  if (process.env.TEST_MISSING_TAG === args[args.indexOf('--ref') + 1]) process.exit(1);\n});\n`, { mode: 0o755 });
  const calls = join(f.directory, "gh-calls");
  const env = { PATH: `${join(f.directory, "bin")}:${process.env.PATH}`, TEST_GH_CALLS: calls, RELEASE_TAG: "v1.2.3-beta.4" };
  for (const [text, name] of [[release, "Dispatch"], [workflow("release-tag"), "Start release builds for the tag"]]) {
    f.write("gh-calls", "");
    const result = f.run(shellStep(text, name), env);
    assert.equal(result.status, 0, result.stderr);
    const commands = readFileSync(calls, "utf8").trim().split("\n").map(JSON.parse);
    assert.deepEqual(commands[0], ["workflow", "run", "desktop-release.yml", "--ref", `refs/tags/${env.RELEASE_TAG}`]);
    if (commands.length > 1) assert.deepEqual(commands[1], ["workflow", "run", "image.yml", "--ref", env.RELEASE_TAG]);
  }
  // A branch-only input must request a missing tag, never fall back to running
  // that branch. GitHub rejects the fully qualified tag ref when it is absent.
  f.write("gh-calls", "");
  assert.notEqual(f.run(shellStep(release, "Dispatch"), { ...env, RELEASE_TAG: "dev", TEST_MISSING_TAG: "refs/tags/dev" }).status, 0);
  assert.deepEqual(JSON.parse(readFileSync(calls, "utf8")), ["workflow", "run", "desktop-release.yml", "--ref", "refs/tags/dev"]);
});
