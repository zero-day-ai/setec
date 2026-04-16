<!-- SPDX-License-Identifier: Apache-2.0 -->
# Setec Examples

Three reference programs, one per common integration pattern, all speaking to the Setec gRPC frontend.

| Example                                   | Pattern                                | When you want it                                                      |
|-------------------------------------------|----------------------------------------|-----------------------------------------------------------------------|
| [`ai-code-exec`](./ai-code-exec/)         | LLM-generated code execution           | You have an agent that writes code and you need to run it safely.     |
| [`ci-sandbox`](./ci-sandbox/)             | Untrusted CI job execution             | You run tests for pull requests and do not trust the branch contents. |
| [`sec-research`](./sec-research/)         | CPU-hungry, potentially-dangerous tool | You run fuzzers, dynamic analyzers, or similar against target code.   |

Each example is a standalone Go module with its own `go.mod`. The local-development `replace` directive points at the parent repo; drop that line when consuming an example outside this source tree.

## Common prerequisites

- A Setec cluster with the gRPC frontend enabled. See [`docs/frontend-api.md`](../docs/frontend-api.md) for the exposed API and installation flags.
- Client TLS material issued by the same CA the frontend trusts: a client certificate, the matching private key, and the CA certificate. The chart can be configured to generate per-tenant client certificates; alternatively issue them from your existing PKI.
- `kubectl` access to the cluster for the `sandbox.yaml` variants.
- Go 1.23 or later to build the clients.
- At least one KVM-capable worker node in the cluster.

## Common build pattern

```bash
cd examples/<name>
go build -o <name> .
```

Each example exposes identical flags for frontend address and TLS material:

- `--addr` &mdash; `host:port` of the gRPC frontend.
- `--client-cert`, `--client-key`, `--ca` &mdash; PEM files for the mTLS handshake.
- `--image`, `--vcpu`, `--memory-mib`, `--timeout` &mdash; resource ceilings for the sandbox.

See the individual READMEs for example-specific flags.

## Choosing a starting point

- If you are integrating Setec into an agent or code-execution product, start with [`ai-code-exec`](./ai-code-exec/).
- If you are building a CI system, start with [`ci-sandbox`](./ci-sandbox/).
- If you are running research or offensive-security workloads, start with [`sec-research`](./sec-research/).

## Where to look next

- [`docs/frontend-api.md`](../docs/frontend-api.md) &mdash; full gRPC surface reference.
- [`docs/snapshots.md`](../docs/snapshots.md) &mdash; pool and snapshot support for sub-second cold starts.
- [`docs/multitenancy.md`](../docs/multitenancy.md) &mdash; per-tenant isolation and policy.
