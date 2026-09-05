#!/usr/bin/env bash

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
backend_dir=$(cd "$script_dir/.." && pwd)
staticcheck_baseline_file="$backend_dir/.lint-baseline/staticcheck.txt"
context_background_baseline_file="$backend_dir/.lint-baseline/context-background.txt"
work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT

cd "$backend_dir"

echo "Checking gofmt..."
find . -type f -name '*.go' -not -path './vendor/*' -exec gofmt -l {} + \
  | sed 's|^\./||' \
  | LC_ALL=C sort > "$work_dir/gofmt.txt"
if [[ -s "$work_dir/gofmt.txt" ]]; then
  echo "The following Go files are not formatted:" >&2
  cat "$work_dir/gofmt.txt" >&2
  exit 1
fi

echo "Checking interface spelling..."
find internal -type f -name '*.go' -not -name '*_test.go' \
  -exec grep -nE 'interface[[:space:]]*\{[[:space:]]*\}' {} + \
  > "$work_dir/interface-legacy.txt" || true
if [[ -s "$work_dir/interface-legacy.txt" ]]; then
  echo "Use any instead of interface{} in production code:" >&2
  cat "$work_dir/interface-legacy.txt" >&2
  exit 1
fi

echo "Checking context.Background policy..."
find internal/application internal/transport \
  -type f \
  -name '*.go' \
  ! -name '*_test.go' \
  -exec awk '/context[[:space:]]*\.[[:space:]]*Background[[:space:]]*\(/ { print FILENAME ":" $0 }' {} + \
  | sed -E \
    -e 's|^\./||' \
    -e 's|^([^:]+):[[:space:]]*|\1: |' \
    -e 's|[[:space:]]+$||' \
  | LC_ALL=C sort > "$work_dir/context-background.current"
LC_ALL=C sort "$context_background_baseline_file" > "$work_dir/context-background.baseline"

if ! cmp -s "$context_background_baseline_file" "$work_dir/context-background.baseline"; then
  echo "context.Background baseline must remain sorted: $context_background_baseline_file" >&2
  exit 1
fi

comm -13 "$work_dir/context-background.baseline" "$work_dir/context-background.current" > "$work_dir/context-background.new"
comm -23 "$work_dir/context-background.baseline" "$work_dir/context-background.current" > "$work_dir/context-background.resolved"

if [[ -s "$work_dir/context-background.new" || -s "$work_dir/context-background.resolved" ]]; then
  if [[ -s "$work_dir/context-background.new" ]]; then
    echo "New context.Background calls in application/transport code:" >&2
    cat "$work_dir/context-background.new" >&2
  fi
  if [[ -s "$work_dir/context-background.resolved" ]]; then
    echo "Resolved context.Background calls must be removed from $context_background_baseline_file:" >&2
    cat "$work_dir/context-background.resolved" >&2
  fi
  exit 1
fi

echo "Running go vet..."
go vet ./...

echo "Running staticcheck..."
set +e
go tool staticcheck ./... \
  > "$work_dir/staticcheck.output" \
  2> "$work_dir/staticcheck.error"
staticcheck_status=$?
set -e

sed -E \
	-e 's|^\./||' \
	-e 's|^([^:]+):[0-9]+:[0-9]+: |\1: |' \
	"$work_dir/staticcheck.output" \
	| LC_ALL=C sort > "$work_dir/staticcheck.current"
LC_ALL=C sort "$staticcheck_baseline_file" > "$work_dir/staticcheck.baseline"

if ! cmp -s "$staticcheck_baseline_file" "$work_dir/staticcheck.baseline"; then
  echo "Staticcheck baseline must remain sorted: $staticcheck_baseline_file" >&2
  exit 1
fi

comm -13 "$work_dir/staticcheck.baseline" "$work_dir/staticcheck.current" > "$work_dir/staticcheck.new"
comm -23 "$work_dir/staticcheck.baseline" "$work_dir/staticcheck.current" > "$work_dir/staticcheck.resolved"

if [[ -s "$work_dir/staticcheck.new" || -s "$work_dir/staticcheck.resolved" ]]; then
  if [[ -s "$work_dir/staticcheck.new" ]]; then
    echo "New staticcheck findings:" >&2
    cat "$work_dir/staticcheck.new" >&2
  fi
  if [[ -s "$work_dir/staticcheck.resolved" ]]; then
    echo "Resolved staticcheck findings must be removed from $staticcheck_baseline_file:" >&2
    cat "$work_dir/staticcheck.resolved" >&2
  fi
  if [[ -s "$work_dir/staticcheck.output" ]]; then
    echo "Full staticcheck output:" >&2
    cat "$work_dir/staticcheck.output" >&2
  fi
  exit 1
fi

if [[ -s "$work_dir/staticcheck.error" ]]; then
  # Go may print module/toolchain download progress to stderr on a clean runner.
  # Keep that bootstrap noise out of the finding baseline, but never hide a
  # substantive staticcheck or tool execution error.
  sed '/^go: downloading /d' "$work_dir/staticcheck.error" > "$work_dir/staticcheck.error.filtered"
  if [[ -s "$work_dir/staticcheck.error.filtered" ]]; then
    echo "staticcheck failed:" >&2
    cat "$work_dir/staticcheck.error.filtered" >&2
    exit 1
  fi
fi

if [[ $staticcheck_status -gt 1 ]]; then
  echo "staticcheck exited with status $staticcheck_status" >&2
  cat "$work_dir/staticcheck.output" >&2
  exit "$staticcheck_status"
fi

if [[ $staticcheck_status -eq 1 && ! -s "$work_dir/staticcheck.output" ]]; then
  echo "staticcheck exited with status 1 without diagnostics" >&2
  exit 1
fi

echo "Running deadcode..."
set +e
go tool deadcode ./... \
  > "$work_dir/deadcode.output" \
  2> "$work_dir/deadcode.error"
deadcode_status=$?
set -e

if [[ -s "$work_dir/deadcode.error" ]]; then
  sed '/^go: downloading /d' "$work_dir/deadcode.error" > "$work_dir/deadcode.error.filtered"
  if [[ -s "$work_dir/deadcode.error.filtered" ]]; then
    echo "deadcode failed:" >&2
    cat "$work_dir/deadcode.error.filtered" >&2
    exit 1
  fi
fi

if [[ $deadcode_status -ne 0 || -s "$work_dir/deadcode.output" ]]; then
  echo "Dead code detected or deadcode failed:" >&2
  cat "$work_dir/deadcode.output" >&2
  exit 1
fi

echo "Go quality checks passed."
