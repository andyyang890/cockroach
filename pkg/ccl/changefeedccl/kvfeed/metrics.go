// Copyright 2024 The Cockroach Authors.
//
// Licensed as a CockroachDB Enterprise file under the Cockroach Community
// License (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//     https://github.com/cockroachdb/cockroach/blob/master/licenses/CCL.txt

package kvfeed

import "github.com/cockroachdb/cockroach/pkg/util/metric"

// TODO look into plumbing required for sliMetrics

// TODO take the parent thing
func NewMetrics() *Metrics {
	return &Metrics{}
}

type Metrics struct {
	Runs         *metric.Counter
	Scans        *metric.Counter     // todo reason
	SkippedScans skippedScanCounters // todo maybe no reason
	Restarts     *metric.Counter
}

type skippedScanCounters struct {
	schemaChangeNoBackfill *metric.Counter
	checkpointSufficient   *metric.Counter
}

// TODO look at pkg/kv/kvclient/kvcoord/dist_sender.go
type restartCounters struct {
	reasonA *metric.Counter
}
