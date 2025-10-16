# Scalability Improvement Plan

## Problem Statement

Current implementation doesn't scale well for large clusters (5000+ nodes) due to:

1. **Per-node status tracking**: `NodeReadinessGateRule.Status.NodeEvaluations` stores detailed evaluation results for every node
2. **Missing status updates**: Node reconciler evaluates nodes but doesn't persist status updates
3. **Status object size**: With 5000 nodes, the status subresource becomes massive (multiple MB)
4. **API server load**: Updating rule status on every node reconciliation = 5000 API calls per condition change

## Current Status Structure (Problematic)

```go
type NodeReadinessGateRuleStatus struct {
    ObservedGeneration int64
    Conditions         []metav1.Condition
    AppliedNodes       []string              // 5000 entries
    NodeEvaluations    []NodeEvaluation      // 5000 entries with full details
    CompletedNodes     []string
    FailedNodes        []NodeFailure
    DryRunResults      *DryRunResults
}

type NodeEvaluation struct {
    NodeName         string
    ConditionResults []ConditionEvaluationResult  // Multiple conditions per node
    TaintStatus      string
    LastEvaluated    metav1.Time
}
```

**Problem:** With 5000 nodes, this status object is ~5-10MB, causing etcd storage issues and slow API operations.

## Solution: Two-Tier Architecture

### Option 2: Aggregated Metrics in Rule Status (Operator View)

Keep `NodeReadinessGateRule` status lightweight with **aggregate metrics only**:

```go
type NodeReadinessGateRuleStatus struct {
    ObservedGeneration int64              `json:"observedGeneration,omitempty"`
    Conditions         []metav1.Condition `json:"conditions,omitempty"`

    // NEW: Aggregated metrics (O(1) space complexity)
    Summary NodeEvaluationSummary `json:"summary"`

    // Only track failures (bounded by failure rate, not node count)
    FailedNodes []NodeFailure `json:"failedNodes,omitempty"`

    // Dry run results (only when dryRun: true)
    DryRunResults *DryRunResults `json:"dryRunResults,omitempty"`

    // REMOVED: AppliedNodes, NodeEvaluations, CompletedNodes
}

type NodeEvaluationSummary struct {
    // Aggregate counts
    TotalApplicableNodes int `json:"totalApplicableNodes"`
    NodesWithTaint       int `json:"nodesWithTaint"`
    NodesWithoutTaint    int `json:"nodesWithoutTaint"`
    NodesReady           int `json:"nodesReady"`         // All conditions satisfied
    NodesNotReady        int `json:"nodesNotReady"`      // Some conditions unsatisfied
    NodesUnknown         int `json:"nodesUnknown"`       // Missing conditions

    // Bootstrap-only specific
    BootstrapCompletedNodes int `json:"bootstrapCompletedNodes,omitempty"`

    // Condition-specific breakdown
    ConditionSummaries []ConditionSummary `json:"conditionSummaries,omitempty"`

    // Metadata
    LastUpdated metav1.Time `json:"lastUpdated"`
}

type ConditionSummary struct {
    Type                string `json:"type"`
    NodesSatisfied      int    `json:"nodesSatisfied"`
    NodesUnsatisfied    int    `json:"nodesUnsatisfied"`
    NodesMissing        int    `json:"nodesMissing"`
}
```

**Benefits:**
- Status size: ~1KB regardless of node count
- Single status update updates all metrics
- Operators see cluster health at a glance
- No per-node enumeration needed

**Example kubectl output:**
```yaml
status:
  summary:
    totalApplicableNodes: 4873
    nodesReady: 4870
    nodesNotReady: 2
    nodesWithTaint: 2
    nodesWithoutTaint: 4871
    conditionSummaries:
    - type: network.k8s.io/CalicoReady
      nodesSatisfied: 4870
      nodesUnsatisfied: 1
      nodesMissing: 2
    lastUpdated: "2025-10-14T06:30:00Z"
  failedNodes:
  - nodeName: worker-1234
    reason: EvaluationError
    message: "Failed to patch node: connection timeout"
    lastUpdated: "2025-10-14T06:29:45Z"
```

### Option 3: Separate NodeReadinessEvaluation CRD (Per-Node Detail)

Create a new CRD for per-node evaluation details:

```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=nre
// +kubebuilder:printcolumn:name="Node",type=string,JSONPath=`.spec.nodeName`
// +kubebuilder:printcolumn:name="Rule",type=string,JSONPath=`.spec.ruleName`
// +kubebuilder:printcolumn:name="Taint",type=string,JSONPath=`.status.taintStatus`
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.allConditionsSatisfied`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type NodeReadinessEvaluation struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   NodeReadinessEvaluationSpec   `json:"spec"`
    Status NodeReadinessEvaluationStatus `json:"status,omitempty"`
}

