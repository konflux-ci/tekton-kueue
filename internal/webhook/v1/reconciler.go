package v1

import (
	"context"
	"time"

	"github.com/konflux-ci/tekton-kueue/pkg/common"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ConfigMapReconciler struct {
	Client client.Client
	Store  *ConfigStore
}

func (r *ConfigMapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	var cm corev1.ConfigMap
	logger.Info("Reconciling ConfigMap")
	if err := r.Client.Get(ctx, req.NamespacedName, &cm); err != nil {
		logger.Error(err, "unable to fetch ConfigMap", "ConfigMap", req.NamespacedName)
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	raw, ok := cm.Data[common.ConfigKey]
	if !ok {
		logger.Info("Key is not present in configmap", "ConfigKey", common.ConfigKey, "ConfigMap", req.NamespacedName)
		return ctrl.Result{}, nil
	}
	if err := r.Store.Update([]byte(raw)); err != nil {
		logger.Error(err, "unable to update config")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}
	return ctrl.Result{}, nil
}
