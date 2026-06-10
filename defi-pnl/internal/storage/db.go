package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/qustavo/sqlhooks/v2"
)

var DB *sql.DB

// sqlLogHooks is a sqlhooks.Hooks implementation that logs every query
// (with args, stmt timing, and any error) via the standard `log` package.
//
// The hook is installed by registering a wrapped driver "postgres-logged"
// in init(); enable/disable with the SQL_LOG env var (default: enabled
// during development, set SQL_LOG=0 to silence).
type sqlLogHooks struct{}

type ctxKey struct{}

var startKey = ctxKey{}

func (sqlLogHooks) Before(ctx context.Context, query string, args ...interface{}) (context.Context, error) {
	return context.WithValue(ctx, startKey, time.Now()), nil
}

func (sqlLogHooks) After(ctx context.Context, query string, args ...interface{}) (context.Context, error) {
	logQuery(ctx, query, args, nil)
	return ctx, nil
}

func (sqlLogHooks) OnError(ctx context.Context, err error, query string, args ...interface{}) error {
	logQuery(ctx, query, args, err)
	return err
}

// logQuery emits a single line per executed statement. Multi-line / heavily
// indented queries (we have a lot of those, e.g. pnl_leaderboard's CTE) are
// collapsed to single-line form so the log stays scannable. Args are listed
// after the query so you can copy-paste straight into psql with substitutions.
func logQuery(ctx context.Context, query string, args []interface{}, err error) {
	dur := time.Duration(0)
	if start, ok := ctx.Value(startKey).(time.Time); ok {
		dur = time.Since(start)
	}
	flat := strings.Join(strings.Fields(query), " ")
	if err != nil {
		log.Printf("sql: ERROR (%v) [%s] args=%v err=%v", dur, flat, args, err)
		return
	}
	if len(args) == 0 {
		log.Printf("sql: (%v) %s", dur, flat)
		return
	}
	log.Printf("sql: (%v) %s args=%v", dur, flat, args)
}

func init() {
	sql.Register("postgres-logged", sqlhooks.Wrap(&pq.Driver{}, sqlLogHooks{}))
}

// Init opens Postgres from DB_URL (e.g. in .env) and verifies the server accepts connections.
// Set SQL_LOG=0 to suppress per-query logging.
func Init() error {
	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		return fmt.Errorf("DB_URL is empty: set it in .env (e.g. postgresql://user:pass@localhost:5432/dbname?sslmode=disable)")
	}

	driver := "postgres-logged"
	if os.Getenv("SQL_LOG") == "0" {
		driver = "postgres"
	}

	var err error
	DB, err = sql.Open(driver, dsn)
	if err != nil {
		return fmt.Errorf("sql.Open: %w", err)
	}
	if err := DB.Ping(); err != nil {
		_ = DB.Close()
		DB = nil
		return fmt.Errorf("database ping failed (check DB_URL, host, port, db name, user/password): %w", err)
	}
	log.Printf("database: connected to %s", dbTarget(dsn))
	return nil
}

// dbTarget returns host:port/db from a Postgres URL for startup logs (no password).
func dbTarget(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return "(invalid DB_URL)"
	}
	hostname := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "5432"
	}
	db := strings.TrimPrefix(u.Path, "/")
	if db == "" {
		db = "?"
	}
	target := hostname + ":" + port + "/" + db
	switch hostname {
	case "postgres":
		return target + " (Docker Compose — not your host localhost:5432)"
	case "localhost", "127.0.0.1":
		return target + " (host machine)"
	default:
		return target
	}
}
