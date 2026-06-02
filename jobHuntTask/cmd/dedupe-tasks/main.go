// Command dedupe-tasks removes duplicate tasks that share the same title and
// calendar due day, keeping the oldest row in each group.
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

	removed, err := taskSvc.CollapseDuplicatePlans(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("removed %d duplicate task(s)\n", removed)
	return nil
}
