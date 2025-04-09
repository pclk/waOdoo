package ngrok

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
	"golang.ngrok.com/ngrok"
	ngrok_config "golang.ngrok.com/ngrok/config"
)

var NgrokURL string

// GetWebhookURL constructs a webhook URL
func BuildURL(baseURL string, path string) string {
	if baseURL == "" {
		return ""
	}

	// Ensure path starts with a slash
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Remove trailing slash from base URL if present
	if strings.HasSuffix(baseURL, "/") {
		baseURL = baseURL[:len(baseURL)-1]
	}

	return baseURL + path
}

// Start ngrok and configure Echo to use it
func ConfigureNgrok(e *echo.Echo) {
	if strings.ToLower(os.Getenv("NGROK_AUTOSTART")) != "true" {
		log.Println("Ngrok is disabled")
		return
	}

	// Get ngrok authtoken from environment
	authtoken := os.Getenv("NGROK_AUTHTOKEN")
	if authtoken == "" {
		log.Fatal("NGROK_AUTHTOKEN is required when ENABLE_NGROK is true")
	}

	// Create ngrok tunnel
	listener, err := ngrok.Listen(context.Background(),
		ngrok_config.HTTPEndpoint(
			ngrok_config.WithDomain(os.Getenv("NGROK_DOMAIN")),
		),
		ngrok.WithAuthtoken(authtoken),
	)
	if err != nil {
		log.Fatalf("Failed to create ngrok tunnel: %v", err)
	}

	NgrokURL = listener.URL()
	log.Printf("Ngrok tunnel established at: %s", NgrokURL)

	// Configure Echo to use the ngrok listener
	e.Listener = listener
}
