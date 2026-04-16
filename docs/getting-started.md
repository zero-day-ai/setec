<!-- SPDX-License-Identifier: Apache-2.0 -->
# Getting Started with Setec

This is a narrative walk-through that takes roughly fifteen minutes and ends with a workload running inside a Firecracker microVM under Kubernetes control. Where the [quickstart](./quickstart.md) says "run this command", this page says "run this command; you should see X; this is happening because Y". If you have done this before, the quickstart is shorter.

Everything here runs against your own cluster. No cloud account, no login, no telemetry.

## Before You Start

You will need:

1. A Kubernetes cluster (1.28 or later) with cluster-admin credentials. A single-node development cluster works, provided the node can run KVM.
2. A worker node with `/dev/kvm` present. Bare-metal Linux is the easy case; a nested-virtualization VM also works if the outer hypervisor permits it.
3. `kubectl` and `helm` 3.8 or later on your workstation.
4. About fifteen minutes of unhurried time.

If you are unsure about the KVM requirement, [`docs/prerequisites.md`](./prerequisites.md) covers how to check and how to label KVM-capable nodes.

## Step 1: Verify KVM

Setec boots Firecracker microVMs. Firecracker needs `/dev/kvm`. The operator is happy to install without it, but every `Sandbox` you launch will stay `Pending` forever. Saving yourself that frustration takes one command on any worker node:

```bash
ls -l /dev/kvm
```

You should see a character device owned by `root:kvm` (or similar). If you see "No such file or directory", the node is not running on bare metal or nested-virtualization is disabled. Fix that first; the rest of this tutorial assumes it is resolved.

## Step 2: Install Kata Containers

Setec does not ship Kata. It treats Kata Containers as a prerequisite and talks to it through the standard Kubernetes `RuntimeClass` abstraction. The upstream `kata-deploy` installer lays down Kata binaries on every labelled node and registers the `kata-fc` `RuntimeClass`:

```bash
kubectl apply -k "github.com/kata-containers/kata-containers/tools/packaging/kata-deploy/kata-deploy/base?ref=main"
kubectl rollout status -n kube-system ds/kata-deploy --timeout=5m
kubectl get runtimeclass kata-fc
```

The last command should print a row showing `kata-fc` with handler `kata-fc`. That's the handle Setec will use to tell Kubernetes "run this pod inside a Firecracker microVM instead of a regular container".

