#!/usr/bin/env bash
# teardown.sh — remove the kvnet KIND cluster
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-kvnet}"

echo "==> Deleting KIND cluster '$CLUSTER_NAME'..."
kind delete cluster --name "$CLUSTER_NAME"
echo "Done."
