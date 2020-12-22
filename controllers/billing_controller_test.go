package controllers_test

import (
	"context"
	"fmt"

	gialv1beta1 "github.com/loblaw-sre/namespace-controller/api/v1beta1"
	"github.com/loblaw-sre/namespace-controller/controllers"
	"github.com/loblaw-sre/namespace-controller/pkg/bq"
	gt "github.com/loblaw-sre/namespace-controller/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	DatasetName = "test-dataset"
	TableName   = "test-table"
)

type QueryRecord struct {
	Query string
	// Params []interface{}
	NSLabelEntries []gt.NSLabelEntry
}

var _ gt.DB = &TestDB{}

type TestDB struct {
	log    []QueryRecord
	output []map[string]string
}

//RunQuery implements types.DB
func (d *TestDB) RunQuery(ctx context.Context, query string, nsLabelEntries []gt.NSLabelEntry) ([]map[string]string, error) {
	// d.log = append(d.log, QueryRecord{Query: query, Params: param})
	d.log = append(d.log, QueryRecord{Query: query, NSLabelEntries: nsLabelEntries})
	return d.output, nil
}

var _ = Describe("Billing Controller", func() {
	var ctx context.Context
	var ns *gialv1beta1.LNamespace
	var br *controllers.BillingReconciler
	var k8sClient client.Client
	var db *TestDB
	BeforeEach(func(done Done) {
		k8sClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
		db = &TestDB{}
		br = &controllers.BillingReconciler{
			Client:      k8sClient,
			Log:         logf.Log,
			DatasetName: DatasetName,
			TableName:   TableName,
			DB:          db,
		}
		ns = &gialv1beta1.LNamespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: DefaultName,
			},
			Spec: gialv1beta1.LNamespaceSpec{
				Billing: map[string]string{
					"budget": "1.0",
				},
			},
		}
		ctx = context.Background()
		close(done)
	})
	When("a namespace is created", func() {
		BeforeEach(func(done Done) {
			err := k8sClient.Create(ctx, ns)
			Expect(err).ToNot(HaveOccurred(), "Creating LNamespace should not have errored.")
			close(done)
		}, TestTimeout)
		Context("ns label entries", func() {
			It("creates ns label entries if ns label entries don't exist", func(done Done) {
				db.output = []map[string]string{}
				_, err := br.Reconcile(ctx, controllerruntime.Request{
					NamespacedName: types.NamespacedName{
						Name: ns.Name,
					},
				})
				Expect(err).ToNot(HaveOccurred(), "Reconciling LNamespace should not have errored.")
				Expect(db.log).To(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Query": Equal(fmt.Sprintf(bq.BQCreateOrUpdateStatement, DatasetName, TableName)),
					// "Params": ConsistOf(ContainElement(MatchAllFields(Fields{
					"NSLabelEntries": ConsistOf(MatchAllFields(Fields{
						"NSName": Equal(DefaultName),
						"Name":   Equal("budget"),
						"Value":  Equal("1.0"),
					})),
				})))
				close(done)
			}, TestTimeout)
			It("no-ops if ns label entries already exist", func(done Done) {
				db.output = []map[string]string{
					{"ns_name": DefaultName, "name": "budget", "value": "1.0"},
				}
				_, err := br.Reconcile(ctx, controllerruntime.Request{
					NamespacedName: types.NamespacedName{
						Name: ns.Name,
					},
				})
				Expect(err).ToNot(HaveOccurred(), "Reconciling LNamespace should not have errored.")
				Expect(db.log).ToNot(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Query": Equal(fmt.Sprintf(bq.BQCreateOrUpdateStatement, DatasetName, TableName)),
					// "Params": ConsistOf(ContainElement(MatchAllFields(Fields{
					"NSLabelEntries": ConsistOf(MatchAllFields(Fields{
						"NSName": Equal(DefaultName),
						"Name":   Equal("budget"),
						"Value":  Equal("1.0"),
					})),
				})))
				close(done)
			}, TestTimeout)
		})
	})
})