type NodeReadinessEvaluationSpec struct {
    // Immutable references
    RuleName string `json:"ruleName"`
    NodeName string `json:"nodeName"`

    // Copy of rule spec at evaluation time (for historical tracking)
    RuleGeneration int64                   `json:"ruleGeneration"`
    Conditions     []ConditionRequirement  `json:"conditions"`
    Taint          TaintSpec               `json:"taint"`
}

type NodeReadinessEvaluationStatus struct {
    // Evaluation results
    ConditionResults        []ConditionEvaluationResult `json:"conditionResults"`
    AllConditionsSatisfied  bool                        `json:"allConditionsSatisfied"`
    TaintStatus             string                      `json:"taintStatus"` // Present, Absent, Unknown

    // Metadata
    LastEvaluated     metav1.Time `json:"lastEvaluated"`
    EvaluationCount   int         `json:"evaluationCount"`

    // Error tracking
    LastError         string      `json:"lastError,omitempty"`
    LastErrorTime     metav1.Time `json:"lastErrorTime,omitempty"`
    ConsecutiveErrors int         `json:"consecutiveErrors,omitempty"`
}
```

**Naming Convention:**
```
<rule-name>-<node-name>
Example: network-readiness-rule-worker2
```

**Benefits:**
- Independent per-node objects (no contention)
- Standard Kubernetes garbage collection (delete rule → delete evaluations)
- Can query specific node: `kubectl get nre network-readiness-rule-worker2`
- Can list by rule: `kubectl get nre -l rule=network-readiness-rule`
- Can list by node: `kubectl get nre -l node=worker2`
- Historical tracking via resource versions

**Example kubectl output:**
```bash
$ kubectl get nodereadinesseval -l rule=network-readiness-rule
NAME                                    NODE        RULE                      TAINT   READY   AGE
network-readiness-rule-worker1          worker1     network-readiness-rule    Absent  true    5m
network-readiness-rule-worker2          worker2     network-readiness-rule    Absent  true    5m
network-readiness-rule-worker3          worker3     network-readiness-rule    Present false   5m
...

$ kubectl get nre network-readiness-rule-worker3 -o yaml
apiVersion: nodereadiness.io/v1alpha1
kind: NodeReadinessEvaluation
metadata:
  name: network-readiness-rule-worker3
  labels:
    rule: network-readiness-rule
    node: worker3
spec:
  ruleName: network-readiness-rule
  nodeName: worker3
  ruleGeneration: 1
  conditions:
  - type: network.k8s.io/CalicoReady
    requiredStatus: "True"
  taint:
    key: readiness.k8s.io/NetworkReady
    effect: NoSchedule
status:
  conditionResults:
  - type: network.k8s.io/CalicoReady
    currentStatus: Unknown
    requiredStatus: "True"
    satisfied: false
    missing: true
  allConditionsSatisfied: false
  taintStatus: Present
  lastEvaluated: "2025-10-14T06:30:15Z"
  evaluationCount: 42
```

## Implementation Plan

### Phase 1: Add NodeReadinessEvaluation CRD (Non-Breaking)

**Files to create/modify:**
1. `api/v1alpha1/nodereadinessevaluation_types.go` - New CRD types
2. `api/v1alpha1/groupversion_info.go` - Register new type
3. `internal/controller/nodereadinessevaluation_controller.go` - Optional controller for cleanup
4. Update `config/crd/bases/` - Generated CRD manifests
5. Update RBAC for new resource

**Controller changes:**
- `evaluateRuleForNode()`: Create/update `NodeReadinessEvaluation` object instead of updating rule status
- Use `ownerReferences` to link evaluation to rule (automatic garbage collection)
- Batch evaluation updates using client-side caching

**Testing:**
- Unit tests for CRD creation/update
- Integration tests for garbage collection
- Scale test with 1000+ nodes

**Timeline:** 2-3 days

### Phase 2: Migrate Rule Status to Aggregated Metrics (Breaking Change)

**Files to modify:**
1. `api/v1alpha1/nodereadinessgaterule_types.go`:
   - Remove `NodeEvaluations []NodeEvaluation`
   - Remove `AppliedNodes []string`
   - Remove `CompletedNodes []string`
   - Add `Summary NodeEvaluationSummary`

2. `internal/controller/nodereadinessgaterule_controller.go`:
   - Update `updateStatusFromEvaluations()` to compute aggregates
   - Remove per-node enumeration logic
   - Add aggregate computation from `NodeReadinessEvaluation` list

3. `internal/controller/node_controller.go`:
   - Update `processNodeAgainstAllRules()` to update aggregates
   - Implement efficient aggregate updates (read current, increment/decrement counters)

**Migration strategy:**
- Version bump to `v1alpha2` (breaking change)
- Migration guide for users
- Conversion webhook for in-place upgrades (optional)

**Testing:**
- Unit tests for aggregate computation
- Integration tests for status updates
- Scale test with 5000 nodes
- Performance benchmarks (status update latency)

**Timeline:** 3-4 days

### Phase 3: Optimize Status Updates (Performance)

**Optimizations:**
1. **Debounced status updates**: Batch multiple node evaluations before updating rule status
   - Use work queue with rate limiting
   - Update rule status at most once per 5 seconds per rule

2. **Efficient aggregate computation**:
   - Cache previous summary
   - Only recompute changed counters (increment/decrement)
   - Avoid full node list enumeration

3. **Status update prioritization**:
   - High priority: Failed nodes (immediate update)
   - Normal priority: Aggregate metrics (5s debounce)
   - Low priority: Dry run results (10s debounce)

**Implementation:**
```go
type ReadinessGateController struct {
    ...
    statusUpdateQueue   workqueue.RateLimitingInterface
    statusDebouncer     map[string]*time.Timer
    statusDebounceLock  sync.Mutex
}

