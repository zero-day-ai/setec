#!/usr/bin/env bash
# Install Kata Containers via kata-deploy and verify the kata-fc RuntimeClass
# appears. kata-containers does NOT publish a Helm chart — the canonical
# install is kubectl apply -k against the upstream kustomize overlay.
#
# kata-deploy in the k3s overlay takes care of:
#   - mounting kata binaries into /opt/kata on the host
#   - patching the k3s containerd config.toml.d/ with the kata runtime handler
#   - restarting containerd via crictl so the new handler is picked up
#
# Hard-fail if RuntimeClass kata-fc does not appear — never silently fall
# back to runc.

set -eo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export KUBECONFIG="${ROOT}/kubeconfig"

# Pin to a stable kata-containers release. 3.15.0 is the latest stable at
# time of writing; bump cautiously and re-verify kata-fc boots via smoke-kata.
KATA_VERSION="${KATA_VERSION:-3.15.0}"

green() { printf '\033[0;32m%s\033[0m\n' "$*"; }
yellow(){ printf '\033[0;33m%s\033[0m\n' "$*"; }
red()   { printf '\033[0;31m%s\033[0m\n' "$*"; }

green "Installing kata-deploy ${KATA_VERSION} into kube-system (k3s overlay)"
kubectl apply -k "github.com/kata-containers/kata-containers/tools/packaging/kata-deploy/kata-deploy/overlays/k3s?ref=${KATA_VERSION}"

green "Waiting for kata-deploy DaemonSet to roll out (up to 10m — first install pulls kata binaries)"
kubectl -n kube-system rollout status ds/kata-deploy --timeout=10m

green "Applying kata RuntimeClass objects"
kubectl apply -k "github.com/kata-containers/kata-containers/tools/packaging/kata-deploy/runtimeclasses?ref=${KATA_VERSION}"

# Verify RuntimeClass kata-fc exists.
deadline=$(( $(date +%s) + 120 ))
while ! kubectl get runtimeclass kata-fc >/dev/null 2>&1; do
    [[ $(date +%s) -gt $deadline ]] && { red "FAIL: RuntimeClass kata-fc did not appear within 2m after kata-deploy"; exit 1; }
    sleep 3
done
green "RuntimeClass kata-fc present"

# kata-deploy restarts containerd via the DaemonSet; no explicit k3s restart
# is needed. But the Ready=true for a new runtime may lag 30–60s; warn the
# user if smoke-kata fails immediately after this script and ask them to
# retry in a minute.
yellow "kata-deploy installed. If smoke-kata fails with 'RuntimeHandler not registered',"
yellow "wait ~60s for containerd to reload and retry."
