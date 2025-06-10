package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/ajaysundark/readiness-gates/pkg/controller"
)

func main() {
	var kubeconfig string
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")

	klog.InitFlags(nil)
	flag.Parse()

	// Create Kubernetes client
	var config *rest.Config
	var err error

	if kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		klog.Fatalf("Failed to create config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to create clientset: %v", err)
	}

	// Create controller
	ctrl := controller.NewReadinessGateController(clientset)

	// Hardcoded readiness requirements for nodes
	readinessRequirements := []controller.NodeReadinessRule{
		// CNI readiness
		{
			ConditionType:  "network.kubernetes.io/CNIReady",
			TaintKey:       "readiness.k8s.io/cni-not-ready",
			TaintEffect:    corev1.TaintEffectNoSchedule,
			RequiredStatus: corev1.ConditionTrue,
		},
	}

	for _, req := range readinessRequirements {
		ctrl.AddReadinessRule(&req)
	}

	// Healthcheck endpoint
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	go http.ListenAndServe(":8081", nil)

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-c
		klog.Info("Received shutdown signal")
		cancel()
	}()

	// Run controller
	if err := ctrl.Run(ctx); err != nil {
		klog.Fatalf("Controller failed: %v", err)
	}
}
