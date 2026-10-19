import assert from "node:assert/strict";
import { test } from "node:test";
import { desktopProvenance } from "./desktop-provenance.mjs";

const sourceSha = "a".repeat(40);
function environment(ref) {
  return {
    GITHUB_SERVER_URL: "https://github.com", GITHUB_REPOSITORY: "o/r",
    GITHUB_SHA: sourceSha, GITHUB_REF: ref,
    GITHUB_WORKFLOW_REF: `o/r/.github/workflows/desktop-release.yml@${ref}`,
    GITHUB_EVENT_NAME: "workflow_dispatch", GITHUB_REPOSITORY_ID: "123", GITHUB_REPOSITORY_OWNER_ID: "456",
    RUNNER_ENVIRONMENT: "github-hosted", GITHUB_RUN_ID: "789", GITHUB_RUN_ATTEMPT: "2",
  };
}

test("tag provenance binds the workflow and release source to the same commit", () => {
  const env = environment("refs/tags/v1.2.3");
  // Direct tag dispatches have no release_tag input; the ref supplies identity.
  const predicate = desktopProvenance({ env, sourceSha });
  assert.deepEqual(predicate.buildDefinition.resolvedDependencies, [
    { name: "workflow", uri: "git+https://github.com/o/r@refs/tags/v1.2.3", digest: { gitCommit: sourceSha } },
    { name: "desktop-source", uri: "git+https://github.com/o/r@refs/tags/v1.2.3", digest: { gitCommit: sourceSha } },
  ]);
  assert.equal(predicate.buildDefinition.externalParameters.workflow.path, ".github/workflows/desktop-release.yml");
  assert.equal(predicate.runDetails.builder.id, "https://github.com/o/r/.github/workflows/desktop-windows.yml@refs/tags/v1.2.3");
  assert.equal(predicate.runDetails.metadata.invocationId, "https://github.com/o/r/actions/runs/789/attempts/2");
});

test("branch artifact provenance preserves branch names containing at signs", () => {
  const env = environment("refs/heads/release@next");
  const predicate = desktopProvenance({ env, sourceSha });
  assert.equal(predicate.buildDefinition.externalParameters.workflow.path, ".github/workflows/desktop-release.yml");
  assert.equal(predicate.buildDefinition.resolvedDependencies[1].uri, `git+https://github.com/o/r@${sourceSha}`);
});

test("provenance rejects cross-ref source commits and misleading release tags", () => {
  const env = environment("refs/tags/v1.2.3");
  assert.throws(() => desktopProvenance({ env, sourceSha: "b".repeat(40) }), /workflow commit/);
  assert.throws(() => desktopProvenance({ env, sourceSha, releaseTag: "v9.9.9" }), /workflow ref/);
  assert.throws(() => desktopProvenance({ env: environment("refs/heads/dev"), sourceSha, releaseTag: "v1.2.3" }), /workflow ref/);
  assert.throws(() => desktopProvenance({ env, sourceSha: "dev" }), /immutable/);
});
