// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package kvevent

// BlockingBufferTestingKnobs are testing knobs for blocking buffers.
type BlockingBufferTestingKnobs struct {
	BeforePop func()
}
