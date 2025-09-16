// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package changefeedccl

import (
	"context"
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/ccl/changefeedccl/cdctest"
	"github.com/cockroachdb/cockroach/pkg/jobs"
	"github.com/cockroachdb/cockroach/pkg/jobs/jobfrontier"
	"github.com/cockroachdb/cockroach/pkg/jobs/jobspb"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/desctestutils"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfra"
	"github.com/cockroachdb/cockroach/pkg/sql/isql"
	"github.com/cockroachdb/cockroach/pkg/sql/rowenc"
	"github.com/cockroachdb/cockroach/pkg/testutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/encoding"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/require"
)

// TestChangefeedFrontierPersistence verifies that changefeeds periodically
// persist their span frontiers to the job info table.
func TestChangefeedFrontierPersistence(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	testFn := func(t *testing.T, s TestServer, f cdctest.TestFeedFactory) {
		sqlDB := sqlutils.MakeSQLRunner(s.DB)
		ctx := context.Background()

		// Set a short interval for frontier persistence.
		sqlDB.Exec(t, "SET CLUSTER SETTING changefeed.progress.frontier_persistence.interval = '5s'")

		// Get frontier persistence metric.
		registry := s.Server.JobRegistry().(*jobs.Registry)
		metric := registry.MetricsStruct().Changefeed.(*Metrics).AggMetrics.Timers.FrontierPersistence

		// Verify metric count starts at zero.
		initialCount, _ := metric.CumulativeSnapshot().Total()
		require.Equal(t, int64(0), initialCount)

		// Create a table and insert some data.
		sqlDB.Exec(t, "CREATE TABLE foo (a INT PRIMARY KEY, b STRING)")
		sqlDB.Exec(t, "INSERT INTO foo VALUES (1, 'a'), (2, 'b'), (3, 'c')")

		// Start a changefeed.
		foo := feed(t, f, "CREATE CHANGEFEED FOR foo")
		defer closeFeed(t, foo)

		// Make sure frontier gets persisted to job_info table.
		jobID := foo.(cdctest.EnterpriseTestFeed).JobID()
		testutils.SucceedsSoon(t, func() error {
			var found bool
			var allSpans []jobspb.ResolvedSpan
			if err := s.Server.InternalDB().(isql.DB).Txn(ctx, func(ctx context.Context, txn isql.Txn) error {
				var err error
				allSpans, found, err = jobfrontier.GetAllResolvedSpans(ctx, txn, jobID)
				if err != nil {
					return err
				}
				return nil
			}); err != nil {
				return err
			}
			if !found {
				return errors.Newf("frontier not yet persisted")
			}
			t.Logf("found resolved spans in job_info table: %+v", allSpans)
			return nil
		})

		// Verify metric count and average latency have sensible values.
		testutils.SucceedsSoon(t, func() error {
			metricSnapshot := metric.CumulativeSnapshot()
			count, _ := metricSnapshot.Total()
			if count == 0 {
				return errors.Newf("metrics not yet updated")
			}
			avgLatency := time.Duration(metricSnapshot.Mean())
			t.Logf("frontier persistence metrics - count: %d, avg latency: %s", count, avgLatency)
			require.Greater(t, count, int64(0))
			require.Greater(t, avgLatency, time.Duration(0))
			return nil
		})
	}

	cdcTest(t, testFn, feedTestEnterpriseSinks)
}

// TestChangefeedFrontierRestore verifies that changefeeds will correctly
// restore progress from persisted span frontiers.
func TestChangefeedFrontierRestore(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	// TODO look at TestChangefeedLaggingSpanCheckpointing

	// TODO test initial scan
	// TODO test lagging spans
	// TODO test schema change backfill

	// TODO for each of these cases, we need to make sure that there are spans at
	// different timestamps, through some form of lag

	// TODO we can ensure that the span frontier looks like what we'd expect

	// TODO also use updated(?) and strip TS to make sure we get the same message
	// again for the messages we want and not the other

	// TODO create a table and split it into two ranges and have one row in each

	testFn := func(t *testing.T, s TestServer, f cdctest.TestFeedFactory) {
		sqlDB := sqlutils.MakeSQLRunner(s.DB)
		ctx := context.Background()

		// Create a table with a single int primary key column
		sqlDB.Exec(t, "CREATE TABLE foo (a INT PRIMARY KEY)")

		// Insert some initial data
		sqlDB.Exec(t, "INSERT INTO foo VALUES (1), (2), (3)")

		// Get the table descriptor to construct the key for row a=1
		codec := s.Server.Codec()
		fooDesc := desctestutils.TestingGetPublicTableDescriptor(
			s.Server.DB(), codec, "d", "foo")

		// Construct the key for row with a=1
		// First get the index key prefix for the primary index
		keyPrefix := rowenc.MakeIndexKeyPrefix(
			codec, fooDesc.GetID(), fooDesc.GetPrimaryIndexID())

		// Encode the value 1 as the primary key
		key := keyPrefix
		key = encoding.EncodeVarintAscending(key, 1)

		// Create a span that contains just this one key
		targetSpan := roachpb.Span{
			Key:    key,
			EndKey: roachpb.Key(key).Next(),
		}

		// Set up testing knob to filter out row with a=1
		knobs := s.Server.TestingKnobs().
			DistSQL.(*execinfra.TestingKnobs).
			Changefeed.(*TestingKnobs)

		knobs.ChangeFrontierKnobs.OnAggregatorProgress = func(resolvedSpans *jobspb.ResolvedSpans) error {
			// Use SpanGroup to subtract the target span from each resolved span
			var filtered []jobspb.ResolvedSpan
			for _, rs := range resolvedSpans.ResolvedSpans {
				// Create a SpanGroup with the original span
				var group roachpb.SpanGroup
				group.Add(rs.Span)

				// Subtract the span containing row a=1
				group.Sub(targetSpan)

				// Add the remaining spans to the filtered list, preserving all other fields
				for _, span := range group.Slice() {
					// Create a copy of the original ResolvedSpan with the modified span
					modified := rs
					modified.Span = span
					filtered = append(filtered, modified)
				}

				if len(group.Slice()) != 1 || !group.Slice()[0].Equal(rs.Span) {
					t.Logf("Modified span: original %s, remaining spans: %v @ %s",
						rs.Span, group.Slice(), rs.Timestamp)
				}
			}
			resolvedSpans.ResolvedSpans = filtered
			return nil
		}

		// Start a changefeed
		foo := feed(t, f, "CREATE CHANGEFEED FOR foo")
		defer closeFeed(t, foo)

		_ = ctx
	}

	cdcTest(t, testFn, feedTestEnterpriseSinks)
}
