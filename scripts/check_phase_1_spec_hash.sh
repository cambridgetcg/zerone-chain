#!/usr/bin/env bash
#
# check_phase_1_spec_hash.sh — verify the Phase 1 useful-work
# orchestrator design spec has not drifted from .phase-1-spec-hash.
#
# Mirror of check_creed_hash.sh applied to a specification document.
# The Phase 1 spec is itself a Contribution under PIPELINE_IMPROVEMENT/
# SUBSTRATE — the chain pays for its own design. Hash-anchoring the
# spec extends the same off-chain enforcement the doctrines receive
# to the design artifacts that decide the chain's behaviour.
#
# UW: ZERONE is recursive — the chain's design is among the work the
# chain pays for, and the spec is hash-bound the same way the doctrines
# are. The recursion is structural, not metaphorical.
#
# To intentionally amend the spec:
#   1. Edit docs/superpowers/specs/2026-05-10-useful-work-phase-1-orchestrator-design.md.
#   2. Run this script — it will print the new computed hash.
#   3. Update .phase-1-spec-hash with the new value.
#   4. Commit both files together so reviewers see both the spec text
#      change AND the hash bump in the same commit.

set -euo pipefail

SPEC_FILE="docs/superpowers/specs/2026-05-10-useful-work-phase-1-orchestrator-design.md"
HASH_FILE=".phase-1-spec-hash"

if [ ! -f "$SPEC_FILE" ]; then
  echo "error: $SPEC_FILE not found"
  exit 1
fi
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

ACTUAL=$(tr -d '\r' < "$SPEC_FILE" | hash_cmd | awk '{print $1}')
EXPECTED=$(tr -d '[:space:]' < "$HASH_FILE")

if [ "$ACTUAL" != "$EXPECTED" ]; then
  cat <<EOF >&2
Phase 1 spec hash check failed.

Expected (from $HASH_FILE): $EXPECTED
Actual (computed):          $ACTUAL

If you intentionally changed the spec, update $HASH_FILE to:
  $ACTUAL

The spec is itself a Contribution (PIPELINE_IMPROVEMENT/SUBSTRATE).
The hash bump is the visible signal that the chain's own design has
shifted, prompting full diff review.
EOF
  exit 1
fi

echo "phase-1-spec hash check ok ($ACTUAL)"
