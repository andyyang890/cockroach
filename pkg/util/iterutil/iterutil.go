// Copyright 2020 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package iterutil

import (
	"iter"

	"github.com/cockroachdb/errors"
)

var errStopIteration = errors.New("stop iteration")

// StopIteration returns a sentinel error that indicates stopping the iteration.
//
// This error should not be propagated further, i.e., if a closure returns
// this error, the loop should break returning nil error. For example:
//
//	f := func(i int) error {
//		if i == 10 {
//			return iterutil.StopIteration()
//		}
//		return nil
//	}
//
//	for i := range slice {
//		if err := f(i); err != nil {
//			return iterutil.Map(err)
//		}
//		// continue when nil error
//	}
func StopIteration() error { return errStopIteration }

// Map the nil if it is StopIteration, or keep the error otherwise
func Map(err error) error {
	if errors.Is(err, errStopIteration) {
		return nil
	}
	return err
}

// Concat takes an arbitrary number of iter.Seq's and concatenates them
// into a single iter.Seq.
func Concat[E any](seqs ...iter.Seq[E]) iter.Seq[E] {
	return func(yield func(E) bool) {
		for _, seq := range seqs {
			for v := range seq {
				if !yield(v) {
					return
				}
			}
		}
	}
}

// Concat2 takes an arbitrary number of iter.Seq2's and concatenates them
// into a single iter.Seq2.
func Concat2[K, V any](seqs ...iter.Seq2[K, V]) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, seq := range seqs {
			for k, v := range seq {
				if !yield(k, v) {
					return
				}
			}
		}
	}
}

// Enumerate takes an iter.Seq and returns an iter.Seq2 where the first value
// is an int counter.
func Enumerate[E any](seq iter.Seq[E]) iter.Seq2[int, E] {
	return func(yield func(int, E) bool) {
		var i int
		for v := range seq {
			if !yield(i, v) {
				return
			}
			i++
		}
	}
}

// Filter2 takes an iter.Seq2 and returns a new iter.Seq2 that filters out any
// elements that do not satisfy the provided filter function.
func Filter2[K, V any](seq iter.Seq2[K, V], f func(K, V) bool) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for k, v := range seq {
			if f(k, v) {
				if !yield(k, v) {
					return
				}
			}
		}
	}
}

// MapSeq takes an iter.Seq and a map function and returns an iter.Seq2 where
// the second value is the result of applying the map to the original value.
func MapSeq[K, V any](seq iter.Seq[K], f func(K) V) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for k := range seq {
			if !yield(k, f(k)) {
				return
			}
		}
	}
}

// Keys takes an iter.Seq2 and returns an iter.Seq that iterates over the
// first value in each pair of values.
func Keys[K, V any](seq iter.Seq2[K, V]) iter.Seq[K] {
	return func(yield func(K) bool) {
		for k := range seq {
			if !yield(k) {
				return
			}
		}
	}
}

// Values takes an iter.Seq2 and returns an iter.Seq that iterates over the
// second value in each pair of values.
func Values[K, V any](seq iter.Seq2[K, V]) iter.Seq[V] {
	return func(yield func(V) bool) {
		for _, v := range seq {
			if !yield(v) {
				return
			}
		}
	}
}

// MinFunc returns the minimum element in seq, using cmp to compare elements.
// If seq has no values, the zero value is returned.
func MinFunc[E any](seq iter.Seq[E], cmp func(E, E) int) E {
	var m E
	for i, v := range Enumerate(seq) {
		if i == 0 || cmp(v, m) < 0 {
			m = v
		}
	}
	return m
}
