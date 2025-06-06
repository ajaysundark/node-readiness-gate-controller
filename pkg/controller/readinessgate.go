package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// ReadinessGateConfig defines the configuration for a node readiness gate
type ReadinessGateConfig struct {
	ConditionType  string                 `json:"conditionType"`
	TaintKey       string                 `json:"taintKey"`
	TaintEffect    corev1.TaintEffect     `json:"taintEffect"`
	NodeSelector   map[string]string      `json:"nodeSelector,omitempty"`
	RequiredStatus corev1.ConditionStatus `json:"requiredStatus"`
}

// ReadinessGateController manages node taints based on node conditions
type ReadinessGateController struct {
	clientset    kubernetes.Interface
	nodeInformer cache.SharedIndexInformer
	config       ReadinessGateConfig
	stopCh       <-chan struct{}
}

// NewReadinessGateController creates a new ReadinessGateController
func NewReadinessGateController(
	clientset kubernetes.Interface,
	config ReadinessGateConfig,
) *ReadinessGateController {

	// Create node informer
	// informersFactory := informers.NewSharedInformerFactory(clientset, 0)
	// nodeInformer := informersFactory.Core().V1().Nodes()

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
		clientset:    clientset,
		nodeInformer: informer,
		config:       config,
	}

	// Add event handlers
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.handleNodeAdd,
		UpdateFunc: controller.handleNodeUpdate,
		DeleteFunc: controller.handleNodeDelete,
	})

	return controller
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
	if !c.nodeMatches(node) {
		klog.Infof("Node %s does not match selector, skipping", node.Name)
		return
	}

	// Get the condition status
	conditionStatus := c.getConditionStatus(node)
	klog.Infof("Node %s condition %s status: %s",
		node.Name, c.config.ConditionType, conditionStatus)

	// Determine if taint should be present
	shouldHaveTaint := conditionStatus != c.config.RequiredStatus
	hasTaint := c.hasTaint(node)

	klog.Infof("Node %s: shouldHaveTaint=%t, hasTaint=%t",
		node.Name, shouldHaveTaint, hasTaint)

	// Update taint if needed
	if shouldHaveTaint && !hasTaint {
		c.addTaint(node)
	} else if !shouldHaveTaint && hasTaint {
		c.removeTaint(node)
	}
}

func (c *ReadinessGateController) nodeMatches(node *corev1.Node) bool {
	if len(c.config.NodeSelector) == 0 {
		return true
	}

	for key, value := range c.config.NodeSelector {
		if nodeValue, exists := node.Labels[key]; !exists || nodeValue != value {
			return false
		}
	}

	return true
}

func (c *ReadinessGateController) getConditionStatus(node *corev1.Node) corev1.ConditionStatus {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeConditionType(c.config.ConditionType) {
			return condition.Status
		}
	}
	return corev1.ConditionUnknown
}

func (c *ReadinessGateController) hasTaint(node *corev1.Node) bool {
	for _, taint := range node.Spec.Taints {
		if taint.Key == c.config.TaintKey && taint.Effect == c.config.TaintEffect {
			return true
		}
	}
	return false
}

func (c *ReadinessGateController) addTaint(node *corev1.Node) {
	klog.Infof("Adding taint %s to node %s", c.config.TaintKey, node.Name)

	newTaint := corev1.Taint{
		Key:       c.config.TaintKey,
		Effect:    c.config.TaintEffect,
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

func (c *ReadinessGateController) removeTaint(node *corev1.Node) {
	klog.Infof("Removing taint %s from node %s", c.config.TaintKey, node.Name)

	// Create a copy of the node
	nodeCopy := node.DeepCopy()

	// Remove the taint
	var newTaints []corev1.Taint
	for _, taint := range nodeCopy.Spec.Taints {
		if !(taint.Key == c.config.TaintKey && taint.Effect == c.config.TaintEffect) {
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
