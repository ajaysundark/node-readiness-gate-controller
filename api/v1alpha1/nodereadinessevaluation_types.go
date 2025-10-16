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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeReadinessEvaluationSpec defines the desired state of NodeReadinessEvaluation
type NodeReadinessEvaluationSpec struct {
	// RuleName is the name of the NodeReadinessRule this evaluation belongs to
	// +kubebuilder:validation:Required
	RuleName string `json:"ruleName"`

	// NodeName is the name of the node being evaluated
	// +kubebuilder:validation:Required
	NodeName string `json:"nodeName"`

	// RuleGeneration is the generation of the rule at evaluation time
	// Used to track if the rule has changed since evaluation
	RuleGeneration int64 `json:"ruleGeneration"`
}

// NodeReadinessEvaluationStatus defines the observed state of NodeReadinessEvaluation
type NodeReadinessEvaluationStatus struct {
	// ConditionResults contains the evaluation result for each condition
	ConditionResults []ConditionEvaluationResult `json:"conditionResults,omitempty"`

	// AllConditionsSatisfied indicates whether all required conditions are satisfied
	AllConditionsSatisfied bool `json:"allConditionsSatisfied"`

	// TaintStatus indicates the current taint state: "Present", "Absent", or "Unknown"
	TaintStatus string `json:"taintStatus"`

	// LastEvaluated is the timestamp of the last evaluation
	LastEvaluated metav1.Time `json:"lastEvaluated"`

	// EvaluationCount tracks how many times this node has been evaluated
	EvaluationCount int `json:"evaluationCount,omitempty"`

	// LastError contains the error message from the last failed evaluation
	LastError string `json:"lastError,omitempty"`

	// LastErrorTime is the timestamp of the last error
	LastErrorTime *metav1.Time `json:"lastErrorTime,omitempty"`

	// ConsecutiveErrors tracks the number of consecutive evaluation failures
	ConsecutiveErrors int `json:"consecutiveErrors,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=nre
// +kubebuilder:printcolumn:name="Node",type=string,JSONPath=`.spec.nodeName`,priority=0
// +kubebuilder:printcolumn:name="Rule",type=string,JSONPath=`.spec.ruleName`,priority=0
// +kubebuilder:printcolumn:name="TaintStatus",type=string,JSONPath=`.status.taintStatus`,priority=0
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.allConditionsSatisfied`,priority=0
// +kubebuilder:printcolumn:name="Evaluations",type=integer,JSONPath=`.status.evaluationCount`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`,priority=0

// NodeReadinessEvaluation represents the evaluation state of a single node against a NodeReadinessRule
type NodeReadinessEvaluation struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of NodeReadinessEvaluation
	// +required
	Spec NodeReadinessEvaluationSpec `json:"spec"`

	// status defines the observed state of NodeReadinessEvaluation
	// +optional
	Status NodeReadinessEvaluationStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// NodeReadinessEvaluationList contains a list of NodeReadinessEvaluation
type NodeReadinessEvaluationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeReadinessEvaluation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeReadinessEvaluation{}, &NodeReadinessEvaluationList{})
}
