#!/bin/bash
set -e
export PATH=$HOME/go/bin:$PATH

echo "╔══════════════════════════════════════════════╗"
echo "║  ZERONE Knowledge Ecology — Live Test        ║"
echo "╚══════════════════════════════════════════════╝"

HEIGHT=$(zeroned status 2>&1 | python3 -c "import sys,json; print(json.load(sys.stdin)['sync_info']['latest_block_height'])")
echo "Starting at block $HEIGHT"

# ─── PHASE 1: Submit 4 claims ───────────────────────────────
echo ""
echo "━━━ Phase 1: Submit Claims ━━━"

echo "  [Alice] Water boils at 100°C (vague)"
zeroned tx knowledge submit-claim \
  "Water boils at 100 degrees Celsius at standard pressure" \
  physics peer_reviewed 1000000 \
  --subject "water boiling point" --predicate "equals 100°C" --scope "standard pressure" \
  --from alice --keyring-backend test --gas 300000 --fees 300000uzrn -y 2>&1 | grep "code:"
sleep 3

echo "  [Bob] Water boils at 99.9743°C (precise — same niche)"
zeroned tx knowledge submit-claim \
  "Water boils at exactly 99.9743 degrees Celsius at 101.325 kPa as defined by the ITS-90 temperature scale" \
  physics peer_reviewed 1000000 \
  --subject "water boiling point" --predicate "equals 99.9743°C" --scope "standard pressure" \
  --from bob --keyring-backend test --gas 300000 --fees 300000uzrn -y 2>&1 | grep "code:"
sleep 3

echo "  [Carol] Water boils at ~95°C at altitude (different niche)"
zeroned tx knowledge submit-claim \
  "Water boils at approximately 95 degrees Celsius at 1524 meters altitude due to reduced atmospheric pressure" \
  physics peer_reviewed 1000000 \
  --subject "water boiling point" --predicate "equals ~95°C" --scope "1524m altitude" \
  --from carol --keyring-backend test --gas 300000 --fees 300000uzrn -y 2>&1 | grep "code:"
sleep 3

echo "  [Dave] Maillard reaction (different domain)"
zeroned tx knowledge submit-claim \
  "The Maillard reaction between amino acids and reducing sugars accelerates significantly above 140 degrees Celsius" \
  chemistry peer_reviewed 1000000 \
  --subject "Maillard reaction temperature" --predicate "accelerates above 140°C" --scope "cooking chemistry" \
  --from dave --keyring-backend test --gas 300000 --fees 300000uzrn -y 2>&1 | grep "code:"
sleep 3

