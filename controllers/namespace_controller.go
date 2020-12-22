/*
Copyright 2020.

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

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	gialv1beta1 "github.com/loblaw-sre/namespace-controller/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IstioTag is the revision tag that istio relies on for directing which istiod to register with
const IstioTag = "istio.io/rev"

// NamespaceReconciler reconciles a Namespace object
type NamespaceReconciler struct {
	client.Client
	Log      logr.Logger
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=gial.lblw.dev,resources=lnamespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gial.lblw.dev,resources=lnamespaces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gial.lblw.dev,resources=lnamespaces/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces;events,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *NamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("namespace", req.Name)

	ns := &gialv1beta1.LNamespace{}
	err := r.Get(ctx, client.ObjectKey{
		Name:      req.Name,
		Namespace: req.Namespace,
	}, ns)
	if apierrors.IsNotFound(err) {
		log.Info("namespace not found. Continuing as if deleted.")
		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, "unable to get namespace definition")
		return ctrl.Result{}, err
	}
	for _, v := range ns.Finalizers {
		if v == metav1.FinalizerOrphanDependents {
			log.Info("namespace is to be orphaned. Continuing without updating dependents.")
			return ctrl.Result{}, nil
		}
	}

	cns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: req.NamespacedName.Name,
		},
	}
	opRes, err := controllerutil.CreateOrPatch(ctx, r, cns, func() error {
		if cns.Annotations == nil {
			cns.Annotations = make(map[string]string)
		}
		if cns.Labels == nil {
			cns.Labels = make(map[string]string)
		}
		cns.Labels[IstioTag] = ns.Spec.IstioRevision
		for k, v := range ns.Spec.Billing {
			cns.Annotations[k] = v
		}

		for k, v := range ns.Spec.NamespaceLabelOverrides {
			cns.Labels[k] = v
		}
		return controllerutil.SetControllerReference(ns, cns, r.Scheme())
	})
	if err != nil {
		log.Error(err, "unable to create or update namespace")
		return ctrl.Result{}, err
	}
	if opRes == controllerutil.OperationResultCreated {
		r.Recorder.Eventf(ns, "Normal", "Create", "Created namespace %s", req.Name)
	} else if opRes == controllerutil.OperationResultUpdated {
		r.Recorder.Eventf(ns, "Normal", "Update", "Updated namespace %s", req.Name)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the NamespaceReconciler with the provided manager
func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gialv1beta1.LNamespace{}).
		Owns(&corev1.Namespace{}).
		Complete(r)
}
