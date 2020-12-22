package types

import "context"

// NSLabelEntry is the representation of a row in the NS Labels table.
type NSLabelEntry struct {
	NSName string
	Name   string
	Value  string
}

// DB allows the abstraction of a database.
type DB interface {
	RunQuery(ctx context.Context, query string, nsLabelEntries []NSLabelEntry) ([]map[string]string, error) //TODO: genericize RunQuery to support arbitrary params instead of hardlocking nsLabelEntries.
}

// NewDBClient returns a DB client built on the context passed in
type NewDBClient func(context.Context) DB