If the rollout fails or the RuntimeClass is missing, stop here and consult [the upstream kata-deploy docs](https://github.com/kata-containers/kata-containers/blob/main/docs/install/kata-deploy/README.md). Setec cannot help you past this gate.

## Step 3: Install Setec

With Kata in place, the Setec install is one helm command:

```bash
helm install setec ./charts/setec \
  --namespace setec-system \
  --create-namespace
```

Helm prints a summary showing the release name, namespace, and the resources it created. There is a `Deployment` for the operator, a `DaemonSet` for the node-agent, a `ClusterRole`, a `ClusterRoleBinding`, a few `ServiceAccounts`, and the `Sandbox`, `SandboxClass`, and `Snapshot` `CustomResourceDefinitions`.

Check the operator is healthy:

```bash
kubectl get deploy -n setec-system
kubectl get pods -n setec-system
```

Both the operator pod and the node-agent pod should be `Running`. Read a few lines of the operator log:

```bash
kubectl -n setec-system logs deployment/setec | head -40
```

Look for a line like `kata_runtime_available: true` and a count of Kata-capable nodes. If that is `false`, go back to step 2; Setec will accept your Sandboxes but nothing will boot.

### What you just did

You installed a Kubernetes operator that watches a set of custom resources, a node-agent that will eventually place Firecracker VMs on the host, and the CRDs that together form Setec's external contract. Nothing launched yet; the cluster is idling in a steady state.

## Step 4: Launch Your First Sandbox

Save this manifest as `hello.yaml`:

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

A tour of the fields:

- `spec.image`: the OCI image whose root filesystem becomes the microVM's rootfs. Any standard image works.
- `spec.command`: what to run inside the VM. If omitted, the image's entrypoint is used.
- `spec.resources.vcpu` and `spec.resources.memory`: the microVM's CPU and memory ceiling. These are hard caps enforced by Firecracker, not Kubernetes requests.
- `spec.lifecycle.timeout`: after this duration the operator terminates the workload and records a timeout status. It stops runaway jobs without you watching them.

Apply it:

```bash
kubectl apply -f hello.yaml
```

### What you just did

You told Kubernetes "I want this workload to run in a microVM with these specific limits". The Setec operator turned that intent into a concrete `Pod` with the `kata-fc` `RuntimeClass`, which Kubelet then hands to Kata, which boots a Firecracker VM and runs your workload inside it. The Sandbox resource is the long-lived record of the job; the pod is its short-lived implementation detail.

## Step 5: Watch It Run

Watch the Sandbox transition through phases:

```bash
kubectl get sandbox -w
```

You will see three phases in sequence:

```
NAME    PHASE      IMAGE                               AGE
hello   Pending    docker.io/library/python:3.12-slim  2s
hello   Running    docker.io/library/python:3.12-slim  8s
hello   Completed  docker.io/library/python:3.12-slim  12s
```

- `Pending` means the operator has accepted the request and is preparing the pod. The image pull and the Firecracker boot happen during this phase.
- `Running` means the VM is up and the workload is executing.
- `Completed` means the process exited cleanly. You will see `Failed` instead if the process exited non-zero, or `TimedOut` if the lifecycle deadline elapsed first.

Press Ctrl-C to stop the watch, then inspect the detail:

```bash
kubectl describe sandbox hello
```

The `Status` block carries the underlying pod name (`hello-vm`), phase transition timestamps, and any events the operator emitted. The final event usually reports workload exit code.

Read the workload output by reading its pod logs:

```bash
kubectl logs hello-vm
```

You should see:

```
hello from a Firecracker microVM
```

If you instead see `error: container not found`, the pod has already been cleaned up because its Sandbox was deleted. That's fine; re-apply the manifest to try again.

### What you just did

You proved end-to-end: Sandbox admitted, pod scheduled, microVM booted, workload ran, exit captured, logs surfaced. Everything you touched used standard Kubernetes verbs and standard pod log retrieval. No Setec-specific CLI.

## Step 6: Clean Up

Delete the Sandbox. Because the backing pod has an `OwnerReference` to the Sandbox, Kubernetes garbage-collects the pod (and therefore tears down the microVM) as soon as the Sandbox is gone:

```bash
kubectl delete sandbox hello
```

If you are done experimenting, uninstall the chart. This leaves the CRD in place, which means any `Sandbox` resources that already exist survive:

```bash
helm uninstall setec --namespace setec-system
```

To remove Setec entirely, including the CRDs (and any remaining `Sandbox` objects with them):

```bash
kubectl delete crd sandboxes.setec.zero-day.ai sandboxclasses.setec.zero-day.ai snapshots.setec.zero-day.ai
```

Kata can stay; it's harmless on its own. If you want it gone, follow the upstream [kata-deploy uninstall](https://github.com/kata-containers/kata-containers/tree/main/tools/packaging/kata-deploy) procedure.

## What You Just Did

In fifteen minutes you installed a Kubernetes-native microVM runtime, declared a workload as a custom resource, and watched Kubernetes orchestrate a Firecracker VM to run it. The only thing your cluster knew how to do beforehand was schedule containers; now it can also schedule hardware-isolated microVMs, described through the same `kubectl apply` pattern that every Kubernetes operator uses.

The point of Setec is that the interface to microVM isolation is the same interface you already use for everything else. The operator does the translation between Kubernetes intent and the Kata + Firecracker runtime below it. There is no new dashboard, no new CLI, no cloud account.

## Next Steps

- [Multi-tenancy](./multitenancy.md) &mdash; tenant labels and per-tenant policy.
- [Snapshots](./snapshots.md) &mdash; pre-warm pool and snapshot-restore for sub-second cold starts.
- [Observability](./observability.md) &mdash; the metrics you should scrape and the alerts we ship.
- [gRPC Frontend API](./frontend-api.md) &mdash; launch Sandboxes programmatically from a client.
- [Examples](../examples/) &mdash; three reference consumer programs (AI code execution, CI sandbox, security research).
- [CRD Reference](./crd-reference.md) &mdash; every field, every default.
