// Copyright 2023 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package upgrades

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/clusterversion"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descs"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/catconstants"
	"github.com/cockroachdb/cockroach/pkg/upgrade"
)

func addInternalDatabaseVersionToSystemDatabaseDescriptor(
	ctx context.Context, cs clusterversion.ClusterVersion, d upgrade.TenantDeps,
) error {
	if err := d.DB.DescsTxn(ctx, func(ctx context.Context, txn descs.Txn) error {
		systemDBDesc, err := txn.Descriptors().MutableByName(txn.KV()).Database(ctx, catconstants.SystemDatabaseName)
		if err != nil {
			return err
		}
		systemDBDesc.InternalDatabaseVersion = &cs.Version
		return txn.Descriptors().WriteDesc(ctx, false /* kvTrace */, systemDBDesc, txn.KV())
	}); err != nil {
		return err
	}
	// TODO(yang): Fill in and unit test.
	return nil
}
