// Copyright 2024 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

// Package checkpoint contains code responsible for handling changefeed
// checkpoints.
package checkpoint

import (
	"container/heap"
	"iter"
	"maps"
	"slices"

	"github.com/cockroachdb/cockroach/pkg/jobs/jobspb"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/settings"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/cockroach/pkg/util/metamorphic"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/cockroachdb/errors"
)

// Make creates a checkpoint with as many spans that should be checkpointed (are
// above the highwater mark) as can fit in maxBytes, along with the earliest
// timestamp of the checkpointed spans. Adjacent spans at the same timestamp
// are merged together to reduce checkpoint size.
func Make(
	overallResolved hlc.Timestamp,
	spans iter.Seq2[roachpb.Span, hlc.Timestamp],
	maxBytes int64,
	metrics *Metrics,
) *jobspb.TimestampSpansMap {
	start := timeutil.Now()

	spanGroupMap := make(map[hlc.Timestamp]*roachpb.SpanGroup)
	for s, ts := range spans {
		if ts.After(overallResolved) {
			if spanGroupMap[ts] == nil {
				spanGroupMap[ts] = new(roachpb.SpanGroup)
			}
			spanGroupMap[ts].Add(s)
		}
	}
	if len(spanGroupMap) == 0 {
		return nil
	}

	checkpointSpansMap := make(map[hlc.Timestamp]roachpb.Spans)
	var totalSpanKeyBytes int64
	for ts, spanGroup := range spanGroupMap {
		for _, sp := range spanGroup.Slice() {
			spanKeyBytes := int64(len(sp.Key)) + int64(len(sp.EndKey))
			if totalSpanKeyBytes+spanKeyBytes > maxBytes {
				break
			}
			checkpointSpansMap[ts] = append(checkpointSpansMap[ts], sp)
			totalSpanKeyBytes += spanKeyBytes
		}
	}
	cp := jobspb.NewTimestampSpansMap(checkpointSpansMap)
	if cp == nil {
		return nil
	}

	if metrics != nil {
		metrics.CreateNanos.RecordValue(int64(timeutil.Since(start)))
		metrics.TotalBytes.RecordValue(int64(cp.Size()))
		metrics.TimestampCount.RecordValue(int64(cp.TimestampCount()))
		metrics.SpanCount.RecordValue(int64(cp.SpanCount()))
	}

	return cp
}

type spanHeapElem struct {
	span        roachpb.Span
	ts          hlc.Timestamp
	effectiveTS hlc.Timestamp
}

type spanHeap []*spanHeapElem

// Len implements sort.Interface.
func (h spanHeap) Len() int { return len(h) }

// Less implements sort.Interface.
func (h spanHeap) Less(i, j int) bool {
	if !h[i].effectiveTS.Equal(h[j].effectiveTS) {
		return h[i].effectiveTS.Less(h[j].effectiveTS)
	}
	return h[i].span.Key.Less(h[j].span.Key)
}

