#!/usr/bin/env bash
# End-to-end integration test: Gibson daemon dispatches a tool call through
# Setec, a microVM runs the Gibson SDK tool-runner-hello image, returns a
# proto response that the daemon unmarshals and delivers to the caller.
#
# Prerequisites:
#   - `make up` completed (k3s + kata + Setec running)
#   - Kind cluster 'gibson' is up, with `extraHosts: host-gateway` enabled
#   - Gibson daemon deployed via Helm
#   - Gibson daemon binary built with `-tags=setec_integration` (see
#     core/gibson/internal/daemon/sandboxed_setec_adapter.go)
#   - Setec is public OR private-repo auth wired into the Gibson build

set -eo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ZERODAY_ROOT="$(cd "${ROOT}/../../.." && pwd)"
KIND_CONTEXT="${KIND_CONTEXT:-kind-gibson}"

green() { printf '\033[0;32m%s\033[0m\n' "$*"; }
yellow(){ printf '\033[0;33m%s\033[0m\n' "$*"; }
red()   { printf '\033[0;31m%s\033[0m\n' "$*"; }

# 1. Verify both clusters are up
green "Verifying k3s cluster (Setec side)"
KUBECONFIG="${ROOT}/kubeconfig" kubectl get nodes | head -3

green "Verifying Kind cluster ${KIND_CONTEXT} (Gibson side)"
kubectl --context="${KIND_CONTEXT}" get nodes | head -3

# 2. Build and load the hello-dev tool runner image
green "Building hello-dev tool runner image"
make -C "${ZERODAY_ROOT}/core/sdk" tool-runner-image TOOL=hello

green "Loading image into k3s containerd"
docker save ghcr.io/zero-day-ai/gibson-tool-runner:hello-dev | \
    sudo KUBECONFIG="${ROOT}/kubeconfig" k3s ctr images import -

# 3. Apply client TLS Secret to Gibson namespace (idempotent, picks up latest PKI)
green "Applying dev mTLS Secret to Gibson Kind cluster"
"${ROOT}/scripts/65-smoke-cross-cluster.sh" >/dev/null 2>&1 || true  # rebuilds generated manifest
kubectl --context="${KIND_CONTEXT}" apply -f "${ROOT}/manifests/gibson-kind/setec-client-tls.generated.yaml"

# 4. Upgrade Gibson Helm release with the sandboxed-tools overlay
green "Upgrading Gibson Helm release with sandboxed-tools overlay"
helm --kube-context="${KIND_CONTEXT}" upgrade gibson "${ZERODAY_ROOT}/enterprise/deploy/helm/gibson" \
    --namespace gibson --reuse-values \
    -f "${ZERODAY_ROOT}/enterprise/deploy/helm/gibson/values-sandboxed-tools.yaml" \
    --wait --timeout=5m

# 5. Invoke the hello tool against the daemon's tool-call gRPC.
#    (Decision point: the exact CLI invocation depends on the Gibson tool-call
#    API surface. The conservative fallback below port-forwards the daemon
#    and uses grpcurl with the SDK tool proto — replace with gibson-cli if a
#    more ergonomic command lands.)
green "Invoking hello tool via gibson daemon"
yellow "  [TODO] Replace with 'gibson-cli tool exec hello --input-string=world' once the CLI command exists."
yellow "  For now this step is a manual placeholder — see README for the grpcurl recipe."

# 6. Assert a Sandbox CR appeared in k3s gibson-dev namespace
green "Checking for Sandbox CR in k3s/gibson-dev"
KUBECONFIG="${ROOT}/kubeconfig" kubectl -n gibson-dev get sandboxes.setec.zero-day.ai -o wide || {
    red "No Sandbox CRs observed. Either the tool call did not fire or Setec did not receive it."
    exit 1
}

green "Integration smoke complete. For trace inspection open Jaeger at the URL printed by:"
green "  kubectl --context=${KIND_CONTEXT} -n gibson get svc jaeger-query"
