#!/usr/bin/env bash
set -euo pipefail

# ═══════════════════════════════════════════════════════════════
# R25-6 Testnet Roleplay — All 8 Scenarios
# ═══════════════════════════════════════════════════════════════

B=./build/zeroned
H0="$HOME/.zeroned/localnet/val0"
H1="$HOME/.zeroned/localnet/val1"
H2="$HOME/.zeroned/localnet/val2"
H3="$HOME/.zeroned/localnet/val3"
NODE="tcp://localhost:26601"
CHAIN="zerone-localnet"

# Common TX flag function
tx0() { $B "$@" --keyring-backend=test --home="$H0" --chain-id="$CHAIN" --node="$NODE" --gas=200000 --fees=200000uzrn -y 2>&1; }
tx1() { $B "$@" --keyring-backend=test --home="$H1" --chain-id="$CHAIN" --node="$NODE" --gas=200000 --fees=200000uzrn -y 2>&1; }
tx2() { $B "$@" --keyring-backend=test --home="$H2" --chain-id="$CHAIN" --node="$NODE" --gas=200000 --fees=200000uzrn -y 2>&1; }
tx3() { $B "$@" --keyring-backend=test --home="$H3" --chain-id="$CHAIN" --node="$NODE" --gas=200000 --fees=200000uzrn -y 2>&1; }
q() { $B "$@" --node="$NODE" 2>&1; }
qj() { $B "$@" --node="$NODE" --output=json 2>&1; }

wait_blocks() {
    local n=$1
    local delay=$(( n * 3 ))
    echo "  ... waiting $delay seconds (~$n blocks)"
    sleep "$delay"
}

extract_event_attr() {
    local txhash=$1 event_type=$2 attr_key=$3
    qj query tx "$txhash" | python3 -c "
import sys, json
data = json.load(sys.stdin)
for evt in data.get('events', []):
    if evt.get('type') == '$event_type':
        for attr in evt.get('attributes', []):
            if attr.get('key') == '$attr_key':
                print(attr['value'])
                sys.exit(0)
print('')
"
}

get_txhash() {
    echo "$1" | grep "txhash:" | awk '{print $2}'
}

get_code() {
    echo "$1" | grep "^code:" | head -1 | awk '{print $2}'
}

ALICE=zrn1yulq2lnk5ymytum50pk7n2ypxz7557vr0hj3vs
BOB=zrn12kf7t89r200unrc9cwm9kl9f20wah5y7sztv6d
ROGUE=zrn12yhvlme06302rmj3njahm766hwpsvhvvq9pces
VAL0=zrn19x242r6eujyr3p4rjcgclve8lmnjxvmg6v4cpl
VAL1=zrn16edknx7gwp8mtl7nsm9jxe2h2gnxwkwht38cc5
VAL2=zrn1tnhw6eghqzwqyzmlka3mgk0lv7k4j6g0yym6uf
VAL3=zrn1tyxaf5jntaxfw5njfhg5yryvpdjnj4vts24jv4

RESULTS_FILE="/tmp/roleplay-results.json"
echo '{}' > "$RESULTS_FILE"

save_result() {
    local scenario=$1 key=$2 value=$3
    python3 -c "
import json
with open('$RESULTS_FILE', 'r') as f:
    data = json.load(f)
if '$scenario' not in data:
    data['$scenario'] = {}
data['$scenario']['$key'] = '$value'
with open('$RESULTS_FILE', 'w') as f:
    json.dump(data, f, indent=2)
"
}

echo "═══════════════════════════════════════════════════════════════"
echo "  SCENARIO 1: Truth Discovery (Happy Path)"
echo "═══════════════════════════════════════════════════════════════"

# Pre-compute verification materials
SALT_V0=$(openssl rand -hex 16)
SALT_V1=$(openssl rand -hex 16)
COMMIT_V0=$( (printf "accept"; printf '%s' "$SALT_V0" | xxd -r -p) | shasum -a 256 | awk '{print $1}')
COMMIT_V1=$( (printf "accept"; printf '%s' "$SALT_V1" | xxd -r -p) | shasum -a 256 | awk '{print $1}')

echo "[S1.1] Alice submits physics claim..."
S1_TX=$(tx0 tx knowledge submit-claim \
    "Gravitational constant G equals 6.674e-11 in SI" \
    general empirical 3000000 \
    --from=alice)
