#!/bin/bash
set -e
export PATH=$HOME/go/bin:$PATH

# Fact IDs (from current chain state)
ALICE_VAGUE="4a115709a20a0b9469e77afd3dee65f4"    # Water boils at 100°C
BOB_PRECISE="78dfe36c7b612bdb5d37edeffb0a6f5c"    # Water boils at 99.9743°C
CAROL_ALT="ab8ae217657e3c7ccffc2afc213511a6"      # Water boils at ~95°C at altitude
DAVE_MAILLARD="5dc21961fbd4e253cf91cec8697b5d02"  # Maillard reaction

echo "╔══════════════════════════════════════════════════════════╗"
echo "║   ZERONE Ecology Simulation — Agent Roleplay            ║"
echo "╠══════════════════════════════════════════════════════════╣"
echo "║  Alice submitted: 'Water boils at 100°C' (vague)       ║"
echo "║  Bob submitted:   'Water boils at 99.9743°C' (precise) ║"
echo "║  Both in same niche. Let natural selection decide.      ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""

snapshot() {
  echo "━━━ Ecology Snapshot (block $(zeroned status 2>&1 | python3 -c "import sys,json; print(json.load(sys.stdin)['sync_info']['latest_block_height'])")) ━━━"
  zeroned query knowledge facts --output json 2>&1 | python3 -c "
import sys,json
d=json.load(sys.stdin)
for f in sorted(d.get('facts',[]), key=lambda x: x.get('niche_key','')):
    s = f.get('structure',{}) or {}
    leader = '👑' if f.get('niche_leader') else '  '
    tax = int(f.get('competition_tax',0) or 0)
    tax_str = f'⚠️  tax={tax}' if tax > 0 else ''
    status_map = {1:'SUBMITTED',2:'VERIFIED',3:'ACTIVE',4:'PROVISIONAL',5:'AT_RISK',6:'EXPIRED',7:'PRUNED',8:'CHALLENGED'}
    status = status_map.get(f.get('status',0), f'status={f.get(\"status\")}')
    energy = int(f.get('energy',0) or 0)
    fitness = int(f.get('fitness_score',0) or 0)
    qcount = int(f.get('query_count',0) or 0)
    rank = f.get('niche_rank','?')
    print(f'{leader} {s.get(\"subject\",\"?\"):25s} | {s.get(\"predicate\",\"?\"):25s} | E={energy:5d} F={fitness:7d} Q={qcount:3d} R={rank} {status} {tax_str}')
"
  echo ""
}

# ─── Initial state ──────────────────────────────────────────
snapshot

# ─── Act 1: Agent queries pour in for the PRECISE fact ──────
echo "🎭 Act 1: Research agents discover Bob's precise fact is more useful"
echo "   Simulating 20 queries to Bob's 99.9743°C fact via gRPC..."
for i in $(seq 1 20); do
  curl -s "http://localhost:1317/zerone/knowledge/v1/facts/$BOB_PRECISE" > /dev/null 2>&1
done
echo "   Simulating 2 queries to Alice's 100°C fact (low demand)..."
for i in $(seq 1 2); do
  curl -s "http://localhost:1317/zerone/knowledge/v1/facts/$ALICE_VAGUE" > /dev/null 2>&1
done
echo "   Simulating 5 queries to Carol's altitude fact..."
for i in $(seq 1 5); do
  curl -s "http://localhost:1317/zerone/knowledge/v1/facts/$CAROL_ALT" > /dev/null 2>&1
done
echo ""

# ─── Act 2: Bob's fact gets patronized ──────────────────────
echo "🎭 Act 2: Eve patronizes Bob's precise fact (values accuracy)"
zeroned tx knowledge patronize-fact $BOB_PRECISE 2000000 1000 \
  --from eve --keyring-backend test --gas 300000 --fees 300000uzrn -y 2>&1 | grep "code:"
sleep 3
echo ""

