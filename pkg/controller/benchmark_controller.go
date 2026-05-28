package controller

import (
	"context"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	benchmarkRunGVK = schema.GroupVersionKind{
		Group:   "trading.benchmarking.platform",
		Version: "v1alpha1",
		Kind:    "BenchmarkRun",
	}
)

// BenchmarkRunReconciler reconciles a BenchmarkRun object
type BenchmarkRunReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// NewReconciler creates a new BenchmarkRunReconciler
func NewReconciler(mgr ctrl.Manager) *BenchmarkRunReconciler {
	return &BenchmarkRunReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
}

// Reconcile reads that state of the cluster for a BenchmarkRun object and makes changes
func (r *BenchmarkRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log.Printf("[controller] Reconciling BenchmarkRun %s/%s", req.Namespace, req.Name)

	// Fetch the BenchmarkRun instance using unstructured client
	run := &unstructured.Unstructured{}
	run.SetGroupVersionKind(benchmarkRunGVK)
	err := r.Get(ctx, req.NamespacedName, run)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Printf("[controller] BenchmarkRun %s not found. Ignoring.", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		log.Printf("[controller] Error fetching BenchmarkRun: %v", err)
		return ctrl.Result{}, err
	}

	// Extract spec values
	spec, found, err := unstructured.NestedMap(run.Object, "spec")
	if err != nil || !found {
		log.Printf("[controller] Failed to get spec from BenchmarkRun: %v", err)
		return ctrl.Result{}, fmt.Errorf("invalid spec: %w", err)
	}

	contestantID, _ := spec["contestantId"].(string)
	submissionID, _ := spec["submissionId"].(string)
	image, _ := spec["image"].(string)
	durationStr, _ := spec["duration"].(string)
	if durationStr == "" {
		durationStr = "60s"
	}
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		duration = 60 * time.Second
	}

	// Check status
	status, found, _ := unstructured.NestedMap(run.Object, "status")
	var state string
	var startedAtStr string
	if found {
		state, _ = status["state"].(string)
		startedAtStr, _ = status["startedAt"].(string)
	}

	runNamespace := fmt.Sprintf("run-%s", submissionID)

	switch state {
	case "":
		// 1. Initial State: Transition to RUNNING and provision resources
		log.Printf("[controller] Initializing run %s in namespace %s", submissionID, runNamespace)
		
		// Create dedicated Namespace
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: runNamespace,
				Labels: map[string]string{
					"run-id": submissionID,
					"role":   "contestant-namespace",
				},
			},
		}
		if err := r.Create(ctx, ns); err != nil && !errors.IsAlreadyExists(err) {
			log.Printf("[controller] Failed to create namespace %s: %v", runNamespace, err)
			return ctrl.Result{}, err
		}

		// Apply ResourceQuota
		quota := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "run-limits",
				Namespace: runNamespace,
			},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					corev1.ResourceLimitsCPU:    resource.MustParse("2.0"),
					corev1.ResourceLimitsMemory: resource.MustParse("1Gi"),
				},
			},
		}
		if err := r.Create(ctx, quota); err != nil && !errors.IsAlreadyExists(err) {
			log.Printf("[controller] Failed to create ResourceQuota: %v", err)
			return ctrl.Result{}, err
		}

		// Apply NetworkPolicy to block all egress
		netPol := &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "block-egress",
				Namespace: runNamespace,
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"role": "contestant",
					},
				},
				PolicyTypes: []networkingv1.PolicyType{
					networkingv1.PolicyTypeEgress,
					networkingv1.PolicyTypeIngress,
				},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					{
						Ports: []networkingv1.NetworkPolicyPort{
							{Port: &intstr.IntOrString{Type: intstr.Int, IntVal: 8080}},
							{Port: &intstr.IntOrString{Type: intstr.Int, IntVal: 2112}},
						},
					},
				},
				Egress: []networkingv1.NetworkPolicyEgressRule{}, // Empty means deny all
			},
		}
		if err := r.Create(ctx, netPol); err != nil && !errors.IsAlreadyExists(err) {
			log.Printf("[controller] Failed to create NetworkPolicy: %v", err)
			return ctrl.Result{}, err
		}

		// Deploy contestant engine Pod
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "contestant-engine",
				Namespace: runNamespace,
				Labels: map[string]string{
					"role":          "contestant",
					"contestant-id": contestantID,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "engine",
						Image: image,
						Ports: []corev1.ContainerPort{
							{ContainerPort: 8080},
							{ContainerPort: 2112},
						},
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1.0"),
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
						},
					},
				},
			},
		}
		if err := r.Create(ctx, pod); err != nil && !errors.IsAlreadyExists(err) {
			log.Printf("[controller] Failed to create contestant pod: %v", err)
			return ctrl.Result{}, err
		}

		// Update Status to Running
		newStatus := map[string]interface{}{
			"state":     "Running",
			"startedAt": time.Now().Format(time.RFC3339),
		}
		unstructured.SetNestedMap(run.Object, newStatus, "status")
		if err := r.Status().Update(ctx, run); err != nil {
			log.Printf("[controller] Failed to update BenchmarkRun status: %v", err)
			return ctrl.Result{}, err
		}

		log.Printf("[controller] Successfully initialized run %s, requeuing for progress monitoring", submissionID)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil

	case "Running":
		// 2. Monitoring State: Check if run duration has elapsed
		startedAt, err := time.Parse(time.RFC3339, startedAtStr)
		if err != nil {
			log.Printf("[controller] Invalid start time, reset to now: %v", err)
			startedAt = time.Now()
		}

		elapsed := time.Since(startedAt)
		if elapsed >= duration {
			// Time is up! Clean up resources
			log.Printf("[controller] Benchmark duration (%v) elapsed for run %s. Cleaning up.", duration, submissionID)
			
			// Delete namespace containing all resources (this automatically deletes pod, quota, netpol)
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: runNamespace,
				},
			}
			if err := r.Delete(ctx, ns); err != nil && !errors.IsNotFound(err) {
				log.Printf("[controller] Failed to delete namespace %s: %v", runNamespace, err)
				return ctrl.Result{}, err
			}

			// Update status to Succeeded
			newStatus := map[string]interface{}{
				"state":       "Succeeded",
				"completedAt": time.Now().Format(time.RFC3339),
				"totalOrders": int64(150000), // Mocked for demo
				"successRate": float64(99.98), // Mocked for demo
			}
			unstructured.SetNestedMap(run.Object, newStatus, "status")
			if err := r.Status().Update(ctx, run); err != nil {
				log.Printf("[controller] Failed to set status to Succeeded: %v", err)
				return ctrl.Result{}, err
			}

			log.Printf("[controller] Run %s successfully completed and cleaned up", submissionID)
			return ctrl.Result{}, nil
		}

		// Requeue until duration completes
		remaining := duration - elapsed
		log.Printf("[controller] Run %s active, remaining: %v", submissionID, remaining)
		return ctrl.Result{RequeueAfter: remaining}, nil

	case "Succeeded", "Failed":
		// Already completed, nothing to do
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BenchmarkRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	run := &unstructured.Unstructured{}
	run.SetGroupVersionKind(benchmarkRunGVK)
	return ctrl.NewControllerManagedBy(mgr).
		For(run).
		Complete(r)
}
