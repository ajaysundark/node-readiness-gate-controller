# Node Readiness Gates
A mechanism to declare, extensible node-readiness pre-requisites (beyond the basic 'Ready' condition) for Kubernetes Nodes.

## Goal:
- Improve scheduling correctness considering standardized Node readiness conditions.
- Improve AutoScaling accuracy.
- Better Node Observability.

More details on the KEP: 

## Design

TODO: insert diagram


This repository provides a PoC for a realistic demonstration of node readiness gates concept in Kind.

### Key Components:

#### Node Daemon (NPD health-checker plugin):
Runs as a DaemonSet, simulates CNI readiness checking, and updates node conditions

#### Readiness Gate Controller:
Watches node conditions and manages a readiness-taint accordingly

To build the controller:

```bash
docker build -t your-registry/readiness-gate-controller:latest .
docker push your-registry/readiness-gate-controller:latest
```

#### Test Workloads:
Demonstrates how workload scheduling is affected by the readiness gates

## Test Flow

1. Bring up a multi-node kind cluster with default CNI disabled.

```bash
cat > values.yaml <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
- role: worker
EOF

kind create cluster --config values.yaml --name nrg
```

2. Load controller image in Kind nodes

```bash
kind load docker-image your-registry/readiness-gate-controller:latest --name nrg
```

3. Deploy readiness-controller into control-plane node

```bash
kubectl apply -f hack/test-workloads/controller-deployment.yaml
```

4. Deploy Calico CNI 

4.1 Apply Calico CNI manifests to the cluster:
```bash
kubectl apply -f https://raw.githubusercontent.com/projectcalico/calico/v3.30.1/manifests/calico.yaml
```

4.2 Apply necessary patches and configurations:
```bash
# Need to patch felix component (calico-node-daemon) to enable healthz endpoint
kubectl patch felixconfigurations.crd.projectcalico.org --type="merge" default --patch-file hack/test-workloads/calico-config-enable-healthz-endpoint.yaml
# Create necessary RBAC for the health-monitoring container
kubectl apply -f hack/test-workloads/calico-rbac-node-status-patch-role.yaml
# Apply patch for calico sidecar to monitor CNI readiness
kubectl patch daemonset calico-node -n kube-system --type=json --patch-file hack/test-workloads/add-cni-patcher-sidecar.json
```

5. Worker A: side-car watches condition (no other action required)

6. Worker B: Additionally NPD watches condition (for high-reliability)

```bash
# Apply NPD manifests for worker B
kubectl apply -f hack/npd-cni-readiness-plugin/npd-config.yaml
kubectl apply -f hack/test-workloads/npd-deployment.yaml
```

## Community, discussion, contribution, and support

Learn how to engage with the Kubernetes community on the [community page](http://kubernetes.io/community/).

You can reach the maintainers of this project at:

- [Slack](https://slack.k8s.io/)
- [Mailing List](https://groups.google.com/a/kubernetes.io/g/dev)

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).

[owners]: https://git.k8s.io/community/contributors/guide/owners.md
[Creative Commons 4.0]: https://git.k8s.io/website/LICENSE