# ─── Act 3: A derivative claim cites Bob's fact ────────────
echo "🎭 Act 3: Carol submits a derivative claim citing Bob's precise fact"
zeroned tx knowledge submit-claim \
  "The specific heat capacity of water at its boiling point of 99.9743°C is 4.216 kJ per kg per K" \
  physics peer_reviewed 1000000 \
  --subject "water specific heat at boiling" \
  --predicate "equals 4.216 kJ/kg/K" \
  --scope "at boiling point" \
  --references "$BOB_PRECISE" \
  --from carol --keyring-backend test \
  --gas 300000 --fees 300000uzrn -y 2>&1 | grep "code:"
sleep 3
echo ""

# ─── Wait for fitness epoch ─────────────────────────────────
echo "⏳ Waiting for fitness epoch (every 30 blocks)..."
START_HEIGHT=$(zeroned status 2>&1 | python3 -c "import sys,json; print(json.load(sys.stdin)['sync_info']['latest_block_height'])")
TARGET=$((((START_HEIGHT / 30) + 1) * 30))
echo "   Current: block $START_HEIGHT — next epoch at block $TARGET"

while true; do
  HEIGHT=$(zeroned status 2>&1 | python3 -c "import sys,json; print(json.load(sys.stdin)['sync_info']['latest_block_height'])")
  if [ "$HEIGHT" -ge "$TARGET" ]; then
    echo "   ✓ Fitness epoch reached at block $HEIGHT"
    break
  fi
  sleep 3
done
sleep 3
echo ""

# ─── Post-epoch snapshot ────────────────────────────────────
echo "📊 After first fitness epoch:"
snapshot

# ─── Act 4: More queries compound Bob's advantage ──────────
echo "🎭 Act 4: Word spreads — agents overwhelmingly prefer the precise fact"
echo "   Simulating 50 more queries to Bob's fact..."
for i in $(seq 1 50); do
  curl -s "http://localhost:1317/zerone/knowledge/v1/facts/$BOB_PRECISE" > /dev/null 2>&1
done
echo "   Alice's fact gets 0 new queries (agents stopped using it)"
echo ""

# ─── Wait for next epoch ────────────────────────────────────
echo "⏳ Waiting for next fitness epoch..."
TARGET=$((TARGET + 30))
while true; do
  HEIGHT=$(zeroned status 2>&1 | python3 -c "import sys,json; print(json.load(sys.stdin)['sync_info']['latest_block_height'])")
  if [ "$HEIGHT" -ge "$TARGET" ]; then
    echo "   ✓ Second epoch at block $HEIGHT"
    break
  fi
  sleep 3
done
sleep 3
echo ""

echo "📊 After second fitness epoch:"
snapshot

# ─── Act 5: One more epoch to see competition tax bite ──────
echo "🎭 Act 5: Another epoch passes — Alice's fact starves under competition tax"
echo "   50 more queries to Bob..."
for i in $(seq 1 50); do
  curl -s "http://localhost:1317/zerone/knowledge/v1/facts/$BOB_PRECISE" > /dev/null 2>&1
done

TARGET=$((TARGET + 30))
echo "⏳ Waiting for third epoch (block $TARGET)..."
while true; do
  HEIGHT=$(zeroned status 2>&1 | python3 -c "import sys,json; print(json.load(sys.stdin)['sync_info']['latest_block_height'])")
  if [ "$HEIGHT" -ge "$TARGET" ]; then
    echo "   ✓ Third epoch at block $HEIGHT"
    break
  fi
  sleep 3
done
sleep 3
echo ""

echo "📊 After third fitness epoch — natural selection in action:"
snapshot

echo "╔══════════════════════════════════════════════════════════╗"
echo "║              Simulation Complete                        ║"
echo "╠══════════════════════════════════════════════════════════╣"
echo "║  Better knowledge wins. That's the design.             ║"
echo "╚══════════════════════════════════════════════════════════╝"
