package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-stomp/stomp"
)

type CorporateAPI struct {
	stompConn      *stomp.Conn
	extSMTSURL     string
	artemisURL     string
	queueName      string
	flow2QueueName string
	ldapUsername   string
	ldapPassword   string
}

type Message struct {
	ID        string            `json:"id"`
	Timestamp string            `json:"timestamp"`
	Topic     string            `json:"topic"`
	Source    string            `json:"source"`
	Headers   map[string]string `json:"headers"`
	Body      json.RawMessage   `json:"body"`
}

func NewCorporateAPI(artemisURL, extSMTSURL, queueName, ldapUsername, ldapPassword string) (*CorporateAPI, error) {
	// Connect to ArtemisMQ
	conn, err := stomp.Dial("tcp", artemisURL,
		stomp.ConnOpt.Login("artemis", "artemis"),
		stomp.ConnOpt.HeartBeat(0, 0))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ArtemisMQ: %w", err)
	}

	return &CorporateAPI{
		stompConn:      conn,
		extSMTSURL:     extSMTSURL,
		artemisURL:     artemisURL,
		queueName:      queueName,
		flow2QueueName: "SMTS_EXT_TEST_QUEUE", // Different queue for Flow 2
		ldapUsername:   ldapUsername,
		ldapPassword:   ldapPassword,
	}, nil
}

// HandleRoot handles all incoming requests and routes them appropriately
func (api *CorporateAPI) HandleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/health" {
		api.HealthCheck(w, r)
		return
	}

	if r.URL.Path == "/oauth/token" {
		api.HandleToken(w, r)
		return
	}

	if r.URL.Path == "/smts/message" {
		api.HandleSMTSMessage(w, r)
		return
	}

	// Handle topic-specific endpoints for Flow 1
	if r.Method == http.MethodPost && r.URL.Path != "/" {
		api.HandleMessageFromEXT(w, r)
		return
	}

	// Default response for other requests
	http.Error(w, "Not found", http.StatusNotFound)
}

// HandleMessageFromEXT handles messages from EXT-SMTS (Flow 1)
func (api *CorporateAPI) HandleMessageFromEXT(w http.ResponseWriter, r *http.Request) {
	topic := r.URL.Path[1:] // Remove leading slash

	// Authenticate request
	if !api.authenticateRequest(r) {
		http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	// Read message body
	var body json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Invalid JSON: %v"}`, err), http.StatusBadRequest)
		return
	}

	// Extract SMTS headers
	messageID := r.Header.Get("X-SMTS-Message-ID")
	timestamp := r.Header.Get("X-SMTS-Timestamp")
	source := r.Header.Get("X-SMTS-Source")

	// Create message for ArtemisMQ
	message := Message{
		ID:        messageID,
		Timestamp: timestamp,
		Topic:     topic,
		Source:    source,
		Headers: map[string]string{
			"X-SMTS-Message-ID": messageID,
			"X-SMTS-Timestamp":  timestamp,
			"X-SMTS-Source":     source,
			"Content-Type":      "application/json",
		},
		Body: body,
	}

	// Marshal message to JSON
	messageBytes, err := json.Marshal(message)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to marshal message: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// Send to ArtemisMQ with proper SMTS headers
	err = api.stompConn.Send(
		api.queueName,
		"application/json",
		messageBytes,
		stomp.SendOpt.Header("smts-message-id", messageID),
		stomp.SendOpt.Header("smts-timestamp", timestamp),
		stomp.SendOpt.Header("smts-source", "artemis"), // Mark as from Artemis for Flow 1
		stomp.SendOpt.Header("smts-topic", topic),
		stomp.SendOpt.Header("persistent", "true"),
	)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to send to ArtemisMQ: %v"}`, err), http.StatusInternalServerError)
		return
	}

	log.Printf("Message forwarded to ArtemisMQ: %s (topic: %s)", messageID, topic)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "accepted",
		"message": "Message forwarded to ArtemisMQ",
		"id":      messageID,
	})
}

