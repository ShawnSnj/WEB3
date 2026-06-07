// Package web wires HTTP handlers to the database, pipeline, and LLM deps.
//
// Handlers are thin: they validate input, delegate to the right collaborator,
// and render either a full page (Tailwind layout) or an HTMX partial.
package web

import (
	"context"
	"embed"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/shawn/twitter-leads-bot/internal/db"
	"github.com/shawn/twitter-leads-bot/internal/models"
	"github.com/shawn/twitter-leads-bot/internal/pipeline"
)

//go:embed templates/*.html
var templatesFS embed.FS

// Server is the HTTP layer. All collaborators are interfaces so handlers stay
// testable; main.go does the concrete wiring.
type Server struct {
	repo            db.Repository
	pipe            *pipeline.Pipeline
	poster          pipeline.Poster
	logger          *slog.Logger
	replyViaWebhook bool
	tmpl            *template.Template
}

func NewServer(repo db.Repository, pipe *pipeline.Pipeline, poster pipeline.Poster, logger *slog.Logger, replyViaWebhook bool, displayLoc *time.Location) (*Server, error) {
	if displayLoc == nil {
		displayLoc, _ = time.LoadLocation("Asia/Taipei")
	}
	funcs := template.FuncMap{
		// fmtLocal renders an instant in the configured dashboard timezone (default Taiwan).
		"fmtLocal": func(t time.Time) string {
			if t.IsZero() {
				return "—"
			}
			return t.In(displayLoc).Format("Jan 2 15:04 MST")
		},
	}
	tmpl, err := template.New("").Funcs(funcs).ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Server{
		repo:            repo,
		pipe:            pipe,
		poster:          poster,
		logger:          logger,
		replyViaWebhook: replyViaWebhook,
		tmpl:            tmpl,
	}, nil
}

func (s *Server) Routes() http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/", s.handleHome).Methods(http.MethodGet)
	r.HandleFunc("/all", s.handleAllLeads).Methods(http.MethodGet)
	r.HandleFunc("/leads", s.handleListLeads).Methods(http.MethodGet)
	r.HandleFunc("/k/{kid:[0-9]+}/review", s.handleKeywordReview).Methods(http.MethodGet)
	r.HandleFunc("/k/{kid:[0-9]+}/leads", s.handleKeywordLeads).Methods(http.MethodGet)

	r.HandleFunc("/leads/{id}/approve", s.handleApprove).Methods(http.MethodPost)
	r.HandleFunc("/leads/{id}/regenerate", s.handleRegenerate).Methods(http.MethodPost)
	r.HandleFunc("/leads/{id}/skip", s.handleSkip).Methods(http.MethodPost)

	r.HandleFunc("/keywords", s.handleKeywordsPage).Methods(http.MethodGet)
	r.HandleFunc("/keywords", s.handleAddKeyword).Methods(http.MethodPost)
	r.HandleFunc("/keywords/{id:[0-9]+}", s.handleUpdateKeyword).Methods(http.MethodPut)
	r.HandleFunc("/keywords/{id:[0-9]+}", s.handleDeleteKeyword).Methods(http.MethodDelete)
	r.HandleFunc("/keywords/{id:[0-9]+}/toggle", s.handleToggleKeyword).Methods(http.MethodPost)
	r.HandleFunc("/keywords/{id:[0-9]+}/run", s.handleRunKeyword).Methods(http.MethodPost)

	return s.logging(r)
}

// ---- page data --------------------------------------------------------------

type pageData struct {
	Page            string
	Active          string
	Now             time.Time
	Leads           []*models.LeadView
	Keywords        []*models.Keyword
	SelectedKeyword *models.Keyword
	ReplyViaWebhook bool
}

func (s *Server) basePage() pageData {
	return pageData{
		Now:             time.Now(),
		ReplyViaWebhook: s.replyViaWebhook,
	}
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	kws, err := s.repo.ListKeywords(r.Context(), false)
	if err != nil {
		s.serverError(w, err)
		return
	}
	d := s.basePage()
	d.Page = "hub"
	d.Active = "home"
	d.Keywords = kws
	s.renderPage(w, d)
}

