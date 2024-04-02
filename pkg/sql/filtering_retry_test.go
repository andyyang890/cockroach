package sql

import (
	"context"
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/kv/kvclient/kvcoord"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/randutil"
)

func TestFilteringTransactionAborted(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	ctx := context.Background()
	rand, _ := randutil.NewTestRand()

	s, db, _ := serverutils.StartServer(t, base.TestServerArgs{
		Knobs: base.TestingKnobs{
			KVClient: &kvcoord.ClientTestingKnobs{
				TransactionRetryFilter: func(transaction roachpb.Transaction) bool {
					fmt.Printf("[filter] transaction ID: %s\n", transaction.ID)
					if transaction.OmitInRangefeeds {
						if rand.Intn(10) < 9 {
							return true
						}
					}
					return false
				},
			},
		},
	})
	defer s.Stopper().Stop(ctx)

	r := sqlutils.MakeSQLRunner(db)
	r.Exec(t, "CREATE TABLE t (x int)")
	for i := 0; i < 100; i++ {
		r.Exec(t, fmt.Sprintf("BEGIN; INSERT INTO t VALUES (%d); COMMIT", i))
	}
}
