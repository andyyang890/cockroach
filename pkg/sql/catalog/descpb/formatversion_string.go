// Copyright 2023 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

// Code generated by "stringer"; DO NOT EDIT.

package descpb

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[BaseFormatVersion-1]
	_ = x[FamilyFormatVersion-2]
	_ = x[InterleavedFormatVersion-3]
}

func (i FormatVersion) String() string {
	switch i {
	case BaseFormatVersion:
		return "BaseFormatVersion"
	case FamilyFormatVersion:
		return "FamilyFormatVersion"
	case InterleavedFormatVersion:
		return "InterleavedFormatVersion"
	default:
		return "FormatVersion(" + strconv.FormatInt(int64(i), 10) + ")"
	}
}