// Swap implements sort.Interface.
func (h spanHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

// Push implements heap.Interface.
func (h *spanHeap) Push(x any) {
	*h = append(*h, x.(*spanHeapElem))
}

// Pop implements heap.Interface.
func (h *spanHeap) Pop() any {
	old := *h
	n := len(old)
	last := old[n-1]
	*h = old[:n-1]
	return last
}

// horizontalTiling is a cluster setting that controls whether to use
// horizontal tiling for changefeed checkpoints. When enabled, this can reduce
// the size of checkpoints by reducing the total number of spans needed.
var horizontalTiling = settings.RegisterBoolSetting(
	settings.ApplicationLevel,
	"changefeed.checkpoint.horizontal_tiling.enabled",
	"when enabled, changefeed checkpoints will use horizontal tiling to reduce checkpoint size",
	metamorphic.ConstantWithTestBool("changefeed.checkpoint.horizontal_tiling.enabled", true),
)

func Make2(
	overallResolved hlc.Timestamp,
	spans iter.Seq2[roachpb.Span, hlc.Timestamp],
	maxBytes int64,
	metrics *Metrics,
) *jobspb.TimestampSpansMap {
	start := timeutil.Now()

	spanGroupMap := make(map[hlc.Timestamp]*roachpb.SpanGroup)
	for s, ts := range spans {
		if ts.After(overallResolved) {
			if spanGroupMap[ts] == nil {
				spanGroupMap[ts] = new(roachpb.SpanGroup)
			}
			spanGroupMap[ts].Add(s)
		}
	}
	if len(spanGroupMap) == 0 {
		return nil
	}

	// TODO BEGIN NEW CODE

	// spanKeys is a string version of roachpb.Span, which is necessary
	// to make it a map key.
	type spanKeys struct {
		Key    string
		EndKey string
	}

	convertToSpanKeys := func(s roachpb.Span) spanKeys {
		return spanKeys{
			Key:    string(s.Key),
			EndKey: string(s.EndKey),
		}
	}

	var h spanHeap
	var acc roachpb.SpanGroup
	heapElemMap := make(map[spanKeys]*spanHeapElem)

	sortedTimestamps := slices.SortedFunc(maps.Keys(spanGroupMap), hlc.Timestamp.Compare)
	for _, ts := range slices.Backward(sortedTimestamps) {
		for sp := range spanGroupMap[ts].All() {
			acc.Add(sp)
		}
		for sp := range acc.All() {
			k := convertToSpanKeys(sp)
			if heapElemMap[k] != nil {
				heapElemMap[k].effectiveTS = ts
				continue
			}
			e := &spanHeapElem{
				span:        sp,
				ts:          ts,
				effectiveTS: ts,
			}
			h = append(h, e)
			heapElemMap[k] = e
		}
	}

	heap.Init(&h)

	checkpointSpansMap := make(map[hlc.Timestamp]roachpb.Spans)
	var totalSpanKeyBytes int64
	for h.Len() > 0 {
		e := heap.Pop(&h).(*spanHeapElem)
		spanKeyBytes := int64(len(e.span.Key) + len(e.span.EndKey))
		if totalSpanKeyBytes+spanKeyBytes > maxBytes {
			break
		}
		checkpointSpansMap[e.ts] = append(checkpointSpansMap[e.ts], e.span)
		totalSpanKeyBytes += spanKeyBytes
	}

	// TODO END NEW CODE

	cp := jobspb.NewTimestampSpansMap(checkpointSpansMap)
	if cp == nil {
		return nil
	}

	if metrics != nil {
		metrics.CreateNanos.RecordValue(int64(timeutil.Since(start)))
		metrics.TotalBytes.RecordValue(int64(cp.Size()))
		metrics.TimestampCount.RecordValue(int64(cp.TimestampCount()))
		metrics.SpanCount.RecordValue(int64(cp.SpanCount()))
	}

	return cp
}

// SpanForwarder is an interface for forwarding spans to a changefeed.
type SpanForwarder interface {
	Forward(span roachpb.Span, ts hlc.Timestamp) (bool, error)
}

// Restore restores the saved progress from a checkpoint to the given SpanForwarder.
func Restore(sf SpanForwarder, checkpoint *jobspb.TimestampSpansMap) error {
	for ts, spans := range checkpoint.All() {
		if ts.IsEmpty() {
			return errors.New("checkpoint timestamp is empty")
		}
		for _, sp := range spans {
			if _, err := sf.Forward(sp, ts); err != nil {
				return err
			}
		}
	}
	return nil
}

// ConvertFromLegacyCheckpoint converts a checkpoint from the legacy format
// into the current format.
func ConvertFromLegacyCheckpoint(
	//lint:ignore SA1019 deprecated usage
	checkpoint *jobspb.ChangefeedProgress_Checkpoint,
	statementTime hlc.Timestamp,
	initialHighWater hlc.Timestamp,
) *jobspb.TimestampSpansMap {
	if checkpoint.IsEmpty() {
		return nil
	}

	checkpointTS := checkpoint.Timestamp

	// Checkpoint records from 21.2 were used only for backfills and did not store
	// the timestamp, since in a backfill it must either be the StatementTime for
	// an initial backfill, or right after the high-water for schema backfills.
	if checkpointTS.IsEmpty() {
		if initialHighWater.IsEmpty() {
			checkpointTS = statementTime
		} else {
			checkpointTS = initialHighWater.Next()
		}
	}

	return jobspb.NewTimestampSpansMap(map[hlc.Timestamp]roachpb.Spans{
		checkpointTS: checkpoint.Spans,
	})
}

// ConvertToLegacyCheckpoint converts a checkpoint from the current format
// into the legacy format.
func ConvertToLegacyCheckpoint(
	checkpoint *jobspb.TimestampSpansMap,
) *jobspb. //lint:ignore SA1019 deprecated usage
						ChangefeedProgress_Checkpoint {
	if checkpoint.IsEmpty() {
		return nil
	}

	// Collect leading spans into a SpanGroup to merge adjacent spans.
	var checkpointSpanGroup roachpb.SpanGroup
	for _, spans := range checkpoint.All() {
		checkpointSpanGroup.Add(spans...)
	}
	if checkpointSpanGroup.Len() == 0 {
		return nil
	}

	//lint:ignore SA1019 deprecated usage
	return &jobspb.ChangefeedProgress_Checkpoint{
		Spans:     checkpointSpanGroup.Slice(),
		Timestamp: checkpoint.MinTimestamp(),
	}
}