// HandleSMTSMessage handles messages from external SMTS via /smts/message endpoint
func (api *CorporateAPI) HandleSMTSMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Authenticate request
	if !api.authenticateRequest(r) {
		http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	// Parse request body
	var request struct {
		Topic string                 `json:"topic"`
		Data  map[string]interface{} `json:"data"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Invalid JSON: %v"}`, err), http.StatusBadRequest)
		return
	}

	if request.Topic == "" {
		http.Error(w, `{"error": "Topic is required"}`, http.StatusBadRequest)
		return
	}

	if request.Data == nil {
		http.Error(w, `{"error": "Data is required"}`, http.StatusBadRequest)
		return
	}

	// Convert data to JSON for the message body
	bodyBytes, err := json.Marshal(request.Data)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to marshal data: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// Generate message ID and timestamp
	messageID := fmt.Sprintf("smts-%s", time.Now().UTC().Format("20060102150405"))
	timestamp := time.Now().UTC().Format(time.RFC3339)

	// Create message for ArtemisMQ
	message := Message{
		ID:        messageID,
		Timestamp: timestamp,
		Topic:     request.Topic,
		Source:    "artemis", // Use "artemis" for Flow 1 messages
		Headers: map[string]string{
			"X-SMTS-Message-ID": messageID,
			"X-SMTS-Timestamp":  timestamp,
			"X-SMTS-Source":     "artemis", // Use "artemis" for Flow 1 messages
			"Content-Type":      "application/json",
		},
		Body: bodyBytes,
	}

	// Marshal message to JSON
	messageBytes, err := json.Marshal(message)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to marshal message: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// Send to ArtemisMQ with proper SMTS headers
	err = api.stompConn.Send(
		api.queueName,
		"application/json",
		messageBytes,
		stomp.SendOpt.Header("smts-message-id", messageID),
		stomp.SendOpt.Header("smts-timestamp", timestamp),
		stomp.SendOpt.Header("smts-source", "artemis"), // Use "artemis" for Flow 1 messages
		stomp.SendOpt.Header("smts-topic", request.Topic),
		stomp.SendOpt.Header("persistent", "true"),
	)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to send to ArtemisMQ: %v"}`, err), http.StatusInternalServerError)
		return
	}

	log.Printf("Message received via /smts/message and forwarded to ArtemisMQ: %s (topic: %s)", messageID, request.Topic)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "accepted",
		"message": "Message forwarded to ArtemisMQ",
		"id":      messageID,
		"topic":   request.Topic,
	})
}

// HealthCheck handles health check endpoint
func (api *CorporateAPI) HealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":    "healthy",
		"service":   "corporate-api",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// HandleToken handles OAuth2 token requests
func (api *CorporateAPI) HandleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		http.Error(w, `{"error": "Invalid form data"}`, http.StatusBadRequest)
		return
	}

	// Validate grant type
	grantType := r.FormValue("grant_type")
	if grantType != "client_credentials" {
		http.Error(w, `{"error": "unsupported_grant_type"}`, http.StatusBadRequest)
		return
	}

	// Validate client credentials
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")

	if clientID != "test-client-id" || clientSecret != "test-client-secret" {
		http.Error(w, `{"error": "invalid_client"}`, http.StatusUnauthorized)
		return
	}

	// Generate token response
	tokenResponse := map[string]interface{}{
		"access_token": "test-access-token-" + time.Now().UTC().Format("20060102150405"),
		"token_type":   "Bearer",
		"expires_in":   3600,
		"scope":        r.FormValue("scope"),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(tokenResponse)
}

// authenticateRequest authenticates incoming requests using API key or Bearer token
func (api *CorporateAPI) authenticateRequest(r *http.Request) bool {
	// Check for API key
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "test-api-key" {
		return true
	}

	// Check for Bearer token
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		// Simple Bearer token validation - in real implementation, this would validate JWT
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			// For testing, accept any token that starts with "test-access-token-"
			return strings.HasPrefix(token, "test-access-token-")
		}
	}

	// For /smts/message endpoint, also check for client credentials authentication
	// This is needed because EXT-SMTS uses client_credentials auth type
	if r.URL.Path == "/smts/message" {
		// Check if we have client credentials headers
		clientID := r.Header.Get("X-Client-ID")
		clientSecret := r.Header.Get("X-Client-Secret")

		if clientID == "test-client-id" && clientSecret == "test-client-secret" {
			return true
		}

	}

	return false
}

