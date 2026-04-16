#!/usr/bin/env bash
# Tear down the local k3s + Setec dev environment. Idempotent — silent on
# missing components.

set -eo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PKI="${ROOT}/pki"
KUBECONFIG_PATH="${ROOT}/kubeconfig"
export KUBECONFIG="${KUBECONFIG_PATH}"

yellow(){ printf '\033[0;33m%s\033[0m\n' "$*"; }

# Best-effort helm uninstall (only if cluster reachable)
if [[ -f "${KUBECONFIG_PATH}" ]] && kubectl get nodes >/dev/null 2>&1; then
    yellow "helm uninstall setec (best-effort)"
    helm uninstall setec -n setec-system 2>/dev/null || true
    yellow "helm uninstall kata-deploy (best-effort)"
    helm uninstall kata-deploy -n kube-system 2>/dev/null || true
fi

# Official k3s uninstaller (silent if not installed)
if [[ -x /usr/local/bin/k3s-uninstall.sh ]]; then
    yellow "Running /usr/local/bin/k3s-uninstall.sh"
    sudo /usr/local/bin/k3s-uninstall.sh
fi

# Working-tree cleanup
rm -rf "${PKI}" "${KUBECONFIG_PATH}"
rm -f "${ROOT}"/manifests/gibson-kind/*.generated.yaml 2>/dev/null || true
yellow "Removed pki/, kubeconfig, generated manifests."
