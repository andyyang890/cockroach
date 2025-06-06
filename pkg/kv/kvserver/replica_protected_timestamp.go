// Copyright 2019 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package kvserver

import (
	"context"
	"time"

	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/gc"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/cockroach/pkg/util/log"
)

// cachedProtectedTimestampState is used to cache information about the state
// of protected timestamps as they pertain to this replica. The data is
// refreshed when the replica examines protected timestamps when being
// considered for gc. It is consulted when determining whether a request can be
// served.
type cachedProtectedTimestampState struct {
	// readAt denotes the timestamp at which this record was read.
	// It is used to coordinate updates to this field. It is also used to
	// ensure that the protected timestamp subsystem can be relied upon. If
	// the cache state is older than the lease start time then it is possible
	// that protected timestamps have not been observed. In this case we must
	// assume that any protected timestamp could exist to provide the contract
	// on verify.
	readAt                      hlc.Timestamp
	earliestProtectionTimestamp hlc.Timestamp
}

// clearIfNotNewer clears the state in ts if it is not newer than the passed
// value. This is used in conjunction with Replica.maybedUpdateCachedProtectedTS().
// This optimization allows most interactions with protected timestamps to
// operate using a shared lock. Only in cases where the cached value is known to
// be older will the update be attempted.
func (ts *cachedProtectedTimestampState) clearIfNotNewer(existing cachedProtectedTimestampState) {
	if !existing.readAt.Less(ts.readAt) {
		*ts = cachedProtectedTimestampState{}
	}
}

// maybeUpdateCachedProtectedTS is used to optimize updates. We learn about
// needs to update the cache while holding Replica.mu for reading but need to
// perform the update with the exclusive lock. This function is intended to
// be deferred.
func (r *Replica) maybeUpdateCachedProtectedTS(ts *cachedProtectedTimestampState) {
	if *ts == (cachedProtectedTimestampState{}) {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.mu.cachedProtectedTS.readAt.Less(ts.readAt) {
		r.mu.cachedProtectedTS = *ts
	}
}

func (r *Replica) readProtectedTimestampsRLocked(
	ctx context.Context,
) (ts cachedProtectedTimestampState, _ error) {
	desc := r.descRLocked()
	gcThreshold := *r.shMu.state.GCThreshold

	sp := roachpb.Span{
		Key:    roachpb.Key(desc.StartKey),
		EndKey: roachpb.Key(desc.EndKey),
	}
	var protectionTimestamps []hlc.Timestamp
	var err error
	protectionTimestamps, ts.readAt, err = r.store.protectedtsReader.GetProtectionTimestamps(ctx, sp)
	if err != nil {
		return ts, err
	}
	earliestTS := hlc.Timestamp{}
	for _, protectionTimestamp := range protectionTimestamps {
		// Check if the timestamp the record was trying to protect is strictly
		// below the GCThreshold, in which case, we know the record does not apply.
		if isValid := gcThreshold.LessEq(protectionTimestamp); !isValid {
			continue
		}

		log.VEventf(ctx, 2, "span: %s has a protection policy protecting: %s",
			sp, protectionTimestamp)

		if earliestTS.IsEmpty() || protectionTimestamp.Less(earliestTS) {
			earliestTS = protectionTimestamp
		}
	}
	ts.earliestProtectionTimestamp = earliestTS
	return ts, nil
}

// checkProtectedTimestampsForGC determines whether the Replica can run GC. If
// the Replica can run GC, this method returns the latest timestamp which can be
// used to determine a valid new GCThreshold. The policy is passed in rather
// than read from the replica state to ensure that the same value used for this
// calculation is used later.
//
// In the case that GC can proceed, four timestamps are returned: The timestamp
// used as the basis to calculate the new gc threshold (used for scoring and
// reporting), the old gc threshold, and the new gc threshold.
func (r *Replica) checkProtectedTimestampsForGC(
	ctx context.Context, gcTTL time.Duration,
) (canGC bool, gcTimestamp, oldThreshold, newThreshold hlc.Timestamp, _ error) {

	// We may be reading the protected timestamp cache while we're holding
	// the Replica.mu for reading. If we do so and find newer state in the cache
	// then we want to, update the replica's cache of its state. The guarantee
	// we provide is that if a record is successfully verified then the Replica's
	// cachedProtectedTS will have a readAt value high enough to include that
	// record.
	var read cachedProtectedTimestampState
	defer r.maybeUpdateCachedProtectedTS(&read)
	r.mu.RLock()
	defer r.mu.RUnlock()
	defer read.clearIfNotNewer(r.mu.cachedProtectedTS)

	oldThreshold = *r.shMu.state.GCThreshold
	lease := *r.shMu.state.Lease

	// read.earliestRecord is the record with the earliest timestamp which is
	// greater than the existing gcThreshold.
	var err error
	read, err = r.readProtectedTimestampsRLocked(ctx)
	if err != nil {
		return false, hlc.Timestamp{}, hlc.Timestamp{}, hlc.Timestamp{}, err
	}

	if read.readAt.IsEmpty() {
		// We don't want to allow GC to proceed if no protected timestamp
		// information is available. This can happen if the initial scan of the
		// rangefeed established by the spanconfig.KVSubscriber hasn't completed
		// yet.
		log.VEventf(ctx, 1,
			"not gc'ing replica %v because protected timestamp information is unavailable", r)
		return false, hlc.Timestamp{}, hlc.Timestamp{}, hlc.Timestamp{}, nil
	}

	if read.readAt.Less(lease.Start.ToTimestamp()) {
		log.VEventf(ctx, 1, "not gc'ing replica %v because current lease %v started after record was"+
			" read %v", r, lease, read.readAt)
		return false, hlc.Timestamp{}, hlc.Timestamp{}, hlc.Timestamp{}, nil
	}

	gcTimestamp = read.readAt
	if !read.earliestProtectionTimestamp.IsEmpty() {
		// NB: we want to allow GC up to the timestamp preceding the earliest valid
		// protection timestamp.
		impliedGCTimestamp := gc.TimestampForThreshold(read.earliestProtectionTimestamp.Prev(), gcTTL)
		if impliedGCTimestamp.Less(gcTimestamp) {
			gcTimestamp = impliedGCTimestamp
		}
	}

	newThreshold = gc.CalculateThreshold(gcTimestamp, gcTTL)

	return true, gcTimestamp, oldThreshold, newThreshold, nil
}
