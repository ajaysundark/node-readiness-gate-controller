/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeReadinessRuleSpec defines the desired state of NodeReadinessRule
type NodeReadinessRuleSpec struct {
	// Conditions specifies a list of node conditions that must be satisfied for this rule to be applied
	// +required
	Conditions []ConditionRequirement `json:"conditions"`

	// EnforcementMode determines how the rule is enforced: either once during node bootstrap or continuously
	// +required
	EnforcementMode EnforcementMode `json:"enforcementMode"`

	// Taint specifies the taint to be applied to nodes that do not satisfy the conditions
	// +required
	Taint TaintSpec `json:"taint"`

	// NodeSelector is a label selector to filter which nodes this rule applies to
	// +optional
	NodeSelector *metav1.LabelSelector `json:"nodeSelector,omitempty"`

	// GracePeriod specifies how long to wait before applying the taint after conditions are not met
	// +optional
	GracePeriod *metav1.Duration `json:"gracePeriod,omitempty"`

	// DryRun if true, shows what would happen without actually applying taints
	// +optional
	DryRun bool `json:"dryRun,omitempty"`
}

// ConditionRequirement specifies a required node condition and its expected status
type ConditionRequirement struct {
	// Type is the name of the node condition to check
	// +required
	Type string `json:"type"`

	// RequiredStatus is the expected status of the condition (True, False, or Unknown)
	// +required
	RequiredStatus corev1.ConditionStatus `json:"requiredStatus"`
}

// TaintSpec defines a Kubernetes taint to be managed by this controller
type TaintSpec struct {
	// Key is the taint key to be applied
	// +required
	Key string `json:"key"`

	// Effect indicates what effect this taint has (NoSchedule, PreferNoSchedule, or NoExecute)
	// +required
	Effect corev1.TaintEffect `json:"effect"`

	// Value is an optional value associated with the taint
	// +optional
	Value string `json:"value,omitempty"`
}

// EnforcementMode defines how the NodeReadinessRule should be enforced
type EnforcementMode string

const (
	// EnforcementModeBootstrapOnly indicates the rule should only be enforced during node initialization
	EnforcementModeBootstrapOnly EnforcementMode = "bootstrap-only"
	// EnforcementModeContinuous indicates the rule should be enforced continuously throughout the node's lifecycle
	EnforcementModeContinuous EnforcementMode = "continuous"
)

// NodeReadinessRuleStatus defines the observed state of NodeReadinessRule
type NodeReadinessRuleStatus struct {
	// ObservedGeneration reflects the generation of the most recently observed NodeReadinessRule
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest available observations of the rule's current state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// AppliedNodes is a list of nodes where this rule has been successfully applied
	// +optional
	AppliedNodes []string `json:"appliedNodes,omitempty"`

	// NodeEvaluations contains detailed evaluation results for each affected node
	// +optional
	NodeEvaluations []NodeEvaluation `json:"nodeEvaluations,omitempty"`

	// CompletedNodes lists nodes that have completed bootstrap-mode evaluation
	// +optional
	CompletedNodes []string `json:"completedNodes,omitempty"`

	// FailedNodes lists nodes where rule application has failed, with failure details
	// +optional
	FailedNodes []NodeFailure `json:"failedNodes,omitempty"`

	// DryRunResults contains the analysis results when running in dry-run mode
	// +optional
	DryRunResults *DryRunResults `json:"dryRunResults,omitempty"`
}

// NodeEvaluation represents the evaluation state of a single node
type NodeEvaluation struct {
	// NodeName is the name of the evaluated node
	// +required
	NodeName string `json:"nodeName"`

	// ConditionResults contains the evaluation results for each condition
	// +required
	ConditionResults []ConditionEvaluationResult `json:"conditionResults"`

	// TaintStatus indicates whether the taint is Present, Absent, or Unknown on the node
	// +required
	TaintStatus string `json:"taintStatus"`

	// LastEvaluated is the timestamp of the most recent evaluation
	// +required
	LastEvaluated metav1.Time `json:"lastEvaluated"`
}

// ConditionEvaluationResult represents the evaluation of a single condition requirement
type ConditionEvaluationResult struct {
	// Type is the name of the evaluated condition
	// +required
	Type string `json:"type"`

	// CurrentStatus is the actual status of the condition on the node
	// +required
	CurrentStatus corev1.ConditionStatus `json:"currentStatus"`

	// RequiredStatus is the status required by the rule
	// +required
	RequiredStatus corev1.ConditionStatus `json:"requiredStatus"`

	// Satisfied indicates whether the condition requirement is met
	// +required
	Satisfied bool `json:"satisfied"`

	// Missing indicates whether the condition is present on the node
	// +required
	Missing bool `json:"missing"`
}

// NodeFailure represents a failure to apply the rule to a specific node
type NodeFailure struct {
	// NodeName is the name of the node where the failure occurred
	// +required
	NodeName string `json:"nodeName"`

	// Reason is a brief reason for the failure
	// +required
	Reason string `json:"reason"`

	// Message is a detailed explanation of the failure
	// +optional
	Message string `json:"message"`

	// LastUpdated is when this failure was last updated
	// +required
	LastUpdated metav1.Time `json:"lastUpdated"`
}

// DryRunResults contains the analysis of what would happen if the rule was applied
type DryRunResults struct {
	// AffectedNodes is the count of nodes that would be affected
	// +required
	AffectedNodes int `json:"affectedNodes"`

	// TaintsToAdd is the count of taints that would be added
	// +required
	TaintsToAdd int `json:"taintsToAdd"`

	// TaintsToRemove is the count of taints that would be removed
	// +required
	TaintsToRemove int `json:"taintsToRemove"`

	// RiskyOperations is the count of operations that might impact workload availability
	// +required
	RiskyOperations int `json:"riskyOperations"`

	// Summary provides a human-readable summary of the dry run analysis
	// +required
	Summary string `json:"summary"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=nrr

// NodeReadinessRule is the Schema for the nodereadinessrules API
type NodeReadinessRule struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of NodeReadinessRule
	// +required
	Spec NodeReadinessRuleSpec `json:"spec"`

	// status defines the observed state of NodeReadinessRule
	// +optional
	Status NodeReadinessRuleStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// NodeReadinessRuleList contains a list of NodeReadinessRule
type NodeReadinessRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeReadinessRule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeReadinessRule{}, &NodeReadinessRuleList{})
}
