#!/usr/bin/env bash
# setup.sh — create a KIND cluster and deploy kvnet manager + agent
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-kvnet}"
IMG="${IMG:-kvnet:dev}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# ── helpers ──────────────────────────────────────────────────────────────────
log() { echo "==> $*"; }

wait_for_secret() {
  local name="$1" ns="$2" timeout="${3:-180}"
  log "Waiting for secret '$name' in namespace '$ns'..."
  local deadline=$(( $(date +%s) + timeout ))
  until kubectl get secret "$name" -n "$ns" &>/dev/null; do
    if (( $(date +%s) > deadline )); then
      echo "ERROR: timed out waiting for secret $name" >&2
      return 1
    fi
    sleep 5
  done
}

# ── kubeconfig merge helper ───────────────────────────────────────────────────
# Write KIND kubeconfig to a temp file, then merge it into ~/.kube/config so
# the existing contexts are preserved. Falls back to creating ~/.kube/config
# if it does not yet exist.
merge_kubeconfig() {
  local tmp_cfg
  tmp_cfg="$(mktemp /tmp/kind-${CLUSTER_NAME}-XXXXXX.yaml)"
  kind get kubeconfig --name "$CLUSTER_NAME" > "$tmp_cfg"

  local kube_dir="$HOME/.kube"
  local kube_cfg="$kube_dir/config"
  mkdir -p "$kube_dir"

  if [[ -f "$kube_cfg" ]]; then
    local backup="${kube_cfg}.bak.$(date +%Y%m%d%H%M%S)"
    cp "$kube_cfg" "$backup"
    log "Backed up existing kubeconfig to $backup"
    KUBECONFIG="${kube_cfg}:${tmp_cfg}" kubectl config view --flatten > "${kube_cfg}.merged"
    mv "${kube_cfg}.merged" "$kube_cfg"
  else
    cp "$tmp_cfg" "$kube_cfg"
  fi

  rm -f "$tmp_cfg"
  log "Merged context 'kind-${CLUSTER_NAME}' into $kube_cfg"
}

# ── cluster ───────────────────────────────────────────────────────────────────
if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
  log "KIND cluster '$CLUSTER_NAME' already exists, skipping creation."
else
  log "Creating KIND cluster '$CLUSTER_NAME'..."
  # --kubeconfig="" prevents kind from touching ~/.kube/config directly;
  # we handle the merge ourselves below.
  kind create cluster --name "$CLUSTER_NAME" --config "$SCRIPT_DIR/kind.yaml" --kubeconfig=""
  merge_kubeconfig
fi

log "Switching to context 'kind-${CLUSTER_NAME}'..."
kubectl config use-context "kind-${CLUSTER_NAME}"

# Label worker nodes so nodeSelector in templates can target them
log "Labeling worker nodes with kvnet.kojuro.date/role=worker..."
kubectl get nodes --no-headers -o custom-columns=NAME:.metadata.name \
  | grep -v -- '-control-plane$' \
  | xargs -I{} kubectl label node {} kvnet.kojuro.date/role=worker --overwrite

# ── cert-manager ──────────────────────────────────────────────────────────────
CERTMANAGER_VERSION="v1.16.3"
if kubectl get crd certificates.cert-manager.io &>/dev/null; then
  log "cert-manager already installed, skipping."
else
  log "Installing cert-manager $CERTMANAGER_VERSION..."
  kubectl apply -f "https://github.com/cert-manager/cert-manager/releases/download/${CERTMANAGER_VERSION}/cert-manager.yaml"
  kubectl wait deployment/cert-manager-webhook \
    -n cert-manager \
    --for=condition=Available \
    --timeout=5m
fi

# ── build & load image ────────────────────────────────────────────────────────
log "Building image '$IMG'..."
cd "$REPO_DIR"
make docker-build IMG="$IMG"

log "Loading image into KIND cluster '$CLUSTER_NAME'..."
kind load docker-image "$IMG" --name "$CLUSTER_NAME"

# ── deploy manager ────────────────────────────────────────────────────────────
log "Deploying kvnet manager (IMG=$IMG)..."
make deploy IMG="$IMG"

wait_for_secret webhook-server-cert kvnet-system 180

log "Waiting for kvnet-controller-manager to be available..."
kubectl wait deployment/kvnet-controller-manager \
  -n kvnet-system \
  --for=condition=Available \
  --timeout=3m

# ── deploy agent ──────────────────────────────────────────────────────────────
log "Deploying kvnet agent DaemonSet..."
# Substitute the image name into the manifest
sed "s|kvnet:dev|${IMG}|g" "$SCRIPT_DIR/agent-daemonset.yaml" \
  | kubectl apply -f -

log "Waiting for kvnet-agent DaemonSet rollout..."
kubectl rollout status daemonset/kvnet-agent -n kvnet-system --timeout=3m

# ── done ──────────────────────────────────────────────────────────────────────
echo ""
echo "Setup complete."
echo "  Cluster : $CLUSTER_NAME"
echo "  Context : kind-$CLUSTER_NAME  (already active)"
echo ""
echo "Switch context:"
echo "  kubectl config use-context kind-$CLUSTER_NAME"
echo "  kubectl config use-context <other-context>     # switch back"
echo "  kubectl config get-contexts                    # list all"
echo ""
echo "Next steps:"
echo "  Apply sample resources : scripts/apply-samples.sh"
echo "  Tear everything down   : scripts/teardown.sh"
