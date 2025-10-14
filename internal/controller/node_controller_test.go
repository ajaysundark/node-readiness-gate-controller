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

package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	nodereadinessiov1alpha1 "github.com/ajaysundark/node-readiness-gate-controller/api/v1alpha1"
)

var _ = Describe("Node Controller", func() {
	const (
		nodeName      = "test-node"
		ruleName      = "test-rule"
		taintKey      = "test-taint"
		conditionType = "TestCondition"
	)

	var (
		ctx                 context.Context
		readinessController *ReadinessGateController
		nodeReconciler      *NodeReconciler
		fakeClientset       *fake.Clientset
		node                *corev1.Node
		rule                *nodereadinessiov1alpha1.NodeReadinessGateRule
		namespacedName      types.NamespacedName
	)

	BeforeEach(func() {
		ctx = context.Background()

		fakeClientset = fake.NewSimpleClientset()
		readinessController = &ReadinessGateController{
			Client:    k8sClient,
			Scheme:    k8sClient.Scheme(),
			clientset: fakeClientset,
			ruleCache: make(map[string]*nodereadinessiov1alpha1.NodeReadinessGateRule),
		}

		nodeReconciler = &NodeReconciler{
			Client:     k8sClient,
			Scheme:     k8sClient.Scheme(),
			Controller: readinessController,
		}
		namespacedName = types.NamespacedName{Name: nodeName}

		node = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   nodeName,
				Labels: map[string]string{"env": "test"},
			},
			Spec: corev1.NodeSpec{
				Taints: []corev1.Taint{
					{Key: taintKey, Effect: corev1.TaintEffectNoSchedule},
				},
			},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{Type: conditionType, Status: corev1.ConditionFalse},
				},
			},
		}

		rule = &nodereadinessiov1alpha1.NodeReadinessGateRule{
			ObjectMeta: metav1.ObjectMeta{
				Name: ruleName,
			},
			Spec: nodereadinessiov1alpha1.NodeReadinessGateRuleSpec{
				Conditions: []nodereadinessiov1alpha1.ConditionRequirement{
					{Type: conditionType, RequiredStatus: corev1.ConditionTrue},
				},
				Taint: nodereadinessiov1alpha1.TaintSpec{
					Key:    taintKey,
					Effect: corev1.TaintEffectNoSchedule,
				},
				NodeSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"env": "test"},
				},
			},
		}
	})

	JustBeforeEach(func() {
		Expect(k8sClient.Create(ctx, node)).To(Succeed())
		Expect(k8sClient.Create(ctx, rule)).To(Succeed())

		// Manually add rule to cache to simulate RuleReconciler
		readinessController.updateRuleCache(ctx, rule)
	})

	AfterEach(func() {
		// Delete node first
		k8sClient.Delete(ctx, node)

		// Remove finalizers from rule before deleting to avoid stuck deletion
		updatedRule := &nodereadinessiov1alpha1.NodeReadinessGateRule{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: ruleName}, updatedRule); err == nil {
			updatedRule.Finalizers = nil
			k8sClient.Update(ctx, updatedRule)
		}
		k8sClient.Delete(ctx, rule)

		readinessController.removeRuleFromCache(ctx, ruleName)
	})

	Context("Helper function tests", func() {
		It("should correctly compare node conditions", func() {
			cond1 := []corev1.NodeCondition{
				{Type: "Ready", Status: corev1.ConditionTrue},
				{Type: "NetworkReady", Status: corev1.ConditionFalse},
			}
			cond2 := []corev1.NodeCondition{
				{Type: "Ready", Status: corev1.ConditionTrue},
				{Type: "NetworkReady", Status: corev1.ConditionFalse},
			}
			cond3 := []corev1.NodeCondition{
				{Type: "Ready", Status: corev1.ConditionFalse},
				{Type: "NetworkReady", Status: corev1.ConditionFalse},
			}
			cond4 := []corev1.NodeCondition{
				{Type: "Ready", Status: corev1.ConditionTrue},
			}

			Expect(conditionsEqual(cond1, cond2)).To(BeTrue(), "identical conditions should be equal")
			Expect(conditionsEqual(cond1, cond3)).To(BeFalse(), "different status should not be equal")
			Expect(conditionsEqual(cond1, cond4)).To(BeFalse(), "different length should not be equal")
		})

		It("should correctly compare node taints", func() {
			taint1 := []corev1.Taint{
				{Key: "key1", Effect: corev1.TaintEffectNoSchedule, Value: "value1"},
				{Key: "key2", Effect: corev1.TaintEffectNoExecute, Value: "value2"},
			}
			taint2 := []corev1.Taint{
				{Key: "key1", Effect: corev1.TaintEffectNoSchedule, Value: "value1"},
				{Key: "key2", Effect: corev1.TaintEffectNoExecute, Value: "value2"},
			}
			taint3 := []corev1.Taint{
				{Key: "key1", Effect: corev1.TaintEffectNoSchedule, Value: "different"},
				{Key: "key2", Effect: corev1.TaintEffectNoExecute, Value: "value2"},
			}
			taint4 := []corev1.Taint{
				{Key: "key1", Effect: corev1.TaintEffectNoSchedule, Value: "value1"},
			}

			Expect(taintsEqual(taint1, taint2)).To(BeTrue(), "identical taints should be equal")
			Expect(taintsEqual(taint1, taint3)).To(BeFalse(), "different value should not be equal")
			Expect(taintsEqual(taint1, taint4)).To(BeFalse(), "different length should not be equal")
		})

		It("should correctly compare node labels", func() {
			labels1 := map[string]string{"env": "prod", "app": "web"}
			labels2 := map[string]string{"env": "prod", "app": "web"}
			labels3 := map[string]string{"env": "dev", "app": "web"}
			labels4 := map[string]string{"env": "prod"}

			Expect(labelsEqual(labels1, labels2)).To(BeTrue(), "identical labels should be equal")
			Expect(labelsEqual(labels1, labels3)).To(BeFalse(), "different value should not be equal")
			Expect(labelsEqual(labels1, labels4)).To(BeFalse(), "different length should not be equal")
		})
	})

	Context("when reconciling a node", func() {
		When("in bootstrap-only mode", func() {
			BeforeEach(func() {
				rule.Spec.EnforcementMode = nodereadinessiov1alpha1.EnforcementModeBootstrapOnly
			})

			It("should remove the taint when conditions are met", func() {
				// Initial state: taint exists
				Expect(node.Spec.Taints).ToNot(BeEmpty())

				// Update condition to be satisfied
				node.Status.Conditions[0].Status = corev1.ConditionTrue
				Expect(k8sClient.Status().Update(ctx, node)).To(Succeed())

				// Reconcile
				_, err := nodeReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
				Expect(err).NotTo(HaveOccurred())

				// Verify taint is removed
				Eventually(func() bool {
					updatedNode := &corev1.Node{}
					_ = k8sClient.Get(ctx, namespacedName, updatedNode)
					for _, taint := range updatedNode.Spec.Taints {
						if taint.Key == taintKey {
							return true
						}
					}
					return false
				}, time.Second*5).Should(BeFalse())

				// Verify bootstrap completion annotation is added
				Eventually(func() map[string]string {
					updatedNode := &corev1.Node{}
					_ = k8sClient.Get(ctx, namespacedName, updatedNode)
					return updatedNode.Annotations
				}).Should(HaveKey("readiness.k8s.io/bootstrap-completed-" + ruleName))
			})

			It("should not re-add the taint if conditions regress after completion", func() {
				// Step 1: Meet conditions and remove taint
				node.Status.Conditions[0].Status = corev1.ConditionTrue
				Expect(k8sClient.Status().Update(ctx, node)).To(Succeed())
				_, err := nodeReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
				Expect(err).NotTo(HaveOccurred())
				Eventually(func() bool {
					updatedNode := &corev1.Node{}
					_ = k8sClient.Get(ctx, namespacedName, updatedNode)
					for _, taint := range updatedNode.Spec.Taints {
						if taint.Key == taintKey {
							return true
						}
					}
					return false
				}, time.Second*5).Should(BeFalse())

				// Step 2: Regress conditions
				updatedNode := &corev1.Node{}
				Expect(k8sClient.Get(ctx, namespacedName, updatedNode)).To(Succeed())
				updatedNode.Status.Conditions[0].Status = corev1.ConditionFalse
				Expect(k8sClient.Status().Update(ctx, updatedNode)).To(Succeed())

				// Reconcile again
				_, err = nodeReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
				Expect(err).NotTo(HaveOccurred())

				// Verify taint is NOT re-added
				Consistently(func() bool {
					recheckedNode := &corev1.Node{}
					_ = k8sClient.Get(ctx, namespacedName, recheckedNode)
					for _, taint := range recheckedNode.Spec.Taints {
						if taint.Key == taintKey {
							return true
						}
					}
					return false
				}, time.Second*2).Should(BeFalse())
			})
		})

		When("in continuous mode", func() {
			BeforeEach(func() {
				rule.Spec.EnforcementMode = nodereadinessiov1alpha1.EnforcementModeContinuous
			})

			It("should remove the taint when conditions are met", func() {
				node.Status.Conditions[0].Status = corev1.ConditionTrue
				Expect(k8sClient.Status().Update(ctx, node)).To(Succeed())

				_, err := nodeReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() bool {
					updatedNode := &corev1.Node{}
					_ = k8sClient.Get(ctx, namespacedName, updatedNode)
					for _, taint := range updatedNode.Spec.Taints {
						if taint.Key == taintKey {
							return true
						}
					}
					return false
				}, time.Second*5).Should(BeFalse())
			})

			It("should re-add the taint if conditions regress", func() {
				// Step 1: Meet conditions and remove taint
				node.Status.Conditions[0].Status = corev1.ConditionTrue
				Expect(k8sClient.Status().Update(ctx, node)).To(Succeed())
				_, err := nodeReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
				Expect(err).NotTo(HaveOccurred())
				Eventually(func() bool {
					updatedNode := &corev1.Node{}
					_ = k8sClient.Get(ctx, namespacedName, updatedNode)
					for _, taint := range updatedNode.Spec.Taints {
						if taint.Key == taintKey {
							return true
						}
					}
					return false
				}, time.Second*5).Should(BeFalse())

				// Step 2: Regress conditions
				updatedNode := &corev1.Node{}
				Expect(k8sClient.Get(ctx, namespacedName, updatedNode)).To(Succeed())
				updatedNode.Status.Conditions[0].Status = corev1.ConditionFalse
				Expect(k8sClient.Status().Update(ctx, updatedNode)).To(Succeed())

				// Reconcile again
				_, err = nodeReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
				Expect(err).NotTo(HaveOccurred())

				// Verify taint IS re-added
				Eventually(func() bool {
					recheckedNode := &corev1.Node{}
					_ = k8sClient.Get(ctx, namespacedName, recheckedNode)
					for _, taint := range recheckedNode.Spec.Taints {
						if taint.Key == taintKey {
							return true
						}
					}
					return false
				}, time.Second*5).Should(BeTrue())
			})
		})

		When("a rule's node selector does not match", func() {
			BeforeEach(func() {
				rule.Spec.NodeSelector.MatchLabels = map[string]string{"env": "non-existent"}
			})

			It("should not remove the taint", func() {
				node.Status.Conditions[0].Status = corev1.ConditionTrue
				Expect(k8sClient.Status().Update(ctx, node)).To(Succeed())

				_, err := nodeReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
				Expect(err).NotTo(HaveOccurred())

				Consistently(func() []corev1.Taint {
					updatedNode := &corev1.Node{}
					_ = k8sClient.Get(ctx, namespacedName, updatedNode)
					return updatedNode.Spec.Taints
				}, time.Second*2).ShouldNot(BeEmpty())
			})
		})
	})
})
