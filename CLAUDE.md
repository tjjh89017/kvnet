# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```sh
# Build binary
make build

# Run unit tests (uses envtest, downloads K8s binaries on first run)
make test

# Run a single test or package
KUBEBUILDER_ASSETS="$(bin/setup-envtest use --bin-dir bin -p path)" go test ./internal/controller/... -run TestBridgeReconciler -v

# Lint
make lint
make lint-fix

# Regenerate CRD manifests and RBAC from kubebuilder markers
make manifests

# Regenerate DeepCopy methods
make generate

# Build and deploy to current kubeconfig cluster
make docker-build deploy IMG=<registry/image:tag>

# E2E tests (requires kind, creates a cluster named kvnet-test-e2e)
make test-e2e
```

After editing any `api/v1alpha1/*_types.go` file, run `make manifests generate` to update CRD YAMLs and DeepCopy methods.

## Architecture

The binary (`cmd/main.go`) runs in two modes selected by the `--agent` flag:

**Manager mode** (Deployment, one replica): watches `*Template` resources and fans them out to per-node CRs. Runs webhooks (disabled with `ENABLE_WEBHOOKS=false`). Requires cert-manager for webhook TLS.

**Agent mode** (DaemonSet, one per node): reads per-node CRs filtered by `NODENAME` env var, then calls `iproute2` / sysfs to configure the actual Linux network devices.

### Template → CR cascade

Each template controller (`internal/controller/*template_controller.go`) follows the same pattern:
1. List nodes matching `spec.nodeSelector`
2. `CreateOrUpdate` a CR named `{nodeName}.{deviceName}` for each matching node, copying `spec.template.spec` into it, and labeling it with `LabelTemplateName`, `LabelTemplateNS`, `LabelNode`
3. Delete orphan CRs (labeled by this template but no longer matching a node)
4. Watch `Node` objects so template controllers re-reconcile when node labels change

### Agent reconcilers

Each agent reconciler (`internal/controller/bridge_controller.go`, `vxlan_controller.go`, `uplink_controller.go`) filters on `LabelNode == NodeName`. It calls `helper.ExecCmd` (wraps `os/exec`) to run `ip link`, `bridge`, etc. The VXLAN reconciler also manages FDB entries for remote VTEPs.

### CR naming

All cluster-scoped CRs are named `{nodeName}.{deviceName}`. Use `helper.ParseDeviceName(crName, nodeName)` to strip the node prefix and recover the device name.

### Key packages

- `api/v1alpha1/` — CRD types. `shared_types.go` has `PortVLANConfig` and `VLANTunnelMapping` (used by both VXLAN and Uplink). `constants.go` has label keys and `FinalizerName`.
- `internal/controller/` — all reconcilers
- `internal/helper/` — `ExecCmd`, `IsBridgeSlave`, `GetInterfaceIP`, `ApplyPortVLANConfig`
- `internal/webhook/v1alpha1/` — defaulting and validation webhooks (currently scaffolding, not yet implemented)
