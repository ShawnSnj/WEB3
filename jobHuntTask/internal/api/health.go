package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/shawn/jobhunttask/internal/database"
)

// registerHealthRoutes wires the liveness and readiness endpoints.
//
// /healthz — process is up; no external checks (used by container runtime).
// /readyz  — process can serve traffic; pings the database.
func registerHealthRoutes(r *gin.Engine, d Deps) {
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"app":     d.Config.App.Name,
			"version": d.Config.App.Version,
			"env":     d.Config.App.Environment,
		})
	})

	r.GET("/readyz", func(c *gin.Context) {
		if err := database.Ping(c.Request.Context(), d.DB); err != nil {
			d.Logger.Warn("readiness check failed", "error", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unavailable",
				"db":     "down",
				"error":  err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status": "ready",
			"db":     "up",
		})
	})
}
