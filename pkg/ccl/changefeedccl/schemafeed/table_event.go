// Copyright 2024 The Cockroach Authors.
//
// Licensed as a CockroachDB Enterprise file under the Cockroach Community
// License (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//     https://github.com/cockroachdb/cockroach/blob/master/licenses/CCL.txt

package schemafeed

import (
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"golang.org/x/exp/slices"
)

// TableEvent represents a change to a table descriptor.
type TableEvent struct {
	Before, After catalog.TableDescriptor
}

// Timestamp refers to the ModificationTime of the After table descriptor.
func (e TableEvent) Timestamp() hlc.Timestamp {
	return e.After.GetModificationTime()
}

// sortedTableEvents contains a sorted list of table events.
// It is used internally by schemaFeed.
// TODO(yang): See if we want to add a mutex.
type sortedTableEvents struct {
	events []TableEvent
}

func (e sortedTableEvents) Peek(atOrBefore hlc.Timestamp) []TableEvent {
	i := e.prefixEndIndex(atOrBefore)
	return e.events[:i]
}

func (e sortedTableEvents) Pop(atOrBefore hlc.Timestamp) []TableEvent {
	i := e.prefixEndIndex(atOrBefore)
	ret := e.events[:i]
	e.events = e.events[i:]
	return ret
}

// prefixEndIndex returns the exclusive end index for the prefix of events that
// have timestamps at or before atOrBefore.
func (e sortedTableEvents) prefixEndIndex(atOrBefore hlc.Timestamp) int {
	i, _ := slices.BinarySearchFunc(e.events, atOrBefore, func(event TableEvent, timestamp hlc.Timestamp) int {
		return event.Timestamp().Compare(timestamp)
	})
	return i
}
