# Node Readiness Gates E2E Test Guide (Kind)

This guide details how to run an end-to-end test for the Node Readiness Gates (NRG) controller using a local Kind cluster.

The test demonstrates a realistic, production-aligned scenario where critical addons run on a dedicated platform node pool, and the NRG controller manages a network readiness taint on a separate application worker node.

### Test Topology

The test uses a 3-node Kind cluster:
1.  **`nrg-test-control-plane`**: The Kubernetes control plane.
2.  **`nrg-test-worker` (Platform Node)**: A dedicated node for running cluster-critical addons. It is labeled `reserved-for=platform` and has a corresponding taint to repel normal application workloads. Cert-manager and the NRG controller will run here.
3.  **`nrg-test-worker2` (Application Node)**: A standard worker node that starts with a `readiness.k8s.io/NetworkReady=pending:NoSchedule` taint, simulating a node that is not yet ready for application traffic.

## Running the Test

### Prerequisites

-   [Docker](https://docs.docker.com/get-docker/)
-   [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
-   [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
-   [Go](https://golang.org/doc/install)

### Step 1: Create the Kind Cluster

The provided Kind configuration creates the 3-node topology with the necessary labels and taints.

```bash
kind create cluster --config config/testing/kind/kind-config.yaml
```

Install CRDs

```bash
make install
```

### Step 2: Build and Load the Controller Image

Build the controller image and load it into the Kind cluster nodes.

```bash
# Build the image
make docker-build IMG=controller:latest

# Load the image into the kind cluster
kind load docker-image controller:latest --name nrg-test
```

### Step 3: Controller Deployment

Deploy the controller image to nrg-test-worker
```bash
make deploy IMG=controller:latest
```

Verify the controller is running on the platform node (`nrg-test-worker`):
```bash
kubectl get pods -n nrgcontroller-system -o wide
```

### Step 4: Verify Initial State

Check that the application worker node (`nrg-test-worker2`) has the `NetworkReady` taint.

```bash
# The output should include 'readiness.k8s.io/NetworkReady'
kubectl get node nrg-test-worker2 -o jsonpath='Taints:{"\n"}{range .spec.taints[*]}{.key}{"\n"}{end}'
```

### Step 5: Deploy the Readiness Rule

Apply the network readiness rule. This will be validated by the webhook.

```bash
kubectl apply -f examples/network-readiness-rule.yaml
```

### Step 7: Deploy Calico CNI with Readiness Reporter

This script injects the readiness sidecar into the Calico deployment.

```bash
chmod +x hack/test-workloads/apply-calico.sh
hack/test-workloads/apply-calico.sh
```

### Step 8: Monitor and Verify Final State

1.  **Check for the new node condition on the application node:**
    ```bash
    # Look for 'network.k8s.io/CalicoReady   True'
    kubectl get node nrg-test-worker2 -o jsonpath='Conditions:{"\n"}{range .status.conditions[*]}{.type}{"\t"}{.status}{"\n"}{end}'
    ```

2.  **Verify the taint has been removed from the application node:**
    ```bash
    # The output should NO LONGER include 'readiness.k8s.io/NetworkReady'
    kubectl get node nrg-test-worker2 -o jsonpath='Taints:{"\n"}{range .spec.taints[*]}{.key}{"\n"}{end}'
    ```

### Step 9: Autoscaling Simulation Test

This section tests how the controller handles new nodes being added to the cluster, simulating an autoscaler.

1.  **Scale up the worker nodes:**
    ```bash
    # Add 2 new worker nodes (for a total of 4 workers)
    hack/test-workloads/kindscaler.sh nrg-test 2
    ```

2.  **Verify new nodes are tainted:**
    ```bash
    # Check the taints on the new nodes
    kubectl get node nrg-test-worker3 -o jsonpath='Taints:{"\n"}{range .spec.taints[*]}{.key}{"\n"}{end}'
    kubectl get node nrg-test-worker4 -o jsonpath='Taints:{"\n"}{range .spec.taints[*]}{.key}{"\n"}{end}'
    ```

3.  **Verify taints are removed after Calico is ready:**
    It may take a minute for the Calico pods to be scheduled and run on the new nodes.
    ```bash
    # Wait and verify the taints are removed from the new nodes
    sleep 60
    kubectl get node nrg-test-worker3 -o jsonpath='Taints:{"\n"}{range .spec.taints[*]}{.key}{"\n"}{end}'
    kubectl get node nrg-test-worker4 -o jsonpath='Taints:{"\n"}{range .spec.taints[*]}{.key}{"\n"}{end}'
    ```

### Step 10: Cleanup

```bash
kind delete cluster --name nrg-test
```