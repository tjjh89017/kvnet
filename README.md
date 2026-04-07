# kvnet

A Kubernetes operator for declarative virtual network management. It uses a **manager/agent** architecture: the manager runs centrally and reconciles template resources into per-node CRs, while the agent runs as a DaemonSet on each node and applies network configuration using `iproute2`.

## Architecture

```
Manager (Deployment)           Agent (DaemonSet, one per node)
  BridgeTemplate       →         Bridge
  VXLANTemplate        →         VXLAN
  UplinkTemplate       →         Uplink
```

Templates use a `nodeSelector` to target nodes. The manager creates a CR for each matching node named `{nodeName}.{deviceName}`. The agent on each node reconciles only its own CRs (filtered by `NODENAME`).

## Custom Resources

### Bridge / BridgeTemplate

Manages a Linux bridge device on a node.

| Field | Type | Description |
|---|---|---|
| `vlanFiltering` | bool | Enable VLAN filtering on the bridge |
| `nfCallIptables` | bool | Enable `nf_call_iptables` (default: false) |

### VXLAN / VXLANTemplate

Manages a VXLAN tunnel interface attached to a bridge.

| Field | Type | Description |
|---|---|---|
| `vni` | int | VXLAN Network Identifier (traditional mode only) |
| `master` | string | Bridge device to attach to |
| `localIP` | string | Source IP for encapsulation (auto-detected from `dev` if empty) |
| `dev` | string | Interface to detect local IP from |
| `mtu` | int | MTU for the VXLAN interface (0 = system default) |
| `learning` | bool | Enable dynamic VTEP learning (disable for EVPN) |
| `bridgeLearning` | bool | Enable bridge-port MAC learning (disable for EVPN) |
| `external` | bool | Single VXLAN device mode (`collect_metadata`); `vni` is ignored |
| `vniFilter` | bool | Enable per-VNI filtering (requires `external: true`) |
| `portVLANConfig` | object | Bridge port VLAN settings (see below) |

### Uplink / UplinkTemplate

Manages a bond interface and attaches it to a bridge.

| Field | Type | Description |
|---|---|---|
| `master` | string | Bridge device to attach to |
| `bondMode` | string | Bonding mode (`balance-rr`, `active-backup`, `802.3ad`, etc.) |
| `bondSlaves` | []string | Slave interfaces for the bond |
| `portVLANConfig` | object | Bridge port VLAN settings (see below) |

### PortVLANConfig

Shared type used by VXLAN and Uplink for bridge port VLAN configuration.

| Field | Type | Description |
|---|---|---|
| `pvid` | int | Native/untagged VLAN ID |
| `vids` | []int | Trunk VLAN IDs allowed on this port |
| `tunnelInfo` | []VLANTunnelMapping | VLAN-to-VNI mappings for single VXLAN device mode |

`VLANTunnelMapping` maps a VLAN ID (or range) to a VNI:

```yaml
tunnelInfo:
  - vid: 100
    vni: 1000
  - vid: 200
    vidEnd: 299   # maps VLANs 200-299 to VNIs 2000-2099
    vni: 2000
```

## VXLAN Modes

**Traditional mode** (`external: false`): one VXLAN device per VNI. Set `vni` in the template.

**External mode** (`external: true`): a single VXLAN device with `collect_metadata`. VNI is determined by the bridge VLAN tunnel mapping in `portVLANConfig.tunnelInfo`. Enable `vniFilter: true` to restrict which VNIs are accepted.

## Testing with KIND

Scripts in `scripts/` automate the full local test cycle.

```sh
# 1. Create cluster, install cert-manager, build image, deploy manager + agent
IMG=kvnet:dev scripts/setup.sh

# 2. Apply BridgeTemplate + VXLANTemplate, verify per-node CRs are created
scripts/apply-samples.sh

# 3. Watch agent reconcile network devices on each node
kubectl logs -n kvnet-system -l app.kubernetes.io/component=agent -f

# 4. Clean up
scripts/teardown.sh
```

`scripts/kind.yaml` creates 1 control-plane + 2 worker nodes. `setup.sh` automatically labels worker nodes with `kvnet.kojuro.date/role=worker`, which the sample templates use as their `nodeSelector`.

To test with a custom image:
```sh
IMG=myregistry/kvnet:test scripts/setup.sh
```

### Sample resources

`scripts/apply-samples.sh` applies the YAMLs in `scripts/samples/` and verifies that the manager creates the expected per-node CRs:

| File | Resource | What it creates |
|---|---|---|
| `scripts/samples/bridgetemplate.yaml` | BridgeTemplate `br0` | `{node}.br0` Bridge CR on each worker |
| `scripts/samples/vxlantemplate.yaml` | VXLANTemplate `vxlan0` | `{node}.vxlan0` VXLAN CR on each worker, attached to `br0`, external mode |

Both templates select nodes with `kvnet.kojuro.date/role=worker`. The agent then reconciles the per-node CRs to configure actual Linux bridge and VXLAN interfaces on each node.

## Building

```sh
make docker-build IMG=<image>:<tag>
make deploy IMG=<image>:<tag>
```

The operator runs as a single binary (`/manager`) with mode controlled by flags:

- `--agent`: Run in agent mode. Requires `NODENAME` environment variable.
- `--leader-elect`: Enable leader election (manager mode).

## Labels

Resources use the following labels:

| Label | Description |
|---|---|
| `kvnet.kojuro.date/template-name` | Name of the template that created this CR |
| `kvnet.kojuro.date/template-namespace` | Namespace of the template |
| `kvnet.kojuro.date/node` | Node this CR is scoped to |

## Troubleshooting

### Too many open files

See: https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files

## TODO

- Fill status conditions for all resources
- WAN attribute on Router (only WAN ports do NAT)
- Router controller watches Subnet changes
- Change Router from Deployment to StatefulSet
- Static IP assignment via KubeVirt VM annotations
- Floating IP management via KubeVirt VM annotations
