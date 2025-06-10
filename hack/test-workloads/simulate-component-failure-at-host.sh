#!/bin/bash

# Disable health server on kind-worker node only
cat <<EOF > disable-felix-health-worker1.yaml
apiVersion: crd.projectcalico.org/v1
kind: FelixConfiguration
metadata:
  name: node.kind-worker
spec:
  healthEnabled: false
EOF
kubectl apply -f disable-felix-health-worker1.yaml

# Stop calico-node container on kind-worker node to force restart
TARGET_POD=$(kubectl get pods -n kube-system --field-selector spec.nodeName=kind-worker -l k8s-app=calico-node -o jsonpath='{.items[0].metadata.name}')
echo "Target pod on kind-worker : $TARGET_POD"
docker exec kind-worker sh  -c 'crictl stop $(crictl ps -q --name calico-node)'
echo "Calico container killed successfully at kind-worker."
echo "Observe 'kubectl logs $TARGET_POD -n kube-system -c cni-status-patcher --follow'"
