#!/usr/bin/env bash
#
# check_specs_and_plans_hashes.sh — verify every spec/plan listed in
# .specs-and-plans-hashes matches its pinned content-hash.
#
# Mirror of check_sub_creed_hashes.sh applied to design specs and
# implementation plans. Each document declares at its tail that it is
# itself a Contribution of class PIPELINE_IMPROVEMENT, lifecycle phase
# SUBSTRATE. UW: the chain pays for its own design and execution; the
# design artifacts are among the work, and are hash-bound the same way
# the doctrines are.
#
# Format of .specs-and-plans-hashes (whitespace-separated triples,
# one per line, blank lines + lines starting with `#` ignored):
#   <short-name> <sha256>  <path/to/doc.md>
#
# To intentionally amend a spec/plan:
#   1. Edit the .md file (including its closing "This document is a
#      Contribution" section if relevant).
#   2. Run this script — it prints the new computed hash on mismatch.
#   3. Update .specs-and-plans-hashes with the new hash on that line.
#   4. If the manifest-of-hashes (.recursion-manifest-hash) exists,
#      recompute it via scripts/check_recursion_manifest.sh.
#   5. Commit all updated files together so reviewers see the doc
#      diff, the hash bump, and the manifest bump in the same commit.

set -euo pipefail

HASH_FILE=".specs-and-plans-hashes"

if [ ! -f "$HASH_FILE" ]; then
  echo "error: $HASH_FILE not found"
  exit 1
fi

hash_cmd() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256
  elif command -v sha256sum >/dev/null 2>&1; then
    sha256sum
  elif command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 | awk '{print $NF}'
  else
    echo "error: need shasum, sha256sum, or openssl" >&2
    exit 1
  fi
}

failed=0
while IFS= read -r line || [ -n "$line" ]; do
  # Skip blank lines and comments.
  case "$line" in
    ''|'#'*) continue ;;
  esac
  # Parse: <name> <hash>  <path>. Use awk so the path may contain
  # arbitrary single-spaces (unlikely, but defensive).
  name=$(echo "$line"  | awk '{print $1}')
  expected=$(echo "$line" | awk '{print $2}')
  doc=$(echo "$line"   | awk '{print $3}')
  if [ -z "$name" ] || [ -z "$expected" ] || [ -z "$doc" ]; then
    continue
  fi
  if [ ! -f "$doc" ]; then
    echo "error: $doc referenced in $HASH_FILE but not found" >&2
    failed=1
    continue
  fi
  actual=$(tr -d '\r' < "$doc" | hash_cmd | awk '{print $1}')
  if [ "$actual" != "$expected" ]; then
    cat <<EOF >&2
spec/plan hash check failed: $name

  doc:        $doc
  expected:   $expected
  actual:     $actual

If you intentionally amended this document, update the matching line
in $HASH_FILE to:
  $name $actual  $doc
EOF
    failed=1
  else
    echo "spec/plan hash check ok ($name: $actual)"
  fi
done < "$HASH_FILE"

if [ "$failed" -ne 0 ]; then
  exit 1
fi
