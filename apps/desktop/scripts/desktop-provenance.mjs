// Record the workflow and source at the same immutable branch or tag commit.
import { pathToFileURL } from "node:url";
import { currentDesktopSource, desktopSource } from "./desktop-source.mjs";

export function desktopProvenance({ env, sourceSha, releaseTag = "" }) {
  const { releaseTag: tag } = desktopSource({ env, sourceSha, releaseTag });
  const repository = `${env.GITHUB_SERVER_URL}/${env.GITHUB_REPOSITORY}`;
  const prefix = `${env.GITHUB_REPOSITORY}/`;
  const suffix = `@${env.GITHUB_REF}`;
  const workflowRef = env.GITHUB_WORKFLOW_REF;
  if (!workflowRef?.startsWith(prefix) || !workflowRef.endsWith(suffix)) throw new Error("invalid workflow reference");
  return {
    buildDefinition: {
      buildType: "https://actions.github.io/buildtypes/workflow/v1",
      externalParameters: {
        workflow: { repository, ref: env.GITHUB_REF, path: workflowRef.slice(prefix.length, -suffix.length) },
      },
      internalParameters: {
        github: {
          event_name: env.GITHUB_EVENT_NAME,
          repository_id: env.GITHUB_REPOSITORY_ID,
          repository_owner_id: env.GITHUB_REPOSITORY_OWNER_ID,
          runner_environment: env.RUNNER_ENVIRONMENT,
        },
      },
      resolvedDependencies: [
        { name: "workflow", uri: `git+${repository}@${env.GITHUB_REF}`, digest: { gitCommit: env.GITHUB_SHA } },
        { name: "desktop-source", uri: `git+${repository}@${tag ? `refs/tags/${tag}` : sourceSha}`, digest: { gitCommit: sourceSha } },
      ],
    },
    runDetails: {
      // This reusable workflow is called locally from desktop-release.yml, so
      // its definition is at the same ref as the calling workflow.
      builder: { id: `${repository}/.github/workflows/desktop-windows.yml@${env.GITHUB_REF}` },
      metadata: { invocationId: `${repository}/actions/runs/${env.GITHUB_RUN_ID}/attempts/${env.GITHUB_RUN_ATTEMPT}` },
    },
  };
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const { sha: sourceSha, releaseTag } = currentDesktopSource();
  console.log(JSON.stringify(desktopProvenance({ env: process.env, sourceSha, releaseTag })));
}
