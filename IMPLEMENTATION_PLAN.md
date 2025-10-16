# Implementation Plan: Scalability + Rename

Since this is unreleased code (v1alpha1), we can make breaking changes without migration concerns.

## Changes to Implement

### 1. Rename CRDs with Short Names
- `NodeReadinessGateRule` → `NodeReadinessRule` (short: `nrr`)
- New: `NodeReadinessEvaluation` (short: `nre`)

### 2. Add Aggregated Status to NodeReadinessRule
- Remove per-node arrays from status
- Add `Summary` field with aggregate metrics
- Keep only `FailedNodes` for debugging

### 3. Create NodeReadinessEvaluation CRD
- Per-node evaluation details
- Owned by rule (garbage collected automatically)
- Labeled for querying by rule or node

## File Changes Checklist

### Phase 1: Create NodeReadinessEvaluation CRD

**New Files:**
- [ ] `api/v1alpha1/nodereadinessevaluation_types.go` - CRD definition

**Modified Files:**
- [ ] `api/v1alpha1/groupversion_info.go` - Register new type in init()

### Phase 2: Rename NodeReadinessGateRule → NodeReadinessRule

**Renamed Files:**
- [ ] `api/v1alpha1/nodereadinessgaterule_types.go` → `nodereadinessrule_types.go`
- [ ] `internal/controller/nodereadinessgaterule_controller.go` → `nodereadinessrule_controller.go`
- [ ] `internal/controller/nodereadinessgaterule_controller_test.go` → `nodereadinessrule_controller_test.go`

**Files to Update (all references):**
- [ ] `api/v1alpha1/nodereadinessrule_types.go` - Type names, comments, kubebuilder markers
- [ ] `internal/controller/nodereadinessrule_controller.go` - All type references
- [ ] `internal/controller/node_controller.go` - Import paths and type references
- [ ] `internal/controller/nodereadinessrule_controller_test.go` - Type references
- [ ] `internal/controller/node_controller_test.go` - Type references
- [ ] `internal/controller/suite_test.go` - Type references
- [ ] `internal/webhook/nodereadinessgaterule_webhook.go` - Rename and update
- [ ] `internal/webhook/nodereadinessgaterule_webhook_test.go` - Rename and update
- [ ] `cmd/main.go` - Import paths and type references
- [ ] `config/crd/bases/*.yaml` - Will be regenerated
- [ ] `config/rbac/*.yaml` - Will be regenerated
- [ ] `config/samples/*.yaml` - Rename example files
- [ ] `examples/*.yaml` - Update apiVersion and kind
- [ ] `test/e2e/*.go` - Update type references
- [ ] `test/e2e/testdata/*.yaml` - Update apiVersion and kind
- [ ] `README.md` - Update all documentation
- [ ] `CONTEXT.md` - Update type references
- [ ] `hack/TEST_README.md` - Update commands

### Phase 3: Update Status Structure

**Modified Files:**
- [ ] `api/v1alpha1/nodereadinessrule_types.go`:
  - Remove `NodeEvaluations []NodeEvaluation`
  - Remove `AppliedNodes []string`
  - Remove `CompletedNodes []string`
  - Add `Summary NodeEvaluationSummary`
  - Keep `FailedNodes []NodeFailure`

- [ ] `internal/controller/nodereadinessrule_controller.go`:
  - Remove `updateNodeEvaluationStatus()` function
  - Add `updateSummaryMetrics()` function
  - Update `processAllNodesForRule()` to compute aggregates

- [ ] `internal/controller/node_controller.go`:
  - Remove `updateNodeEvaluationStatus()` function
  - Add `createOrUpdateEvaluation()` function to manage NodeReadinessEvaluation objects
  - Update `processNodeAgainstAllRules()` to create evaluation objects

### Phase 4: Update Tests

**Modified Files:**
- [ ] `internal/controller/nodereadinessrule_controller_test.go` - Update assertions
- [ ] `internal/controller/node_controller_test.go` - Update assertions
- [ ] `test/e2e/e2e_test.go` - Update type names and assertions

### Phase 5: Regenerate Manifests

**Commands:**
```bash
make manifests  # Regenerate CRDs and RBAC
make generate   # Regenerate deepcopy
```

**Generated Files:**
- [ ] `config/crd/bases/nodereadiness.io_nodereadinessrules.yaml`
- [ ] `config/crd/bases/nodereadiness.io_nodereadinessevaluations.yaml`
- [ ] `config/rbac/role.yaml`
- [ ] `api/v1alpha1/zz_generated.deepcopy.go`

