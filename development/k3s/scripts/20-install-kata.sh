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

# ─────────────────────────────────────────────────────────────────────────
# kata-deploy 3.28+ requires k3s's rendered /var/lib/rancher/k3s/agent/etc/
# containerd/config.toml to import the drop-in directory config.toml.d so
# kata-deploy can drop its runtime handler fragments there at install time.
# k3s regenerates config.toml from a Go template on every restart; the
# supported customisation is to copy config.toml.tmpl.example to
# config.toml.tmpl and edit it. An earlier version of this script used
# `{{ template "base" . }}` to inherit the default — that named template
# does not exist in k3s 1.31.x, so the rendered config.toml was effectively
# empty and containerd started without the CRI plugin. We now copy the
# full example verbatim and prepend the imports line at the top.
# Idempotent: only writes the template if it's missing or doesn't already
# contain the imports line; only restarts k3s if we changed it.
# ─────────────────────────────────────────────────────────────────────────
K3S_CONTAINERD_DIR=/var/lib/rancher/k3s/agent/etc/containerd
K3S_TMPL="${K3S_CONTAINERD_DIR}/config.toml.tmpl"
K3S_TMPL_EXAMPLE="${K3S_CONTAINERD_DIR}/config.toml.tmpl.example"
IMPORTS_LINE='imports = ["/var/lib/rancher/k3s/agent/etc/containerd/config.toml.d/*.toml"]'

need_template=0
if ! sudo test -f "${K3S_TMPL}"; then
    need_template=1
elif ! sudo grep -Fq 'config.toml.d/*.toml' "${K3S_TMPL}"; then
    need_template=1
fi

if [[ ${need_template} -eq 1 ]]; then
    if ! sudo test -f "${K3S_TMPL_EXAMPLE}"; then
        red "FAIL: ${K3S_TMPL_EXAMPLE} is missing."
        red "      k3s writes this file after it has booted containerd at least once."
        red "      Run 'sudo systemctl restart k3s' and wait 30s, then re-run this script."
        exit 1
    fi
    green "Writing ${K3S_TMPL} (example template + drop-in imports prefix)"
    sudo mkdir -p "${K3S_CONTAINERD_DIR}"
    sudo sh -c "
        {
            printf '# Managed by opensource/setec/development/k3s/scripts/20-install-kata.sh\n'
            printf '# Prepends the drop-in imports line that kata-deploy 3.28+ requires.\n'
            printf '%s\n\n' '${IMPORTS_LINE}'
            cat '${K3S_TMPL_EXAMPLE}'
        } > '${K3S_TMPL}'
    "

    green "Restarting k3s so it regenerates containerd/config.toml with the drop-in import"
    sudo systemctl restart k3s
    deadline=$(( $(date +%s) + 120 ))
    while ! kubectl get nodes 2>/dev/null | grep -q ' Ready '; do
        [[ $(date +%s) -gt $deadline ]] && {
            red "FAIL: k3s did not return to Ready within 2m after restart"
            red "      Inspect: sudo journalctl -u k3s -n 50 --no-pager"
            red "      If containerd is stuck: sudo rm ${K3S_TMPL} && sudo systemctl restart k3s"
            exit 1
        }
        sleep 2
    done
    # Force any stuck kata-deploy pods from a prior run to re-roll against
    # the new containerd config.
    kubectl -n kube-system delete pods -l name=kata-deploy --ignore-not-found=true --wait=false 2>/dev/null || true
fi

# The kata-deploy chart depends on node-feature-discovery. helm dependency
# build requires the subchart's source repo to be registered first.
if ! helm repo list 2>/dev/null | awk '{print $2}' | grep -q '^https://kubernetes-sigs.github.io/node-feature-discovery/charts$'; then
    green "Registering node-feature-discovery helm repo"
    helm repo add nfd https://kubernetes-sigs.github.io/node-feature-discovery/charts
fi
helm repo update nfd >/dev/null 2>&1 || true

green "helm dependency build (fetches node-feature-discovery subchart)"
helm dependency build "${CHART_PATH}"

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