func (r *ReadinessGateController) enqueueStatusUpdate(ruleName string) {
    r.statusDebounceLock.Lock()
    defer r.statusDebounceLock.Unlock()

    // Cancel existing timer
    if timer, exists := r.statusDebouncer[ruleName]; exists {
        timer.Stop()
    }

    // Create new debounced timer
    r.statusDebouncer[ruleName] = time.AfterFunc(5*time.Second, func() {
        r.statusUpdateQueue.Add(ruleName)
    })
}
```

**Testing:**
- Load test with 5000 nodes and 10 rules
- Measure API call rate reduction
- Measure status lag (time from evaluation to status update)
- Benchmark memory usage

**Timeline:** 2-3 days

## Expected Outcomes

### Before (Current)
- Rule status size: **5-10 MB** (5000 nodes)
- API calls per reconciliation: **1 per node** (5000 total)
- etcd storage: **~50 MB per rule**
- Status update time: **2-5 seconds** (large object serialization)

### After (Optimized)
- Rule status size: **~1 KB** (aggregates only)
- Evaluation object size: **~500 bytes per node** (2.5 MB total for 5000 nodes)
- API calls per reconciliation: **1 per node** (evaluation) + **1 per 5s** (rule status)
- etcd storage: **~3 MB per rule** (2.5 MB evaluations + 1 KB status)
- Status update time: **<100 ms** (small object)
- Status lag: **<5 seconds** (debounced)

### Scalability Improvements
- **50x reduction** in rule status size
- **100x reduction** in status update frequency
- **Linear scaling** to 10,000+ nodes
- **No API server throttling** from status updates

## Rollout Strategy

### Stage 1: Alpha (v1alpha2)
- Deploy to test clusters only
- Gather performance metrics
- Iterate on aggregate metrics based on operator feedback
- Document migration path

### Stage 2: Beta (v1beta1)
- Deploy to staging environments
- Provide migration tooling
- Update documentation and examples
- Publish performance benchmarks

### Stage 3: GA (v1)
- Stable API contract
- Full backward compatibility guarantees
- Production-ready at scale

## Backward Compatibility

### For Users
- **Breaking change**: Existing status fields removed
- **Migration required**: Update monitoring/alerting that reads status
- **New queries**: Use `NodeReadinessEvaluation` for per-node details

### Migration Script
```bash
#!/bin/bash
# Backup existing rule statuses
kubectl get nodereadinessgaterules -o yaml > backup-rules.yaml

# Apply v1alpha2 CRDs
kubectl apply -f config/crd/bases/

# Controller will automatically:
# 1. Create NodeReadinessEvaluation objects for each node
# 2. Compute aggregates and update rule status
# 3. Garbage collect old evaluation objects when rules deleted
```

## Open Questions

1. **Evaluation object retention**: Should we auto-delete evaluations for nodes that no longer match selectors?
   - **Decision**: Yes, use Kubernetes garbage collection with `ownerReferences`

2. **Aggregate computation frequency**: How often to recompute aggregates?
   - **Decision**: Debounced (5s) with immediate updates for failures

3. **Historical evaluation tracking**: Should we keep evaluation history?
   - **Decision**: No, use metrics/logging for historical data (separate from CRD)

4. **Cross-rule queries**: How to find all rules affecting a node?
   - **Decision**: Label evaluations with both `rule` and `node` labels

5. **API pagination**: Do we need pagination for large node lists?
   - **Decision**: No, with per-node evaluations, each object is small enough

## Success Metrics

- ✅ Cluster with 5000 nodes reconciles without API throttling
- ✅ Rule status update latency < 100ms
- ✅ etcd storage per rule < 5 MB
- ✅ Status lag < 5 seconds in steady state
- ✅ Zero status update errors under load
- ✅ Operator can query node-specific details via `kubectl get nre`

## References

- [Kubernetes API Conventions - Status](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status)
- [etcd Storage Limits](https://etcd.io/docs/v3.5/dev-guide/limit/)
- [Controller Runtime - Optimistic Concurrency](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/client#Client)
