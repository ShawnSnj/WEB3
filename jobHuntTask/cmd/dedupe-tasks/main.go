// Command dedupe-tasks removes duplicate tasks:
//   - same title on the same calendar due day (keeps oldest)
//   - same title across different days while still pending/in-progress
//     (keeps the non-carried copy with the earliest due date)
//
// Usage: go run ./cmd/dedupe-tasks
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/shawn/jobhunttask/internal/calendar"
	"github.com/shawn/jobhunttask/internal/config"
	"github.com/shawn/jobhunttask/internal/database"
	"github.com/shawn/jobhunttask/internal/logger"
	"github.com/shawn/jobhunttask/internal/repository"
	"github.com/shawn/jobhunttask/internal/service"
)

func main() {
	if err := run(); err != nil {
		slog.Error("dedupe failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cal, err := calendar.Load(cfg.App.Timezone)
	if err != nil {
		return err
	}
	log := logger.New(cfg.Log)

	ctx := context.Background()
	pool, err := database.NewPool(ctx, cfg.Database, log)
	if err != nil {
		return err
	}
	defer pool.Close()

	taskRepo := repository.NewPostgresTaskRepository(pool)
	taskSvc := service.NewTaskService(taskRepo, service.SystemClock, cal)

	groups, err := taskSvc.FindDuplicatePendingByTitle(ctx)
	if err != nil {
		return err
	}
	if len(groups) > 0 {
		fmt.Printf("Found %d duplicate title group(s):\n", len(groups))
		for _, g := range groups {
			fmt.Printf("  • %q (%d copies)\n", g.Title, len(g.Tasks))
		}
		fmt.Println()
	}

	sameDay, err := taskSvc.CollapseDuplicatePlans(ctx)
	if err != nil {
		return err
	}
	crossDay, err := taskSvc.CollapseDuplicatePendingByTitle(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Removed %d duplicate(s) — %d same-day, %d cross-day pending\n",
		sameDay+crossDay, sameDay, crossDay)
	return nil
}
