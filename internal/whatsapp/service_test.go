package whatsapp

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
)

func setup(t *testing.T) (*Service, *assert.Assertions) {
	if err := godotenv.Load("../../.env"); err != nil {
		log.Println("Warning: .env file not found or could not be loaded. Using existing environment variables.")
	}
	t.Logf("META_WA_API_VERSION: %v", os.Getenv("META_WA_API_VERSION"))
	_, service := New()
	a := assert.New(t)
	return service, a
}

func TestListPhoneNumbers(t *testing.T) {
	service, a := setup(t)

	phones, err := service.ListPhoneNumbers()

	a.NoError(err, "ListPhoneNumbers() failed with error: %v", err)
	a.NotNil(phones, "Phone response shouldn't be nil")

	if phones != nil {
		t.Logf("Received %d phone numbers", len(phones.Data))
		for i, phone := range phones.Data {
			t.Logf("Phone %d: ID=%s, Name=%s, Number=%s",
				i, phone.ID, phone.VerifiedName, phone.DisplayPhoneNumber)
		}
	}
}

func TestNewService(t *testing.T) {
	a := assert.New(t)

	// Load environment variables
	if err := godotenv.Load("../../.env"); err != nil {
		log.Println("Warning: .env file not found or could not be loaded. Using existing environment variables.")
	}

	// Get expected values from environment
	expectedToken := os.Getenv("META_WA_ACCESS_TOKEN")
	expectedBusinessID := os.Getenv("META_WA_BUSINESS_ID")
	expectedAPIVersion := os.Getenv("META_WA_API_VERSION")
	expectedWebhookSecret := os.Getenv("META_WA_WEBHOOK_SECRET")

	// Verify environment variables are set for meaningful test
	if expectedToken == "" || expectedBusinessID == "" || expectedAPIVersion == "" {
		t.Log("Warning: One or more environment variables not set, test may not be valuable")
	}

	// Create new service
	service := NewService()

	// Validate service properties match environment variables
	a.Equal(expectedToken, service.AccessToken, "AccessToken should match environment variable")
	a.Equal(expectedBusinessID, service.BusinessAccountID, "BusinessAccountID should match environment variable")
	a.Equal(expectedAPIVersion, service.APIVersion, "APIVersion should match environment variable")
	a.Equal(expectedWebhookSecret, service.WebhookSecret, "WebhookSecret should match environment variable")
}

func TestSendMessage(t *testing.T) {
	// Skip test if TEST_SEND_MESSAGE is not "true"
	if os.Getenv("TEST_SEND_MESSAGE") != "true" {
		t.Skip("Skipping sending actual message. Set TEST_SEND_MESSAGE=true to enable.")
	}

	service, a := setup(t)
	ctx := context.Background()

	// Get test phone number from environment or use a default test number
	testPhoneNumber := os.Getenv("TEST_PHONE")
	t.Logf("TEST_PHONE: %v", testPhoneNumber)
	if testPhoneNumber == "" {
		t.Log("Warning: TEST_PHONE not set, test may fail")
		testPhoneNumber = "+6598765432" // Example number, replace as needed
	}

	// Create a real message with timestamp to identify in logs
	timestamp := time.Now().Format(time.RFC3339)
	msg := OutgoingMessage{
		To:      testPhoneNumber,
		Message: "Test message sent at " + timestamp,
	}

	// Send the actual message
	t.Logf("Sending test message to %s", testPhoneNumber)
	response, err := service.SendMessage(ctx, msg)

	// Verify response
	a.NoError(err, "SendMessage should not return an error")
	a.NotNil(response, "Response should not be nil")

	if response != nil {
		t.Logf("Response: Success=%v, Message=%s, ID=%s",
			response.Success, response.Message, response.ID)

		// Verify MessageResponse reflects successful API interaction
		a.True(response.Success, "Response should indicate success")
		a.Equal("Message sent successfully", response.Message)

		// The ID field should be populated with the WhatsApp message ID
		// WhatsApp message IDs typically start with "wamid."
		a.NotEmpty(response.ID, "Message ID should not be empty")
		a.Contains(response.ID, "wamid.", "Message ID should follow WhatsApp ID format")
	}
}

func TestProcessWebhook(t *testing.T) {
	service, a := setup(t)
	ctx := context.Background()

	// Create a realistic webhook payload based on Meta's documentation
	// This is as close to an integration test as we can get without receiving a real webhook
	webhookPayload := `{
		"object": "whatsapp_business_account",
		"entry": [{
			"id": "123456789",
			"changes": [{
				"value": {
					"messaging_product": "whatsapp",
					"metadata": {
						"display_phone_number": "6591234567",
						"phone_number_id": "9876543210"
					},
					"contacts": [{
						"profile": {
							"name": "Test User"
						},
						"wa_id": "6598765432"
					}],
					"messages": [{
						"from": "6598765432",
						"id": "wamid.test123",
						"timestamp": "1677721357",
						"text": {
							"body": "Hello, this is a test message"
						},
						"type": "text"
					}]
				},
				"field": "messages"
			}]
		}]
	}`

	// Process the webhook
	err := service.ProcessWebhook(ctx, []byte(webhookPayload))

	// Verify no errors occurred during processing
	a.NoError(err, "ProcessWebhook should not return an error for valid payload")
	t.Log("Successfully processed webhook payload")
}

func TestProcessWebhookInvalidPayload(t *testing.T) {
	service, a := setup(t)
	ctx := context.Background()

	// Test with an invalid JSON payload
	invalidPayload := `{"object": "whatsapp_business_account", "entry": [}`

	// Process the webhook
	err := service.ProcessWebhook(ctx, []byte(invalidPayload))

	// Verify error handling works properly
	a.Error(err, "ProcessWebhook should return an error for invalid JSON")
	a.Contains(err.Error(), "failed to parse webhook payload",
		"Error message should indicate parsing failure")
	t.Log("Correctly detected invalid webhook payload")
}
