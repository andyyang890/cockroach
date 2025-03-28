// Copyright 2021 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package kvfeed

import (
	"github.com/cockroachdb/cockroach/pkg/ccl/changefeedccl/kvevent"
	"github.com/cockroachdb/cockroach/pkg/jobs/jobspb"
	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/kv/kvclient/kvcoord"
	"github.com/cockroachdb/cockroach/pkg/kv/kvpb"
)

// TestingKnobs are the testing knobs for kvfeed.
type TestingKnobs struct {
	// BeforeScanRequest is a callback invoked before issuing Scan request.
	BeforeScanRequest func(b *kv.Batch) error
	// OnRangeFeedValue invoked when rangefeed receives a value.
	OnRangeFeedValue func() error
	// ShouldSkipCheckpoint invoked when rangefed receives a checkpoint.
	// Returns true if checkpoint should be skipped.
	ShouldSkipCheckpoint func(*kvpb.RangeFeedCheckpoint) bool
	// OnRangeFeedStart invoked when rangefeed starts.  It is given
	// the list of SpanTimePairs.
	OnRangeFeedStart func(spans []kvcoord.SpanTimePair)
	// EndTimeReached is a callback that may return true to indicate the
	// feed should exit because its end time has been reached.
	EndTimeReached func() bool
	// RangefeedOptions lets the kvfeed override rangefeed settings.
	RangefeedOptions []kvcoord.RangeFeedOption
	// TODO consider moving this to kvevent
	BeforeBufferAdd func(jobspb.ResolvedSpan)
	// AfterWriterClose is invoked when the writer is closed,
	// with the error if applicable.
	AfterWriterClose            func(reason error, closeErr error)
	FeedToAggregatorBufferKnobs kvevent.BlockingBufferTestingKnobs
}

// ModuleTestingKnobs is part of the base.ModuleTestingKnobs interface.
func (*TestingKnobs) ModuleTestingKnobs() {}
