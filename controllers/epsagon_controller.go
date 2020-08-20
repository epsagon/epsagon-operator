package controllers

import (
	"context"

	v1 "k8s.io/api/core/v1"
	rbacbeta1 "k8s.io/api/rbac/v1beta1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	integrationv1alpha1 "github.com/epsagon/epsagon-operator/api/v1alpha1"
)

// EpsagonReconciler reconciles a Epsagon object
type EpsagonReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=integration.epsagon.com,resources=epsagons,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=integration.epsagon.com,resources=epsagons/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=serviceAccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterRole,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterRoleBinding,verbs=get;list;watch;create;update;patch;delete

// Reconcile k8s state will make sure epsagon roles are deployed
func (r *EpsagonReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	epsagon := &integrationv1alpha1.Epsagon{}
	_ = r.Log.WithValues("epsagon", req.NamespacedName)
	err := r.Get(ctx, req.NamespacedName, epsagon)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			r.Log.Info("Epsagon resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}

		r.Log.Error(err, "Failed to get Epsagon resource")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager will watch Epsagon resources
func (r *EpsagonReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&integrationv1alpha1.Epsagon{}).
		Owns(&v1.ServiceAccount{}).
		Owns(&rbacbeta1.ClusterRole{}).
		Owns(&rbacbeta1.ClusterRoleBinding{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}
