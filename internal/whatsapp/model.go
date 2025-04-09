package whatsapp

// WebhookMessage represents an incoming WhatsApp message from the webhook
type WebhookMessage struct {
	SenderID    string                 `json:"sender_id"`
	RecipientID string                 `json:"recipient_id"`
	Timestamp   string                 `json:"timestamp"`
	MessageID   string                 `json:"message_id"`
	Body        string                 `json:"body"`
	Type        string                 `json:"type"`
	Media       map[string]interface{} `json:"media,omitempty"`
}

// MessageResponse represents the API response for message operations
type MessageResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	ID      string `json:"id,omitempty"`
}
