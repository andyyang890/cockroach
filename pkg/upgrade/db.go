// Copyright 2023 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package upgrade

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/clusterversion"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descs"
	"github.com/cockroachdb/cockroach/pkg/sql/isql"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/catconstants"
)

// DB is a wrapper on a descs.DB and is used for upgrade dependencies.
type DB struct {
	descs.DB
}

func MakeDB(db descs.DB) DB {
	return DB{DB: db}
}

// SystemDatabaseSchemaChangeTxn should be used for transactions that will
// perform schema changes on the system database since it will handle bumping
// the internal database version field on the system database descriptor.
func (db DB) SystemDatabaseSchemaChangeTxn(
	ctx context.Context,
	cs clusterversion.ClusterVersion,
	f func(context.Context, descs.Txn) error,
	opts ...isql.TxnOption,
) error {
	return db.DescsTxn(
		ctx,
		func(ctx context.Context, txn descs.Txn) error {
			if err := f(ctx, txn); err != nil {
				return err
			}
			systemDBDesc, err := txn.Descriptors().MutableByName(txn.KV()).Database(ctx, catconstants.SystemDatabaseName)
			if err != nil {
				return err
			}
			systemDBDesc.InternalDatabaseVersion = &cs.Version
			return txn.Descriptors().WriteDesc(ctx, false /* kvTrace */, systemDBDesc, txn.KV())
		},
		opts...,
	)
}
