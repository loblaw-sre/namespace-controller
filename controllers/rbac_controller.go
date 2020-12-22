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
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gialv1beta1 "github.com/loblaw-sre/namespace-controller/api/v1beta1"
	"github.com/loblaw-sre/namespace-controller/pkg/utils"
	rbacv1 "k8s.io/api/rbac/v1"
)

// RBACReconciler reconciles a Namespace with its RBAC resources
type RBACReconciler struct {
	client.Client
	Log      logr.Logger
	Recorder record.EventRecorder
}

const (
	// LabelKey for RBAC Type (self-impersonator, sudoer-group, etc.)
	LabelKey = "gial.lblw.dev/rbac-type"
	// LabelSelfImpersonator value for all RBAC related to self impersonation
	LabelSelfImpersonator = "self-impersonator"
	// LabelSudoerImpersonator value for all RBAC related to sudoer impersonation
	LabelSudoerImpersonator = "sudoer-impersonator"
	// LabelSudoerPermissions value for all RBAC related to sudoer group permissions
	LabelSudoerPermissions = "sudoer-permissions"
	// LabelManagerPermissions value for all RBAC related to manager permissions
	LabelManagerPermissions = "manager-permissions"
	// LabelDeveloperPermissions value for all RBAC related to developer permissions
	LabelDeveloperPermissions = "developer-permissions"
)

// +kubebuilder:rbac:groups=gial.lblw.dev,resources=lnamespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gial.lblw.dev,resources=lnamespaces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gial.lblw.dev,resources=lnamespaces/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=users;groups,verbs=impersonate
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings;roles;rolebindings,verbs=get;list;watch;create;update;patch;delete;bind

// UpdateSelfImpersonators ClusterRoles and Bindings
func (r *RBACReconciler) UpdateSelfImpersonators(ctx context.Context, ns *gialv1beta1.LNamespace) error {
	log := r.Log.WithValues("namespace", ns.Name)
	for _, v := range ns.Spec.Sudoers {
		clusterRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: utils.Slug(v.Name) + "-impersonator",
			},
		}
		log.Info("updating clusterRole", "name", clusterRole.Name)
		_, err := controllerutil.CreateOrPatch(ctx, r, clusterRole, func() error {
			clusterRole.Rules = []rbacv1.PolicyRule{
				{
					APIGroups:     []string{""},
					Verbs:         []string{"impersonate"},
					ResourceNames: []string{v.Name},
					Resources:     []string{"users"},
				},
			}
			if clusterRole.Labels == nil {
				clusterRole.Labels = make(map[string]string)
			}
			clusterRole.Labels[LabelKey] = LabelSelfImpersonator

			return controllerutil.SetOwnerReference(ns, clusterRole, r.Scheme())
		})
		if err != nil {
			log.Error(err, "unable to create or update impersonator cluster role", "User", v.Name)
			return err
		}
		clusterRoleBinding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: utils.Slug(v.Name) + "-impersonator",
			},
		}
		log.Info("updating clusterRoleBinding", "name", clusterRole.Name)
		_, err = controllerutil.CreateOrPatch(ctx, r, clusterRoleBinding, func() error {
			clusterRoleBinding.RoleRef = rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Name:     clusterRole.Name,
				Kind:     "ClusterRole",
			}
			clusterRoleBinding.Subjects = []rbacv1.Subject{v}
			if clusterRoleBinding.Labels == nil {
				clusterRoleBinding.Labels = make(map[string]string)
			}
			clusterRoleBinding.Labels[LabelKey] = LabelSelfImpersonator
			return controllerutil.SetOwnerReference(ns, clusterRoleBinding, r.Scheme())
		})
		if err != nil {
			log.Error(err, "unable to create or update impersonator cluster role binding", "User", v.Name)
			return err
		}
	}
	// cleanup hanging sudoers
	l := &rbacv1.ClusterRoleList{}
	selector, err := labels.Parse(LabelKey + "=" + LabelSelfImpersonator)
	if err != nil {
		log.Error(err, "unable to generate List Options for self impersonators")
		return err
	}
	listOptions := &client.ListOptions{
		LabelSelector: selector,
	}
	err = r.List(ctx, l, listOptions)
	if err != nil {
		log.Error(err, "unable to list cluster roles to clean up sudoers")
		return err
	}

	sudoerSet := make(map[string]bool)
	for _, v := range ns.Spec.Sudoers {
		sudoerSet[v.Name] = true
	}
	for _, v := range l.Items {
		dv := &v
		_, err := controllerutil.CreateOrUpdate(ctx, r, dv, func() error {
			var ownerRef []metav1.OwnerReference
			for _, ref := range dv.OwnerReferences {
				if ref.UID == ns.UID {
					if ok := sudoerSet[dv.Rules[0].ResourceNames[0]]; !ok {
						log.Info("marking for disinheriting", "resource", v.Name)
						ref.UID = "mark-for-deletion"
					}
				}
				ownerRef = append(ownerRef, ref)
			}
			dv.OwnerReferences = ownerRef
			return nil
		})
		if err != nil {
			log.Error(err, fmt.Sprintf("error updating %s", v.Name))
			return err
		}
	}

	bl := &rbacv1.ClusterRoleBindingList{}
	err = r.List(ctx, bl, listOptions)
	if err != nil {
		log.Error(err, "unable to list cluster role bindings to clean up sudoers")
		return err
	}
	for _, v := range bl.Items {
		dv := &v
		_, err := controllerutil.CreateOrUpdate(ctx, r, dv, func() error {
			var ownerRef []metav1.OwnerReference
			for _, ref := range dv.OwnerReferences {
				if ref.UID == ns.UID {
					if ok := sudoerSet[dv.Subjects[0].Name]; !ok {
						log.Info("marking for disinheriting", "resource", v.Name)
						ref.UID = "mark-for-deletion"
					}
				}
				ownerRef = append(ownerRef, ref)
			}
			dv.OwnerReferences = ownerRef
			return nil
		})
		if err != nil {
			log.Error(err, fmt.Sprintf("error updating %s", v.Name))
			return err
		}
	}
	return nil
}

