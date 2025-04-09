package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type Service struct {
	AccessToken       string
	BusinessAccountID string
	APIVersion        string
	WebhookSecret     string
}

func NewService() *Service {
	return &Service{
		AccessToken:       os.Getenv("META_WA_ACCESS_TOKEN"),
		BusinessAccountID: os.Getenv("META_WA_BUSINESS_ID"),
		APIVersion:        os.Getenv("META_WA_API_VERSION"),
		WebhookSecret:     os.Getenv("META_WA_WEBHOOK_SECRET"),
	}
}

// OutgoingMessage represents a message to be sent via WhatsApp
type OutgoingMessage struct {
	To       string `json:"to" example:"6598232744"`
	Message  string `json:"message" example:"hi"`
	MediaURL string `json:"media_url,omitempty"`
}

// SendMessage implements sending a message via Meta's WhatsApp Business API
func (s *Service) SendMessage(c context.Context, msg OutgoingMessage) (*MessageResponse, error) {
	// Build the Graph API URL
	var phoneID string

	phones, err := s.ListPhoneNumbers()
	if err != nil {
		return nil, fmt.Errorf("failed to list phone numbers: %w", err)
	}
	phone := phones.Data[0]
	log.Printf("agent phone: %v", phone)
	phoneID = phone.ID

	apiURL := fmt.Sprintf("https://graph.facebook.com/%s/%s/messages",
		s.APIVersion, phoneID)

	// Format recipient phone number according to Meta requirements
	// Remove "+" prefix if present, as Meta doesn't want it
	recipient := strings.TrimPrefix(msg.To, "+")

	// Prepare message request body according to Meta's format
	requestBody := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                recipient,
		"type":              "text",
		"text": map[string]string{
			"body": msg.Message,
		},
	}

	// Convert to JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(
		c,
		"POST",
		apiURL,
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.AccessToken)

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	log.Printf("Sent message, response body: %v", string(body))

	// Check for error response
	if resp.StatusCode >= 400 {
		return &MessageResponse{
			Success: false,
			Message: fmt.Sprintf("API error (status %d): %s", resp.StatusCode, string(body)),
		}, nil
	}

	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract message ID
	var messageID string
	if messages, ok := result["messages"].([]interface{}); ok && len(messages) > 0 {
		if msgMap, ok := messages[0].(map[string]interface{}); ok {
			if id, ok := msgMap["id"].(string); ok {
				messageID = id
			}
		}
	}

	return &MessageResponse{
		Success: true,
		Message: "Message sent successfully",
		ID:      messageID,
	}, nil
}

// ProcessWebhook handles Meta's WhatsApp webhook
func (s *Service) ProcessWebhook(ctx context.Context, payload []byte) error {
	// Parse the webhook payload
	var webhookData struct {
		Object string `json:"object"`
		Entry  []struct {
			ID      string `json:"id"`
			Changes []struct {
				Value struct {
					MessagingProduct string `json:"messaging_product"`
					Metadata         struct {
						DisplayPhoneNumber string `json:"display_phone_number"`
						PhoneNumberID      string `json:"phone_number_id"`
					} `json:"metadata"`
					Contacts []struct {
						Profile struct {
							Name string `json:"name"`
						} `json:"profile"`
						WaID string `json:"wa_id"`
					} `json:"contacts"`
					Messages []struct {
						From      string `json:"from"`
						ID        string `json:"id"`
						Timestamp string `json:"timestamp"`
						Text      struct {
							Body string `json:"body"`
						} `json:"text,omitempty"`
						Type string `json:"type"`
					} `json:"messages"`
				} `json:"value"`
				Field string `json:"field"`
			} `json:"changes"`
		} `json:"entry"`
	}

	if err := json.Unmarshal(payload, &webhookData); err != nil {
		return fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	// Process entry data
	for _, entry := range webhookData.Entry {
		for _, change := range entry.Changes {
			if change.Field == "messages" {
				// Process each message
				for _, message := range change.Value.Messages {
					// Extract and convert to our internal WebhookMessage format
					senderID := message.From
					messageText := ""
					if message.Type == "text" {
						messageText = message.Text.Body
					}

					internalMsg := WebhookMessage{
						SenderID:    senderID,
						RecipientID: change.Value.Metadata.PhoneNumberID,
						Timestamp:   message.Timestamp,
						MessageID:   message.ID,
						Body:        messageText,
						Type:        message.Type,
					}

					// Log the received message
					fmt.Printf("Received message: %+v\n", internalMsg)

					// Here you would typically:
					// 1. Save the message to a database
					// 2. Process it based on content
					// 3. Potentially trigger an automated response
				}
			}
		}
	}

	return nil
}

// PhoneNumbersResponse represents the response structure from the phone numbers API
type PhoneNumbersResponse struct {
	Data   []PhoneNumber `json:"data"`
	Paging struct {
		Cursors struct {
			Before string `json:"before,omitempty"`
			After  string `json:"after,omitempty"`
		} `json:"cursors,omitempty"`
		Next string `json:"next,omitempty"`
	} `json:"paging,omitempty"`
}

// PhoneNumber represents a phone number entry from the WhatsApp Business API
type PhoneNumber struct {
	ID                 string `json:"id"`
	VerifiedName       string `json:"verified_name"`
	DisplayPhoneNumber string `json:"display_phone_number"`
	QualityRating      string `json:"quality_rating"`
}

// ListPhoneNumbers retrieves all WhatsApp phone numbers associated with a business account
func (s *Service) ListPhoneNumbers() (*PhoneNumbersResponse, error) {
	// Build the Graph API URL
	apiURL := fmt.Sprintf("https://graph.facebook.com/%s/%s/phone_numbers",
		s.APIVersion, s.BusinessAccountID)

	// Create HTTP request without context
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add access token to query parameters
	q := req.URL.Query()
	q.Add("access_token", s.AccessToken)
	req.URL.RawQuery = q.Encode()

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for error response
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var phoneNumbersResp PhoneNumbersResponse
	if err := json.Unmarshal(body, &phoneNumbersResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &phoneNumbersResp, nil
}
