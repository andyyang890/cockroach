// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package iterutil_test

import (
	"iter"
	"maps"
	"slices"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/util/iterutil"
	"github.com/stretchr/testify/require"
)

func TestConcat(t *testing.T) {
	for name, tc := range map[string]struct {
		input [][]int
	}{
		"zero seqs": {
			input: nil,
		},
		"one seq": {
			input: [][]int{{1, 2, 3}},
		},
		"multiple seqs": {
			input: [][]int{{1, 2, 3}, {4}, {5, 6}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			var seqs []iter.Seq[int]
			var expected []int
			for _, s := range tc.input {
				seqs = append(seqs, slices.Values(s))
				expected = append(expected, s...)
			}
			actual := slices.Collect(iterutil.Concat(seqs...))
			require.Equal(t, expected, actual)
		})
	}
}

func TestConcat2(t *testing.T) {
	for name, tc := range map[string]struct {
		input []map[int]int
	}{
		"zero seqs": {
			input: []map[int]int{},
		},
		"one seq": {
			input: []map[int]int{
				{1: 1, 2: 2, 3: 3},
			},
		},
		"multiple seqs": {
			input: []map[int]int{
				{1: 1, 2: 2, 3: 3},
				{4: 4},
				{5: 5, 6: 6},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			var seqs []iter.Seq2[int, int]
			expected := make(map[int]int)
			for _, s := range tc.input {
				seqs = append(seqs, maps.All(s))
				for k, v := range s {
					expected[k] = v
				}
			}
			actual := maps.Collect(iterutil.Concat2(seqs...))
			require.Equal(t, expected, actual)
		})
	}
}

func TestEnumerate(t *testing.T) {
	for name, tc := range map[string]struct {
		input []int
	}{
		"empty": {
			input: nil,
		},
		"one element": {
			input: []int{1},
		},
		"multiple elements": {
			input: []int{1, 4, 9},
		},
	} {
		t.Run(name, func(t *testing.T) {
			expected := maps.Collect(slices.All(tc.input))
			actual := maps.Collect(iterutil.Enumerate(slices.Values(tc.input)))
			require.Equal(t, expected, actual)
		})
	}
}

func TestFilter2(t *testing.T) {
	// TODO(yang): Fill in.
}

func TestMapSeq(t *testing.T) {
	for name, tc := range map[string]struct {
		input []int
		f     func(int) int
	}{
		"empty": {
			input: nil,
			f:     func(x int) int { return x + 1 },
		},
		"one element": {
			input: []int{1},
			f:     func(x int) int { return x + 1 },
		},
		"multiple elements": {
			input: []int{1},
			f:     func(x int) int { return x + 1 },
		},
	} {
		t.Run(name, func(t *testing.T) {
			var expected []int
			for _, x := range tc.input {
				expected = append(expected, tc.f(x))
			}
			res := iterutil.MapSeq(slices.Values(tc.input), tc.f)
			require.Equal(t, tc.input, slices.Collect(iterutil.Keys(res)))
			require.Equal(t, expected, slices.Collect(iterutil.Values(res)))
		})
	}
}

func TestKeys(t *testing.T) {
	for name, tc := range map[string]struct {
		input map[int]struct{}
	}{
		"empty": {
			input: nil,
		},
		"one element": {
			input: map[int]struct{}{
				1: {},
			},
		},
		"multiple elements": {
			input: map[int]struct{}{
				1: {},
				4: {},
				9: {},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			expected := slices.Collect(maps.Keys(tc.input))
			actual := slices.Collect(iterutil.Keys(maps.All(tc.input)))
			require.ElementsMatch(t, expected, actual)
		})
	}
}

func TestValues(t *testing.T) {
	for name, tc := range map[string]struct {
		input map[int]int
	}{
		"empty": {
			input: nil,
		},
		"one element": {
			input: map[int]int{
				1: 1,
			},
		},
		"multiple elements": {
			input: map[int]int{
				1: 1,
				4: 2,
				9: 3,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			expected := slices.Collect(maps.Values(tc.input))
			actual := slices.Collect(iterutil.Values(maps.All(tc.input)))
			require.ElementsMatch(t, expected, actual)
		})
	}
}

func TestMinFunc(t *testing.T) {
	intCmp := func(a, b int) int {
		return a - b
	}

	for name, tc := range map[string]struct {
		input []int
	}{
		"empty": {
			input: nil,
		},
		"one element": {
			input: []int{1},
		},
		"multiple elements": {
			input: []int{1, 3, 2},
		},
		"multiple elements with zero value": {
			input: []int{1, 0, 3, 2},
		},
	} {
		t.Run(name, func(t *testing.T) {
			m := iterutil.MinFunc(slices.Values(tc.input), intCmp)
			if len(tc.input) == 0 {
				require.Equal(t, 0, m)
			} else {
				require.Equal(t, slices.MinFunc(tc.input, intCmp), m)
			}
		})
	}
}
