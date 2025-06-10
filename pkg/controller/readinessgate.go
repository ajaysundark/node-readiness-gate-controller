package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// NodeReadinessRule defines the desired state for a component readiness in a node
type NodeReadinessRule struct {
	ConditionType  string                 `json:"conditionType"`
	TaintKey       string                 `json:"taintKey"`
	TaintEffect    corev1.TaintEffect     `json:"taintEffect"`
	NodeSelector   map[string]string      `json:"nodeSelector,omitempty"`
	RequiredStatus corev1.ConditionStatus `json:"requiredStatus"`
}

// ReadinessGateController manages node taints based on node conditions
type ReadinessGateController struct {
	clientset      kubernetes.Interface
	nodeInformer   cache.SharedIndexInformer
	readinessRules map[string]*NodeReadinessRule // conditionType -> readiness-rule
}

// NewReadinessGateController creates a new ReadinessGateController
func NewReadinessGateController(
	clientset kubernetes.Interface,
) *ReadinessGateController {
	watchlist := cache.NewListWatchFromClient(
		clientset.CoreV1().RESTClient(),
		"nodes",
		metav1.NamespaceAll,
		fields.Everything(),
	)

	informer := cache.NewSharedIndexInformer(
		watchlist,
		&corev1.Node{},
		time.Second*30,
		cache.Indexers{},
	)

	controller := &ReadinessGateController{
		clientset:      clientset,
		nodeInformer:   informer,
		readinessRules: make(map[string]*NodeReadinessRule),
	}

	// Add event handlers
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.handleNodeAdd,
		UpdateFunc: controller.handleNodeUpdate,
		DeleteFunc: controller.handleNodeDelete,
	})

	return controller
}

// AddReadinessRule updates the cached NodeReadinessRules
func (r *ReadinessGateController) AddReadinessRule(rule *NodeReadinessRule) {
	klog.Infof("Adding readiness rule: %s", rule.ConditionType)
	r.readinessRules[rule.ConditionType] = rule
}

// RemoveReadinessRule removes a readiness rule from the controller
func (r *ReadinessGateController) RemoveReadinessRule(conditionType string) {
	delete(r.readinessRules, conditionType)
	klog.Infof("Removed readiness rule: %s", conditionType)
}

// Run starts the controller
func (c *ReadinessGateController) Run(ctx context.Context) error {
	klog.Info("Starting ReadinessGate Controller")

	// Start the informer
	go c.nodeInformer.Run(ctx.Done())

	// Wait for caches to sync
	if !cache.WaitForCacheSync(ctx.Done(), c.nodeInformer.HasSynced) {
		return fmt.Errorf("failed to sync caches")
	}

	klog.Info("ReadinessGate Controller synced and ready")

	// Block until context is done
	<-ctx.Done()
	klog.Info("Shutting down ReadinessGate Controller")

	return nil
}

func (c *ReadinessGateController) handleNodeAdd(obj interface{}) {
	node := obj.(*corev1.Node)
	klog.Infof("Node added: %s", node.Name)
	c.processNode(node)
}

func (c *ReadinessGateController) handleNodeUpdate(oldObj, newObj interface{}) {
	node := newObj.(*corev1.Node)
	klog.Infof("Node updated: %s", node.Name)
	c.processNode(node)
}

func (c *ReadinessGateController) handleNodeDelete(obj interface{}) {
	node := obj.(*corev1.Node)
	klog.Infof("Node deleted: %s", node.Name)
	// No action needed for deletion
}

func (c *ReadinessGateController) processNode(node *corev1.Node) {
	// Check if node matches selector
	applicableRules := c.getApplicableRulesForNode(node)

	for _, readinessRule := range applicableRules {
		// Retrieve the condition status
		conditionStatus := c.getConditionStatus(node, readinessRule.ConditionType)

		// Determine if taint should be present
		shouldHaveTaint := conditionStatus != readinessRule.RequiredStatus
		hasTaint := c.hasTaint(node, readinessRule.TaintKey, readinessRule.TaintEffect)

		klog.V(4).Info("Processing condition",
			"node", node.Name,
			"conditionType", readinessRule.ConditionType,
			"conditionStatus", conditionStatus,
			"requiredStatus", readinessRule.RequiredStatus,
			"shouldHaveTaint", shouldHaveTaint,
			"hasTaint", hasTaint)

		// Update taint if needed
		if shouldHaveTaint && !hasTaint {
			c.addTaint(node, readinessRule.TaintKey, readinessRule.TaintEffect)
		} else if !shouldHaveTaint && hasTaint {
			c.removeTaint(node, readinessRule.TaintKey, readinessRule.TaintEffect)
		}
	}
}

func (r *ReadinessGateController) getApplicableRulesForNode(node *corev1.Node) []*NodeReadinessRule {
	var applicableRules []*NodeReadinessRule
	for _, rule := range r.readinessRules {
		if r.ruleAppliesTo(rule, node) {
			applicableRules = append(applicableRules, rule)
		}
	}
	return applicableRules
}

// ruleAppliesTo check if node matches selector
func (r *ReadinessGateController) ruleAppliesTo(rule *NodeReadinessRule, node *corev1.Node) bool {
	if len(rule.NodeSelector) == 0 {
		return true
	}

	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: rule.NodeSelector,
	})
	if err != nil {
		klog.Errorf("Failed to parse node selector: %v", err)
		return false
	}

	return selector.Matches(labels.Set(node.Labels))
}

func (c *ReadinessGateController) getConditionStatus(node *corev1.Node, conditionType string) corev1.ConditionStatus {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeConditionType(conditionType) {
			return condition.Status
		}
	}
	return corev1.ConditionUnknown
}

func (c *ReadinessGateController) hasTaint(node *corev1.Node, taintKey string, taintEffect corev1.TaintEffect) bool {
	for _, taint := range node.Spec.Taints {
		if taint.Key == taintKey && taint.Effect == taintEffect {
			return true
		}
	}
	return false
}

func (c *ReadinessGateController) addTaint(node *corev1.Node, taintKey string, taintEffect corev1.TaintEffect) {
	klog.Infof("Adding taint %s to node %s", taintKey, node.Name)

	newTaint := corev1.Taint{
		Key:       taintKey,
		Effect:    taintEffect,
		TimeAdded: &metav1.Time{Time: time.Now()},
	}

	// Create a copy of the node
	nodeCopy := node.DeepCopy()
	nodeCopy.Spec.Taints = append(nodeCopy.Spec.Taints, newTaint)

	// Update the node
	_, err := c.clientset.CoreV1().Nodes().Update(
		context.TODO(), nodeCopy, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Failed to add taint to node %s: %v", node.Name, err)
	} else {
		klog.Infof("Successfully added taint to node %s", node.Name)
	}
}

func (c *ReadinessGateController) removeTaint(node *corev1.Node, taintKey string, taintEffect corev1.TaintEffect) {
	klog.Infof("Removing taint %s from node %s", taintKey, node.Name)

	// Create a copy of the node
	nodeCopy := node.DeepCopy()

	// Remove the taint
	var newTaints []corev1.Taint
	for _, taint := range nodeCopy.Spec.Taints {
		if !(taint.Key == taintKey && taint.Effect == taintEffect) {
			newTaints = append(newTaints, taint)
		}
	}
	nodeCopy.Spec.Taints = newTaints

	// Update the node
	_, err := c.clientset.CoreV1().Nodes().Update(
		context.TODO(), nodeCopy, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Failed to remove taint from node %s: %v", node.Name, err)
	} else {
		klog.Infof("Successfully removed taint from node %s", node.Name)
	}
}
