package aggregator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	crm "github.com/shawn/jobhunttask/internal/crm/model"
)

// Source fetches normalized job listings from an external board.
type Source interface {
	Name() string
	Fetch(ctx context.Context) ([]crm.RawJob, error)
}

// Registry runs all configured sources.
type Registry struct {
	sources []Source
	log     *slog.Logger
	client  *http.Client
}

// CollectResult holds raw jobs plus per-source diagnostics.
type CollectResult struct {
	Jobs          []crm.RawJob
	Sources       []SourceFetchStats
	FilteredOut   int
}

// SourceFetchStats counts fetch outcomes for one board.
type SourceFetchStats struct {
	Source   string
	Fetched  int
	Filtered int
	Error    string
}

func New(log *slog.Logger) *Registry {
	r := &Registry{
		log: log,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	r.sources = []Source{
		&remoteOK{client: r.client},
		&web3Career{client: r.client},
		&cryptoJobsList{client: r.client},
		&greenhouseBoards{client: r.client},
		&leverBoards{client: r.client},
		&ashbyBoards{client: r.client},
	}
	return r
}

func (r *Registry) Sources() []string {
	names := make([]string, len(r.sources))
	for i, s := range r.sources {
		names[i] = s.Name()
	}
	return names
}

func (r *Registry) CollectAll(ctx context.Context) ([]crm.RawJob, error) {
	res, err := r.CollectDetailed(ctx)
	if err != nil {
		return nil, err
	}
	return res.Jobs, nil
}

func (r *Registry) CollectDetailed(ctx context.Context) (*CollectResult, error) {
	var all []crm.RawJob
	var stats []SourceFetchStats
	for _, src := range r.sources {
		st := SourceFetchStats{Source: src.Name()}
		jobs, err := src.Fetch(ctx)
		if err != nil {
			st.Error = err.Error()
			r.log.Warn("job source fetch failed",
				slog.String("source", src.Name()),
				slog.String("error", err.Error()),
			)
			stats = append(stats, st)
			continue
		}
		st.Fetched = len(jobs)
		r.log.Info("jobs collected", slog.String("source", src.Name()), slog.Int("count", len(jobs)))
		all = append(all, jobs...)
		stats = append(stats, st)
	}
	before := len(all)
	filtered := filterRelevant(all)
	out := &CollectResult{
		Jobs:        dedupeRawJobs(filtered),
		Sources:     stats,
		FilteredOut: before - len(filtered),
	}
	return out, nil
}

func dedupeRawJobs(jobs []crm.RawJob) []crm.RawJob {
	seenKey := map[string]struct{}{}
	seenURL := map[string]struct{}{}
	out := make([]crm.RawJob, 0, len(jobs))
	for _, j := range jobs {
		key := strings.ToLower(j.Source + ":" + j.ExternalID)
		if _, ok := seenKey[key]; ok {
			continue
		}
		seenKey[key] = struct{}{}
		url := strings.TrimSpace(strings.ToLower(j.ApplicationURL))
		if url != "" {
			if _, ok := seenURL[url]; ok {
				continue
			}
			seenURL[url] = struct{}{}
		}
		out = append(out, j)
	}
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var backendPattern = regexp.MustCompile(`(?i)(backend|platform|infrastructure|distributed|staff|senior|engineer|go\b|golang|java|kafka|web3|blockchain|protocol)`)

func filterRelevant(jobs []crm.RawJob) []crm.RawJob {
	out := make([]crm.RawJob, 0, len(jobs))
	seen := make(map[string]struct{})
	for _, j := range jobs {
		key := strings.ToLower(j.Source + ":" + j.ExternalID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		text := j.Title + " " + j.Description
		if !backendPattern.MatchString(text) {
			continue
		}
		out = append(out, j)
	}
	return out
}

// ---------------------------------------------------------------------------
// RemoteOK — public JSON API
// ---------------------------------------------------------------------------

type remoteOK struct{ client *http.Client }

func (s *remoteOK) Name() string { return "remoteok" }

func (s *remoteOK) Fetch(ctx context.Context) ([]crm.RawJob, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://remoteok.com/api", nil)
	req.Header.Set("User-Agent", "jobhunt-crm/1.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	var rows []map[string]any
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, err
	}
	var out []crm.RawJob
	for _, row := range rows {
		id := fmt.Sprint(row["id"])
		if id == "" || id == "<nil>" {
			continue
		}
		title := fmt.Sprint(row["position"])
		if title == "" {
			title = fmt.Sprint(row["title"])
		}
		desc := fmt.Sprint(row["description"])
		tags := toStringSlice(row["tags"])
		skills := append(tags, extractSkills(desc)...)
		company := fmt.Sprint(row["company"])
		url := fmt.Sprint(row["url"])
		if url == "" {
			url = fmt.Sprint(row["apply_url"])
		}
		salary := fmt.Sprint(row["salary"])
		var posted *time.Time
		if epoch, ok := row["epoch"].(float64); ok && epoch > 0 {
			t := time.Unix(int64(epoch), 0).UTC()
			posted = &t
		}
		out = append(out, crm.RawJob{
			ExternalID:     id,
			Source:         s.Name(),
			Title:          title,
			CompanyName:    company,
			SalaryRaw:      salary,
			Location:       "Remote",
			Remote:         true,
			Description:    desc,
			RequiredSkills: skills,
			ApplicationURL: url,
			PostedAt:       posted,
			Seniority:      inferSeniority(title),
			Web3:           isWeb3(title + " " + desc + " " + strings.Join(tags, " ")),
			RawPayload:     row,
		})
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Web3 Career — HTML/API hybrid (RSS-style public listings)
// ---------------------------------------------------------------------------

type web3Career struct{ client *http.Client }

func (s *web3Career) Name() string { return "web3_career" }

func (s *web3Career) Fetch(ctx context.Context) ([]crm.RawJob, error) {
	token := os.Getenv("WEB3_CAREER_API_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("WEB3_CAREER_API_TOKEN not set — request a free key at https://web3.career/web3-jobs-api")
	}
	return fetchJSONJobs(ctx, s.client, s.Name(),
		fmt.Sprintf("https://web3.career/api/v1/jobs?remote=true&limit=50&token=%s", token),
		func(row map[string]any) (crm.RawJob, bool) {
			id := fmt.Sprint(row["id"])
			if id == "" {
				id = fmt.Sprint(row["job_id"])
			}
			title := fmt.Sprint(row["title"])
			if id == "" || title == "" {
				return crm.RawJob{}, false
			}
			desc := fmt.Sprint(row["description"])
			if desc == "" {
				desc = fmt.Sprint(row["body"])
			}
			loc := fmt.Sprint(row["location"])
			url := fmt.Sprint(row["url"])
			if url == "" {
				url = fmt.Sprint(row["apply_url"])
			}
			return crm.RawJob{
				ExternalID:     id,
				Source:         s.Name(),
				Title:          title,
				CompanyName:    fmt.Sprint(row["company"]),
				Location:       loc,
				Remote:         strings.Contains(strings.ToLower(loc), "remote") || row["remote"] == true,
				Description:    desc,
				RequiredSkills: extractSkills(desc),
				ApplicationURL: url,
				Seniority:      inferSeniority(title),
				Web3:           true,
				RawPayload:     row,
			}, true
		})
}

// ---------------------------------------------------------------------------
// CryptoJobsList
// ---------------------------------------------------------------------------

type cryptoJobsList struct{ client *http.Client }

func (s *cryptoJobsList) Name() string { return "cryptojobslist" }

func (s *cryptoJobsList) Fetch(ctx context.Context) ([]crm.RawJob, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.cryptojobslist.com/v1/jobs?remote=1&limit=50", nil)
	req.Header.Set("User-Agent", "jobhunt-crm/1.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cryptojobslist: status %d (API may block automated access)", resp.StatusCode)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	var envelope struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, nil
	}
	var out []crm.RawJob
	for _, row := range envelope.Data {
		id := fmt.Sprint(row["id"])
		title := fmt.Sprint(row["title"])
		if id == "" || title == "" {
			continue
		}
		desc := fmt.Sprint(row["description"])
		out = append(out, crm.RawJob{
			ExternalID:     id,
			Source:         s.Name(),
			Title:          title,
			CompanyName:    fmt.Sprint(row["company_name"]),
			Location:       fmt.Sprint(row["location"]),
			Remote:         true,
			Description:    desc,
			RequiredSkills: extractSkills(desc),
			ApplicationURL: fmt.Sprint(row["url"]),
			Seniority:      inferSeniority(title),
			Web3:           true,
			RawPayload:     row,
		})
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// ATS board fetchers (Greenhouse, Lever, Ashby) — configured company slugs
// ---------------------------------------------------------------------------

type greenhouseBoards struct{ client *http.Client }

func (s *greenhouseBoards) Name() string { return "greenhouse" }

var greenhouseSlugs = []string{"gitlab", "hashicorp", "cloudflare", "stripe", "figma"}

func (s *greenhouseBoards) Fetch(ctx context.Context) ([]crm.RawJob, error) {
	return fetchATSBoards(ctx, s.client, s.Name(), greenhouseSlugs, func(slug string) string {
		return fmt.Sprintf("https://boards-api.greenhouse.io/v1/boards/%s/jobs", slug)
	}, parseGreenhouse)
}

type leverBoards struct{ client *http.Client }

func (s *leverBoards) Name() string { return "lever" }

var leverSlugs = []string{"spotify", "palantir", "netflix", "plaid", "notion"}

func (s *leverBoards) Fetch(ctx context.Context) ([]crm.RawJob, error) {
	return fetchATSBoards(ctx, s.client, s.Name(), leverSlugs, func(slug string) string {
		return fmt.Sprintf("https://api.lever.co/v0/postings/%s?mode=json", slug)
	}, parseLever)
}

type ashbyBoards struct{ client *http.Client }

func (s *ashbyBoards) Name() string { return "ashby" }

var ashbySlugs = []string{"linear", "ramp", "openai"}

func (s *ashbyBoards) Fetch(ctx context.Context) ([]crm.RawJob, error) {
	return fetchATSBoards(ctx, s.client, s.Name(), ashbySlugs, func(slug string) string {
		return fmt.Sprintf("https://api.ashbyhq.com/posting-api/job-board/%s", slug)
	}, parseAshby)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type parseFn func(slug string, body []byte) []crm.RawJob

func fetchATSBoards(ctx context.Context, client *http.Client, source string, slugs []string, urlFn func(string) string, parse parseFn) ([]crm.RawJob, error) {
	var out []crm.RawJob
	for _, slug := range slugs {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, urlFn(slug), nil)
		req.Header.Set("User-Agent", "jobhunt-crm/1.0")
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			continue
		}
		out = append(out, parse(slug, body)...)
	}
	return out, nil
}

func parseGreenhouse(slug string, body []byte) []crm.RawJob {
	var envelope struct {
		Jobs []struct {
			ID          int    `json:"id"`
			Title       string `json:"title"`
			Location    struct{ Name string `json:"name"` } `json:"location"`
			AbsoluteURL string `json:"absolute_url"`
			Content     string `json:"content"`
			UpdatedAt   string `json:"updated_at"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil
	}
	var out []crm.RawJob
	for _, j := range envelope.Jobs {
		loc := j.Location.Name
		remote := strings.Contains(strings.ToLower(loc), "remote")
		var posted *time.Time
		if t, err := time.Parse(time.RFC3339, j.UpdatedAt); err == nil {
			posted = &t
		}
		out = append(out, crm.RawJob{
			ExternalID:     fmt.Sprintf("%s-%d", slug, j.ID),
			Source:         "greenhouse",
			Title:          j.Title,
			CompanyName:    strings.Title(slug),
			Location:       loc,
			Remote:         remote,
			Description:    stripHTML(j.Content),
			RequiredSkills: extractSkills(j.Content),
			ApplicationURL: j.AbsoluteURL,
			PostedAt:       posted,
			Seniority:      inferSeniority(j.Title),
			Web3:           isWeb3(j.Title + " " + j.Content),
		})
	}
	return out
}

func parseLever(slug string, body []byte) []crm.RawJob {
	var rows []map[string]any
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil
	}
	var out []crm.RawJob
	for _, row := range rows {
		id := fmt.Sprint(row["id"])
		title := fmt.Sprint(row["text"])
		desc := fmt.Sprint(row["descriptionPlain"])
		categories, _ := row["categories"].(map[string]any)
		loc := ""
		if categories != nil {
			loc = fmt.Sprint(categories["location"])
		}
		out = append(out, crm.RawJob{
			ExternalID:     slug + "-" + id,
			Source:         "lever",
			Title:          title,
			CompanyName:    strings.Title(slug),
			Location:       loc,
			Remote:         strings.Contains(strings.ToLower(loc), "remote"),
			Description:    desc,
			RequiredSkills: extractSkills(desc),
			ApplicationURL: fmt.Sprint(row["hostedUrl"]),
			Seniority:      inferSeniority(title),
			Web3:           isWeb3(title + " " + desc),
			RawPayload:     row,
		})
	}
	return out
}

func parseAshby(slug string, body []byte) []crm.RawJob {
	var envelope struct {
		Jobs []struct {
			ID             string `json:"id"`
			Title          string `json:"title"`
			Location       string `json:"location"`
			DescriptionHTML string `json:"descriptionHtml"`
			JobURL         string `json:"jobUrl"`
			IsRemote       bool   `json:"isRemote"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil
	}
	var out []crm.RawJob
	for _, j := range envelope.Jobs {
		desc := stripHTML(j.DescriptionHTML)
		out = append(out, crm.RawJob{
			ExternalID:     slug + "-" + j.ID,
			Source:         "ashby",
			Title:          j.Title,
			CompanyName:    strings.Title(slug),
			Location:       j.Location,
			Remote:         j.IsRemote || strings.Contains(strings.ToLower(j.Location), "remote"),
			Description:    desc,
			RequiredSkills: extractSkills(desc),
			ApplicationURL: j.JobURL,
			Seniority:      inferSeniority(j.Title),
			Web3:           isWeb3(j.Title + " " + desc),
		})
	}
	return out
}

func fetchJSONJobs(ctx context.Context, client *http.Client, source, url string, mapFn func(map[string]any) (crm.RawJob, bool)) ([]crm.RawJob, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("User-Agent", "jobhunt-crm/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: status %d", source, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Jobs []map[string]any `json:"jobs"`
		Data []map[string]any `json:"data"`
	}
	_ = json.Unmarshal(body, &envelope)
	rows := envelope.Jobs
	if len(rows) == 0 {
		rows = envelope.Data
	}
	if len(rows) == 0 {
		var flat []map[string]any
		if json.Unmarshal(body, &flat) == nil {
			rows = flat
		}
	}
	var out []crm.RawJob
	for _, row := range rows {
		if j, ok := mapFn(row); ok {
			j.Source = source
			out = append(out, j)
		}
	}
	return out, nil
}

var skillKeywords = []string{
	"go", "golang", "java", "kafka", "sql", "postgresql", "mysql", "redis",
	"kubernetes", "k8s", "docker", "aws", "gcp", "azure", "terraform",
	"grpc", "distributed systems", "microservices", "ci/cd", "rust", "python",
	"blockchain", "web3", "solidity", "ethereum", "defi",
}

var skillRe = regexp.MustCompile(`(?i)\b(` + strings.Join(skillKeywords, "|") + `)\b`)

func extractSkills(text string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, m := range skillRe.FindAllString(text, -1) {
		norm := normalizeSkill(m)
		if _, ok := seen[norm]; ok {
			continue
		}
		seen[norm] = struct{}{}
		out = append(out, norm)
	}
	return out
}

func normalizeSkill(s string) string {
	s = strings.TrimSpace(s)
	lower := strings.ToLower(s)
	switch lower {
	case "golang":
		return "Go"
	case "k8s":
		return "Kubernetes"
	case "postgresql", "mysql":
		return "SQL"
	default:
		return strings.Title(lower)
	}
}

func inferSeniority(title string) string {
	t := strings.ToLower(title)
	switch {
	case strings.Contains(t, "staff") || strings.Contains(t, "principal"):
		return "staff"
	case strings.Contains(t, "senior") || strings.Contains(t, "sr."):
		return "senior"
	case strings.Contains(t, "lead"):
		return "lead"
	case strings.Contains(t, "junior") || strings.Contains(t, "entry"):
		return "junior"
	default:
		return "mid"
	}
}

func isWeb3(text string) bool {
	t := strings.ToLower(text)
	keywords := []string{"web3", "blockchain", "crypto", "defi", "ethereum", "solidity", "protocol", "wallet", "nft"}
	for _, k := range keywords {
		if strings.Contains(t, k) {
			return true
		}
	}
	return false
}

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

func stripHTML(s string) string {
	return htmlTagRe.ReplaceAllString(s, " ")
}

func toStringSlice(v any) []string {
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			out = append(out, fmt.Sprint(x))
		}
		return out
	case []string:
		return t
	default:
		return nil
	}
}