// StartMessageConsumer starts consuming messages from ArtemisMQ for Flow 2 (INT→EXT)
func (api *CorporateAPI) StartMessageConsumer() {
	go func() {
		for {
			// Subscribe to Flow 2 queue (different from Flow 1 queue)
			sub, err := api.stompConn.Subscribe(api.flow2QueueName, stomp.AckAuto)
			if err != nil {
				log.Printf("Failed to subscribe to ArtemisMQ queue: %v, retrying in 5 seconds...", err)
				time.Sleep(5 * time.Second)
				continue
			}

			log.Printf("Started consuming messages from ArtemisMQ queue: %s", api.flow2QueueName)

			for {
				msg := <-sub.C
				if msg == nil {
					log.Printf("Subscription channel closed, reconnecting...")
					break
				}

				// Process the message
				if err := api.processArtemisMessage(msg); err != nil {
					log.Printf("Failed to process message from ArtemisMQ: %v", err)
				}
			}

			// If we get here, the subscription was closed, so we'll reconnect
			time.Sleep(2 * time.Second)
		}
	}()
}

// processArtemisMessage processes a message received from ArtemisMQ
func (api *CorporateAPI) processArtemisMessage(msg *stomp.Message) error {
	var message Message
	if err := json.Unmarshal(msg.Body, &message); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	log.Printf("Received message from ArtemisMQ: %s (topic: %s)", message.ID, message.Topic)

	// Forward to EXT-SMTS
	extSMTSURL := fmt.Sprintf("%s/corp_message/%s", api.extSMTSURL, message.Topic)

	client := &http.Client{Timeout: 10 * time.Second}

	// Create request body
	messageBytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequest("POST", extSMTSURL, bytes.NewReader(messageBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-api-key")
	req.Header.Set("X-SMTS-Message-ID", message.ID)
	req.Header.Set("X-SMTS-Timestamp", message.Timestamp)
	req.Header.Set("X-SMTS-Source", "corporate-api") // Mark as from corporate API for Flow 2

	// Add LDAP Basic Auth header
	auth := api.ldapUsername + ":" + api.ldapPassword
	basicAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
	req.Header.Set("Authorization", basicAuth)

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Warning: Failed to send to EXT-SMTS: %v (message will be retried)", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Warning: EXT-SMTS returned status %d for message %s", resp.StatusCode, message.ID)
	} else {
		log.Printf("Message delivered to EXT-SMTS: %s (topic: %s)", message.ID, message.Topic)
	}

	return nil
}

func main() {
	// Configuration
	artemisURL := GetEnvWithDefault("ARTEMIS_URL", "artemis-test:61613")
	extSMTSURL := GetEnvWithDefault("EXT_SMTS_URL", "http://smts-ext-test:18082")
	queueName := GetEnvWithDefault("ARTEMIS_QUEUE", "SMTS_INT_TEST_QUEUE")
	ldapUsername := GetEnvWithDefault("LDAP_USERNAME", "testuser")
	ldapPassword := GetEnvWithDefault("LDAP_PASSWORD", "testpass")
	port := GetEnvWithDefault("PORT", "8080")

	// Initialize corporate API
	api, err := NewCorporateAPI(artemisURL, extSMTSURL, queueName, ldapUsername, ldapPassword)
	if err != nil {
		log.Fatalf("Failed to initialize corporate API: %v", err)
	}
	defer api.stompConn.Disconnect()

	// Start consuming messages from ArtemisMQ for Flow 2
	api.StartMessageConsumer()

	// Setup routes
	routes := map[string]http.HandlerFunc{
		"/health": HealthHandler("corporate-api"),
		"/":       api.HandleRoot,
	}

	// Log additional startup information
	log.Printf("ArtemisMQ URL: %s", artemisURL)
	log.Printf("EXT-SMTS URL: %s", extSMTSURL)
	log.Printf("Artemis Queue: %s", queueName)

	// Start server
	if err := StartServer(port, "Corporate API Server", routes); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
