import assert from "node:assert/strict";
import { test } from "node:test";
import { desktopProvenance } from "./desktop-provenance.mjs";

test("provenance distinguishes the release source from the default-branch workflow", () => {
  const env = {
    GITHUB_SERVER_URL: "https://github.com", GITHUB_REPOSITORY: "o/r",
    GITHUB_SHA: "a".repeat(40), GITHUB_REF: "refs/heads/release@next",
    GITHUB_WORKFLOW_REF: "o/r/.github/workflows/desktop-release.yml@refs/heads/release@next",
    GITHUB_EVENT_NAME: "workflow_dispatch", GITHUB_REPOSITORY_ID: "123", GITHUB_REPOSITORY_OWNER_ID: "456",
    RUNNER_ENVIRONMENT: "github-hosted", GITHUB_RUN_ID: "789", GITHUB_RUN_ATTEMPT: "2",
  };
  const sourceSha = "b".repeat(40);
  const predicate = desktopProvenance({ env, sourceSha, releaseTag: "v1.2.3" });
  assert.deepEqual(predicate.buildDefinition.resolvedDependencies, [
    { name: "workflow", uri: "git+https://github.com/o/r@refs/heads/release@next", digest: { gitCommit: env.GITHUB_SHA } },
    { name: "desktop-source", uri: "git+https://github.com/o/r@refs/tags/v1.2.3", digest: { gitCommit: sourceSha } },
  ]);
  assert.equal(predicate.buildDefinition.externalParameters.workflow.path, ".github/workflows/desktop-release.yml");
  assert.equal(predicate.runDetails.builder.id, "https://github.com/o/r/.github/workflows/desktop-windows.yml@refs/heads/release@next");
  assert.equal(predicate.runDetails.metadata.invocationId, "https://github.com/o/r/actions/runs/789/attempts/2");
  const preview = desktopProvenance({ env, sourceSha });
  assert.equal(preview.buildDefinition.resolvedDependencies[1].uri, `git+https://github.com/o/r@${sourceSha}`);
  assert.throws(() => desktopProvenance({ env, sourceSha: "dev" }), /immutable/);
});
