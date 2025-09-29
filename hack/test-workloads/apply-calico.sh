#!/bin/bash

set -e

KUBECTL_ARGS="$@"

YQ_VERSION="v4.44.1"
YQ_PATH="/tmp/yq"

# Check if yq is installed, if not download it.
if [ ! -f "$YQ_PATH" ]; then
    echo "yq not found at $YQ_PATH, downloading..."
    ARCH=$(uname -m)
    case $ARCH in
        x86_64)
            ARCH="amd64"
            ;;
        aarch64)
            ARCH="arm64"
            ;;
        *)
            echo "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac
    YQ_BINARY="yq_linux_${ARCH}"
    curl -sL "https://github.com/mikefarah/yq/releases/download/${YQ_VERSION}/${YQ_BINARY}" -o "$YQ_PATH"
    chmod +x "$YQ_PATH"
fi

# Download the Calico manifest
curl -sL https://raw.githubusercontent.com/projectcalico/calico/v3.30.1/manifests/calico.yaml -o calico.yaml

# Add the cni-status-patcher sidecar
"$YQ_PATH" e -i 'select(.kind == "DaemonSet" and .metadata.name == "calico-node").spec.template.spec.containers += [load("hack/test-workloads/cni-patcher-sidecar.yaml")]' calico.yaml

# Apply the manifest twice. The first time, it will create the CRDs and ServiceAccounts.
# The second time, it will create the rest of the resources, which should now be able to find the ServiceAccount.
kubectl apply $KUBECTL_ARGS -f calico.yaml || true
kubectl apply $KUBECTL_ARGS -f calico.yaml

# Apply the RBAC rules
kubectl apply $KUBECTL_ARGS -f hack/test-workloads/calico-rbac-node-status-patch-role.yaml
