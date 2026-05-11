#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════
# Recursion Check — verify every recursion in docs/RECURSIVE_ZERONE.md holds
# ═══════════════════════════════════════════════════════════════════════════
#
# Runs the binding tests for each numbered recursion in the doctrine
# (RECURSIVE_ZERONE.md). Prints PASS/FAIL per recursion. Exit 0 if all
# recursions still hold; exit 1 if any recursion's binding test fails.
#
# Use this before merging changes that touch any module participating in
# the recursion catalog (sponsorship, substrate_bridge, claiming_pot,
# vesting_rewards, knowledge, creed, work_creed).
#
# Usage:
#   scripts/recursion-check.sh         # run all
#   scripts/recursion-check.sh quick   # one-test-per-recursion sample
#
# ═══════════════════════════════════════════════════════════════════════════

set -uo pipefail

MODE="${1:-full}"

RED='\033[1;31m'
GREEN='\033[1;32m'
BLUE='\033[1;34m'
YELLOW='\033[1;33m'
RESET='\033[0m'

PASS=0
FAIL=0
TOTAL=0
FAILED_RECURSIONS=()

run_recursion() {
  local n="$1"
  local title="$2"
  local test_pattern="$3"

  TOTAL=$((TOTAL + 1))
  printf "  %b[%2d]%b %-58s " "${BLUE}" "${n}" "${RESET}" "${title}"

  # -count=1 disables Go's test cache so we get a real run.
  if go test -count=1 -run "${test_pattern}" ./tests/cross_stack/ >/tmp/recursion-check-${n}.log 2>&1; then
    printf "%bPASS%b\n" "${GREEN}" "${RESET}"
    PASS=$((PASS + 1))
  else
    printf "%bFAIL%b\n" "${RED}" "${RESET}"
    FAIL=$((FAIL + 1))
    FAILED_RECURSIONS+=("${n}: ${title}")
    if [ "${MODE}" = "verbose" ]; then
      sed -n '/^---/,/^FAIL/p' /tmp/recursion-check-${n}.log | head -20
    fi
  fi
}

echo
echo "═══════════════════════════════════════════════════════════════════"
echo "  Recursion Check — docs/RECURSIVE_ZERONE.md"
echo "═══════════════════════════════════════════════════════════════════"
echo

# Recursion 1: chain attests to its own becoming
run_recursion 1 "chain attests to its own becoming" \
  "TestZeroneSelfAdapter"

# Recursion 2: chain pays for its own self-documentation
run_recursion 2 "chain pays for its own self-documentation" \
  "TestZeroneSelf_FullEconomicLoop|TestZeroneSelf_MultipleFulfillmentsCompoundEarnings"

# Recursion 4: chain's lineage graph includes its own commits
run_recursion 4 "lineage graph includes the chain's own commits" \
  "TestRecursiveLineage_DownstreamWorkPaysUpstreamSelfAttester|TestRecursiveLineage_MultipleCitationsCompound"

# Recursion 5: creed cannot move faster than governance
run_recursion 5 "creed cannot move faster than governance" \
  "TestTruthSeeking_CreedHashIsPinned"

# Recursion 7: participation grows through participation
run_recursion 7 "participation grows through participation" \
  "TestLateBootstrap|TestScenario13e_BootstrapPotsDoNotExpire"

# Recursion 8: economy is hard-capped and self-circulating
run_recursion 8 "economy is hard-capped and self-circulating" \
  "TestEmissionCap_BootstrapClaimMintsOnDemand|TestScenario13_ZeroTeamAllocationAtGenesis|TestScenario13c_ClaimingPotMinterPermission|TestSponsorship_NoMintingHappens"

# Recursion 10a: recursion catalog audits itself
run_recursion 10 "recursion catalog audits itself" \
  "TestRecursiveZerone_TestNamesCitedInDoctrineExist"

# Recursion 10b: chain audits its own voice
run_recursion 10 "chain audits its own voice (per-event doctrine bound)" \
  "TestRecursiveVoiceAudit_EveryEventInTheLoopIsDoctrineBound"

echo
echo "═══════════════════════════════════════════════════════════════════"
if [ "${FAIL}" -eq 0 ]; then
  printf "  %bAll %d recursions hold.%b\n" "${GREEN}" "${TOTAL}" "${RESET}"
  echo "  The chain participates in its own systems; every loop terminates"
  echo "  at a verifiable artifact bound by the test layer."
  echo "═══════════════════════════════════════════════════════════════════"
  exit 0
else
  printf "  %b%d/%d recursions failed.%b\n" "${RED}" "${FAIL}" "${TOTAL}" "${RESET}"
  for r in "${FAILED_RECURSIONS[@]}"; do
    printf "    %b%s%b\n" "${YELLOW}" "${r}" "${RESET}"
  done
  echo "  Logs in /tmp/recursion-check-*.log"
  echo "═══════════════════════════════════════════════════════════════════"
  exit 1
fi
