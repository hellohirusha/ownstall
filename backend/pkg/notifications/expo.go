package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// expoPushEndpoint is Expo's hosted push service, which fans out to
// FCM (Android) and APNs (iOS) for us.
const expoPushEndpoint = "https://exp.host/--/api/v2/push/send"

// expoBatchLimit is the maximum number of messages Expo accepts per request
const expoBatchLimit = 100

var pushClient = &http.Client{Timeout: 15 * time.Second}

// Message is a single push notification addressed to one Expo token
type Message struct {
	To    string            `json:"to"`
	Title string            `json:"title"`
	Body  string            `json:"body"`
	Data  map[string]string `json:"data,omitempty"`
	Sound string            `json:"sound"`
}

// pushTicket is one entry of Expo's response array
type pushTicket struct {
	Status  string `json:"status"`
	ID      string `json:"id"`
	Message string `json:"message"`
	Details struct {
		Error string `json:"error"`
	} `json:"details"`
}

type pushResponse struct {
	Data   []pushTicket `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// SendPushNotification sends a push notification via Expo's push API.
// This is FREE — Expo handles the delivery to FCM and APNS.
// Returns the tokens Expo rejected as permanently invalid, so the
// caller can deactivate them.
func SendPushNotification(ctx context.Context, tokens []string, title, body string, data map[string]string) ([]string, error) {
	if len(tokens) == 0 {
		return nil, nil
	}

	var invalid []string

	// Expo caps each request at 100 messages
	for start := 0; start < len(tokens); start += expoBatchLimit {
		end := start + expoBatchLimit
		if end > len(tokens) {
			end = len(tokens)
		}
		batch := tokens[start:end]

		messages := make([]Message, 0, len(batch))
		for _, token := range batch {
			messages = append(messages, Message{
				To:    token,
				Title: title,
				Body:  body,
				Data:  data,
				Sound: "default",
			})
		}

		batchInvalid, err := sendBatch(ctx, messages, batch)
		if err != nil {
			return invalid, err
		}
		invalid = append(invalid, batchInvalid...)
	}

	return invalid, nil
}

func sendBatch(ctx context.Context, messages []Message, tokens []string) ([]string, error) {
	payload, err := json.Marshal(messages)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal push payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		expoPushEndpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to build push request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := pushClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("expo push failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("expo push returned HTTP %d", resp.StatusCode)
	}

	var result pushResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// Delivery was accepted; we just cannot read the receipts
		fmt.Printf("WARNING: failed to decode expo push response: %v\n", err)
		return nil, nil
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("expo push error: %s", result.Errors[0].Message)
	}

	// Expo reports per-message failures in the ticket array rather than
	// the HTTP status — a token the user uninstalled shows up here
	var invalid []string
	for i, ticket := range result.Data {
		if ticket.Status != "error" {
			continue
		}
		fmt.Printf("push to token rejected: %s (%s)\n", ticket.Message, ticket.Details.Error)
		if ticket.Details.Error == "DeviceNotRegistered" && i < len(tokens) {
			invalid = append(invalid, tokens[i])
		}
	}

	return invalid, nil
}
