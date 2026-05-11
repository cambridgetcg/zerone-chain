#!/usr/bin/env bash
#
# check_recursion_doctrine_hash.sh — verify RECURSIVE_ZERONE.md has not
# drifted from the pinned hash in .recursion-doctrine-hash.
#
# The recursion-doctrine catalog (docs/RECURSIVE_ZERONE.md) names every
# loop the chain participates in inside its own systems. Each recursion
# is bound by tests (verified by `make recursion-check`). The doctrine
# itself is governance-tier — silent amendment of the recursion
# catalog would weaken the visible audit trail that makes the recursions
# legible.
#
# This script applies the same off-chain discipline that protects
# TRUTH_SEEKING.md (.creed-hash) to RECURSIVE_ZERONE.md: any change to
# the doctrine must come paired with a hash bump that's visible in
# the PR diff. Reviewers see the .recursion-doctrine-hash change as
# the signal that the catalog has shifted.
#
# To intentionally amend the recursion catalog:
#   1. Edit docs/RECURSIVE_ZERONE.md (add a recursion, refine a
#      "Closed by:", document a new binding test).
#   2. Run this script — it prints the new computed hash on failure.
#   3. Update .recursion-doctrine-hash with the new value.
#   4. Commit both files together; the doctrine + hash pair is the
#      visible signal to reviewers.
#
# If you added a new "Closed by:" referencing a binding test, also
# update scripts/recursion-check.sh to include a `run_recursion` line
# for that test — that's the operational mirror of the catalog change.

set -euo pipefail

DOCTRINE_FILE="docs/RECURSIVE_ZERONE.md"
HASH_FILE=".recursion-doctrine-hash"

if [ ! -f "$DOCTRINE_FILE" ]; then
  echo "error: $DOCTRINE_FILE not found" >&2
  exit 1
fi
if [ ! -f "$HASH_FILE" ]; then
  echo "error: $HASH_FILE not found" >&2
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

ACTUAL=$(tr -d '\r' < "$DOCTRINE_FILE" | hash_cmd | awk '{print $1}')
EXPECTED=$(tr -d '[:space:]' < "$HASH_FILE")

if [ "$ACTUAL" != "$EXPECTED" ]; then
  cat <<EOF >&2
RECURSIVE_ZERONE.md hash check failed.

Expected (from $HASH_FILE): $EXPECTED
Actual (computed):          $ACTUAL

If you intentionally amended the recursion catalog, update $HASH_FILE to:
  $ACTUAL

Then commit both files together. Reviewers see the hash bump in the PR
diff as the signal that the recursion catalog has changed, prompting
review of:
  - any new "## N." recursion added
  - any "Closed by:" binding-test addition (must also appear in
    scripts/recursion-check.sh)
  - any modification to the five-layer enforcement claims

This is the off-chain discipline that protects the catalog the same
way .creed-hash protects TRUTH_SEEKING.md: silent amendment is refused
at PR-merge time.
EOF
  exit 1
fi

echo "recursion-doctrine hash check ok ($ACTUAL)"
