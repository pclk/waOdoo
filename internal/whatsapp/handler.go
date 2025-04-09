package whatsapp

import (
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
)

// Handler handles WhatsApp-related HTTP endpoints
type Handler struct {
	service *Service
}

func New() (*Handler, *Service) {
	service := NewService()
	return &Handler{service: service}, service
}

func (h *Handler) RegisterRoutes(e *echo.Echo) {
	group := e.Group("/whatsapp")
	group.POST("/send", h.SendMessage)
	group.POST("/webhook", h.ReceiveWebhook)
	group.GET("/webhook", h.VerifyWebhook)
}

// @Summary      Send a WhatsApp message
// @Description  Sends a WhatsApp message to the specified number
// @Param        message  body      OutgoingMessage  true  "Message details"
// @Tags         whatsapp
// @Success      200      {object}  MessageResponse
// @Failure      400      {object}  MessageResponse
// @Failure      500      {object}  MessageResponse
// @Router       /whatsapp/send [post]
func (h *Handler) SendMessage(c echo.Context) error {
	var msg OutgoingMessage
	if err := c.Bind(&msg); err != nil {
		return c.JSON(http.StatusBadRequest, MessageResponse{
			Success: false,
			Message: "Invalid request format",
		})
	}

	// Validate request
	if msg.To == "" || msg.Message == "" {
		return c.JSON(http.StatusBadRequest, MessageResponse{
			Success: false,
			Message: "Both 'to' and 'message' fields are required",
		})
	}

	// Send message
	resp, err := h.service.SendMessage(c.Request().Context(), msg)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, MessageResponse{
			Success: false,
			Message: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, resp)
}

// ReceiveWebhook handles incoming WhatsApp webhook requests
// @Summary      Receive a WhatsApp webhook
// @Description  Process incoming WhatsApp webhook notifications
// @Tags         whatsapp
// @Success      200      {object}  MessageResponse
// @Failure      400      {object}  MessageResponse
// @Failure      500      {object}  MessageResponse
// @Router       /whatsapp/webhook [post]
func (h *Handler) ReceiveWebhook(c echo.Context) error {
	// Read the request body
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, MessageResponse{
			Success: false,
			Message: "Failed to read request body",
		})
	}

	// Process the webhook
	if err := h.service.ProcessWebhook(c.Request().Context(), body); err != nil {
		return c.JSON(http.StatusInternalServerError, MessageResponse{
			Success: false,
			Message: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, MessageResponse{
		Success: true,
		Message: "Webhook processed successfully",
	})
}

// VerifyWebhook handles the webhook verification required by Meta's WhatsApp API
// @Summary      Verify WhatsApp webhook
// @Description  Verifies the WhatsApp webhook with Meta's verification challenge
// @Tags         whatsapp
// @Success      200  {string}  string  "Challenge response"
// @Failure      403  {string}  string  "Verification failed"
// @Router       /whatsapp/webhook [get]
func (h *Handler) VerifyWebhook(c echo.Context) error {
	// Get query parameters
	mode := c.QueryParam("hub.mode")
	token := c.QueryParam("hub.verify_token")
	challenge := c.QueryParam("hub.challenge")

	// Verify that mode and token match expected values
	if mode == "subscribe" && token == h.service.WebhookSecret {
		return c.String(http.StatusOK, challenge)
	}

	return c.String(http.StatusForbidden, "Verification failed")
}
