#!/bin/bash
# Proto-Go Consistency Audit
# Verifies that generated Go code matches proto definitions.
# Exit 1 if any mismatch found.
#
# Usage: bash scripts/proto-audit.sh
# Or:    make proto-check

set -e

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

FAIL=0

echo "=== Step 1: Regenerating proto ==="
make proto-gen

echo ""
echo "=== Step 2: Checking for drift in generated files ==="
if ! git diff --quiet -- '*.pb.go' '*.pb.gw.go'; then
    echo "ERROR: Proto-generated files are stale. Run 'make proto-gen' and commit the results."
    echo ""
    git diff --stat -- '*.pb.go' '*.pb.gw.go'
    FAIL=1
else
    echo "OK: No drift detected."
fi

echo ""
echo "=== Step 3: Checking for hand-rolled gRPC services ==="
EXT_FILES=$(find x/ -name "query_ext.go" -path "*/types/*" 2>/dev/null || true)
if [ -n "$EXT_FILES" ]; then
    echo "ERROR: Hand-rolled gRPC services found (must migrate to proto Query service):"
    echo "$EXT_FILES"
    FAIL=1
else
    echo "OK: No query_ext.go files."
fi

echo ""
echo "=== Step 4: Checking for RegisterQueryExtServer ==="
EXT_REGS=$(grep -rn "RegisterQueryExtServer" x/*/module.go 2>/dev/null || true)
if [ -n "$EXT_REGS" ]; then
    echo "ERROR: QueryExt registrations found (must use proto-generated services):"
    echo "$EXT_REGS"
    FAIL=1
else
    echo "OK: No RegisterQueryExtServer calls."
fi

if [ "$FAIL" -ne 0 ]; then
    echo ""
    echo "FAILED: Proto consistency check found issues."
    exit 1
fi

echo ""
echo "All proto files consistent."
