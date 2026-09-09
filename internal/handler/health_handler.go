package handler

import (
	"context"
	"net/http"
	"time"

	"billing-payment-api/internal/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type HealthHandler struct {
	db *gorm.DB
}

func NewHealthHandler(db *gorm.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

func (h *HealthHandler) Check(c *gin.Context) {
	sqlDB, err := h.db.DB()
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "database unavailable", err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		response.Error(c, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "database unavailable", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "database": "ok"})
}
