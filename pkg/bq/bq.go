package bq

import (
	"context"

	"cloud.google.com/go/bigquery"
	"github.com/loblaw-sre/namespace-controller/pkg/types"
	"github.com/pkg/errors"
	"google.golang.org/api/iterator"
)

const (
	// BQCreateOrUpdateStatement serves as the statement that bigquery uses for CreateOrUpdate statements.
	// Wrap this in a fmt.Sprintf with the dataset name and table name for best results!
	// the one and only parameter in this should be a []types.NSLabelEntry.
	BQCreateOrUpdateStatement = `MERGE %s.%s T
	USING (SELECT * FROM UNNEST(?)) S
	ON T.ns_name = S.NSName AND T.name = S.Name
	WHEN MATCHED THEN
		UPDATE SET value = S.value
	WHEN NOT MATCHED THEN
		INSERT (name, value, ns_name) VALUES (S.Name, s.Value, s.NSName)`

	// BQDeleteStatement serves as the statement that bigquery uses for CreateOrUpdate statements.
	// Wrap this in a fmt.Sprintf with the dataset name and table name for best results!
	// the one and only parameter in this should be a []types.NSLabelEntry of the things that should be deleted.
	BQDeleteStatement = `DELETE %s.%s T WHERE EXISTS (SELECT * FROM UNNEST(?) as S WHERE T.ns_name = S.NSName AND T.name = S.Name)`
)

var _ types.DB = &BigQuery{}

// BigQuery is an implementation of types.DB that uses BigQuery as a backing storage.
type BigQuery struct {
	ProjectID string
}

// RunQuery implements types.DB
func (bq *BigQuery) RunQuery(ctx context.Context, query string, nsLabelEntries []types.NSLabelEntry) ([]map[string]string, error) {
	client, err := bigquery.NewClient(ctx, bq.ProjectID)
	if err != nil {
		return nil, errors.Wrap(err, "error initializing bigquery client")
	}
	q := client.Query(query)
	if nsLabelEntries != nil {
		q.Parameters = []bigquery.QueryParameter{
			{Value: nsLabelEntries},
		}
	}
	// for _, v := range param {
	// 	q.Parameters = append(q.Parameters, bigquery.QueryParameter{
	// 		Value: v,
	// 	})
	// }
	it, err := q.Read(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error reading from bigquery")
	}
	out := []map[string]string{}
	for {
		var values map[string]bigquery.Value
		err := it.Next(&values)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, errors.Wrap(err, "error processing results")
		}
		item := map[string]string{}
		for k, v := range values {
			item[k] = v.(string)
		}
		out = append(out, item)
	}
	return out, nil
}
