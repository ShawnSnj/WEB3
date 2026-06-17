// Command reschedule-tasks spreads today + overdue tasks onto future days.
//
// Usage: go run ./cmd/reschedule-tasks
//        go run ./cmd/reschedule-tasks -per-day=2
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/shawn/jobhunttask/internal/calendar"
	"github.com/shawn/jobhunttask/internal/config"
	"github.com/shawn/jobhunttask/internal/database"
	"github.com/shawn/jobhunttask/internal/logger"
	"github.com/shawn/jobhunttask/internal/repository"
	"github.com/shawn/jobhunttask/internal/service"
)

func main() {
	perDay := flag.Int("per-day", 2, "tasks per calendar day")
	flag.Parse()

	if err := run(*perDay); err != nil {
		slog.Error("reschedule failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(perDay int) error {
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
	now := service.SystemClock.Now()

	res, err := taskSvc.RescheduleTodayAndOverdue(ctx, perDay)
	if err != nil {
		return err
	}

	tomorrow := cal.FormatDate(cal.StartOfDay(now).Add(24 * time.Hour))
	fmt.Printf("Rescheduled %d task(s) — %d per day starting tomorrow (%s)\n\n",
		res.Moved, perDay, tomorrow)

	byDay := map[string][]service.RescheduleAssignment{}
	for _, a := range res.Plan {
		byDay[a.NewDue] = append(byDay[a.NewDue], a)
	}
	for _, day := range sortedKeys(byDay) {
		fmt.Printf("  %s\n", day)
		for _, a := range byDay[day] {
			fmt.Printf("    • %s\n", a.Title)
		}
	}

	if len(res.Errors) > 0 {
		fmt.Printf("\n%d error(s):\n", len(res.Errors))
		for _, e := range res.Errors {
			fmt.Printf("  - %v\n", e)
		}
		return res.Errors[0]
	}
	return nil
}

func sortedKeys(m map[string][]service.RescheduleAssignment) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

func sortStrings(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}
