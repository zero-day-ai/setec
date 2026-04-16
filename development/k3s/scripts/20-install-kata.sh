#!/usr/bin/env bash
# Install Kata Containers via kata-deploy and verify the kata-fc RuntimeClass
# appears. The kata-containers project now ships an official Helm chart at
#   github.com/kata-containers/kata-containers/tools/packaging/kata-deploy/helm-chart/kata-deploy
# (added in 3.x), which natively supports k3s via k8sDistribution=k3s.
#
# Helm can't install directly from a subdirectory of a remote git repo, so
# the script shallow-clones kata-containers into a local cache and helm
# installs from that path.
#
# Hard-fail if RuntimeClass kata-fc does not appear — never silently fall
# back to runc.

set -eo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export KUBECONFIG="${ROOT}/kubeconfig"

KATA_VERSION="${KATA_VERSION:-3.28.0}"
KATA_CACHE="${KATA_CACHE:-${ROOT}/.cache/kata-containers-${KATA_VERSION}}"
CHART_PATH="${KATA_CACHE}/tools/packaging/kata-deploy/helm-chart/kata-deploy"

green() { printf '\033[0;32m%s\033[0m\n' "$*"; }
yellow(){ printf '\033[0;33m%s\033[0m\n' "$*"; }
red()   { printf '\033[0;31m%s\033[0m\n' "$*"; }

# Shallow-clone the tagged release (idempotent).
if [[ ! -d "${CHART_PATH}" ]]; then
    green "Fetching kata-containers ${KATA_VERSION} into ${KATA_CACHE}"
    mkdir -p "$(dirname "${KATA_CACHE}")"
    git clone --depth=1 --branch "${KATA_VERSION}" \
        https://github.com/kata-containers/kata-containers \
        "${KATA_CACHE}" 2>&1 | tail -3
else
    yellow "Reusing cached kata-containers at ${KATA_CACHE}"
fi

[[ -f "${CHART_PATH}/Chart.yaml" ]] || {
    red "FAIL: expected chart at ${CHART_PATH} — layout changed upstream?"
    exit 1
}

green "helm upgrade --install kata-deploy (k3s distribution)"
helm upgrade --install kata-deploy "${CHART_PATH}" \
    --namespace kube-system \
    --set k8sDistribution=k3s \
    --wait --timeout=10m

green "Waiting for kata-deploy DaemonSet to report Ready"
kubectl -n kube-system rollout status ds/kata-deploy --timeout=10m

# The chart installs RuntimeClass objects for kata-qemu / kata-clh / kata-fc
# / kata-dragonball via a post-install Job. Wait for them to appear.
deadline=$(( $(date +%s) + 180 ))
while ! kubectl get runtimeclass kata-fc >/dev/null 2>&1; do
    [[ $(date +%s) -gt $deadline ]] && {
        red "FAIL: RuntimeClass kata-fc did not appear within 3m"
        echo '--- installed runtimeclasses ---'
        kubectl get runtimeclass 2>&1 || true
        echo '--- kata-deploy pod status ---'
        kubectl -n kube-system get pods -l name=kata-deploy 2>&1 || true
        exit 1
    }
    sleep 3
done
green "RuntimeClass kata-fc present"

yellow "If smoke-kata fails with 'RuntimeHandler not registered' shortly"
yellow "after this script, wait ~60s for containerd to reload and retry."