# ─── Extract round IDs ──────────────────────────────────────
echo ""
echo "━━━ Extracting Round IDs ━━━"
ROUND_IDS=$(zeroned query txs --query "tx.height>0" --output json 2>&1 | python3 -c "
import sys,json
d=json.load(sys.stdin)
rounds = []
for tx in d.get('txs',[]):
    for ev in tx.get('events',[]):
        if ev['type'] == 'zerone.knowledge.verification_round_created':
            attrs = {a['key']:a['value'] for a in ev['attributes']}
            rounds.append(attrs.get('round_id',''))
for r in rounds:
    print(r)
")
echo "$ROUND_IDS"
ROUND_ARRAY=($ROUND_IDS)

# ─── PHASE 2: Submit commitments ────────────────────────────
echo ""
echo "━━━ Phase 2: Submit Commitments ━━━"
SALT_HEX="deadbeef01020304"
SALT_BYTES=$(python3 -c "print(bytes.fromhex('$SALT_HEX').decode('latin-1'))")

for ROUND in "${ROUND_ARRAY[@]}"; do
  # Reveal check uses SHA256(vote_string + salt_bytes)
  HASH=$(python3 -c "
import hashlib
h = hashlib.sha256()
h.update(b'accept')
h.update(bytes.fromhex('$SALT_HEX'))
print(h.hexdigest())
")
  echo "  Committing to round $ROUND (hash: ${HASH:0:16}...)"
  zeroned tx knowledge submit-commitment $ROUND $HASH \
    --from validator --keyring-backend test \
    --gas 300000 --fees 300000uzrn -y 2>&1 | grep "code:"
  sleep 3
done

# ─── Wait for reveal phase ──────────────────────────────────
echo ""
echo "━━━ Waiting for Reveal Phase ━━━"
echo "  Commit phase = 50 blocks (~2 min). Waiting..."

while true; do
  HEIGHT=$(zeroned status 2>&1 | python3 -c "import sys,json; print(json.load(sys.stdin)['sync_info']['latest_block_height'])")
  # Check if first round is in reveal phase
  PHASE=$(zeroned query knowledge verification-round ${ROUND_ARRAY[0]} --output json 2>&1 | python3 -c "
import sys,json
try:
    d=json.load(sys.stdin)
    print(d.get('round',{}).get('phase',''))
except:
    print('unknown')
" 2>/dev/null)
  echo "  Block $HEIGHT — Phase: $PHASE"
  if [[ "$PHASE" == "2" || "$PHASE" == *"REVEAL"* ]]; then
    echo "  ✓ Reveal phase reached!"
    break
  fi
  if [[ "$PHASE" == "4" || "$PHASE" == *"COMPLETE"* ]]; then
    echo "  ⚠️ Round already complete — missed the window!"
    break
  fi
  sleep 5
done

# ─── PHASE 3: Submit reveals ────────────────────────────────
echo ""
echo "━━━ Phase 3: Submit Reveals ━━━"

for ROUND in "${ROUND_ARRAY[@]}"; do
  echo "  Revealing for round $ROUND"
  zeroned tx knowledge submit-reveal $ROUND accept $SALT_HEX \
    --from validator --keyring-backend test \
    --gas 300000 --fees 300000uzrn -y 2>&1 | grep -E "code:|raw_log"
  sleep 3
done

# ─── Wait for aggregation ───────────────────────────────────
echo ""
echo "━━━ Waiting for Aggregation ━━━"
while true; do
  HEIGHT=$(zeroned status 2>&1 | python3 -c "import sys,json; print(json.load(sys.stdin)['sync_info']['latest_block_height'])")
  PHASE=$(zeroned query knowledge verification-round ${ROUND_ARRAY[0]} --output json 2>&1 | python3 -c "
import sys,json
try:
    d=json.load(sys.stdin)
    print(d.get('round',{}).get('phase',''))
except:
    print('unknown')
" 2>/dev/null)
  echo "  Block $HEIGHT — Phase: $PHASE"
  if [[ "$PHASE" == "4" || "$PHASE" == *"COMPLETE"* ]]; then
    echo "  ✓ Aggregation complete!"
    break
  fi
  sleep 5
done

# ─── PHASE 4: Check the ecology ─────────────────────────────
echo ""
echo "━━━ Phase 4: Knowledge Ecology Status ━━━"

zeroned query knowledge facts --output json 2>&1 | python3 -c "
import sys,json
d=json.load(sys.stdin)
facts = d.get('facts',[])
print(f'Total facts: {len(facts)}')
print()
for f in facts:
    s = f.get('structure',{}) or {}
    subj = s.get('subject','?')
    scope = s.get('scope','?')
    nk = f.get('niche_key','')
    print(f'📄 Fact {f[\"id\"][:12]}...')
    print(f'   Subject: {subj}')
    print(f'   Scope:   {scope}')
    print(f'   Domain:  {f.get(\"domain\",\"?\")}')
    print(f'   Status:  {f.get(\"status\",\"?\")}')
    print(f'   Energy:  {f.get(\"energy\",0)} / {f.get(\"energy_cap\",0)}')
    print(f'   Fitness: {f.get(\"fitness_score\",0)}')
    print(f'   Niche:   key={nk[:16]}... leader={f.get(\"niche_leader\",\"?\")} rank={f.get(\"niche_rank\",\"?\")} size={f.get(\"niche_size\",\"?\")}')
    print(f'   CompTax: {f.get(\"competition_tax\",0)}')
    print()
"

echo ""
echo "━━━ Ecology Test Complete ━━━"
echo "Next: wait for fitness epoch (block ~30) to see competition scores update"
