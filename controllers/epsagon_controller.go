package controllers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"

	v1 "k8s.io/api/core/v1"
	rbacbeta1 "k8s.io/api/rbac/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	integrationv1alpha1 "github.com/epsagon/epsagon-operator/api/v1alpha1"
)

const epsagonMonitoring = "epsagon-monitoring"
const epsagonPrometheus = "epsagon-prometheus"

// EpsagonReconciler reconciles a Epsagon object
type EpsagonReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=integration.epsagon.com,resources=epsagons,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=integration.epsagon.com,resources=epsagons/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrole,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebinding,verbs=get;list;watch;create;update;patch;delete

// Reconcile k8s state will make sure epsagon roles are deployed
func (r *EpsagonReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.TODO()
	epsagonResource := &integrationv1alpha1.Epsagon{}
	_ = r.Log.WithValues("epsagon", req.NamespacedName)
	err := r.Get(ctx, req.NamespacedName, epsagonResource)
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

	epsagonResource.Status.Status = "Processing"
	r.Status().Update(ctx, epsagonResource)

	if err := r.deployEpsagonRole(); err != nil {
		epsagonResource.Status.Status = "Failed to deploy roles"
		r.Status().Update(ctx, epsagonResource)
		return ctrl.Result{}, err
	}

	if err := r.sendKeysToEpsagon(epsagonResource); err != nil {
		epsagonResource.Status.Status = "Integration Failed"
		epsagonResource.Status.IntegrationError = err.Error()
		r.Status().Update(ctx, epsagonResource)
		return ctrl.Result{}, err
	}

	epsagonResource.Status.Status = "Integrated Sucessfuly"
	r.Status().Update(ctx, epsagonResource)
	return ctrl.Result{}, nil
}

func (r *EpsagonReconciler) getSAToken() (string, error) {
	ctx := context.TODO()
	sa := &v1.ServiceAccount{}
	saKey := client.ObjectKey{Name: epsagonMonitoring, Namespace: epsagonMonitoring}
	if err := r.Get(ctx, saKey, sa); err != nil {
		return "", err
	}
	secretKey := client.ObjectKey{Name: sa.Secrets[0].Name, Namespace: epsagonMonitoring}
	secret := &v1.Secret{}
	if err := r.Get(ctx, secretKey, secret); err != nil {
		return "", err
	}
	decoded, err := base64.StdEncoding.DecodeString(string(secret.Data["token"]))
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func (r *EpsagonReconciler) sendKeysToEpsagon(epsagonResource *integrationv1alpha1.Epsagon) error {
	roleToken, err := r.getSAToken()
	if err != nil {
		return err
	}
	buf, err := json.Marshal(map[string]string{
		"k8s_cluster_url": epsagonResource.Spec.ClusterEndpoint,
		"epagon_token":    epsagonResource.Spec.EpsagonToken,
		"cluster_token":   roleToken,
		"operator-used":   "True",
	})
	_, err = http.Post("https://api.epsagon.com/containers/k8s/add_cluster_by_token", "application/json", bytes.NewReader(buf))
	return err
}

func (r *EpsagonReconciler) deployEpsagonRole() error {
	ctx := context.TODO()
	sa := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      epsagonMonitoring,
			Namespace: epsagonMonitoring,
		},
	}
	clusterRole := &rbacbeta1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: epsagonPrometheus,
		},
		Rules: []rbacbeta1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{
					"nodes", "nodes/proxy", "services", "services/proxy",
					"endpoints", "pods", "pods/proxy", "pods/log", "namespaces", "configmaps"},
				Verbs: []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"extensions"},
				Resources: []string{"ingresses"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"extensions", "apps"},
				Resources: []string{"deployments"},
				Verbs:     []string{"get", "list", "watch", "update"},
			},
			{
				NonResourceURLs: []string{"/metrics"},
				Verbs:           []string{"get"},
			},
		},
	}

	clusterRoleBinding := &rbacbeta1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: epsagonPrometheus,
		},
		RoleRef: rbacbeta1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     epsagonPrometheus,
		},
		Subjects: []rbacbeta1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      epsagonMonitoring,
				Namespace: epsagonMonitoring,
			},
		},
	}

	testSa := &v1.ServiceAccount{}
	if err := r.Get(ctx, client.ObjectKey{Name: sa.ObjectMeta.Name, Namespace: sa.ObjectMeta.Namespace}, testSa); err != nil {
		if err := r.Create(ctx, sa); err != nil {
			return err
		}
	}
	if err := r.Create(ctx, clusterRole); err != nil {
		return err
	}
	if err := r.Create(ctx, clusterRoleBinding); err != nil {
		return err
	}

	return nil
}

func onlyEpsagonMonitoring() predicate.Predicate {
	return predicate.Funcs{
		GenericFunc: func(e event.GenericEvent) bool {
			if e.Meta.GetNamespace() == epsagonMonitoring {
				return true
			}
			return false
		},
	}
}

// SetupWithManager will watch Epsagon resources
func (r *EpsagonReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&integrationv1alpha1.Epsagon{}).
		Owns(&v1.ServiceAccount{}).
		// Owns(&rbacbeta1.ClusterRole{}).
		// Owns(&rbacbeta1.ClusterRoleBinding{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		WithEventFilter(onlyEpsagonMonitoring()).
		Complete(r)
}
