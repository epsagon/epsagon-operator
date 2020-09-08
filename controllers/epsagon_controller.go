package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"time"

	v1 "k8s.io/api/core/v1"
	rbacbeta1 "k8s.io/api/rbac/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	integrationv1alpha1 "github.com/epsagon/epsagon-operator/api/v1alpha1"
)

const epsagonMonitoring = "epsagon-monitoring"
const epsagonPrometheus = "epsagon-prometheus"
const epsagonAPIEndpoint = "https://api.epsagon.com/"
const epsagonTESTAPIEndpoint = "https://devapi.epsagon.com/"
const addCluster = "containers/k8s/add_cluster_by_token"
const checkConnection = "containers/k8s/check_cluster_connection"
const deleteCluster = "containers/k8s/delete_cluster_by_token"
const integratedSuccessfuly = "Integrated Successfuly"
const epsagonFinalizer = "finalizer.integration.epsagon.com"

// EpsagonReconciler reconciles a Epsagon object
type EpsagonReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=integration.epsagon.com,resources=epsagons,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=integration.epsagon.com,resources=epsagons/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete;escalate;bind
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete

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

	if isBeingDeleted := epsagonResource.GetDeletionTimestamp(); isBeingDeleted != nil {
		if contains(epsagonResource.GetFinalizers(), epsagonFinalizer) {
			controllerutil.RemoveFinalizer(epsagonResource, epsagonFinalizer)
			if err = r.Update(ctx, epsagonResource); err != nil {
				return r.manageError(epsagonResource, err)
			}
			return r.deleteEpsagonIntegration(epsagonResource)
		}
	}
	if !contains(epsagonResource.GetFinalizers(), epsagonFinalizer) {
		if err := r.addFinalizer(epsagonResource); err != nil {
			return r.manageError(epsagonResource, err)
		}
	}

	epsagonResource.Status.Status = "Processing"
	r.Status().Update(ctx, epsagonResource)

	if err := r.deployEpsagonRole(); err != nil {
		epsagonResource.Status.Status = "Failed to deploy roles"
		return r.manageError(epsagonResource, err)
	}

	if err := r.sendKeysToEpsagon(epsagonResource); err != nil {
		epsagonResource.Status.Status = "Integration Failed"
		return r.manageError(epsagonResource, err)
	}

	epsagonResource.Status.Status = integratedSuccessfuly
	r.Status().Update(ctx, epsagonResource)
	return ctrl.Result{}, nil
}

func (r *EpsagonReconciler) deleteEpsagonIntegration(epsagon *integrationv1alpha1.Epsagon) (ctrl.Result, error) {
	r.Log.Info("Removing integration from Epsagon")
	buf, err := r.getIntegrationParams(epsagon)
	if err != nil {
		return r.manageError(epsagon, err)
	}
	err = callEpsagonAPI(buf, r.Log, deleteCluster)
	if err == nil {
		return ctrl.Result{}, nil
	}
	return r.manageError(epsagon, err)
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
	encodedToken := secret.Data["token"]
	return string(encodedToken), nil
}

func callEpsagonAPI(buf []byte, logger logr.Logger, path string) error {
	var err error
	var resp *http.Response
	if os.Getenv("EPSAGON_TEST") == "TRUE" {
		resp, err = http.Post(epsagonTESTAPIEndpoint+path, "application/json", bytes.NewReader(buf))
	} else {
		resp, err = http.Post(epsagonAPIEndpoint+path, "application/json", bytes.NewReader(buf))
	}
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	logger.Info("Epsagon api response:", "StatusCode", resp.StatusCode, "body", string(body))
	if resp.StatusCode != 200 {
		return fmt.Errorf(string(body))
	}
	return err
}

func (r *EpsagonReconciler) getIntegrationParams(epsagonResource *integrationv1alpha1.Epsagon) ([]byte, error) {
	roleToken, err := r.getSAToken()
	if err != nil {
		return []byte{}, err
	}
	buf, err := json.Marshal(map[string]string{
		"k8s_cluster_url":    epsagonResource.Spec.ClusterEndpoint,
		"epsagon_token":      epsagonResource.Spec.EpsagonToken,
		"cluster_token":      roleToken,
		"integration_source": "Operator",
	})
	if err != nil {
		return []byte{}, err
	}
	return buf, nil
}

func (r *EpsagonReconciler) sendKeysToEpsagon(epsagonResource *integrationv1alpha1.Epsagon) error {
	buf, err := r.getIntegrationParams(epsagonResource)
	if err != nil {
		return nil
	}

	if err = callEpsagonAPI(buf, r.Log, checkConnection); err != nil {
		return err
	}
	return callEpsagonAPI(buf, r.Log, addCluster)
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
	testCR := &rbacbeta1.ClusterRole{}
	if err := r.Get(ctx, client.ObjectKey{Name: clusterRole.ObjectMeta.Name}, testCR); err != nil {
		if err := r.Create(ctx, clusterRole); err != nil {
			return err
		}
	}
	testCRB := &rbacbeta1.ClusterRoleBinding{}
	if err := r.Get(ctx, client.ObjectKey{Name: clusterRoleBinding.ObjectMeta.Name}, testCRB); err != nil {
		if err := r.Create(ctx, clusterRoleBinding); err != nil {
			return err
		}
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
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.MetaNew.GetGeneration() == e.MetaOld.GetGeneration() {
				return false
			}
			return true
		},
	}
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
		WithEventFilter(onlyEpsagonMonitoring()).
		Complete(r)
}

func (r *EpsagonReconciler) manageError(epsagon *integrationv1alpha1.Epsagon, issue error) (ctrl.Result, error) {
	var retryInterval time.Duration
	lastUpdate := epsagon.Status.LastUpdate
	epsagon.Status = integrationv1alpha1.EpsagonStatus{
		LastUpdate: metav1.Now(),
		Reason:     issue.Error(),
		Status:     epsagon.Status.Status,
	}
	err := r.Status().Update(context.Background(), epsagon)

	if err != nil {
		r.Log.Error(err, "Unable to update status")
		return ctrl.Result{
			RequeueAfter: time.Second,
			Requeue:      true,
		}, nil
	}
	if lastUpdate.IsZero() {
		retryInterval = time.Second
	} else {
		retryInterval = epsagon.Status.LastUpdate.Sub(lastUpdate.Time).Round(time.Second)
	}
	return ctrl.Result{
		RequeueAfter: time.Duration(math.Min(float64(retryInterval.Nanoseconds()*2), float64(time.Hour.Nanoseconds()*6))),
		Requeue:      true,
	}, nil
}

func (r *EpsagonReconciler) addFinalizer(epsagon *integrationv1alpha1.Epsagon) error {
	r.Log.Info("Adding Finalizer for the Epsagon")
	controllerutil.AddFinalizer(epsagon, epsagonFinalizer)

	// Update CR
	err := r.Update(context.TODO(), epsagon)
	if err != nil {
		r.Log.Error(err, "Failed to update Epsagon with finalizer")
		return err
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
