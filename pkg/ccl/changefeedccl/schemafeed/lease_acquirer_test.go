// Copyright 2023 The Cockroach Authors.
//
// Licensed as a CockroachDB Enterprise file under the Cockroach Community
// License (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//     https://github.com/cockroachdb/cockroach/blob/master/licenses/CCL.txt

package schemafeed

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/lease"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
)

type testTableDescriptor struct {
	catalog.TableDescriptor
	id descpb.ID
}

type testLeaseAcquirer struct {
	versions map[descpb.ID]catalog.TableDescriptor
}

func (t *testLeaseAcquirer) Acquire(
	ctx context.Context, timestamp hlc.Timestamp, id descpb.ID,
) (lease.LeasedDescriptor, error) {
	t := descpb.TableDescriptor{ID: id, ModificationTime: timestamp}

}

func (t *testLeaseAcquirer) AcquireFreshestFromStore(ctx context.Context, id descpb.ID) error {

}

func (t *testLeaseAcquirer) Codec() keys.SQLCodec {
	return keys.SQLCodec{}
}

var _ leaseAcquirer = (*testLeaseAcquirer)(nil)

type testLeasedDescriptor struct {
	lease.LeasedDescriptor
	id descpb.ID
}

func (t *testLeasedDescriptor) Underlying() catalog.Descriptor {
	return testTableDescriptor{id: t.id}
}
