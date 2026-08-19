package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/undeschimb/undeschimb/internal/service"
)

func NewRouter(comparison *service.ComparisonService, corsOrigin string) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), cors(corsOrigin))
	router.GET("/healthz", func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	api := router.Group("/api/v1")
	api.GET("/comparison", func(context *gin.Context) {
		amount, err := decimal.NewFromString(strings.TrimSpace(context.Query("amount")))
		if err != nil {
			context.JSON(http.StatusBadRequest, gin.H{"error": "amount must be a decimal number"})
			return
		}
		response, err := comparison.Compare(context.Request.Context(), service.ComparisonRequest{
			From: context.Query("from"), To: context.Query("to"), Amount: amount,
		})
		if err != nil {
			context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		context.JSON(http.StatusOK, response)
	})
	api.GET("/history", func(context *gin.Context) {
		days, err := strconv.Atoi(context.DefaultQuery("period", "30"))
		if err != nil {
			context.JSON(http.StatusBadRequest, gin.H{"error": "period must be a number"})
			return
		}
		side := context.Query("side")
		if side == "" {
			side = context.Query("direction")
		}
		response, err := comparison.History(context.Request.Context(), context.Query("provider"), context.Query("currency"), side, days)
		if err != nil {
			context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		context.JSON(http.StatusOK, response)
	})
	return router
}

func cors(origin string) gin.HandlerFunc {
	return func(context *gin.Context) {
		context.Header("Access-Control-Allow-Origin", origin)
		context.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
		context.Header("Access-Control-Allow-Headers", "Content-Type")
		if context.Request.Method == http.MethodOptions {
			context.Status(http.StatusNoContent)
			context.Abort()
			return
		}
		context.Next()
	}
}
