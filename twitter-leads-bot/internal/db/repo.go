// Package db is the persistence boundary for the bot. Handlers and the
// pipeline depend on the Repository interface, never on lib/pq directly, so
// the rest of the app stays mockable.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	_ "github.com/lib/pq"

	"github.com/shawn/twitter-leads-bot/internal/models"
)

type Repository interface {
	// Keywords
	ListKeywords(ctx context.Context, enabledOnly bool) ([]*models.Keyword, error)
	AddKeyword(ctx context.Context, kw string) (*models.Keyword, error)
	UpdateKeyword(ctx context.Context, id int64, kw string) error
	DeleteKeyword(ctx context.Context, id int64) error
	DeleteKeywords(ctx context.Context, ids []int64) error
	SetKeywordEnabled(ctx context.Context, id int64, enabled bool) error
	TouchKeywordSearched(ctx context.Context, kw string) error
	GetKeyword(ctx context.Context, id int64) (*models.Keyword, error)

	// Tweets
	TweetExists(ctx context.Context, tweetID string) (bool, error)
	InsertTweet(ctx context.Context, t *models.ProcessedTweet) error

	// Analyses
	UpsertAnalysis(ctx context.Context, a *models.Analysis) error
	UpdateAnalysisReply(ctx context.Context, tweetID, reply string) error
	UpdateAnalysisStatus(ctx context.Context, tweetID, status string) error

	// Dashboard reads
	// Dashboard reads. Pass empty keyword to include all keywords.
	// sort: "" (default newest), "score", "newest" / "time_desc", "oldest" / "time_asc".
	ListLeads(ctx context.Context, status string, minScore int, keyword string, sort string, limit int) ([]*models.LeadView, error)
	GetLead(ctx context.Context, tweetID string) (*models.LeadView, error)

	Close() error
}

type postgresRepo struct {
	db *sql.DB
}

func New(dsn string) (Repository, error) {
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &postgresRepo{db: conn}, nil
}

func (r *postgresRepo) Close() error { return r.db.Close() }

// ---- keywords ---------------------------------------------------------------

const keywordCols = `id, keyword, enabled, last_searched_at, created_at`

func (r *postgresRepo) ListKeywords(ctx context.Context, enabledOnly bool) ([]*models.Keyword, error) {
	// LEFT JOIN to count tweets per keyword in a single round-trip; cheap
	// enough at our scale that we don't bother caching it.
	q := `
		SELECT k.id, k.keyword, k.enabled, k.last_searched_at, k.created_at,
		       COALESCE(COUNT(t.tweet_id), 0)
		FROM search_keywords k
		LEFT JOIN processed_tweets t ON t.keyword = k.keyword
	`
	if enabledOnly {
		q += ` WHERE k.enabled = TRUE`
	}
	q += ` GROUP BY k.id ORDER BY k.created_at DESC`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []*models.Keyword{}
	for rows.Next() {
		k := &models.Keyword{}
		if err := rows.Scan(&k.ID, &k.Keyword, &k.Enabled, &k.LastSearchedAt, &k.CreatedAt, &k.TweetCount); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (r *postgresRepo) AddKeyword(ctx context.Context, kw string) (*models.Keyword, error) {
	const q = `
		INSERT INTO search_keywords (keyword) VALUES ($1)
		RETURNING ` + keywordCols
	k := &models.Keyword{}
	err := r.db.QueryRowContext(ctx, q, kw).Scan(&k.ID, &k.Keyword, &k.Enabled, &k.LastSearchedAt, &k.CreatedAt)
	return k, err
}

func (r *postgresRepo) UpdateKeyword(ctx context.Context, id int64, kw string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE search_keywords SET keyword = $1 WHERE id = $2`, kw, id)
	return err
}

func (r *postgresRepo) DeleteKeyword(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM search_keywords WHERE id = $1`, id)
	return err
}

func (r *postgresRepo) DeleteKeywords(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	q := fmt.Sprintf(`DELETE FROM search_keywords WHERE id IN (%s)`, strings.Join(placeholders, ","))
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *postgresRepo) SetKeywordEnabled(ctx context.Context, id int64, enabled bool) error {
	_, err := r.db.ExecContext(ctx, `UPDATE search_keywords SET enabled = $1 WHERE id = $2`, enabled, id)
	return err
}

func (r *postgresRepo) TouchKeywordSearched(ctx context.Context, kw string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE search_keywords SET last_searched_at = NOW() WHERE keyword = $1`, kw)
	return err
}

func (r *postgresRepo) GetKeyword(ctx context.Context, id int64) (*models.Keyword, error) {
	const q = `SELECT ` + keywordCols + ` FROM search_keywords WHERE id = $1`
	k := &models.Keyword{}
	err := r.db.QueryRowContext(ctx, q, id).Scan(&k.ID, &k.Keyword, &k.Enabled, &k.LastSearchedAt, &k.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return k, err
}

// ---- tweets -----------------------------------------------------------------

func (r *postgresRepo) TweetExists(ctx context.Context, id string) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT 1 FROM processed_tweets WHERE tweet_id = $1`, id).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *postgresRepo) InsertTweet(ctx context.Context, t *models.ProcessedTweet) error {
	const q = `
		INSERT INTO processed_tweets (tweet_id, keyword, username, text, likes, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (tweet_id) DO NOTHING`
	_, err := r.db.ExecContext(ctx, q, t.TweetID, t.Keyword, t.Username, t.Text, t.Likes, t.CreatedAt)
	return err
}

