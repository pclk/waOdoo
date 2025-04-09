package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/MarceloPetrucio/go-scalar-api-reference"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/pclk/waOdoo/docs" // Generated docs package
	"github.com/pclk/waOdoo/internal/database"
	"github.com/pclk/waOdoo/internal/ngrok"
	"github.com/pclk/waOdoo/internal/whatsapp" // Import WhatsApp package
	"gopkg.in/natefinch/lumberjack.v2"
)

// @title           waOdoo API
// @version         1.0
// @description     API server with WhatsApp Business API integration, PostgreSQL database, and Scalar docs.
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @host      localhost:1323
// @BasePath  /
//
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT Authorization header using the Bearer scheme

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found or could not be loaded. Using existing environment variables.")
	}

	if os.Getenv("APP_ENV") != "production" {
		log.Println("Generating Swagger documentation...")
		cmd := exec.Command("swag", "init", "--parseDependency", "--parseInternal")
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("Warning: Failed to generate Swagger docs: %v\n%s", err, output)
		} else {
			log.Println("Swagger documentation generated successfully")
		}
	}

	// Configure Lumberjack for log rotation
	logFile := &lumberjack.Logger{
		Filename:   "./logs/app.log",
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     28, // days
		Compress:   true,
	}

	// Create directory for logs if it doesn't exist
	if err := os.MkdirAll("./logs", 0755); err != nil {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	// Create a multi-writer to write logs to both file and stdout
	multiWriter := io.MultiWriter(os.Stdout, logFile)

	// Configure the standard logger to use our multi-writer
	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// Set up clean shutdown handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Initialize database connection
	db, err := database.New(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Log database connection information
	version, err := db.GetVersion()
	if err != nil {
		log.Fatalf("Failed to get database version: %v", err)
	}
	log.Printf("Connected to database: %s", version)

	// Create a new Echo instance
	e := echo.New()

	// Set Echo logger output
	e.Logger.SetOutput(multiWriter)

	// Middleware
	e.Use(middleware.Recover())

	// Custom logger middleware
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Output: multiWriter,
		Format: "${time_rfc3339} ${remote_ip} ${method} ${uri} ${status} ${latency_human}\n",
	}))

	waHandler, _ := whatsapp.New()

	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("db", db)
			return next(c)
		}
	})

	e.GET("/health", healthCheck)
	e.GET("/dbinfo", getDatabaseInfo(db))

	// Scalar API reference endpoint
	e.GET("/reference", scalarAPIReference)

	// Register routes
	waHandler.RegisterRoutes(e)

	// Log server startup info
	localAddr := ":1323"
	log.Println("Starting server on port 1323...")

	ngrok.ConfigureNgrok(e)
	go func() {
		if err := e.Start(localAddr); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatal("shutting down the server")
		}
	}()

	// Later in the code where you display ngrok info:
	if ngrok.NgrokURL != "" {
		log.Printf("🚀 ngrok tunnel active: %s -> http://localhost:1323", ngrok.NgrokURL)

		// API documentation URL via ngrok
		docsURL := ngrok.BuildURL(ngrok.NgrokURL, "/reference")
		log.Printf("📚 API documentation available at: %s", docsURL)

		log.Printf("🕳️ Tunnel available at http://localhost:4040")
	}
	// Wait for interrupt signal to gracefully shut down the server
	<-quit
	log.Println("Shutting down server...")

	// Give outstanding requests a deadline for completion
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}
}

// @Summary      Health check endpoint
// @Description  returns the health status of the API
// @Tags         health
// @Success      200  {object}  map[string]string
// @Router       /health [get]
func healthCheck(c echo.Context) error {
	log.Println("Handling health check request")
	return c.JSON(http.StatusOK, map[string]string{
		"status": "healthy",
	})
}

// ScalarAPIReference handles rendering the API documentation using Scalar
func scalarAPIReference(c echo.Context) error {
	htmlContent, err := scalar.ApiReferenceHTML(&scalar.Options{
		SpecURL: "./docs/swagger.json", // Path to your generated OpenAPI spec
		CustomOptions: scalar.CustomOptions{
			PageTitle: "waOdoo API",
		},
		DarkMode: true,
	})
	if err != nil {
		log.Printf("Failed to generate Scalar API reference: %v", err)
		return c.String(http.StatusInternalServerError, "Failed to generate API documentation")
	}

	return c.HTML(http.StatusOK, htmlContent)
}

// @Summary      Database information endpoint
// @Description  Returns version and connection status information about the database
// @Tags         health
// @Success      200  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /dbinfo [get]
func getDatabaseInfo(db *database.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		version, err := db.GetVersion()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Failed to get database information",
			})
		}

		return c.JSON(http.StatusOK, map[string]string{
			"version": version,
			"status":  "connected",
		})
	}
}
