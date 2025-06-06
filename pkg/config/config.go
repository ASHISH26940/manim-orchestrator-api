package config

import(
	"os"
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)

type Config struct{
	DatabaseURL string
	Host string
	Port string
	JwtSecret string
	GeminiAPIKey string
	ManimRendererURL   string
}

func LoadConfig() *Config{
	err:=godotenv.Load()
	if err!=nil{
		log.Fatalf("Error loading .env file: %v", err)
	}
	cfg:=&Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Host: os.Getenv("HOST"),
		Port: os.Getenv("PORT"),
		JwtSecret: os.Getenv("JWT_SECRET"),
		GeminiAPIKey: os.Getenv("GEMINI_API_KEY"),
		ManimRendererURL: os.Getenv("MANIM_RENDERER_URL"),
	}

	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.JwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is not set. This is critical for authentication.")
	}
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}
	if cfg.GeminiAPIKey == "" {
		log.Fatal("GEMINI_API_KEY is not set")
	}
	if cfg.ManimRendererURL == ""{
		log.Fatal("MANIM RENDERER is empty")
	}

	return cfg
}