S1_CODE=$(get_code "$S1_TX")
S1_HASH=$(get_txhash "$S1_TX")
echo "  code=$S1_CODE txhash=$S1_HASH"
save_result "s1" "claim_code" "$S1_CODE"

sleep 4
S1_CLAIM=$(extract_event_attr "$S1_HASH" "zerone.knowledge.submit_claim" "claim_id")
S1_ROUND=$(extract_event_attr "$S1_HASH" "zerone.knowledge.verification_round_created" "round_id")
echo "  claim_id=$S1_CLAIM round_id=$S1_ROUND"
save_result "s1" "claim_id" "$S1_CLAIM"
save_result "s1" "round_id" "$S1_ROUND"

echo "[S1.2] Sage-1 (val0) commits verification..."
S1_C0=$(tx0 tx knowledge submit-commitment "$S1_ROUND" "$COMMIT_V0" --from=val0)
echo "  code=$(get_code "$S1_C0")"

sleep 2
echo "[S1.3] Sage-2 (val1) commits verification..."
S1_C1=$(tx1 tx knowledge submit-commitment "$S1_ROUND" "$COMMIT_V1" --from=val1)
echo "  code=$(get_code "$S1_C1")"

echo "[S1.4] Waiting for reveal phase..."
wait_blocks 12

echo "[S1.5] Sage-1 reveals vote..."
S1_R0=$(tx0 tx knowledge submit-reveal "$S1_ROUND" accept "$SALT_V0" --from=val0)
echo "  code=$(get_code "$S1_R0")"

sleep 2
echo "[S1.6] Sage-2 reveals vote..."
S1_R1=$(tx1 tx knowledge submit-reveal "$S1_ROUND" accept "$SALT_V1" --from=val1)
echo "  code=$(get_code "$S1_R1")"

echo "[S1.7] Waiting for aggregation..."
wait_blocks 7

echo "[S1.8] Checking round result..."
S1_ROUND_RESULT=$(q query knowledge verification-round "$S1_ROUND")
echo "$S1_ROUND_RESULT" | head -20
S1_VERDICT=$(echo "$S1_ROUND_RESULT" | grep "verdict:" | head -1 | awk '{print $2}')
save_result "s1" "verdict" "$S1_VERDICT"

# Check if fact was created
S1_FACT_ID=""
if echo "$S1_ROUND_RESULT" | grep -q "fact_id:"; then
    S1_FACT_ID=$(echo "$S1_ROUND_RESULT" | grep "fact_id:" | awk '{print $2}')
fi
# Try to get fact from claim
if [ -z "$S1_FACT_ID" ]; then
    S1_FACTS=$(q query knowledge facts-by-submitter "$ALICE")
    if echo "$S1_FACTS" | grep -q "id:"; then
        S1_FACT_ID=$(echo "$S1_FACTS" | grep "  id:" | head -1 | awk '{print $2}')
    fi
fi
echo "  fact_id=$S1_FACT_ID"
save_result "s1" "fact_id" "${S1_FACT_ID:-none}"

if [ -n "$S1_FACT_ID" ]; then
    echo "[S1.9] Bob patronises the fact..."
    S1_PAT=$(tx0 tx knowledge patronize-fact "$S1_FACT_ID" 20000000 200 --from=bob)
    echo "  patronize code=$(get_code "$S1_PAT")"
    save_result "s1" "patronize_code" "$(get_code "$S1_PAT")"
else
    echo "[S1.9] SKIP: No fact to patronise (verdict was $S1_VERDICT)"
    save_result "s1" "patronize_code" "SKIP"
fi

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "  SCENARIO 2: Challenge Flow (Adversarial)"
echo "═══════════════════════════════════════════════════════════════"

# Pre-compute
SALT_R0=$(openssl rand -hex 16)
SALT_R1=$(openssl rand -hex 16)
COMMIT_R0=$( (printf "accept"; printf '%s' "$SALT_R0" | xxd -r -p) | shasum -a 256 | awk '{print $1}')
COMMIT_R1=$( (printf "accept"; printf '%s' "$SALT_R1" | xxd -r -p) | shasum -a 256 | awk '{print $1}')

