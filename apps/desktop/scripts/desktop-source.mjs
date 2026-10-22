import { execFileSync } from "node:child_process";
import { pathToFileURL } from "node:url";

// Source inputs describe the run's commit; they must never select another ref
// whose code would execute with this run's cache and signing capabilities.
export function desktopSource({ env, sourceSha, releaseTag = env.RELEASE_TAG ?? "" }) {
  if ([env.GITHUB_SHA, sourceSha].some((sha) => !/^[0-9a-f]{40}$/.test(sha ?? ""))) throw new Error("workflow requires an immutable source commit");
  if (sourceSha !== env.GITHUB_SHA) throw new Error("checkout does not match the workflow commit");
  if (env.SOURCE_SHA !== undefined && env.SOURCE_SHA !== sourceSha) throw new Error("requested source does not match the workflow commit");
  const ref = env.GITHUB_REF ?? "";
  if (!ref.startsWith("refs/heads/") && !ref.startsWith("refs/tags/")) throw new Error("expected a branch or tag workflow ref");
  const tag = ref.startsWith("refs/tags/") ? ref.slice("refs/tags/".length) : "";
  if (releaseTag && releaseTag !== tag) throw new Error("release tag does not match the workflow ref; dispatch the workflow on that tag");
  return { sha: sourceSha, releaseTag: tag };
}

export function currentDesktopSource(env = process.env) {
  const sourceSha = execFileSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).trim();
  return desktopSource({ env, sourceSha });
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  currentDesktopSource();
}
