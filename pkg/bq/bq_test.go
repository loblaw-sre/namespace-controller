// +build integration

package bq_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"cloud.google.com/go/bigquery"
	"github.com/loblaw-sre/namespace-controller/pkg/bq"
	"github.com/loblaw-sre/namespace-controller/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
)

const TestTimeout = 10

var (
	TableName   = os.Getenv("NC_TEST_TABLE_NAME")
	DatasetName = os.Getenv("NC_TEST_DATASET_NAME")
	ProjectID   = os.Getenv("NC_TEST_PROJECT_ID")
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"BigQuery",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = SynchronizedBeforeSuite(func() []byte {
	ctx := context.Background()
	client, err := bigquery.NewClient(ctx, ProjectID)
	Expect(err).ToNot(HaveOccurred())
	err = client.Dataset(DatasetName).Create(ctx, &bigquery.DatasetMetadata{})
	Expect(err).ToNot(HaveOccurred())
	err = client.Dataset(DatasetName).Table(TableName).Create(ctx, &bigquery.TableMetadata{
		Schema: bigquery.Schema{
			{
				Name:     "ns_name",
				Type:     bigquery.StringFieldType,
				Required: false,
			},
			{
				Name:     "name",
				Type:     bigquery.StringFieldType,
				Required: false,
			},
			{
				Name:     "value",
				Type:     bigquery.StringFieldType,
				Required: false,
			},
		},
	})
	Expect(err).ToNot(HaveOccurred())
	return nil
}, func(b []byte) {}, 60)

var _ = SynchronizedAfterSuite(func() {}, func() {
	ctx := context.Background()
	client, err := bigquery.NewClient(ctx, ProjectID)
	Expect(err).ToNot(HaveOccurred())
	err = client.Dataset(DatasetName).DeleteWithContents(ctx)
	_ = client
	Expect(err).ToNot(HaveOccurred())
}, 60)

var _ = Describe("big query client", func() {
	var testID string
	var ctx context.Context
	var b bq.BigQuery
	When("an NS Label is created", func() {
		BeforeEach(func(done Done) {
			testID = rand.String(10)
			ctx = context.Background()
			b = bq.BigQuery{
				ProjectID: ProjectID,
			}
			query := fmt.Sprintf(bq.BQCreateOrUpdateStatement, DatasetName, TableName)
			_, err := b.RunQuery(ctx, query, []types.NSLabelEntry{{NSName: testID, Name: "key", Value: "value"}})
			Expect(err).ToNot(HaveOccurred())
			close(done)
		}, TestTimeout)
		It("actually creates the ns label record", func(done Done) {
			query := fmt.Sprintf("SELECT * FROM %s.%s WHERE ns_name = %s", DatasetName, TableName, testID)
			res, err := b.RunQuery(ctx, query, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(ContainElement(MatchAllKeys(Keys{
				"ns_name": Equal(testID),
				"name":    Equal("key"),
				"value":   Equal("value"),
			})))
			close(done)
		}, TestTimeout)
		When("that NSLabel is deleted", func() {
			BeforeEach(func(done Done) {
				query := fmt.Sprintf(bq.BQDeleteStatement, DatasetName, TableName)
				_, err := b.RunQuery(ctx, query, []types.NSLabelEntry{{NSName: testID, Name: "key", Value: "value"}})
				Expect(err).ToNot(HaveOccurred())
				close(done)
			}, TestTimeout)
			It("is actually deleted", func(done Done) {
				query := fmt.Sprintf("SELECT * FROM %s.%s WHERE ns_name = %s", DatasetName, TableName, testID)
				res, err := b.RunQuery(ctx, query, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).ToNot(ContainElement(MatchAllKeys(Keys{
					"ns_name": Equal(testID),
					"name":    Equal("key"),
					"value":   Equal("value"),
				})))
				close(done)
			}, TestTimeout)
		})
	})
})
