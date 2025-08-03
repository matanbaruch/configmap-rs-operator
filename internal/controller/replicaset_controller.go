package controller

import (
	"context"
	"regexp"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/matanbaruch/configmap-rs-operator/internal/config"
)

// ReplicaSetReconciler reconciles a ReplicaSet object
type ReplicaSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *config.OperatorConfig
	StartTime time.Time
}

//+kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *ReplicaSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("replicaset", req.NamespacedName)

	// Check if namespace matches our selection criteria
	if !r.shouldProcessNamespace(req.Namespace) {
		logger.V(1).Info("Skipping ReplicaSet in unmatched namespace", "namespace", req.Namespace)
		return ctrl.Result{}, nil
	}

	// Fetch the ReplicaSet instance
	var rs appsv1.ReplicaSet
	if err := r.Get(ctx, req.NamespacedName, &rs); err != nil {
		if errors.IsNotFound(err) {
			logger.V(1).Info("ReplicaSet not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get ReplicaSet")
		return ctrl.Result{}, err
	}

	// Only process ReplicaSets created after the operator started
	// This prevents processing existing ReplicaSets when the operator starts
	creationTime := rs.CreationTimestamp.Time
	if creationTime.Before(r.StartTime) {
		logger.Info("Skipping ReplicaSet created before operator start", "name", rs.Name, "created", creationTime.Format(time.RFC3339), "operatorStart", r.StartTime.Format(time.RFC3339))
		return ctrl.Result{}, nil
	}

	if r.Config.Debug {
		logger.Info("Processing recently created ReplicaSet", "name", rs.Name, "namespace", rs.Namespace, "age", time.Since(creationTime), "created", creationTime.Format(time.RFC3339), "operatorStart", r.StartTime.Format(time.RFC3339))
	}

	// Extract ConfigMaps referenced as volumes
	configMapNames := r.extractConfigMapVolumes(&rs)
	if len(configMapNames) == 0 {
		logger.V(1).Info("No ConfigMaps found in ReplicaSet volumes")
		return ctrl.Result{}, nil
	}

	if r.Config.Debug {
		logger.Info("Found ConfigMaps in volumes", "configmaps", configMapNames)
	}

	// Process each ConfigMap
	for _, cmName := range configMapNames {
		if err := r.processConfigMap(ctx, rs.Namespace, cmName, &rs, logger); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *ReplicaSetReconciler) shouldProcessNamespace(namespace string) bool {
	if len(r.Config.NamespaceRegex) == 0 {
		return true
	}

	for _, pattern := range r.Config.NamespaceRegex {
		matched, err := regexp.MatchString(pattern, namespace)
		if err != nil {
			// Log error but don't fail reconciliation
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

func (r *ReplicaSetReconciler) extractConfigMapVolumes(rs *appsv1.ReplicaSet) []string {
	var configMapNames []string
	configMapSet := make(map[string]bool)

	// Check all containers in all pods
	for _, container := range rs.Spec.Template.Spec.Containers {
		for _, volumeMount := range container.VolumeMounts {
			// Find corresponding volume
			for _, volume := range rs.Spec.Template.Spec.Volumes {
				if volume.Name == volumeMount.Name && volume.ConfigMap != nil {
					if !configMapSet[volume.ConfigMap.Name] {
						configMapSet[volume.ConfigMap.Name] = true
						configMapNames = append(configMapNames, volume.ConfigMap.Name)
					}
				}
			}
		}
	}

	// Check init containers as well
	for _, container := range rs.Spec.Template.Spec.InitContainers {
		for _, volumeMount := range container.VolumeMounts {
			for _, volume := range rs.Spec.Template.Spec.Volumes {
				if volume.Name == volumeMount.Name && volume.ConfigMap != nil {
					if !configMapSet[volume.ConfigMap.Name] {
						configMapSet[volume.ConfigMap.Name] = true
						configMapNames = append(configMapNames, volume.ConfigMap.Name)
					}
				}
			}
		}
	}

	return configMapNames
}

func (r *ReplicaSetReconciler) processConfigMap(ctx context.Context, namespace, name string, rs *appsv1.ReplicaSet, logger logr.Logger) error {
	// Get the ConfigMap
	var cm corev1.ConfigMap
	cmKey := types.NamespacedName{Name: name, Namespace: namespace}
	if err := r.Get(ctx, cmKey, &cm); err != nil {
		if errors.IsNotFound(err) {
			logger.V(1).Info("ConfigMap not found", "configmap", name)
			return nil
		}
		logger.Error(err, "Failed to get ConfigMap", "configmap", name)
		return err
	}

	// Check if ReplicaSet is already an owner
	if r.isOwnerReferencePresent(&cm, rs) {
		if r.Config.Debug {
			logger.Info("OwnerReference already exists", "configmap", name, "replicaset", rs.Name)
		}
		return nil
	}

	if r.Config.DryRun {
		logger.Info("DRY-RUN: Would add OwnerReference", "configmap", name, "replicaset", rs.Name)
		return nil
	}

	// Add owner reference
	if err := controllerutil.SetOwnerReference(rs, &cm, r.Scheme); err != nil {
		logger.Error(err, "Failed to set owner reference", "configmap", name, "replicaset", rs.Name)
		return err
	}

	// Update the ConfigMap
	if err := r.Update(ctx, &cm); err != nil {
		logger.Error(err, "Failed to update ConfigMap with owner reference", "configmap", name)
		return err
	}

	logger.Info("Added OwnerReference to ConfigMap", "configmap", name, "replicaset", rs.Name)
	return nil
}

func (r *ReplicaSetReconciler) isOwnerReferencePresent(cm *corev1.ConfigMap, rs *appsv1.ReplicaSet) bool {
	for _, ownerRef := range cm.OwnerReferences {
		if ownerRef.Kind == "ReplicaSet" && ownerRef.Name == rs.Name && ownerRef.UID == rs.UID {
			return true
		}
	}
	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReplicaSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create a predicate that only processes CREATE events
	replicaSetPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			// Only process CREATE events for ReplicaSets created after operator start
			rs, ok := e.Object.(*appsv1.ReplicaSet)
			if !ok {
				return false
			}
			return rs.CreationTimestamp.Time.After(r.StartTime)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Don't process UPDATE events - we only care about new ReplicaSets
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// Don't process DELETE events - Kubernetes GC handles cleanup automatically
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			// Don't process generic events
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.ReplicaSet{}).
		WithEventFilter(replicaSetPredicate).
		Complete(r)
}
