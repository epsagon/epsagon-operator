package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

func (r *EpsagonReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("epsagon", req.NamespacedName)

	// your logic here

	return ctrl.Result{}, nil
}

func (r *EpsagonReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&integrationv1alpha1.Epsagon{}).
		Complete(r)
}
