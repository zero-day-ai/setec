<!-- SPDX-License-Identifier: Apache-2.0 -->
<p align="center">
  <img src="docs/assets/logo-128.png" alt="Setec" width="128" height="128">
</p>

<h1 align="center">Setec</h1>

<p align="center"><strong>microVM isolation as a Kubernetes primitive.</strong></p>

<p align="center">
  <a href="https://github.com/zero-day-ai/setec/actions/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/zero-day-ai/setec/ci.yml?branch=main&label=ci"></a>
  <a href="https://github.com/zero-day-ai/setec/releases"><img alt="Latest release" src="https://img.shields.io/github/v/release/zero-day-ai/setec?include_prereleases&sort=semver"></a>
  <a href="./LICENSE"><img alt="License: Apache-2.0" src="https://img.shields.io/badge/license-Apache--2.0-blue"></a>
  <a href="https://api.scorecard.dev/projects/github.com/zero-day-ai/setec"><img alt="OSSF Scorecard" src="https://api.scorecard.dev/projects/github.com/zero-day-ai/setec/badge"></a>
  <a href="https://github.com/zero-day-ai/setec/actions/workflows/codeql.yml"><img alt="CodeQL" src="https://img.shields.io/github/actions/workflow/status/zero-day-ai/setec/codeql.yml?branch=main&label=codeql"></a>
  <img alt="Kubernetes" src="https://img.shields.io/badge/kubernetes-1.28%2B-blue">
</p>

---

Setec is a Kubernetes operator that runs workloads inside [Firecracker](https://firecracker-microvm.github.io/) microVMs via [Kata Containers](https://katacontainers.io/). Declare a `Sandbox` custom resource and the operator materialises a hardware-isolated microVM for you, complete with lifecycle control, a programmatic gRPC frontend, snapshot / restore, and a pre-warm pool that targets sub-100ms cold starts. Cloud-agnostic, self-hostable, Apache 2.0.

> **Status: pre-release / alpha.** The CRD is `v1alpha1`. Breaking changes are possible before `v1`.

## Highlights

- **Single-CRD API.** `kubectl apply -f sandbox.yaml` and you have a microVM. No separate CLI, no dashboard, no SaaS.
- **Firecracker snapshots.** Capture, restore, and reuse paused VM state through the `Snapshot` resource.
- **Pre-warm pool.** Each node keeps a configurable pool of paused microVMs ready; pool-claimed sandboxes target sub-100ms P50 cold start on prepared hosts.
- **Multi-tenant.** Tenant identity from namespace labels or mTLS; per-Sandbox `NetworkPolicy`; tenant scoping on the gRPC frontend.
- **Observability shipped.** Prometheus metrics and OpenTelemetry traces emitted by default; Grafana dashboard and alert rules ship with the chart.
- **gRPC frontend.** `SandboxService` with mTLS for programmatic consumers. See [examples](examples/).
- **Cloud-agnostic.** Any Kubernetes cluster whose worker nodes can run Kata Containers.
- **Small surface.** Four distroless binaries: operator, node-agent, frontend, and the pool launcher.

## Quick install

```bash
helm install setec oci://ghcr.io/zero-day-ai/charts/setec \
  --namespace setec-system \
  --create-namespace
```

Local install from a checked-out tree:

```bash
helm install setec ./charts/setec \
  --namespace setec-system \
  --create-namespace
```

Prerequisites: a Kubernetes 1.28+ cluster, at least one worker node with `/dev/kvm`, and [Kata Containers](https://katacontainers.io/docs/how-to/how-to-use-kata-containers-with-kata-deploy/) installed so the `kata-fc` `RuntimeClass` is present. [`docs/prerequisites.md`](docs/prerequisites.md) has the full check-list.

## Why microVM isolation

Containers share a kernel. For trusted workloads that's fine. For workloads you do not trust - code your LLM just generated, a test suite from an outside pull request, a fuzzer you are pointing at a parser - a kernel boundary between the workload and the host is the only honest answer. Firecracker gives you that boundary in under a second per VM and a fraction of the memory overhead of a traditional hypervisor. Setec makes that boundary declarative, reusable, and operable by anyone who already knows Kubernetes.

## Example: a complete Sandbox

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

```bash
kubectl apply -f hello.yaml
kubectl get sandbox hello -w
kubectl logs hello-vm
kubectl delete sandbox hello
```

## Next steps

- **New here:** the 15-minute narrative walkthrough in [`docs/getting-started.md`](docs/getting-started.md).
- **In a hurry:** the terse [quickstart](docs/quickstart.md).
- **Writing a consumer:** three reference programs under [`examples/`](examples/) covering AI code execution, CI sandboxing, and security research.
- **Operating a cluster:** the [docs hub](docs/README.md) groups guides, reference, and operations pages.

## Development

Setec follows the standard [kubebuilder](https://kubebuilder.io/) v4 layout. Most workflows are Makefile targets:

```bash
make generate     # regenerate deepcopy code
make manifests    # regenerate CRD manifests
make build        # build the operator + setec-pool-vm binaries
make test         # run unit + envtest suites
make lint         # run golangci-lint
make helm-lint    # lint the Helm chart
make e2e          # bare-metal E2E suite (requires KVM + Kata)
```

Non-trivial changes go through a short design-before-code cycle described in [`CONTRIBUTING.md`](CONTRIBUTING.md).

## Community

- [`CONTRIBUTING.md`](CONTRIBUTING.md) &mdash; dev setup, commit style, DCO, PR process.
- [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md) &mdash; Contributor Covenant 2.1.
- [`GOVERNANCE.md`](GOVERNANCE.md) &mdash; roles, decision-making, maintainership.
- [`SECURITY.md`](SECURITY.md) &mdash; private vulnerability reporting and response timeline.
- [`CHANGELOG.md`](CHANGELOG.md) &mdash; release history.

## License

Apache 2.0. Full text in [`LICENSE`](LICENSE).

---

The name is a 1990s-movie reference. The goal is not to be cute; it is for hardware-isolated workloads to be boring infrastructure.
