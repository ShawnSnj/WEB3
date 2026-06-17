// Package api wires HTTP routing and HTTP-shaped handlers. It does not
// contain business logic — handlers translate between HTTP and the service
// layer, nothing more.
package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shawn/jobhunttask/internal/config"
	crmsvc "github.com/shawn/jobhunttask/internal/crm/service"
	"github.com/shawn/jobhunttask/internal/service"
)

// Deps bundles everything a router needs. Adding a new dependency here is
// the canonical extension point for future steps (services, scheduler,
// authenticator, etc.).
type Deps struct {
	Config             config.Config
	Logger             *slog.Logger
	DB                 *pgxpool.Pool
	TaskService        *service.TaskService
	ReviewService      *service.DailyReviewService
	TaskSessionService *service.TaskSessionService
	MetricsService     *service.MetricsService
	SuggestionService  *service.SuggestionService
	CRMService         *crmsvc.CRM
}

// NewRouter builds the Gin engine with global middleware and registers
// all route groups.
func NewRouter(d Deps) *gin.Engine {
	if d.Config.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(
		gin.Recovery(),
		requestLogger(d.Logger),
	)

	registerHealthRoutes(r, d)

	v1 := r.Group("/api/v1")
	if d.TaskService != nil {
		newTaskHandler(d.TaskService).register(v1)
	}
	if d.ReviewService != nil {
		newReviewHandler(d.ReviewService).register(v1)
	}
	if d.TaskSessionService != nil {
		newSessionHandler(d.TaskSessionService).register(v1)
	}
	if d.MetricsService != nil {
		newMetricsHandler(d.MetricsService).register(v1)
	}
	if d.SuggestionService != nil {
		newSuggestionHandler(d.SuggestionService).register(v1)
	}
	if d.CRMService != nil {
		newCRMHandler(d.CRMService).register(v1)
		registerJobsAlias(v1, d.CRMService)
	}

	// CORS for Next.js CRM frontend
	r.Use(crmCORS(d.Config))

	return r
}

func crmCORS(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := cfg.CRM.FrontendOrigin
		if origin == "" {
			// CRM is embedded at /crm on the same origin — no CORS header needed.
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
			c.Next()
			return
		}
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// requestLogger is a minimal structured access log middleware.
func requestLogger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info("http request",
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", c.Writer.Status()),
			slog.Duration("latency", time.Since(start)),
			slog.String("ip", c.ClientIP()),
		)
	}
}
