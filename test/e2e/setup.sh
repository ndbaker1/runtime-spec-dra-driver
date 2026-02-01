#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
CLUSTER_NAME="${CLUSTER_NAME:-runtime-spec-dra-test}"
IMAGE_NAME="${IMAGE_NAME:-runtime-spec-dra-driver}"
VERSION="${VERSION:-test}"

echo "==> Creating KinD cluster with NRI and DRA enabled..."
kind create cluster --config="${SCRIPT_DIR}/kind-config.yaml" --name="${CLUSTER_NAME}" --wait 5m || {
    echo "Cluster may already exist, continuing..."
}

echo "==> Building driver image..."
cd "${ROOT_DIR}"
make -f deployments/container/Makefile build IMAGE_NAME="${IMAGE_NAME}" VERSION="${VERSION}"

echo "==> Loading image into KinD..."
kind load docker-image "${IMAGE_NAME}:${VERSION}" --name="${CLUSTER_NAME}"

echo "==> Installing driver via Helm..."
helm upgrade --install runtime-spec-dra-driver \
    "${ROOT_DIR}/deployments/helm/runtime-spec-dra-driver" \
    --set image.repository="${IMAGE_NAME}" \
    --set image.tag="${VERSION}" \
    --set image.pullPolicy=Never \
    --set allowDefaultNamespace=true \
    --set nri.enabled=true \
    --wait --timeout=120s

echo "==> Waiting for DaemonSet to be ready..."
kubectl rollout status daemonset/runtime-spec-dra-driver-kubeletplugin --timeout=120s

echo "==> Setup complete!"
echo ""
echo "To run tests: ./test/e2e/run-tests.sh"
echo "To cleanup:   kind delete cluster --name=${CLUSTER_NAME}"
