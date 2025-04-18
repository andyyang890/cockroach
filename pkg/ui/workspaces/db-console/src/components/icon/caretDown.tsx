// Copyright 2020 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

import * as React from "react";

export interface IconProps {
  fill?: string;
}

CaretDown.defaultProps = {
  fill: "#475872",
};

export function CaretDown(props: IconProps) {
  const { fill } = props;

  return (
    <svg width={8} height={6} viewBox="0 0 8 6" fill="none" {...props}>
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M1.39.45a.667.667 0 10-1.003.878l3.111 3.555a.667.667 0 001.004 0l3.11-3.555A.667.667 0 106.61.45L4 3.432 1.39.45z"
        fill={fill}
      />
    </svg>
  );
}
