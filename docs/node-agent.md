# Node agent

The node agent is a privileged DaemonSet that manages node-level
infrastructure required by Kata Containers: a devicemapper thin-pool
for microVM snapshots, optional OCI image prefetch, and Prometheus
metrics for thin-pool fill state.

## Prerequisites

- Bare-metal or nested-virt-capable Linux node with `/dev/kvm`.
- `dmsetup` and `blockdev` available on the host. Both are shipped by
  every mainstream distro's `util-linux` and `lvm2` packages.
- Two unused block devices (or LVM logical volumes) to serve as the
  data and metadata volumes for the thin-pool. Size depends on Sandbox
  workload, typically 100GB+ for data and 1GB+ for metadata.

## Installation

Enable via the Helm chart:

```bash
helm upgrade --install setec charts/setec \
  --set nodeAgent.enabled=true \
  --set nodeAgent.thinpoolDataDevice=/dev/vdb \
  --set nodeAgent.thinpoolMetadataDevice=/dev/vdc \
  --set nodeAgent.fillThreshold=80
```

The agent targets nodes labeled `katacontainers.io/kata-runtime=true`
(override via `nodeAgent.nodeSelector`). It runs privileged because
`dmsetup` writes into the device-mapper kernel interface; `SYS_ADMIN` is
the only Linux capability it adds beyond the default drop-all baseline.

## Thin-pool provisioning

On startup the agent:

1. Verifies `/dev/kvm` is present. Exits non-zero if not.
2. Calls `dmsetup status <pool>`. If the pool already exists, the
   agent treats Ensure as a no-op; it never reconfigures an existing
   pool to avoid destructive behaviour on rolling restart.
3. If the pool is absent, calls `blockdev --getsz` on the data device
   to derive the sector count and runs `dmsetup create` with a
   standard thin-pool table. Containerd must be configured to use
   the new pool — the agent does NOT edit containerd's config today;
   an administrator or a kata-deploy-managed snippet is responsible.

## Image prefetch

SandboxClasses can reference custom kernel or rootfs OCI images via
`spec.kernelImage` / `spec.rootfsImage`. Passing
`--prefetch-images=<space-separated-refs>` pulls each image into the
node's containerd content store on agent startup, eliminating cold-pull
latency at Sandbox launch.

The Phase 2 binary ships with a log-only puller stub. Plumbing a
real containerd client is a follow-up once the Helm chart exposes the
containerd socket path; until then the agent logs intent so operators
can validate configuration without a behavioural regression.

## Monitoring loop

Every 30 seconds the agent samples the pool via `dmsetup status` and
updates the `setec_node_thinpool_used_bytes` and `_total_bytes`
gauges. When the fill percentage exceeds `fillThreshold` (default 80)
the agent logs a warning; future revisions will emit a
`SetecThinPoolDegraded=true` `NodeCondition`.

## Troubleshooting

- Agent exits immediately → check `/dev/kvm` exists on the node.
- `dmsetup create` fails → verify the data/metadata devices are
  unmounted and not already claimed by another LVM or device-mapper
  operation.
- Pool exists but containerd cannot use it → confirm containerd's
  `plugins."io.containerd.snapshotter.v1.devmapper"` stanza matches
  the pool name. The agent never writes to containerd's config.
