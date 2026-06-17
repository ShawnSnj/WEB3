package aggregator_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/shawn/jobhunttask/internal/crm/aggregator"
)

func TestCollectDetailedLive(t *testing.T) {
	if os.Getenv("LIVE_FETCH") != "1" {
		t.Skip("set LIVE_FETCH=1 to hit external APIs")
	}
	r := aggregator.New(slog.Default())
	res, err := r.CollectDetailed(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("jobs=%d filtered=%d", len(res.Jobs), res.FilteredOut)
	for _, s := range res.Sources {
		t.Logf("%s fetched=%d err=%q", s.Source, s.Fetched, s.Error)
	}
	if len(res.Jobs) == 0 {
		t.Fatal("expected jobs from at least one source")
	}
}
