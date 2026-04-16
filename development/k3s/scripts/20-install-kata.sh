#!/usr/bin/env bash
# Install kata-deploy via Helm and verify the kata-fc RuntimeClass appears.
# Hard-fail if RuntimeClass kata-fc is missing — never silently fall back to runc.

set -eo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export KUBECONFIG="${ROOT}/kubeconfig"

KATA_CHART_VERSION="${KATA_CHART_VERSION:-3.13.0}"
KATA_RELEASE="${KATA_RELEASE:-kata-deploy}"
KATA_NAMESPACE="${KATA_NAMESPACE:-kube-system}"

green() { printf '\033[0;32m%s\033[0m\n' "$*"; }
yellow(){ printf '\033[0;33m%s\033[0m\n' "$*"; }
red()   { printf '\033[0;31m%s\033[0m\n' "$*"; }

# Add kata helm repo (idempotent)
if ! helm repo list 2>/dev/null | grep -q '^kata-deploy'; then
    helm repo add kata-deploy https://kata-containers.github.io/kata-deploy
fi
helm repo update kata-deploy >/dev/null

green "Installing kata-deploy ${KATA_CHART_VERSION} into ${KATA_NAMESPACE}"
helm upgrade --install "${KATA_RELEASE}" kata-deploy/kata-deploy \
    --namespace "${KATA_NAMESPACE}" \
    --version "${KATA_CHART_VERSION}" \
    --set env.shims="fc" \
    --wait --timeout=5m

# Wait for RuntimeClass kata-fc
deadline=$(( $(date +%s) + 120 ))
while ! kubectl get runtimeclass kata-fc >/dev/null 2>&1; do
    [[ $(date +%s) -gt $deadline ]] && { red "FAIL: RuntimeClass kata-fc did not appear within 2m"; exit 1; }
    sleep 3
done
green "RuntimeClass kata-fc present"