echo "[S2.1] Rogue submits bogus claim..."
S2_TX=$(tx0 tx knowledge submit-claim \
    "Speed of light varies with observer mood" \
    general empirical 1000000 \
    --from=rogue)
S2_CODE=$(get_code "$S2_TX")
S2_HASH=$(get_txhash "$S2_TX")
echo "  code=$S2_CODE txhash=$S2_HASH"
save_result "s2" "claim_code" "$S2_CODE"

sleep 4
S2_CLAIM=$(extract_event_attr "$S2_HASH" "zerone.knowledge.submit_claim" "claim_id")
S2_ROUND=$(extract_event_attr "$S2_HASH" "zerone.knowledge.verification_round_created" "round_id")
echo "  claim_id=$S2_CLAIM round_id=$S2_ROUND"

# Verify the bogus claim (stub evaluator accepts everything)
echo "[S2.2] Sage-1 and Sage-2 verify (accept — stub evaluator)..."
tx0 tx knowledge submit-commitment "$S2_ROUND" "$COMMIT_R0" --from=val0 | head -1
sleep 2
tx1 tx knowledge submit-commitment "$S2_ROUND" "$COMMIT_R1" --from=val1 | head -1

wait_blocks 12
tx0 tx knowledge submit-reveal "$S2_ROUND" accept "$SALT_R0" --from=val0 | head -1
sleep 2
tx1 tx knowledge submit-reveal "$S2_ROUND" accept "$SALT_R1" --from=val1 | head -1

wait_blocks 7

# Get fact ID
S2_ROUND_RESULT=$(q query knowledge verification-round "$S2_ROUND")
echo "$S2_ROUND_RESULT" | grep -E "verdict:|phase:"
S2_VERDICT=$(echo "$S2_ROUND_RESULT" | grep "verdict:" | head -1 | awk '{print $2}')
save_result "s2" "verdict" "$S2_VERDICT"

# Get Rogue's fact
S2_FACTS=$(q query knowledge facts-by-submitter "$ROGUE")
S2_FACT_ID=""
if echo "$S2_FACTS" | grep -q "id:"; then
    S2_FACT_ID=$(echo "$S2_FACTS" | grep "  id:" | head -1 | awk '{print $2}')
fi
echo "  rogue_fact_id=$S2_FACT_ID"
save_result "s2" "fact_id" "${S2_FACT_ID:-none}"

if [ -n "$S2_FACT_ID" ]; then
    echo "[S2.3] Alice challenges the bogus fact..."
    S2_CHAL=$(tx0 tx knowledge challenge-fact "$S2_FACT_ID" 11000000 \
        "No empirical basis - contradicts special relativity" --from=alice)
    echo "  challenge code=$(get_code "$S2_CHAL")"
    save_result "s2" "challenge_code" "$(get_code "$S2_CHAL")"

    echo "[S2.4] Initiating dispute..."
    S2_DISP=$(tx0 tx disputes initiate-dispute "$S2_FACT_ID" 1000000 \
        "Claim contradicts well-established physics" --from=alice)
    S2_DISP_CODE=$(get_code "$S2_DISP")
    S2_DISP_HASH=$(get_txhash "$S2_DISP")
    echo "  dispute code=$S2_DISP_CODE"
    save_result "s2" "dispute_code" "$S2_DISP_CODE"

    if [ "$S2_DISP_CODE" = "0" ]; then
        sleep 4
        S2_DISPUTE_ID=$(extract_event_attr "$S2_DISP_HASH" "zerone.disputes.dispute_initiated" "dispute_id")
        if [ -z "$S2_DISPUTE_ID" ]; then
            # Try alternate event name
            S2_DISPUTE_ID=$(extract_event_attr "$S2_DISP_HASH" "zerone.disputes.initiate_dispute" "dispute_id")
        fi
        echo "  dispute_id=$S2_DISPUTE_ID"
        save_result "s2" "dispute_id" "${S2_DISPUTE_ID:-none}"

        if [ -n "$S2_DISPUTE_ID" ]; then
            echo "[S2.5] Arbiter (val2) votes on dispute..."
            S2_VOTE=$(tx2 tx disputes arbiter-vote "$S2_DISPUTE_ID" "challenger" \
                "Claim is pseudoscience" --from=val2)
            echo "  vote code=$(get_code "$S2_VOTE")"
            save_result "s2" "arbiter_vote_code" "$(get_code "$S2_VOTE")"
        fi
    fi
