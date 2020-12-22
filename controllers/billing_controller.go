package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	gialv1beta1 "github.com/loblaw-sre/namespace-controller/api/v1beta1"
	"github.com/loblaw-sre/namespace-controller/pkg/bq"
	"github.com/loblaw-sre/namespace-controller/pkg/types"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type BillingReconciler struct {
	client.Client
	Log         logr.Logger
	DatasetName string
	TableName   string
	DB          types.DB
}

func (r *BillingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("namespace", req.Name)
	ns := &gialv1beta1.LNamespace{}
	{
		err := r.Get(ctx, client.ObjectKey{
			Name:      req.Name,
			Namespace: req.Namespace,
		}, ns)
		if apierrors.IsNotFound(err) {
			log.Info("namespace not found. Continuing as if deleted.")
			//TODO: cleanup BQ
			return ctrl.Result{}, nil
		} else if err != nil {
			log.Error(err, "unable to get namespace definition")
			return ctrl.Result{}, err
		}
	}
	// see if ns labels are already created
	// TODO: this should be parameterized, but cannot be supported via the current
	// bq util. SQL inject should be low risk here, since namespace names are
	// similarly constricted anyway
	query := fmt.Sprintf("SELECT * FROM %s.%s WHERE ns_name='%s'", r.DatasetName, r.TableName, req.Name)
	res, err := r.DB.RunQuery(ctx, query, nil)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "unable to retrieve current ns labels")
	}
	unpackedLabels := make(map[string]string)
	toDelete := make(map[string]bool)
	for _, row := range res {
		unpackedLabels[row["name"]] = row["value"]
		toDelete[row["name"]] = true
	}

	entriesToUpdate := []types.NSLabelEntry{}
	for k, v := range ns.Spec.Billing {
		value, ok := unpackedLabels[k]
		update := value != v || !ok
		if update {
			entriesToUpdate = append(entriesToUpdate, types.NSLabelEntry{
				NSName: ns.Name,
				Name:   k,
				Value:  v,
			})
		}
		delete(toDelete, k) // don't delete if found
	}
	if len(entriesToUpdate) > 0 {
		if _, err := r.DB.RunQuery(ctx, fmt.Sprintf(bq.BQCreateOrUpdateStatement, r.DatasetName, r.TableName), entriesToUpdate); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "unable to update ns labels")
		}
	}
	if len(toDelete) > 0 {
		entriesToDelete := []types.NSLabelEntry{}
		for k := range toDelete {
			entriesToDelete = append(entriesToDelete, types.NSLabelEntry{
				NSName: ns.Name,
				Name:   k,
			})
		}
		if _, err := r.DB.RunQuery(ctx, fmt.Sprintf(bq.BQDeleteStatement, r.DatasetName, r.TableName), entriesToDelete); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "unable to delete ns labels")
		}
	}
	return ctrl.Result{}, nil
}

func (r *BillingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gialv1beta1.LNamespace{}).
		Complete(r)
}
