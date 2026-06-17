// Package config loads application configuration from environment variables.
//
// The 12-factor approach: a single typed struct is parsed once at startup,
// then passed down to whichever components need it. .env files are loaded
// transparently in development; in production the process environment is
// authoritative.
package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config is the root configuration aggregate.
type Config struct {
	App       App
	HTTP      HTTP
	Database  Database
	Log       Log
	Scheduler Scheduler
	Reminder  Reminder
	CRM       CRM
	OpenAI    OpenAI
	Kafka     Kafka
}

// Scheduler holds cron specs for the background automation jobs. Each spec
// is a standard 5-field cron expression (no seconds). An empty value
// disables that job.
type Scheduler struct {
	Enabled                 bool          `env:"SCHEDULER_ENABLED"                 envDefault:"true"`
	TimeZone                string        `env:"SCHEDULER_TZ"                      envDefault:"UTC"`
	JobTimeout              time.Duration `env:"SCHEDULER_JOB_TIMEOUT"             envDefault:"2m"`
	MorningReminderSpec     string        `env:"CRON_MORNING_REMINDER"             envDefault:"0 9 * * *"`
	EveningReviewSpec       string        `env:"CRON_EVENING_REVIEW"               envDefault:"0 21 * * *"`
	WeeklyReviewSpec        string        `env:"CRON_WEEKLY_REVIEW"                envDefault:"0 20 * * 0"`
	OverdueScannerSpec      string        `env:"CRON_OVERDUE_SCANNER"              envDefault:"*/15 * * * *"`
	AutoCarryOverSpec       string        `env:"CRON_AUTO_CARRY_OVER"              envDefault:"5 0 * * *"`
	DailyRolloverSpec       string        `env:"CRON_DAILY_ROLLOVER"               envDefault:"0 0 * * *"`
	DailyRolloverOnStart    bool          `env:"DAILY_ROLLOVER_ON_START"           envDefault:"true"`
	ReminderDispatcherSpec  string        `env:"CRON_REMINDER_DISPATCHER"          envDefault:"* * * * *"`
}

// Reminder tunes the dispatcher behaviour.
type Reminder struct {
	MaxAttempts int `env:"REMINDER_MAX_ATTEMPTS" envDefault:"5"`
	BatchSize   int `env:"REMINDER_BATCH_SIZE"   envDefault:"100"`
}

// CRM holds AI Job Hunt CRM settings.
type CRM struct {
	Enabled           bool   `env:"CRM_ENABLED"             envDefault:"true"`
	BasePath          string `env:"CRM_BASE_PATH"           envDefault:"/crm"`
	FrontendOrigin    string `env:"CRM_FRONTEND_ORIGIN"     envDefault:""` // legacy; CORS only if set
	DailyPipelineSpec string `env:"CRON_CRM_DAILY_PIPELINE" envDefault:"0 7 * * *"`
}

// OpenAI configures the LLM client for matching, resume, and outreach.
type OpenAI struct {
	APIKey  string `env:"OPENAI_API_KEY"`
	Model   string `env:"OPENAI_MODEL"   envDefault:"gpt-4o-mini"`
	BaseURL string `env:"OPENAI_BASE_URL" envDefault:"https://api.openai.com/v1"`
}

// Kafka configures async job scoring pipeline.
type Kafka struct {
	Brokers string `env:"KAFKA_BROKERS" envDefault:""`
	GroupID string `env:"KAFKA_GROUP_ID" envDefault:"crm-scorer"`
}

// App holds high-level application metadata.
type App struct {
	Name        string `env:"APP_NAME"        envDefault:"jobhunt-task"`
	Environment string `env:"APP_ENV"         envDefault:"development"` // development | staging | production
	Version     string `env:"APP_VERSION"     envDefault:"0.1.0"`
	Timezone    string `env:"APP_TIMEZONE"    envDefault:"UTC"`
}

// HTTP holds server-related configuration.
type HTTP struct {
	Host            string        `env:"HTTP_HOST"             envDefault:"0.0.0.0"`
	Port            int           `env:"HTTP_PORT"             envDefault:"8082"`
	ReadTimeout     time.Duration `env:"HTTP_READ_TIMEOUT"     envDefault:"15s"`
	WriteTimeout    time.Duration `env:"HTTP_WRITE_TIMEOUT"    envDefault:"15s"`
	IdleTimeout     time.Duration `env:"HTTP_IDLE_TIMEOUT"     envDefault:"60s"`
	ShutdownTimeout time.Duration `env:"HTTP_SHUTDOWN_TIMEOUT" envDefault:"15s"`
}

// Addr returns the host:port pair suitable for net.Listen.
func (h HTTP) Addr() string {
	return fmt.Sprintf("%s:%d", h.Host, h.Port)
}

// Database holds PostgreSQL connection settings. Either a full DSN (DATABASE_URL)
// can be provided, or the discrete fields will be assembled into one.
type Database struct {
	URL             string        `env:"DATABASE_URL"`
	Host            string        `env:"DB_HOST"              envDefault:"localhost"`
	Port            int           `env:"DB_PORT"              envDefault:"5432"`
	User            string        `env:"DB_USER"              envDefault:"jobhunt"`
	Password        string        `env:"DB_PASSWORD"          envDefault:"jobhunt"`
	Name            string        `env:"DB_NAME"              envDefault:"jobhunt"`
	SSLMode         string        `env:"DB_SSLMODE"           envDefault:"disable"`
	MaxConns        int32         `env:"DB_MAX_CONNS"         envDefault:"20"`
	MinConns        int32         `env:"DB_MIN_CONNS"         envDefault:"2"`
	MaxConnLifetime time.Duration `env:"DB_MAX_CONN_LIFETIME" envDefault:"1h"`
	MaxConnIdleTime time.Duration `env:"DB_MAX_CONN_IDLE"     envDefault:"30m"`
	ConnectTimeout  time.Duration `env:"DB_CONNECT_TIMEOUT"   envDefault:"5s"`
}

// DSN returns a PostgreSQL connection string. If DATABASE_URL is set it
// takes precedence; otherwise the discrete fields are assembled.
func (d Database) DSN() string {
	if d.URL != "" {
		return d.URL
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.Name, d.SSLMode,
	)
}

// Log holds logger configuration.
type Log struct {
	Level  string `env:"LOG_LEVEL"  envDefault:"info"` // debug | info | warn | error
	Format string `env:"LOG_FORMAT" envDefault:"json"` // json | text
}

// Load reads configuration from the process environment. If a .env file
// is present in the working directory it is loaded first (non-fatal if
// missing — useful only in local development).
func Load() (Config, error) {
	_ = godotenv.Load() // best-effort; not an error if missing

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse env: %w", err)
	}
	return cfg, nil
}

// IsProduction returns true when running in production.
func (c Config) IsProduction() bool {
	return c.App.Environment == "production"
}