// ---- analyses ---------------------------------------------------------------

func (r *postgresRepo) UpsertAnalysis(ctx context.Context, a *models.Analysis) error {
	const q = `
		INSERT INTO lead_analysis (tweet_id, score, reason, reply_suggestion, status, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (tweet_id) DO UPDATE SET
			score = EXCLUDED.score,
			reason = EXCLUDED.reason,
			reply_suggestion = EXCLUDED.reply_suggestion,
			status = EXCLUDED.status,
			updated_at = NOW()`
	_, err := r.db.ExecContext(ctx, q, a.TweetID, a.Score, a.Reason, a.ReplySuggestion, a.Status)
	return err
}

func (r *postgresRepo) UpdateAnalysisReply(ctx context.Context, tweetID, reply string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE lead_analysis SET reply_suggestion = $1, updated_at = NOW() WHERE tweet_id = $2`,
		reply, tweetID)
	return err
}

func (r *postgresRepo) UpdateAnalysisStatus(ctx context.Context, tweetID, status string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE lead_analysis SET status = $1, updated_at = NOW() WHERE tweet_id = $2`,
		status, tweetID)
	return err
}

// ---- dashboard reads --------------------------------------------------------

const leadCols = `
	t.tweet_id, t.keyword, t.username, t.text, t.likes, t.created_at, t.seen_at,
	a.score, a.reason, a.reply_suggestion, a.status, a.created_at, a.updated_at`

func scanLead(scanner interface{ Scan(...any) error }) (*models.LeadView, error) {
	v := &models.LeadView{}
	v.Analysis.TweetID = "" // filled below; same as tweet id
	err := scanner.Scan(
		&v.Tweet.TweetID, &v.Tweet.Keyword, &v.Tweet.Username, &v.Tweet.Text,
		&v.Tweet.Likes, &v.Tweet.CreatedAt, &v.Tweet.SeenAt,
		&v.Analysis.Score, &v.Analysis.Reason, &v.Analysis.ReplySuggestion,
		&v.Analysis.Status, &v.Analysis.CreatedAt, &v.Analysis.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	v.Analysis.TweetID = v.Tweet.TweetID
	return v, nil
}

func leadSortOrder(sort string) string {
	switch strings.ToLower(strings.TrimSpace(sort)) {
	case "score":
		return "score"
	case "newest", "time_desc":
		return "newest"
	case "oldest", "time_asc":
		return "oldest"
	default:
		return "newest"
	}
}

func (r *postgresRepo) ListLeads(ctx context.Context, status string, minScore int, keyword string, sort string, limit int) ([]*models.LeadView, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	n := 1
	args := []any{}
	var b strings.Builder
	b.WriteString(`SELECT ` + leadCols + `
		FROM processed_tweets t JOIN lead_analysis a ON a.tweet_id = t.tweet_id
		WHERE a.score >= $`)
	b.WriteString(strconv.Itoa(n))
	n++
	args = append(args, minScore)
	if strings.TrimSpace(keyword) != "" {
		b.WriteString(` AND t.keyword = $`)
		b.WriteString(strconv.Itoa(n))
		n++
		args = append(args, strings.TrimSpace(keyword))
	}
	if status != "" && status != "all" {
		b.WriteString(` AND a.status = $`)
		b.WriteString(strconv.Itoa(n))
		n++
		args = append(args, status)
	}
	b.WriteString(` ORDER BY `)
	switch leadSortOrder(sort) {
	case "newest":
		b.WriteString(`t.created_at DESC, a.score DESC`)
	case "oldest":
		b.WriteString(`t.created_at ASC, a.score DESC`)
	default:
		b.WriteString(`a.score DESC, a.updated_at DESC`)
	}
	b.WriteString(` LIMIT $`)
	b.WriteString(strconv.Itoa(n))
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, b.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []*models.LeadView{}
	for rows.Next() {
		v, err := scanLead(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *postgresRepo) GetLead(ctx context.Context, tweetID string) (*models.LeadView, error) {
	const q = `
		SELECT ` + leadCols + `
		FROM processed_tweets t JOIN lead_analysis a ON a.tweet_id = t.tweet_id
		WHERE t.tweet_id = $1`
	row := r.db.QueryRowContext(ctx, q, tweetID)
	v, err := scanLead(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return v, err
}