## Implementation Order

### Step 1: Create NodeReadinessEvaluation types (no controller logic yet)
```bash
# Just add the types, don't wire up controller yet
# This allows us to generate CRDs and test the structure
touch api/v1alpha1/nodereadinessevaluation_types.go
# Edit and add type definitions
make manifests
```

### Step 2: Rename NodeReadinessGateRule → NodeReadinessRule
```bash
# Rename files
mv api/v1alpha1/nodereadinessgaterule_types.go api/v1alpha1/nodereadinessrule_types.go
mv internal/controller/nodereadinessgaterule_controller.go internal/controller/nodereadinessrule_controller.go
mv internal/controller/nodereadinessgaterule_controller_test.go internal/controller/nodereadinessrule_controller_test.go
mv internal/webhook/nodereadinessgaterule_webhook.go internal/webhook/nodereadinessrule_webhook.go
mv internal/webhook/nodereadinessgaterule_webhook_test.go internal/webhook/nodereadinessrule_webhook_test.go

# Update all type references (find/replace)
# NodeReadinessGateRule → NodeReadinessRule
# nodereadinessgaterule → nodereadinessrule
# NodeReadinessGateRuleSpec → NodeReadinessRuleSpec
# etc.
```

### Step 3: Update status structure (aggregates + remove per-node)
```bash
# Edit api/v1alpha1/nodereadinessrule_types.go
# Add Summary field, remove NodeEvaluations
make manifests
```

### Step 4: Wire up evaluation object creation in controller
```bash
# Update internal/controller/node_controller.go
# Add createOrUpdateEvaluation() function
# Update evaluateRuleForNode() to create evaluation objects
```

### Step 5: Update summary computation in controller
```bash
# Update internal/controller/nodereadinessrule_controller.go
# Add computeSummary() function
# Update rule reconciler to compute aggregates from evaluations
```

### Step 6: Update all tests
```bash
make test
make test-e2e
```

### Step 7: Update documentation
```bash
# Update README.md, CONTEXT.md, examples/
```

## New Type Definitions (Quick Reference)

### NodeReadinessEvaluation
```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=nre
// +kubebuilder:printcolumn:name="Node",type=string,JSONPath=`.spec.nodeName`
// +kubebuilder:printcolumn:name="Rule",type=string,JSONPath=`.spec.ruleName`
// +kubebuilder:printcolumn:name="TaintStatus",type=string,JSONPath=`.status.taintStatus`
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.allConditionsSatisfied`

type NodeReadinessEvaluation struct {
    metav1.TypeMeta
    metav1.ObjectMeta
    Spec   NodeReadinessEvaluationSpec
    Status NodeReadinessEvaluationStatus
}

type NodeReadinessEvaluationSpec struct {
    RuleName       string
    NodeName       string
    RuleGeneration int64
}

type NodeReadinessEvaluationStatus struct {
    ConditionResults       []ConditionEvaluationResult
    AllConditionsSatisfied bool
    TaintStatus            string  // "Present", "Absent", "Unknown"
    LastEvaluated          metav1.Time
}
```

### NodeReadinessRule (updated status)
```go
// +kubebuilder:resource:scope=Cluster,shortName=nrr

type NodeReadinessRuleStatus struct {
    ObservedGeneration int64
    Conditions         []metav1.Condition

    // NEW: Aggregated summary
    Summary NodeEvaluationSummary

    // Keep for debugging
    FailedNodes []NodeFailure

    // Keep for dry run
    DryRunResults *DryRunResults

    // REMOVED: AppliedNodes, NodeEvaluations, CompletedNodes
}

type NodeEvaluationSummary struct {
    TotalApplicableNodes    int
    NodesWithTaint          int
    NodesWithoutTaint       int
    NodesReady              int
    NodesNotReady           int
    BootstrapCompletedNodes int
    ConditionSummaries      []ConditionSummary
    LastUpdated             metav1.Time
}

