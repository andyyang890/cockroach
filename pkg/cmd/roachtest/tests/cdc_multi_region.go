// Copyright 2024 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package tests

import (
	"context"
	"fmt"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachtest/cluster"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachtest/option"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachtest/registry"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachtest/test"
	"github.com/cockroachdb/cockroach/pkg/roachprod/install"
	"time"
)

func registerCDCMultiRegion(r registry.Registry) {
	r.Add(registry.TestSpec{
		Name:             "cdc/multi-region/sink=null",
		Owner:            registry.OwnerCDC,
		Cluster:          r.MakeClusterSpec(9),
		CompatibleClouds: registry.OnlyGCE,
		Suites:           registry.Suites(registry.Nightly),
		RequiresLicense:  true,
		Run:              runCDCMultiRegionNullSink,
	})
}

func runCDCMultiRegionNullSink(ctx context.Context, t test.Test, c cluster.Cluster) {
	startOpts := option.DefaultStartOpts()
	startOpts.RoachprodOpts.ExtraArgs = []string{"--vmodule=distsql_physical_planner=2,changefeed_dist=2,followerreads=2,oracle=2"}
	settings := install.MakeClusterSettings(install.NumRacksOption(3))
	c.Start(ctx, t.L(), startOpts, settings)
	m := c.NewMonitor(ctx, c.All())

	restart := func(n int) error {
		cmd := fmt.Sprintf("./cockroach node drain --certs-dir=%s --port={pgport:%d} --self", install.CockroachNodeCertsDir, n)
		if err := c.RunE(ctx, option.WithNodes(c.Node(n)), cmd); err != nil {
			return err
		}
		m.ExpectDeath()
		c.Stop(ctx, t.L(), option.DefaultStopOpts(), c.Node(n))
		opts := startOpts
		opts.RoachprodOpts.IsRestart = true
		c.Start(ctx, t.L(), opts, settings, c.Node(n))
		m.ResetDeaths()
		return nil
	}

	db := c.Conn(ctx, t.L(), 1)
	for _, s := range []string{
		`ALTER RANGE default CONFIGURE ZONE USING num_replicas = 5, constraints = '{+rack=0: 2, +rack=1: 2, +rack=2: 1}', lease_preferences = '[[+rack=0], [+rack=1], [+rack=2]]'`,
		//`SET CLUSTER SETTING changefeed.random_replica_selection.enabled = false`,
		`SET CLUSTER SETTING kv.rangefeed.enabled = true`,
	} {
		if _, err := db.Exec(s); err != nil {
			t.Fatal(err)
		}
	}

	t.L().Printf("restarting rack=1 nodes")
	for _, n := range []int{2, 5, 8} {
		if err := restart(n); err != nil {
			t.Fatal(err)
		}
	}

	for _, s := range []string{
		`CREATE TABLE t (id PRIMARY KEY, data) AS SELECT generate_series(1, 1000000), gen_random_uuid()`,
		`ALTER TABLE t SPLIT AT SELECT id FROM t ORDER BY random() LIMIT 100`,
	} {
		if _, err := db.Exec(s); err != nil {
			t.Fatal(err)
		}
	}

	// TODO(yang): Maybe replace this with something that checks the number of replicas.
	t.L().Printf("wait for ranges to replicate")
	time.Sleep(time.Minute)

	t.L().Printf("starting changefeed")
	var jobID int
	if err := db.QueryRow(
		`CREATE CHANGEFEED FOR t INTO 'null://' WITH initial_scan = 'only', execution_locality = 'rack=1'`,
	).Scan(&jobID); err != nil {
		t.Fatal(err)
	}

	t.L().Printf("waiting for changefeed %d", jobID)
	if _, err := db.ExecContext(ctx, "SHOW JOB WHEN COMPLETE $1", jobID); err != nil {
		t.Fatal(err)
	}

	m.Wait()
	t.Fatal("fail test to collect artifacts")
}
