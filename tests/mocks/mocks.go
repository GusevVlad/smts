package mocks

import (
	"context"
	"encoding/json"
	"time"

	"github.com/corporate/smts/internal/message"
	"github.com/corporate/smts/pkg/types"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// MockAPIClient mocks the corporate API client
type MockAPIClient struct {
	mock.Mock
}

func NewMockAPIClient() *MockAPIClient {
	return &MockAPIClient{}
}

func (m *MockAPIClient) DeliverMessage(ctx context.Context, msg *types.Message) (*types.DeliveryResult, error) {
	args := m.Called(ctx, msg)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.DeliveryResult), args.Error(1)
}

func (m *MockAPIClient) ValidateMessage(ctx context.Context, msg *types.Message) (*types.DLPValidationResponse, error) {
	args := m.Called(ctx, msg)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.DLPValidationResponse), args.Error(1)
}

func (m *MockAPIClient) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockArtemisClient mocks the ArtemisMQ client
type MockArtemisClient struct {
	mock.Mock
}

func NewMockArtemisClient() *MockArtemisClient {
	return &MockArtemisClient{}
}

func (m *MockArtemisClient) Start(ctx context.Context, processor *message.Processor) error {
	args := m.Called(ctx, processor)
	return args.Error(0)
}

func (m *MockArtemisClient) PublishMessage(msg *types.Message) error {
	args := m.Called(msg)
	return args.Error(0)
}

func (m *MockArtemisClient) Stop() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockArtemisClient) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockArtemisClient) IsConnected() bool {
	args := m.Called()
	return args.Bool(0)
}

// MockNATSClient mocks the NATS client
type MockNATSClient struct {
	mock.Mock
}

func NewMockNATSClient() *MockNATSClient {
	return &MockNATSClient{}
}

func (m *MockNATSClient) CreateStream(streamConfig *types.StreamConfig) error {
	args := m.Called(streamConfig)
	return args.Error(0)
}

func (m *MockNATSClient) GetStreamInfo(streamName string) (*nats.StreamInfo, error) {
	args := m.Called(streamName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*nats.StreamInfo), args.Error(1)
}

func (m *MockNATSClient) DeleteStream(streamName string) error {
	args := m.Called(streamName)
	return args.Error(0)
}

func (m *MockNATSClient) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockNATSClient) Close() {
	m.Called()
}

func (m *MockNATSClient) GetJetStream() nats.JetStreamContext {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(nats.JetStreamContext)
}

func (m *MockNATSClient) GetConnection() *nats.Conn {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*nats.Conn)
}

func (m *MockNATSClient) IsConnected() bool {
	args := m.Called()
	return args.Bool(0)
}

// MockMessageHandler mocks the message handler interface
type MockMessageHandler struct {
	mock.Mock
}

func NewMockMessageHandler() *MockMessageHandler {
	return &MockMessageHandler{}
}

func (m *MockMessageHandler) HandleMessage(ctx context.Context, msg *types.Message) (*types.DeliveryResult, error) {
	args := m.Called(ctx, msg)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.DeliveryResult), args.Error(1)
}

// MockLogger mocks the logger
type MockLogger struct {
	mock.Mock
}

func NewMockLogger() *MockLogger {
	return &MockLogger{}
}

func (m *MockLogger) Debug(msg string, fields ...zap.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Info(msg string, fields ...zap.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Warn(msg string, fields ...zap.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Error(msg string, fields ...zap.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Fatal(msg string, fields ...zap.Field) {
	m.Called(msg, fields)
}

// HTTP Client Mocks
type MockHTTPClient struct {
	mock.Mock
}

func (m *MockHTTPClient) R() *MockRequest {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*MockRequest)
}

func (m *MockHTTPClient) SetTimeout(timeout time.Duration) *MockHTTPClient {
	m.Called(timeout)
	return m
}

func (m *MockHTTPClient) SetRetryCount(count int) *MockHTTPClient {
	m.Called(count)
	return m
}

func (m *MockHTTPClient) SetHeader(key, value string) *MockHTTPClient {
	m.Called(key, value)
	return m
}

type MockRequest struct {
	mock.Mock
}

func (m *MockRequest) SetContext(ctx context.Context) *MockRequest {
	m.Called(ctx)
	return m
}

func (m *MockRequest) SetBody(body interface{}) *MockRequest {
	m.Called(body)
	return m
}

func (m *MockRequest) SetHeader(key, value string) *MockRequest {
	m.Called(key, value)
	return m
}

func (m *MockRequest) Post(url string) (*MockResponse, error) {
	args := m.Called(url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*MockResponse), args.Error(1)
}

func (m *MockRequest) Get(url string) (*MockResponse, error) {
	args := m.Called(url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*MockResponse), args.Error(1)
}

type MockResponse struct {
	mock.Mock
	statusCode int
	body       []byte
}

func (m *MockResponse) StatusCode() int {
	return m.statusCode
}

func (m *MockResponse) String() string {
	return string(m.body)
}

func (m *MockResponse) Body() []byte {
	return m.body
}

func (m *MockResponse) SetStatusCode(code int) {
	m.statusCode = code
}

func (m *MockResponse) SetBody(body []byte) {
	m.body = body
}

// Test utilities
func CreateTestMessage(topic string, body interface{}) *types.Message {
	bodyJSON, _ := json.Marshal(body)
	return &types.Message{
		ID:        "test-message-" + time.Now().Format("20060102150405"),
		Timestamp: time.Now().UTC(),
		Topic:     topic,
		Source:    "test",
		Headers:   make(map[string]string),
		Body:      bodyJSON,
	}
}

func CreateTestConfig(deploymentType string) *types.Config {
	config := types.DefaultConfig()
	config.Deployment.Type = deploymentType
	
	if deploymentType == "int" {
		config.DLP.Enabled = true
		config.Artemis.Enabled = true
	}
	
	// Configure test topics
	config.Topics.Topics = map[string]types.TopicPermission{
		"monterra.event": {
			ReadRoles:   []string{"reader"},
			WriteRoles:  []string{"writer"},
			Description: "Test topic for monterra events",
		},
	}
	
	return config
}

// Mock types for NATS interfaces
type MockJetStreamContext struct {
	mock.Mock
}

func (m *MockJetStreamContext) AddStream(cfg *nats.StreamConfig) (*nats.StreamInfo, error) {
	args := m.Called(cfg)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*nats.StreamInfo), args.Error(1)
}

func (m *MockJetStreamContext) UpdateStream(cfg *nats.StreamConfig) (*nats.StreamInfo, error) {
	args := m.Called(cfg)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*nats.StreamInfo), args.Error(1)
}

func (m *MockJetStreamContext) DeleteStream(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *MockJetStreamContext) StreamInfo(name string) (*nats.StreamInfo, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*nats.StreamInfo), args.Error(1)
}

type MockConnection struct {
	mock.Mock
}

func (m *MockConnection) Publish(subject string, data []byte) error {
	args := m.Called(subject, data)
	return args.Error(0)
}

func (m *MockConnection) SubscribeSync(subject string) (*nats.Subscription, error) {
	args := m.Called(subject)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*nats.Subscription), args.Error(1)
}

func (m *MockConnection) IsConnected() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockConnection) IsClosed() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockConnection) Close() {
	m.Called()
}

type MockSubscription struct {
	mock.Mock
}

func (m *MockSubscription) NextMsgWithContext(ctx context.Context) (*nats.Msg, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*nats.Msg), args.Error(1)
}

func (m *MockSubscription) Unsubscribe() error {
	args := m.Called()
	return args.Error(0)
}