type ConditionSummary struct {
    Type             string
    NodesSatisfied   int
    NodesUnsatisfied int
    NodesMissing     int
}
```

## Controller Logic Changes

### Creating Evaluation Objects
```go
// In node_controller.go - after evaluating node against rule
func (r *ReadinessGateController) createOrUpdateEvaluation(
    ctx context.Context,
    rule *readinessv1alpha1.NodeReadinessRule,
    node *corev1.Node,
    conditionResults []readinessv1alpha1.ConditionEvaluationResult,
    taintStatus string,
) error {
    evalName := fmt.Sprintf("%s-%s", rule.Name, node.Name)

    eval := &readinessv1alpha1.NodeReadinessEvaluation{}
    err := r.Get(ctx, client.ObjectKey{Name: evalName}, eval)

    if err != nil && errors.IsNotFound(err) {
        // Create new
        eval = &readinessv1alpha1.NodeReadinessEvaluation{
            ObjectMeta: metav1.ObjectMeta{
                Name: evalName,
                Labels: map[string]string{
                    "rule": rule.Name,
                    "node": node.Name,
                },
                OwnerReferences: []metav1.OwnerReference{
                    *metav1.NewControllerRef(rule, readinessv1alpha1.GroupVersion.WithKind("NodeReadinessRule")),
                },
            },
            Spec: readinessv1alpha1.NodeReadinessEvaluationSpec{
                RuleName:       rule.Name,
                NodeName:       node.Name,
                RuleGeneration: rule.Generation,
            },
        }
        if err := r.Create(ctx, eval); err != nil {
            return err
        }
    }

    // Update status
    eval.Status.ConditionResults = conditionResults
    eval.Status.AllConditionsSatisfied = allConditionsSatisfied(conditionResults)
    eval.Status.TaintStatus = taintStatus
    eval.Status.LastEvaluated = metav1.Now()

    return r.Status().Update(ctx, eval)
}
```

### Computing Summary
```go
// In nodereadinessrule_controller.go
func (r *RuleReconciler) computeSummary(
    ctx context.Context,
    rule *readinessv1alpha1.NodeReadinessRule,
) (readinessv1alpha1.NodeEvaluationSummary, error) {
    // List all evaluations for this rule
    evalList := &readinessv1alpha1.NodeReadinessEvaluationList{}
    err := r.List(ctx, evalList, client.MatchingLabels{"rule": rule.Name})
    if err != nil {
        return readinessv1alpha1.NodeEvaluationSummary{}, err
    }

    summary := readinessv1alpha1.NodeEvaluationSummary{
        TotalApplicableNodes: len(evalList.Items),
        LastUpdated:          metav1.Now(),
    }

    conditionCounts := make(map[string]*readinessv1alpha1.ConditionSummary)

    for _, eval := range evalList.Items {
        // Count taint status
        if eval.Status.TaintStatus == "Present" {
            summary.NodesWithTaint++
        } else {
            summary.NodesWithoutTaint++
        }

        // Count readiness
        if eval.Status.AllConditionsSatisfied {
            summary.NodesReady++
        } else {
            summary.NodesNotReady++
        }

        // Count condition-specific metrics
        for _, condResult := range eval.Status.ConditionResults {
            if conditionCounts[condResult.Type] == nil {
                conditionCounts[condResult.Type] = &readinessv1alpha1.ConditionSummary{
                    Type: condResult.Type,
                }
            }
            cs := conditionCounts[condResult.Type]
            if condResult.Missing {
                cs.NodesMissing++
            } else if condResult.Satisfied {
                cs.NodesSatisfied++
            } else {
                cs.NodesUnsatisfied++
            }
        }
    }

    // Convert map to slice
    for _, cs := range conditionCounts {
        summary.ConditionSummaries = append(summary.ConditionSummaries, *cs)
    }

    return summary, nil
}
```

## Testing Strategy

1. **Unit tests**: Test evaluation object creation and summary computation
2. **Integration tests**: Test garbage collection when rule is deleted
3. **E2E tests**: Test with multiple nodes and rules
4. **Scale test**: Test with 1000+ mock nodes (use kwok)

## Risks and Mitigations

**Risk**: Evaluation object creation adds API call overhead
- **Mitigation**: Use client-side caching, batch updates

**Risk**: Summary computation requires listing all evaluations
- **Mitigation**: Use indexed labels for efficient queries, cache results

**Risk**: Evaluation objects proliferate and consume etcd space
- **Mitigation**: Garbage collection via ownerReferences, monitor etcd usage

## Success Criteria

- [ ] `kubectl get nrr` works (short name)
- [ ] `kubectl get nre` works (short name)
- [ ] Rule status shows aggregate summary
- [ ] Per-node details visible via `kubectl get nre <name>`
- [ ] Evaluation objects deleted when rule deleted
- [ ] Tests pass
- [ ] Documentation updated
