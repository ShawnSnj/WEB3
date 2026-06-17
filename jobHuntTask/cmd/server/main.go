// Command server is the entrypoint of the job-hunt task tracker.
//
// Responsibilities live ONLY here:
//   - load configuration
//   - construct dependencies (logger, db pool, router, http server)
//   - run until a signal arrives
//   - shut down gracefully in reverse order of construction
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shawn/jobhunttask/internal/api"
	"github.com/shawn/jobhunttask/internal/calendar"
	"github.com/shawn/jobhunttask/internal/config"
	"github.com/shawn/jobhunttask/internal/crm/ai"
	crmkafka "github.com/shawn/jobhunttask/internal/crm/kafka"
	crmrepo "github.com/shawn/jobhunttask/internal/crm/repository"
	crmsvc "github.com/shawn/jobhunttask/internal/crm/service"
	"github.com/shawn/jobhunttask/internal/database"
	"github.com/shawn/jobhunttask/internal/logger"
	"github.com/shawn/jobhunttask/internal/repository"
	"github.com/shawn/jobhunttask/internal/scheduler"
	"github.com/shawn/jobhunttask/internal/service"
	"github.com/shawn/jobhunttask/internal/web"
)

func main() {
	if err := run(); err != nil {
		// Logger may not be ready yet — fall back to stderr.
		slog.Error("fatal", slog.String("error", err.Error()))
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
	log.Info("starting application",
		slog.String("name", cfg.App.Name),
		slog.String("version", cfg.App.Version),
		slog.String("env", cfg.App.Environment),
		slog.String("timezone", cfg.App.Timezone),
		slog.Bool("crm_enabled", cfg.CRM.Enabled),
	)
	if cfg.CRM.BasePath != "" {
		web.SetCRMPath(cfg.CRM.BasePath)
	}

	// Root context cancelled on SIGINT/SIGTERM.
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Database
	pool, err := database.NewPool(rootCtx, cfg.Database, log)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Repositories + services
	taskRepo := repository.NewPostgresTaskRepository(pool)
	reviewRepo := repository.NewPostgresReviewRepository(pool)
	sessionRepo := repository.NewPostgresTaskSessionRepository(pool)
	reminderRepo := repository.NewPostgresReminderRepository(pool)
	metricsRepo := repository.NewPostgresMetricsRepository(pool)
	suggestionRepo := repository.NewPostgresSuggestionRepository(pool)

	taskSvc := service.NewTaskService(taskRepo, service.SystemClock, cal)
	reviewSvc := service.NewDailyReviewService(reviewRepo, service.SystemClock)
	sessionSvc := service.NewTaskSessionService(sessionRepo, taskSvc, service.SystemClock)
	taskSvc.SetSessionGuard(sessionSvc)
	metricsSvc := service.NewMetricsService(metricsRepo, service.SystemClock, cal)
	suggestionSvc := service.NewSuggestionService(
		suggestionRepo, metricsRepo, metricsSvc, nil, service.SystemClock,
		service.SuggestionServiceConfig{},
	)
	notifier := service.NewSlogNotifier(log.With(slog.String("component", "notifier")))
	reminderSvc := service.NewReminderService(
		reminderRepo, notifier, service.SystemClock,
		log.With(slog.String("component", "reminder")),
		service.ReminderServiceConfig{
			MaxAttempts: cfg.Reminder.MaxAttempts,
			BatchSize:   cfg.Reminder.BatchSize,
		},
	)

	// Scheduler — use APP_TIMEZONE when SCHEDULER_TZ is unset so midnight jobs
	// align with task calendar boundaries.
	schedTZ := cfg.Scheduler.TimeZone
	if schedTZ == "" {
		schedTZ = cfg.App.Timezone
	}
	sched, err := scheduler.New(scheduler.Config{
		Enabled:                cfg.Scheduler.Enabled,
		TimeZone:               schedTZ,
		JobTimeout:             cfg.Scheduler.JobTimeout,
		MorningReminderSpec:    cfg.Scheduler.MorningReminderSpec,
		EveningReviewSpec:      cfg.Scheduler.EveningReviewSpec,
		WeeklyReviewSpec:       cfg.Scheduler.WeeklyReviewSpec,
		OverdueScannerSpec:     cfg.Scheduler.OverdueScannerSpec,
		AutoCarryOverSpec:      cfg.Scheduler.AutoCarryOverSpec,
		DailyRolloverSpec:      cfg.Scheduler.DailyRolloverSpec,
		ReminderDispatcherSpec: cfg.Scheduler.ReminderDispatcherSpec,
	}, log.With(slog.String("component", "scheduler")))
	if err != nil {
		return err
	}
	schedDeps := scheduler.Deps{
		Tasks:                taskSvc,
		Reminders:            reminderSvc,
		Sessions:             sessionSvc,
		Clock:                service.SystemClock,
		Logger:               log,
		DailyRolloverOnStart: cfg.Scheduler.DailyRolloverOnStart,
	}
	if err := scheduler.RegisterJobs(sched, schedDeps); err != nil {
		return err
	}

	// CRM — AI job hunt pipeline
	var crmService *crmsvc.CRM
	if cfg.CRM.Enabled {
		crmStore := crmrepo.New(pool)
		aiClient := ai.NewClient(cfg.OpenAI.APIKey, cfg.OpenAI.Model, cfg.OpenAI.BaseURL)
		var publisher crmsvc.EventPublisher
		if pub, err := crmkafka.NewPublisher(crmkafka.Config{
			Brokers: crmkafka.ParseBrokers(cfg.Kafka.Brokers),
		}, log.With(slog.String("component", "kafka"))); err == nil && pub != nil {
			publisher = pub
			defer pub.Close()
		}
		crmService = crmsvc.New(crmStore, log.With(slog.String("component", "crm")), aiClient, publisher)
		if spec := cfg.CRM.DailyPipelineSpec; spec != "" {
			if err := sched.Register("crm_daily_pipeline", spec, func(ctx context.Context) error {
				return crmService.RunDailyPipeline(ctx)
			}); err != nil {
				return err
			}
		}
	}
	startupCtx, startupCancel := context.WithTimeout(rootCtx, cfg.Scheduler.JobTimeout)
	if err := scheduler.RunStartupJobs(startupCtx, schedDeps); err != nil {
		log.Warn("daily rollover on startup failed", slog.String("error", err.Error()))
	}
	startupCancel()
	sched.Start()
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
		defer cancel()
		_ = sched.Stop(stopCtx)
	}()

	// HTTP server
	router := api.NewRouter(api.Deps{
		Config:             cfg,
		Logger:             log,
		DB:                 pool,
		TaskService:        taskSvc,
		ReviewService:      reviewSvc,
		TaskSessionService: sessionSvc,
		MetricsService:     metricsSvc,
		SuggestionService:  suggestionSvc,
		CRMService:         crmService,
	})

	// Server-rendered web UI (HTML pages + static assets).
	renderer, err := web.New()
	if err != nil {
		return fmt.Errorf("init web renderer: %w", err)
	}
	if err := web.MountStatic(router); err != nil {
		return fmt.Errorf("mount static: %w", err)
	}
	if cfg.CRM.Enabled {
		if err := web.MountCRM(router); err != nil {
			return fmt.Errorf("mount crm ui: %w", err)
		}
	}
	web.RegisterRoutes(router, renderer)

	// Wire the data-backed dashboard (full page + per-card refresh endpoints).
	dashboard := web.NewDashboardHandler(
		renderer, taskSvc, reviewSvc, reminderSvc,
		metricsSvc, suggestionSvc, service.SystemClock, cal,
		log.With(slog.String("component", "dashboard")),
	)
	dashboard.Register(router)

	taskNoteRepo := repository.NewPostgresTaskNoteRepository(pool)
	taskNoteSvc := service.NewTaskNoteService(taskNoteRepo, taskRepo)

	// Wire the data-backed tasks page (full CRUD + state transitions + bulk).
	tasksPage := web.NewTasksHandler(
		renderer, taskSvc, sessionSvc, taskNoteSvc, service.SystemClock, cal,
		log.With(slog.String("component", "tasks_ui")),
	)
	tasksPage.Register(router)

	dailyReview := web.NewDailyReviewHandler(
		renderer, reviewSvc, taskSvc, sessionSvc, service.SystemClock,
		log.With(slog.String("component", "daily_review_ui")),
	)
	dailyReview.Register(router)

	weeklyReviewRepo := repository.NewPostgresWeeklyReviewRepository(pool)
	weeklyReviewSvc := service.NewWeeklyReviewService(weeklyReviewRepo, service.SystemClock)
	weeklyReview := web.NewWeeklyReviewHandler(
		renderer, weeklyReviewSvc, metricsSvc, sessionSvc, suggestionSvc,
		service.SystemClock, log.With(slog.String("component", "weekly_review_ui")),
	)
	weeklyReview.Register(router)

	analytics := web.NewAnalyticsHandler(
		renderer, metricsSvc, service.SystemClock,
		log.With(slog.String("component", "analytics_ui")),
	)
	analytics.Register(router)

	srv := &http.Server{
		Addr:         cfg.HTTP.Addr(),
		Handler:      router,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info("http server listening", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case <-rootCtx.Done():
		log.Info("shutdown signal received")
	case err := <-serverErr:
		log.Error("http server error", slog.String("error", err.Error()))
		return err
	}

	shutdownStart := time.Now()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", slog.String("error", err.Error()))
		return err
	}
	log.Info("shutdown complete", slog.Duration("took", time.Since(shutdownStart)))
	return nil
}
