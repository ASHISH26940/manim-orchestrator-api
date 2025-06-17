package main
import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ASHISH26940/manim-orchestrator-api/pkg/llm"
	"github.com/ASHISH26940/manim-orchestrator-api/pkg/config"
	"github.com/ASHISH26940/manim-orchestrator-api/pkg/db"
	"github.com/ASHISH26940/manim-orchestrator-api/pkg/handlers"
	"github.com/ASHISH26940/manim-orchestrator-api/pkg/middleware" // <--- Import middleware package
	"github.com/ASHISH26940/manim-orchestrator-api/pkg/utils" 
	"github.com/gin-gonic/gin"
	cors "github.com/gin-contrib/cors"
	log "github.com/sirupsen/logrus"                           // Structured logger
)

func main(){
	log.SetOutput(gin.DefaultWriter)
	log.SetLevel(log.InfoLevel)
	log.SetFormatter(&log.JSONFormatter{})
	log.Info("Starting Manim Orchestrator API...")

	cfg:=config.LoadConfig()

	if err:=db.InitDB(cfg.DatabaseURL); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.CloseDB()

	llmClient, err := llm.NewGeminiService(cfg.GeminiAPIKey)
	if err != nil {
		log.Fatalf("Failed to initialize LLM client: %v", err)
	}
	defer llmClient.Close()
	
	apiHandlers := handlers.NewHandlers(cfg, llmClient)

	router:=gin.Default()

	// --- CORS CONFIGURATION ---
	// Configure CORS middleware
	router.Use(cors.New(cors.Config{
		// Allow requests from your Next.js frontend development server
		AllowOrigins: []string{"http://localhost:3000,https://manime-frontend-gen.vercel.app"},
		// Allow common HTTP methods
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		// Allow common headers, including Authorization for JWT
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
		// Allow sending of credentials (like cookies or HTTP authentication headers)
		// Set to true if you are using cookies/session, otherwise false. For JWT in Authorization header, it's generally fine.
		AllowCredentials: true,
		// MaxAge defines the maximum age of the CORS preflight request in seconds.
		// During this time, the browser will not send a preflight request for the same URL.
		MaxAge: 12 * time.Hour,
	}))
	

	router.GET("/health",handlers.HealthCheck)
	router.POST("/api/projects/render-callback", apiHandlers.HandleRenderCallback) // <--- CRITICAL: Callback route
	router.POST("/api/merge_videos",apiHandlers.MergeVideosHandler)

	authRoutes:=router.Group("/auth")
	{
		authRoutes.POST("/register",handlers.RegisterUser)
		authRoutes.POST("/login", handlers.LoginUser)
		
	}

	protectedRoutes := router.Group("/api")
	protectedRoutes.Use(middleware.AuthMiddleware()) // <--- Apply the middleware here
	{
		// Example protected endpoint
		protectedRoutes.GET("/profile", func(c *gin.Context) {
			// Access user claims from the context
			claims, exists := middleware.GetUserClaimsFromContext(c)
			if !exists {
				log.Error("User claims not found in context for protected route.")
				utils.ResponseWithError(c, http.StatusInternalServerError, "Authentication error: User claims not found", nil)
				return
			}
			utils.ResponseWithSuccess(c, http.StatusOK, "Welcome to your profile!", gin.H{
				"user_id":  claims.UserID,
				"email":    claims.Email,
				"username": claims.Username,
			})
		})
		protectedRoutes.POST("/delete",handlers.DeleteUser)
		// Other protected routes will go here in future iterations
		// protectedRoutes.POST("/projects", handlers.CreateProject)

		projectsRoutes := protectedRoutes.Group("/projects")
		{
			projectsRoutes.POST("", handlers.CreateManimProject)                // POST /api/projects
			projectsRoutes.GET("", handlers.GetUserManimProjects)               // GET /api/projects
			projectsRoutes.GET("/:id", handlers.GetManimProjectByID)            // GET /api/projects/:id
			projectsRoutes.PUT("/:id", handlers.UpdateManimProject)             // PUT /api/projects/:id
			projectsRoutes.DELETE("/:id", handlers.DeleteManimProject)          // DELETE /api/projects/:id
			// --- NEW: Trigger Generation and Render Endpoint ---
			projectsRoutes.POST("/:id/generate-render", apiHandlers.TriggerManimGenerationAndRender)
		}
	}

	srv:=&http.Server{
		Addr: ":"+cfg.Port,
		Handler: router,
	}

	go func(){
		log.Infof("Server listening on %s:%s", cfg.Host, cfg.Port)
		if err:=srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server with a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL (cannot be caught)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Info("Server exited gracefully.")
}