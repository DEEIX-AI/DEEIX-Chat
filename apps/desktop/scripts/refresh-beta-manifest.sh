#!/usr/bin/env bash
# Point the rolling `desktop-beta` release at a published release's updater
# manifest when that release is newer than what beta installs currently get.
# Beta installs must also receive stable releases, so every published release
# is a candidate. Usage: refresh-beta-manifest.sh <tag> ; needs GH_TOKEN and
# GH_REPO (owner/name).
set -euo pipefail

tag="${1:?usage: refresh-beta-manifest.sh <tag>}"
repo="${GH_REPO:?GH_REPO must be owner/name}"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

release_json="$(gh release view "$tag" --repo "$repo" --json isDraft,assets)"
if [[ "$(jq -r .isDraft <<<"$release_json")" == "true" ]]; then
  echo "$tag is still a draft; nothing to publish"
  exit 0
fi
if ! jq -e '.assets[] | select(.name == "latest.json")' <<<"$release_json" >/dev/null; then
  echo "$tag has no latest.json yet; the desktop build will refresh the channel when it finishes"
  exit 0
fi

gh release download "$tag" --repo "$repo" --pattern latest.json --dir "$work/next"
gh release view desktop-beta --repo "$repo" >/dev/null 2>&1 || \
  gh release create desktop-beta --repo "$repo" --prerelease \
    --title "Desktop beta channel" \
    --notes "Rolling updater manifest for beta installs. Do not delete; the beta app polls this tag."
gh release download desktop-beta --repo "$repo" --pattern latest.json --dir "$work/current" 2>/dev/null || true

newer="$(node - "$work" <<'JS'
const fs = require("fs");
const dir = process.argv[2];
const next = JSON.parse(fs.readFileSync(`${dir}/next/latest.json`, "utf8")).version;
const currentPath = `${dir}/current/latest.json`;
const current = fs.existsSync(currentPath) ? JSON.parse(fs.readFileSync(currentPath, "utf8")).version : "0.0.0";
// Semver precedence: numeric core, then a release outranks any prerelease of the same core.
const parse = (v) => { const [core, pre = ""] = v.replace(/^v/, "").split("-"); return { core: core.split(".").map(Number), pre: pre ? pre.split(".") : null }; };
const cmp = (a, b) => {
  for (let i = 0; i < 3; i++) if (a.core[i] !== b.core[i]) return a.core[i] - b.core[i];
  if (!a.pre && !b.pre) return 0; if (!a.pre) return 1; if (!b.pre) return -1;
  for (let i = 0; i < Math.max(a.pre.length, b.pre.length); i++) {
    const x = a.pre[i], y = b.pre[i]; if (x === undefined) return -1; if (y === undefined) return 1;
    const nx = Number(x), ny = Number(y); const bothNum = !Number.isNaN(nx) && !Number.isNaN(ny);
    if (bothNum ? nx !== ny : x !== y) return bothNum ? nx - ny : (x < y ? -1 : 1);
  }
  return 0;
};
console.log(cmp(parse(next), parse(current)) > 0 ? "yes" : "no");
JS
)"

if [[ "$newer" != "yes" ]]; then
  echo "desktop-beta already serves a version at least as new as $tag"
  exit 0
fi
gh release upload desktop-beta "$work/next/latest.json" --repo "$repo" --clobber
echo "desktop-beta now serves $tag"
