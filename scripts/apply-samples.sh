#!/usr/bin/env bash
# apply-samples.sh — apply BridgeTemplate + VXLANTemplate and verify per-node CRs are created
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

log() { echo "==> $*"; }

# ── apply templates ───────────────────────────────────────────────────────────
log "Applying BridgeTemplate..."
kubectl apply -f "$SCRIPT_DIR/samples/bridgetemplate.yaml"

log "Applying VXLANTemplate..."
kubectl apply -f "$SCRIPT_DIR/samples/vxlantemplate.yaml"

# ── count expected nodes ──────────────────────────────────────────────────────
WORKER_COUNT=$(kubectl get nodes -l kvnet.kojuro.date/role=worker --no-headers 2>/dev/null | wc -l | tr -d ' ')
if [[ "$WORKER_COUNT" -eq 0 ]]; then
  echo "ERROR: No nodes labeled 'kvnet.kojuro.date/role=worker' found."
  echo "       Run scripts/setup.sh first, or label nodes manually:"
  echo "       kubectl label node <node-name> kvnet.kojuro.date/role=worker"
  exit 1
fi
log "Expecting $WORKER_COUNT Bridge CR(s) and $WORKER_COUNT VXLAN CR(s) to be created..."

# ── wait for CRs ─────────────────────────────────────────────────────────────
wait_for_crs() {
  local kind="$1" expected="$2" timeout=60
  local deadline=$(( $(date +%s) + timeout ))
  while true; do
    local count
    count=$(kubectl get "$kind" --no-headers 2>/dev/null | wc -l | tr -d ' ')
    if [[ "$count" -ge "$expected" ]]; then
      echo "    OK: found $count $kind CR(s)"
      return 0
    fi
    if (( $(date +%s) > deadline )); then
      echo "ERROR: timed out. Expected $expected $kind CR(s), got $count." >&2
      kubectl get "$kind" 2>/dev/null || true
      return 1
    fi
    sleep 3
  done
}

wait_for_crs bridges "$WORKER_COUNT"
wait_for_crs vxlans  "$WORKER_COUNT"

# ── show results ──────────────────────────────────────────────────────────────
echo ""
log "Bridge CRs:"
kubectl get bridges -o wide

echo ""
log "VXLAN CRs:"
kubectl get vxlans -o wide

echo ""
log "BridgeTemplate status:"
kubectl get bridgetemplate br0 -o jsonpath='{.status.conditions[*]}' | python3 -m json.tool 2>/dev/null \
  || kubectl get bridgetemplate br0 -o yaml | grep -A10 'conditions:'

echo ""
log "VXLANTemplate status:"
kubectl get vxlantemplate vxlan0 -o jsonpath='{.status.conditions[*]}' | python3 -m json.tool 2>/dev/null \
  || kubectl get vxlantemplate vxlan0 -o yaml | grep -A10 'conditions:'

echo ""
echo "All checks passed."
echo ""
echo "To watch agent reconcile the network devices on a node:"
echo "  kubectl logs -n kvnet-system -l app.kubernetes.io/component=agent -f"
