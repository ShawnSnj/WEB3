// Command crm-worker consumes Kafka job-ingest events and scores listings.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/shawn/jobhunttask/internal/config"
	"github.com/shawn/jobhunttask/internal/crm/ai"
	crmkafka "github.com/shawn/jobhunttask/internal/crm/kafka"
	crmrepo "github.com/shawn/jobhunttask/internal/crm/repository"
	crmsvc "github.com/shawn/jobhunttask/internal/crm/service"
	"github.com/shawn/jobhunttask/internal/database"
	"github.com/shawn/jobhunttask/internal/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", slog.String("error", err.Error()))
		os.Exit(1)
	}
	log := logger.New(cfg.Log)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.NewPool(ctx, cfg.Database, log)
	if err != nil {
		log.Error("db", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	store := crmrepo.New(pool)
	aiClient := ai.NewClient(cfg.OpenAI.APIKey, cfg.OpenAI.Model, cfg.OpenAI.BaseURL)
	crm := crmsvc.New(store, log, aiClient, nil)

	consumer, err := crmkafka.NewConsumer(crmkafka.Config{
		Brokers: crmkafka.ParseBrokers(cfg.Kafka.Brokers),
		GroupID: cfg.Kafka.GroupID,
	}, crm, log)
	if err != nil || consumer == nil {
		log.Error("kafka consumer unavailable — set KAFKA_BROKERS")
		os.Exit(1)
	}
	defer consumer.Close()

	log.Info("crm-worker started")
	if err := consumer.Run(ctx); err != nil {
		log.Error("worker stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