func (s *Server) handleAllLeads(w http.ResponseWriter, r *http.Request) {
	leads, err := s.repo.ListLeads(r.Context(), "all", 0, "", "newest", 60)
	if err != nil {
		s.serverError(w, err)
		return
	}
	d := s.basePage()
	d.Page = "all"
	d.Active = "all"
	d.Leads = leads
	s.renderPage(w, d)
}

func (s *Server) handleKeywordReview(w http.ResponseWriter, r *http.Request) {
	k, err := s.keywordByKid(r)
	if err != nil {
		s.serverError(w, err)
		return
	}
	if k == nil {
		http.NotFound(w, r)
		return
	}
	// Show everything by default — new leads stay at score 0 until “New suggestion”.
	// and would be hidden if we defaulted to pending + min score 5.
	leads, err := s.repo.ListLeads(r.Context(), "all", 0, k.Keyword, "newest", 60)
	if err != nil {
		s.serverError(w, err)
		return
	}
	d := s.basePage()
	d.Page = "review"
	d.Active = "review"
	d.SelectedKeyword = k
	d.Leads = leads
	s.renderPage(w, d)
}

func (s *Server) handleKeywordLeads(w http.ResponseWriter, r *http.Request) {
	k, err := s.keywordByKid(r)
	if err != nil {
		s.serverError(w, err)
		return
	}
	if k == nil {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	status := q.Get("status")
	if status == "" {
		status = "all"
	}
	minScore, _ := strconv.Atoi(q.Get("min_score"))
	sort := q.Get("sort")
	leads, err := s.repo.ListLeads(r.Context(), status, minScore, k.Keyword, sort, 60)
	if err != nil {
		s.serverError(w, err)
		return
	}
	d := s.basePage()
	d.SelectedKeyword = k
	d.Leads = leads
	s.renderPartial(w, "leads", d)
}

func (s *Server) keywordByKid(r *http.Request) (*models.Keyword, error) {
	kid, err := strconv.ParseInt(mux.Vars(r)["kid"], 10, 64)
	if err != nil || kid <= 0 {
		return nil, nil
	}
	return s.repo.GetKeyword(r.Context(), kid)
}

func (s *Server) handleListLeads(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := q.Get("status")
	if status == "" {
		status = "all"
	}
	minScore, _ := strconv.Atoi(q.Get("min_score"))
	keyword := strings.TrimSpace(q.Get("keyword"))
	sort := q.Get("sort")
	leads, err := s.repo.ListLeads(r.Context(), status, minScore, keyword, sort, 60)
	if err != nil {
		s.serverError(w, err)
		return
	}
	d := s.basePage()
	d.Leads = leads
	s.renderPartial(w, "leads", d)
}

func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	lead := s.requireLead(w, r)
	if lead == nil {
		return
	}
	if lead.Analysis.ReplySuggestion == "" {
		http.Error(w, "no reply suggestion to send", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if err := s.poster.Reply(ctx, lead.Tweet.TweetID, lead.Analysis.ReplySuggestion); err != nil {
		s.serverError(w, err)
		return
	}
	if err := s.repo.UpdateAnalysisStatus(ctx, lead.Tweet.TweetID, models.StatusSent); err != nil {
		s.serverError(w, err)
		return
	}
	lead.Analysis.Status = models.StatusSent
	s.renderPartial(w, "lead_card", lead)
}

func (s *Server) handleRegenerate(w http.ResponseWriter, r *http.Request) {
	lead := s.requireLead(w, r)
	if lead == nil {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()

	if err := s.pipe.RunLeadAnalysis(ctx, lead.Tweet.TweetID, lead.Tweet.Text); err != nil {
		http.Error(w, "analysis failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	lead, err := s.repo.GetLead(ctx, lead.Tweet.TweetID)
	if err != nil {
		s.serverError(w, err)
		return
	}
	if lead == nil {
		http.Error(w, "lead not found after analysis", http.StatusNotFound)
		return
	}
	s.renderPartial(w, "lead_card", lead)
}

func (s *Server) handleSkip(w http.ResponseWriter, r *http.Request) {
	lead := s.requireLead(w, r)
	if lead == nil {
		return
	}
	if err := s.repo.UpdateAnalysisStatus(r.Context(), lead.Tweet.TweetID, models.StatusSkipped); err != nil {
		s.serverError(w, err)
		return
	}
	lead.Analysis.Status = models.StatusSkipped
	s.renderPartial(w, "lead_card", lead)
}

func (s *Server) handleKeywordsPage(w http.ResponseWriter, r *http.Request) {
	kws, err := s.repo.ListKeywords(r.Context(), false)
	if err != nil {
		s.serverError(w, err)
		return
	}
	d := s.basePage()
	d.Page = "keywords"
	d.Active = "keywords"
	d.Keywords = kws
	s.renderPage(w, d)
}

func (s *Server) handleAddKeyword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	kw := strings.TrimSpace(r.FormValue("keyword"))
	if kw == "" {
		http.Error(w, "keyword required", http.StatusBadRequest)
		return
	}
	if _, err := s.repo.AddKeyword(r.Context(), kw); err != nil {
		s.serverError(w, err)
		return
	}
	s.renderKeywordList(w, r)
}

func (s *Server) handleUpdateKeyword(w http.ResponseWriter, r *http.Request) {
	id := parseID(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	kw := strings.TrimSpace(r.FormValue("keyword"))
	if kw == "" {
		http.Error(w, "keyword required", http.StatusBadRequest)
		return
	}
	if err := s.repo.UpdateKeyword(r.Context(), id, kw); err != nil {
		s.serverError(w, err)
		return
	}
	s.renderKeywordRow(w, r, id)
}

func (s *Server) handleDeleteKeyword(w http.ResponseWriter, r *http.Request) {
	id := parseID(r)
	if err := s.repo.DeleteKeyword(r.Context(), id); err != nil {
		s.serverError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleToggleKeyword(w http.ResponseWriter, r *http.Request) {
	id := parseID(r)
	k, err := s.repo.GetKeyword(r.Context(), id)
	if err != nil || k == nil {
		http.NotFound(w, r)
		return
	}
	if err := s.repo.SetKeywordEnabled(r.Context(), id, !k.Enabled); err != nil {
		s.serverError(w, err)
		return
	}
	s.renderKeywordRow(w, r, id)
}

func (s *Server) handleRunKeyword(w http.ResponseWriter, r *http.Request) {
	id := parseID(r)
	k, err := s.repo.GetKeyword(r.Context(), id)
	if err != nil || k == nil {
		http.NotFound(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if _, err := s.pipe.RunKeyword(ctx, k.Keyword); err != nil {
		http.Error(w, "run failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	s.renderKeywordRow(w, r, id)
}

func (s *Server) requireLead(w http.ResponseWriter, r *http.Request) *models.LeadView {
	id := mux.Vars(r)["id"]
	lead, err := s.repo.GetLead(r.Context(), id)
	if err != nil {
		s.serverError(w, err)
		return nil
	}
	if lead == nil {
		http.NotFound(w, r)
		return nil
	}
	return lead
}

func (s *Server) renderKeywordList(w http.ResponseWriter, r *http.Request) {
	kws, err := s.repo.ListKeywords(r.Context(), false)
	if err != nil {
		s.serverError(w, err)
		return
	}
	d := s.basePage()
	d.Keywords = kws
	s.renderPartial(w, "keyword_list", d)
}

func (s *Server) renderKeywordRow(w http.ResponseWriter, r *http.Request, id int64) {
	kws, err := s.repo.ListKeywords(r.Context(), false)
	if err != nil {
		s.serverError(w, err)
		return
	}
	for _, k := range kws {
		if k.ID == id {
			s.renderPartial(w, "keyword_row", k)
			return
		}
	}
	http.NotFound(w, r)
}

func (s *Server) renderPage(w http.ResponseWriter, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data.Now = time.Now()
	data.ReplyViaWebhook = s.replyViaWebhook
	if err := s.tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		s.logger.Error("render page", slog.String("page", data.Page), slog.Any("err", err))
	}
}

func (s *Server) renderPartial(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		s.logger.Error("render partial", slog.String("name", name), slog.Any("err", err))
	}
}

func (s *Server) serverError(w http.ResponseWriter, err error) {
	s.logger.Error("server error", slog.Any("err", err))
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func parseID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	return id
}

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sr := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sr, r)
		s.logger.Info("http",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", sr.status),
			slog.Duration("dur", time.Since(start)),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
