# Quickstart

This guide takes a prepared user from an empty cluster to a running
`Sandbox` in under 15 minutes.

You need working familiarity with `kubectl` and `helm`. If something goes
wrong at the Kata layer, the upstream
[Kata Containers documentation](https://katacontainers.io/docs/) is the
authoritative reference — Setec does not install, manage, or modify Kata.

## 1. Prerequisites

Before you start, verify all of the following on your workstation:

- [ ] A Kubernetes **1.28+** cluster you can reach with `kubectl`.
- [ ] At least one worker Node is **KVM-capable** (bare-metal Linux, or a
      VM with nested virtualization enabled). Setec runs Firecracker
      microVMs, which require `/dev/kvm`.
- [ ] `kubectl` configured for the target cluster (`kubectl cluster-info`
      succeeds).
- [ ] `helm` 3.8 or later (`helm version`).
- [ ] Cluster-admin permission in the target cluster for the duration of
      the install (needed to register the CRD and ClusterRole).

If you are not sure whether a Node can run Kata, see
[docs/prerequisites.md](prerequisites.md) for how to check and how to
label Kata-capable Nodes.

## 2. Install Kata Containers

Setec depends on Kata Containers being installed cluster-side so that a
`kata-fc` `RuntimeClass` is registered and Kata binaries are present on
worker Nodes. Setec does not install Kata for you.

The upstream project ships `kata-deploy`, a manifest-driven installer that
lays down the Kata binaries on every labeled Node and creates the Kata
RuntimeClasses:

```bash
kubectl apply -k "github.com/kata-containers/kata-containers/tools/packaging/kata-deploy/kata-deploy/base?ref=main"
```

Wait for the DaemonSet to roll out, then confirm the RuntimeClass exists:

```bash
kubectl rollout status -n kube-system ds/kata-deploy --timeout=5m
kubectl get runtimeclass kata-fc
```

Expected output:

```
NAME      HANDLER   AGE
kata-fc   kata-fc   1m
```

If `kata-fc` is missing, re-check the `kata-deploy` rollout logs and the
[upstream installation guide](https://github.com/kata-containers/kata-containers/blob/main/docs/install/README.md).
Setec cannot run workloads without it.

## 3. Install Setec

Install from the OCI chart registry:

```bash
helm install setec oci://ghcr.io/zero-day-ai/charts/setec \
  --namespace setec-system \
  --create-namespace
```

Or, if you are installing from a checked-out source tree:

```bash
helm install setec ./charts/setec \
  --namespace setec-system \
  --create-namespace
```

Verify the operator is running:

```bash
kubectl get deploy -n setec-system
kubectl get pods -n setec-system
```

Expected: one Deployment named `setec` with one ready replica. The pod
should be `Running`.

Check the operator's view of the cluster:

```bash
kubectl -n setec-system logs deployment/setec | head -40
```

You should see a startup log line reporting `kata_runtime_available: true`
and a count of Kata-capable Nodes. If `kata_runtime_available: false`, go
back to step 2 — Setec will start anyway, but any `Sandbox` you apply will
stay in `Pending` with a `RuntimeUnavailable` event.

## 4. Apply your first Sandbox

Save the following as `hello.yaml`:

```yaml
apiVersion: setec.zero-day.ai/v1alpha1
kind: Sandbox
metadata:
  name: hello
  namespace: default
spec:
  image: docker.io/library/python:3.12-slim
  command:
    - python
    - -c
    - "print('hello from a Firecracker microVM')"
  resources:
    vcpu: 1
    memory: 512Mi
  lifecycle:
    timeout: 5m
```

Apply it:

```bash
kubectl apply -f hello.yaml
```

## 5. Observe the lifecycle

Watch the Sandbox transition through its phases:

```bash
kubectl get sandbox -w
```

Expected phase sequence:

```
NAME    PHASE      IMAGE                               AGE
hello   Pending    docker.io/library/python:3.12-slim   2s
hello   Running    docker.io/library/python:3.12-slim   8s
hello   Completed  docker.io/library/python:3.12-slim   12s
```

`Pending` → `Running` is the microVM cold start (image pull + Firecracker
boot). `Running` → `Completed` tracks the workload executing and exiting.

Inspect the event stream and status detail:

```bash
kubectl describe sandbox hello
```

## 6. Read the workload output

The Sandbox spawns a Pod named `<sandbox-name>-vm`. Read its logs like any
other Pod:

```bash
kubectl logs hello-vm
```

Expected:

```
hello from a Firecracker microVM
```

## 7. Cleanup

Delete the Sandbox — the backing Pod is garbage-collected via its
OwnerReference, which terminates the microVM:

```bash
kubectl delete sandbox hello
```

Uninstall the operator (preserves any remaining `Sandbox` resources and
the CRD):

```bash
helm uninstall setec --namespace setec-system
```

Remove the CRD (this also deletes every `Sandbox` in the cluster because
the CRD owns them):

```bash
kubectl delete crd sandboxes.setec.zero-day.ai
```

Remove Kata Containers if you no longer need it — follow the
[kata-deploy uninstall procedure](https://github.com/kata-containers/kata-containers/tree/main/tools/packaging/kata-deploy).

## Next steps

- [docs/crd-reference.md](crd-reference.md) — full field reference for the
  `Sandbox` CRD.
- [docs/prerequisites.md](prerequisites.md) — deeper explanation of KVM,
  nested virtualization, and Node labeling.
- [charts/setec/README.md](../charts/setec/README.md) — Helm values,
  upgrade, and uninstall.
- [docs/dev-smoke-test.md](dev-smoke-test.md) — the maintainer's
  pre-release smoke-test checklist.