// UpdateSudoerGroupImpersonators ClusterRole and Binding
func (r *RBACReconciler) UpdateSudoerGroupImpersonators(ctx context.Context, ns *gialv1beta1.LNamespace) error {
	log := r.Log.WithValues("namespace", ns.Name)
	{
		sgr := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns.GetSudoersGroupName(),
			},
		}
		_, err := controllerutil.CreateOrUpdate(ctx, r, sgr, func() error {
			sgr.Rules = []rbacv1.PolicyRule{
				{
					APIGroups:     []string{""},
					Verbs:         []string{"impersonate"},
					ResourceNames: []string{ns.GetSudoersGroupName()},
					Resources:     []string{"groups"},
				},
			}
			if sgr.Labels == nil {
				sgr.Labels = make(map[string]string)
			}
			sgr.Labels[LabelKey] = LabelSudoerImpersonator
			return controllerutil.SetOwnerReference(ns, sgr, r.Scheme())
		})
		if err != nil {
			log.Error(err, fmt.Sprintf("error updating sudoer group role"))
			return err
		}
	}
	{
		sgrb := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns.GetSudoersGroupName(),
			},
		}
		_, err := controllerutil.CreateOrUpdate(ctx, r, sgrb, func() error {
			sgrb.RoleRef = rbacv1.RoleRef{
				Name:     ns.GetSudoersGroupName(),
				Kind:     "ClusterRole",
				APIGroup: "rbac.authorization.k8s.io",
			}
			sgrb.Subjects = ns.Spec.Sudoers
			if sgrb.Labels == nil {
				sgrb.Labels = make(map[string]string)
			}
			sgrb.Labels[LabelKey] = LabelSudoerImpersonator
			return controllerutil.SetOwnerReference(ns, sgrb, r.Scheme())
		})
		if err != nil {
			log.Error(err, fmt.Sprintf("error updating sudoer group binding"))
			return err
		}
	}
	return nil
}

// UpdateSudoerPermissions ensures correct permissions for the sudoer group of this namespace
func (r *RBACReconciler) UpdateSudoerPermissions(ctx context.Context, ns *gialv1beta1.LNamespace) error {
	log := r.Log.WithValues()

	{ // crb is the ClusterRoleBinding that enables sudoers to edit their own namespaces
		crb := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns.Name + "-sudoeditor",
			},
		}
		_, err := controllerutil.CreateOrUpdate(ctx, r, crb, func() error {
			if crb.Labels == nil {
				crb.Labels = make(map[string]string)
			}
			crb.Labels[LabelKey] = LabelSudoerPermissions
			crb.RoleRef = rbacv1.RoleRef{
				Name: ns.Name + "-editor",
				Kind: "ClusterRole",
			}
			crb.Subjects = []rbacv1.Subject{
				{
					Name: ns.GetSudoersGroupName(),
					Kind: "Group",
				},
			}
			return controllerutil.SetControllerReference(ns, crb, r.Scheme())
		})
		if err != nil {
			log.Error(err, "unable to create sudoer cluster role binding for editing its own namespace")
			return err
		}
	}

	// carb is the cluster-admin role binding inside of the namespace
	{
		carb := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ns.GetSudoersGroupName(),
				Namespace: ns.Name,
			},
		}
		_, err := controllerutil.CreateOrUpdate(ctx, r, carb, func() error {
			if carb.Labels == nil {
				carb.Labels = make(map[string]string)
			}
			carb.Labels[LabelKey] = LabelSudoerPermissions
			carb.RoleRef = rbacv1.RoleRef{
				Name: "cluster-admin",
				Kind: "ClusterRole",
			}
			carb.Subjects = []rbacv1.Subject{{Name: ns.GetSudoersGroupName(), Kind: "Group"}}
			return controllerutil.SetOwnerReference(ns, carb, r.Scheme())
		})
		if err != nil {
			log.Error(err, "unable to create sudoer role binding to cluster-admin within the namespace")
			return err
		}
	}
	return nil
}

