package kafka

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"

	"github.com/shawn/jobhunttask/internal/crm/service"
	crm "github.com/shawn/jobhunttask/internal/crm/model"
)

const (
	TopicJobsIngested      = "crm.jobs.ingested"
	TopicJobsScored        = "crm.jobs.scored"
	TopicApplicationChanged = "crm.application.changed"
	TopicMissionGenerated  = "crm.mission.generated"
)

type Config struct {
	Brokers []string
	GroupID string
}

type Publisher struct {
	ingestWriter  *kafka.Writer
	scoredWriter  *kafka.Writer
	missionWriter *kafka.Writer
	log           *slog.Logger
}

func NewPublisher(cfg Config, log *slog.Logger) (*Publisher, error) {
	if len(cfg.Brokers) == 0 {
		return nil, nil
	}
	addr := kafka.TCP(cfg.Brokers...)
	newWriter := func(topic string) *kafka.Writer {
		return &kafka.Writer{
			Addr:         addr,
			Topic:        topic,
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireOne,
			Async:        true,
		}
	}
	return &Publisher{
		ingestWriter:  newWriter(TopicJobsIngested),
		scoredWriter:  newWriter(TopicJobsScored),
		missionWriter: newWriter(TopicMissionGenerated),
		log:           log,
	}, nil
}

type jobEvent struct {
	JobID     string    `json:"job_id"`
	Event     string    `json:"event"`
	FitScore  int       `json:"fit_score,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type missionEvent struct {
	BriefDate         string    `json:"brief_date"`
	ApplyJobID        string    `json:"apply_job_id,omitempty"`
	EstimatedMinutes  int       `json:"estimated_minutes"`
	Timestamp         time.Time `json:"timestamp"`
}

func (p *Publisher) PublishJobIngested(ctx context.Context, jobID uuid.UUID) error {
	if p == nil || p.ingestWriter == nil {
		return nil
	}
	payload, _ := json.Marshal(jobEvent{
		JobID:     jobID.String(),
		Event:     "ingested",
		Timestamp: time.Now().UTC(),
	})
	return p.ingestWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(jobID.String()),
		Value: payload,
	})
}

func (p *Publisher) PublishJobScored(ctx context.Context, jobID uuid.UUID, fitScore int) error {
	if p == nil || p.scoredWriter == nil {
		return nil
	}
	payload, _ := json.Marshal(jobEvent{
		JobID:     jobID.String(),
		Event:     "scored",
		FitScore:  fitScore,
		Timestamp: time.Now().UTC(),
	})
	return p.scoredWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(jobID.String()),
		Value: payload,
	})
}

func (p *Publisher) PublishMissionGenerated(ctx context.Context, brief *crm.DailyBrief) error {
	if p == nil || p.missionWriter == nil || brief == nil {
		return nil
	}
	ev := missionEvent{
		BriefDate:        brief.BriefDate.Format("2006-01-02"),
		EstimatedMinutes: brief.EstimatedMinutes,
		Timestamp:        time.Now().UTC(),
	}
	if brief.ApplyJob != nil {
		ev.ApplyJobID = brief.ApplyJob.ID.String()
	}
	payload, _ := json.Marshal(ev)
	return p.missionWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(ev.BriefDate),
		Value: payload,
	})
}

func (p *Publisher) Close() error {
	if p == nil {
		return nil
	}
	var err error
	for _, w := range []*kafka.Writer{p.ingestWriter, p.scoredWriter, p.missionWriter} {
		if w != nil {
			if e := w.Close(); e != nil {
				err = e
			}
		}
	}
	return err
}

// Consumer scores jobs from the ingest topic.
type Consumer struct {
	reader *kafka.Reader
	crm    *service.CRM
	log    *slog.Logger
}

func NewConsumer(cfg Config, crm *service.CRM, log *slog.Logger) (*Consumer, error) {
	if len(cfg.Brokers) == 0 {
		return nil, nil
	}
	group := cfg.GroupID
	if group == "" {
		group = "crm-scorer"
	}
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  cfg.Brokers,
		GroupID:  group,
		Topic:    TopicJobsIngested,
		MinBytes: 1,
		MaxBytes: 1e6,
		MaxWait:  500 * time.Millisecond,
	})
	return &Consumer{reader: r, crm: crm, log: log}, nil
}

func (c *Consumer) Run(ctx context.Context) error {
	if c == nil || c.reader == nil {
		return nil
	}
	c.log.Info("kafka consumer started", slog.String("topic", TopicJobsIngested))
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			c.log.Warn("kafka fetch error", slog.String("error", err.Error()))
			continue
		}
		var ev jobEvent
		if err := json.Unmarshal(msg.Value, &ev); err == nil {
			id, err := uuid.Parse(ev.JobID)
			if err == nil {
				if _, err := c.crm.ScoreJob(ctx, id); err != nil {
					c.log.Warn("score job failed", slog.String("job_id", ev.JobID), slog.String("error", err.Error()))
				}
			}
		}
		_ = c.reader.CommitMessages(ctx, msg)
	}
}

func (c *Consumer) Close() error {
	if c == nil || c.reader == nil {
		return nil
	}
	return c.reader.Close()
}

func ParseBrokers(raw string) []string {
	if raw == "" {
		return nil
	}
	var out []string
	for _, p := range splitComma(raw) {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func splitComma(s string) []string {
	var parts []string
	for _, p := range fmtSplit(s) {
		parts = append(parts, p)
	}
	return parts
}

func fmtSplit(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			part := trim(s[start:i])
			out = append(out, part)
			start = i + 1
		}
	}
	return out
}

func trim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

// Ensure Publisher implements service.EventPublisher
var _ service.EventPublisher = (*Publisher)(nil)