else
    echo "[S2.3-5] SKIP: Bogus claim didn't become a fact (verdict=$S2_VERDICT)"
    save_result "s2" "challenge_code" "SKIP"
fi

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "  SCENARIO 3: Partnership Collaboration"
echo "═══════════════════════════════════════════════════════════════"

echo "[S3.1] Alice proposes partnership with Sage-1 (val0)..."
S3_PROP=$(tx0 tx partnerships propose "$VAL0" 100000000 1 --from=alice)
S3_PROP_CODE=$(get_code "$S3_PROP")
S3_PROP_HASH=$(get_txhash "$S3_PROP")
echo "  propose code=$S3_PROP_CODE"
save_result "s3" "propose_code" "$S3_PROP_CODE"

if [ "$S3_PROP_CODE" = "0" ]; then
    sleep 4
    # Get partnership ID from events
    S3_PID=$(extract_event_attr "$S3_PROP_HASH" "zerone.partnerships.partnership_proposed" "partnership_id")
    if [ -z "$S3_PID" ]; then
        S3_PID=$(extract_event_attr "$S3_PROP_HASH" "zerone.partnerships.propose_partnership" "partnership_id")
    fi
    # Try listing partnerships
    if [ -z "$S3_PID" ]; then
        S3_PID=$(qj query partnerships partnerships | python3 -c "
import sys,json
data=json.load(sys.stdin)
for p in data.get('partnerships', []):
    print(p.get('id', ''))
    break
" 2>/dev/null || echo "")
    fi
    echo "  partnership_id=$S3_PID"
    save_result "s3" "partnership_id" "${S3_PID:-none}"

    if [ -n "$S3_PID" ]; then
        echo "[S3.2] Sage-1 (val0) accepts partnership..."
        S3_ACC=$(tx0 tx partnerships accept "$S3_PID" 100000000 --from=val0)
        echo "  accept code=$(get_code "$S3_ACC")"
        save_result "s3" "accept_code" "$(get_code "$S3_ACC")"

        sleep 3
        echo "[S3.3] Querying partnership..."
        q query partnerships partnership "$S3_PID" | head -20
    fi
fi

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "  SCENARIO 4: Domain Expansion"
echo "═══════════════════════════════════════════════════════════════"

echo "[S4.1] Bob proposes 'bioethics' domain..."
S4_PROP=$(tx0 tx knowledge propose-domain "bioethics" \
    "Bioethics and medical ethics frameworks" 4 --from=bob)
S4_PROP_CODE=$(get_code "$S4_PROP")
echo "  propose code=$S4_PROP_CODE"
save_result "s4" "propose_code" "$S4_PROP_CODE"

sleep 3
echo "[S4.2] Sage-1 endorses..."
S4_E1=$(tx0 tx knowledge endorse-domain-proposal "bioethics" --from=val0)
echo "  endorse1 code=$(get_code "$S4_E1")"

sleep 3
echo "[S4.3] Sage-2 endorses..."
S4_E2=$(tx1 tx knowledge endorse-domain-proposal "bioethics" --from=val1)
echo "  endorse2 code=$(get_code "$S4_E2")"

sleep 3
echo "[S4.4] Arbiter endorses..."
S4_E3=$(tx2 tx knowledge endorse-domain-proposal "bioethics" --from=val2)
echo "  endorse3 code=$(get_code "$S4_E3")"

sleep 3
echo "[S4.5] Checking domain status..."
S4_DOM=$(q query knowledge domain "bioethics")
echo "$S4_DOM"
S4_STATUS=$(echo "$S4_DOM" | grep "status:" | awk '{print $2}')
save_result "s4" "domain_status" "$S4_STATUS"

echo "[S4.6] Bob submits first claim in bioethics..."
S4_CLAIM=$(tx0 tx knowledge submit-claim \
    "Informed consent requires understanding risks benefits and alternatives" \
    bioethics analytic 2000000 --from=bob)
echo "  claim code=$(get_code "$S4_CLAIM")"
save_result "s4" "first_claim_code" "$(get_code "$S4_CLAIM")"

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "  SCENARIO 5: Qualification Gate Test"
echo "═══════════════════════════════════════════════════════════════"

echo "[S5.1] Sage-2 qualifies in general domain..."
S5_QUAL=$(tx1 tx qualification qualify-by-stake "general" 100000000 --from=val1)
echo "  qualify code=$(get_code "$S5_QUAL")"
save_result "s5" "qualify_code" "$(get_code "$S5_QUAL")"

sleep 3

# Get Bob's claim round from bioethics (from S4)
S4_CLAIM_HASH=$(get_txhash "$S4_CLAIM")
sleep 2
S5_BIOETHICS_ROUND=""
if [ -n "$S4_CLAIM_HASH" ]; then
    S5_BIOETHICS_ROUND=$(extract_event_attr "$S4_CLAIM_HASH" "zerone.knowledge.verification_round_created" "round_id")
fi
echo "  bioethics_round=$S5_BIOETHICS_ROUND"

if [ -n "$S5_BIOETHICS_ROUND" ]; then
    echo "[S5.2] Sage-2 tries to verify in bioethics (NOT qualified)..."
    SALT_GATE=$(openssl rand -hex 16)
    COMMIT_GATE=$( (printf "accept"; printf '%s' "$SALT_GATE" | xxd -r -p) | shasum -a 256 | awk '{print $1}')
    S5_GATE=$(tx1 tx knowledge submit-commitment "$S5_BIOETHICS_ROUND" "$COMMIT_GATE" --from=val1)
    S5_GATE_CODE=$(get_code "$S5_GATE")
    echo "  unqualified_commit code=$S5_GATE_CODE"
    save_result "s5" "unqualified_commit_code" "$S5_GATE_CODE"
    if [ "$S5_GATE_CODE" = "0" ]; then
        echo "  ⚠ QUALIFICATION GATE NOT ENFORCED — accepted unqualified verifier"
        save_result "s5" "gate_enforced" "NO"
    else
        echo "  ✓ Qualification gate enforced — rejected unqualified verifier"
        save_result "s5" "gate_enforced" "YES"
    fi
else
    echo "[S5.2] SKIP: No bioethics round to test"
    save_result "s5" "gate_enforced" "SKIP"
fi

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "  SCENARIO 6: Research Bounty"
echo "═══════════════════════════════════════════════════════════════"

CURRENT_BLOCK=$(q status | python3 -c "import sys,json; print(json.load(sys.stdin)['sync_info']['latest_block_height'])")
DEADLINE=$(( CURRENT_BLOCK + 50000 ))

echo "[S6.1] Alice creates research bounty..."
S6_BOUNTY=$(tx0 tx research create-bounty \
    "Replicate gravitational constant measurement" \
    "Independent measurement of G using torsion balance" \
    50000000 "$DEADLINE" --from=alice)
S6_BOUNTY_CODE=$(get_code "$S6_BOUNTY")
S6_BOUNTY_HASH=$(get_txhash "$S6_BOUNTY")
echo "  bounty code=$S6_BOUNTY_CODE"
save_result "s6" "create_bounty_code" "$S6_BOUNTY_CODE"

if [ "$S6_BOUNTY_CODE" = "0" ]; then
    sleep 4
    S6_BOUNTY_ID=$(extract_event_attr "$S6_BOUNTY_HASH" "zerone.research.bounty_created" "bounty_id")
    if [ -z "$S6_BOUNTY_ID" ]; then
        S6_BOUNTY_ID=$(extract_event_attr "$S6_BOUNTY_HASH" "zerone.research.create_bounty" "bounty_id")
    fi
    # Fallback: try querying bounties
    if [ -z "$S6_BOUNTY_ID" ]; then
        S6_BOUNTY_ID=$(qj query research bounties | python3 -c "
import sys,json
data=json.load(sys.stdin)
for b in data.get('bounties', []):
    print(b.get('id', ''))
    break
" 2>/dev/null || echo "")
    fi
    echo "  bounty_id=$S6_BOUNTY_ID"
    save_result "s6" "bounty_id" "${S6_BOUNTY_ID:-none}"

    if [ -n "$S6_BOUNTY_ID" ]; then
        echo "[S6.2] Sage-1 claims bounty..."
        S6_CLAIM_B=$(tx0 tx research claim-bounty "$S6_BOUNTY_ID" --from=val0)
        echo "  claim_bounty code=$(get_code "$S6_CLAIM_B")"
        save_result "s6" "claim_bounty_code" "$(get_code "$S6_CLAIM_B")"

        echo "[S6.3] Sage-1 tries to fulfil bounty..."
        sleep 3
        S6_FULFIL=$(tx0 tx research fulfill-bounty "$S6_BOUNTY_ID" "$VAL0" --from=val0)
        S6_FULFIL_CODE=$(get_code "$S6_FULFIL")
        echo "  fulfil code=$S6_FULFIL_CODE"
        save_result "s6" "fulfil_code" "$S6_FULFIL_CODE"
        if [ "$S6_FULFIL_CODE" != "0" ]; then
            echo "  ⚠ Fulfil requires governance authority (known issue from R25-5)"
        fi
    fi
fi

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "  SCENARIO 7: Capture Defense (Domain Flooding)"
echo "═══════════════════════════════════════════════════════════════"

echo "[S7.1] Rogue submits 5 claims rapidly..."
for i in 1 2 3 4 5; do
    S7_FLOOD=$(tx0 tx knowledge submit-claim \
        "Dubious claim number $i to flood general domain" \
        general empirical 1000000 --from=rogue)
    echo "  claim $i code=$(get_code "$S7_FLOOD")"
    sleep 3
done
save_result "s7" "flood_claims" "5"

echo "[S7.2] Analyzing domain for capture..."
S7_ANALYZE=$(tx0 tx capture-defense analyze-domain "general" --from=val0)
echo "  analyze code=$(get_code "$S7_ANALYZE")"
save_result "s7" "analyze_code" "$(get_code "$S7_ANALYZE")"

echo "[S7.3] Alice submits capture challenge..."
S7_CCHAL=$(tx0 tx capture-challenge submit-challenge "general" \
    "Account $ROGUE is flooding domain with low-quality claims" \
    10000000 --from=alice)
echo "  capture_challenge code=$(get_code "$S7_CCHAL")"
save_result "s7" "capture_challenge_code" "$(get_code "$S7_CCHAL")"

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "  SCENARIO 8: Coercion Signal (Partnership Safety)"
echo "═══════════════════════════════════════════════════════════════"

# Use partnership from S3 if it exists
S3_PID_FINAL=""
S3_PID_DATA=$(cat "$RESULTS_FILE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('s3',{}).get('partnership_id',''))" 2>/dev/null || echo "")
S3_PID_FINAL="$S3_PID_DATA"

if [ -n "$S3_PID_FINAL" ] && [ "$S3_PID_FINAL" != "none" ]; then
    echo "[S8.1] Sage-1 raises coercion signal on partnership $S3_PID_FINAL..."
    S8_COERCE=$(tx0 tx partnerships raise-coercion "$S3_PID_FINAL" --from=val0)
    echo "  coercion code=$(get_code "$S8_COERCE")"
    save_result "s8" "coercion_code" "$(get_code "$S8_COERCE")"

    sleep 3
    echo "[S8.2] Checking partnership status..."
    S8_STATUS=$(q query partnerships partnership "$S3_PID_FINAL")
    echo "$S8_STATUS" | grep -E "status:|cooperation_score:"
    save_result "s8" "post_coercion_status" "$(echo "$S8_STATUS" | grep 'status:' | head -1 | awk '{print $2}')"

    echo "[S8.3] Sage-1 triggers safety freeze..."
    S8_FREEZE=$(tx0 tx partnerships safety-freeze "$S3_PID_FINAL" --from=val0)
    echo "  freeze code=$(get_code "$S8_FREEZE")"
    save_result "s8" "freeze_code" "$(get_code "$S8_FREEZE")"

    sleep 3
    echo "[S8.4] Checking frozen partnership..."
    S8_FROZEN=$(q query partnerships partnership "$S3_PID_FINAL")
    echo "$S8_FROZEN" | grep -E "status:|cooperation_score:"
    save_result "s8" "post_freeze_status" "$(echo "$S8_FROZEN" | grep 'status:' | head -1 | awk '{print $2}')"
else
    echo "[S8] SKIP: No partnership from S3 to test"
    save_result "s8" "coercion_code" "SKIP"
fi

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "  RESULTS SUMMARY"
echo "═══════════════════════════════════════════════════════════════"
cat "$RESULTS_FILE"
echo ""
echo "Done. Results saved to $RESULTS_FILE"