// UpdateDeveloperPermissions updates developer permissions on the cluster
func (r *RBACReconciler) UpdateDeveloperPermissions(ctx context.Context, ns *gialv1beta1.LNamespace) error {
	log := r.Log.WithValues("namespace", ns.Name)
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "developer",
			Namespace: ns.Name,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r, rb, func() error {
		if rb.Labels == nil {
			rb.Labels = make(map[string]string)
		}
		rb.Labels[LabelKey] = LabelDeveloperPermissions
		rb.RoleRef = rbacv1.RoleRef{
			Name: "admin",
			Kind: "ClusterRole",
		}
		rb.Subjects = ns.Spec.Developers
		return controllerutil.SetControllerReference(ns, rb, r.Scheme())
	})
	if err != nil {
		log.Error(err, "unable to create developer role binding")
		return err
	}
	return nil
}

// UpdateManagerPermissions updates manager permissions on the cluster
func (r *RBACReconciler) UpdateManagerPermissions(ctx context.Context, ns *gialv1beta1.LNamespace) error {
	log := r.Log.WithValues("namespace", ns.Name)
	{ //cr is the ClusterRole that enables sudoers to edit their own namespaces
		cr := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns.Name + "-editor",
			},
		}
		_, err := controllerutil.CreateOrUpdate(ctx, r, cr, func() error {
			if cr.Labels == nil {
				cr.Labels = make(map[string]string)
			}
			cr.Labels[LabelKey] = LabelManagerPermissions
			cr.Rules = []rbacv1.PolicyRule{
				{
					APIGroups: []string{"gial.lblw.dev"},
					Verbs: []string{
						"update",
						"patch",
						"delete",
					},
					ResourceNames: []string{ns.Name},
					Resources:     []string{"lnamespaces"},
				},
			}
			return controllerutil.SetControllerReference(ns, cr, r.Scheme())
		})
		if err != nil {
			log.Error(err, "unable to create manager cluster role for editing its own namespace")
			return err
		}
	}
	{ //crb is the ClusterRoleBinding that enables managers to edit their own namespaces
		crb := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns.Name + "-manager",
			},
		}
		_, err := controllerutil.CreateOrUpdate(ctx, r, crb, func() error {
			if crb.Labels == nil {
				crb.Labels = make(map[string]string)
			}
			crb.Labels[LabelKey] = LabelManagerPermissions
			crb.Subjects = ns.Spec.Managers
			crb.RoleRef = rbacv1.RoleRef{
				Name: ns.Name + "-editor",
				Kind: "ClusterRole",
			}
			return controllerutil.SetControllerReference(ns, crb, r.Scheme())
		})
		if err != nil {
			log.Error(err, "unable to create manager cluster role binding for editing its own namespace")
			return err
		}
	}
	return nil
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *RBACReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

	err = r.UpdateSelfImpersonators(ctx, ns)
	if err != nil {
		log.Error(err, "unable to update self impersonators")
		return ctrl.Result{}, err
	}

	err = r.UpdateSudoerGroupImpersonators(ctx, ns)
	if err != nil {
		log.Error(err, "unable to update sudoer impersonators")
		return ctrl.Result{}, err
	}

	err = r.UpdateSudoerPermissions(ctx, ns)
	if err != nil {
		log.Error(err, "unable to update sudoer permissions")
		return ctrl.Result{}, err
	}

	err = r.UpdateDeveloperPermissions(ctx, ns)
	if err != nil {
		log.Error(err, "unable to update developer permissions")
		return ctrl.Result{}, err
	}

	err = r.UpdateManagerPermissions(ctx, ns)
	if err != nil {
		log.Error(err, "unable to update developer permissions")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the RBACReconciler with the provided manager
func (r *RBACReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gialv1beta1.LNamespace{}).
		Owns(&rbacv1.RoleBinding{}).
		// this controller does not use Owns for ClusterRoles and
		// ClusterRoleBindings, because RBAC resources should update all LNamespaces
		// in its ownerReferences for self impersonator cluster roles and bindings.
		Watches(&source.Kind{
			Type: &rbacv1.ClusterRole{},
		}, &handler.EnqueueRequestForOwner{
			OwnerType:    &gialv1beta1.LNamespace{},
			IsController: false,
		}).
		Watches(&source.Kind{
			Type: &rbacv1.ClusterRoleBinding{},
		}, &handler.EnqueueRequestForOwner{
			OwnerType:    &gialv1beta1.LNamespace{},
			IsController: false,
		}).
		Complete(r)
}
