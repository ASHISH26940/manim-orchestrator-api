package handlers
import (
	"net/http"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func HealthCheck(c *gin.Context){
	log.Info("Health check endpoint hit")
	c.JSON(http.StatusOK,gin.H{
		"status":  "ok",
		"message": "Manim Orchestrator API is running",
	})
}