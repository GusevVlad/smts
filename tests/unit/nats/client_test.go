package nats_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"smts/internal/nats"
	"smts/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// isNATSServerAvailable checks if an external NATS server is running on the specified port
func isNATSServerAvailable(host string, port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, fmt.Sprintf("%d", port)), 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func TestNewClient(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	tests := []struct {
		name          string
		config        *types.NATSConfig
		wantErr       bool
		wantEmbedded  bool
	}{
		{
			name: "success with embedded server",
			config: &types.NATSConfig{
				Embedded: true,
				Host:     "localhost",
				Port:     14224, // Use unique port for unit tests
			},
			wantErr:      false,
			wantEmbedded: true,
		},
		{
			name: "success with external server",
			config: &types.NATSConfig{
				Embedded: false,
				Host:     "localhost",
				Port:     14225, // Use unique port for unit tests
			},
			wantErr:      false,
			wantEmbedded: false,
		},
		{
			name: "invalid port",
			config: &types.NATSConfig{
				Embedded: true,
				Host:     "localhost",
				Port:     -1, // invalid port (negative value)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip external server test if no NATS server is available
			if tt.name == "success with external server" && !isNATSServerAvailable(tt.config.Host, tt.config.Port) {
				t.Skip("Skipping external server test: no NATS server available on localhost:14225")
			}
			
			client, err := nats.NewClient(tt.config, logger)
			
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				
				// Cleanup
				if client != nil {
					client.Close()
				}
			}
		})
	}
}

func TestClient_HealthCheck(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	config := &types.NATSConfig{
		Embedded: true,
		Host:     "localhost",
		Port:     14226, // Use unique port for unit tests
	}
	
	client, err := nats.NewClient(config, logger)
	require.NoError(t, err)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = client.HealthCheck(ctx)
	assert.NoError(t, err)
}

func TestClient_CreateStream(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	config := &types.NATSConfig{
		Embedded: true,
		Host:     "localhost",
		Port:     14227, // Use unique port for unit tests
	}
	
	client, err := nats.NewClient(config, logger)
	require.NoError(t, err)
	defer client.Close()

	streamConfig := &types.StreamConfig{
		Name:      "test-stream",
		Subjects:  []string{"test.>"},
		Retention: "workqueue",
		MaxAge:    "1h",
		Storage:   "memory",
		Replicas:  1,
	}

	err = client.CreateStream(streamConfig)
	assert.NoError(t, err)

	// Verify stream was created
	info, err := client.GetStreamInfo("test-stream")
	assert.NoError(t, err)
	assert.Equal(t, "test-stream", info.Config.Name)
	assert.Equal(t, []string{"test.>"}, info.Config.Subjects)
}

func TestClient_DeleteStream(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	config := &types.NATSConfig{
		Embedded: true,
		Host:     "localhost",
		Port:     14228, // Use unique port for unit tests
	}
	
	client, err := nats.NewClient(config, logger)
	require.NoError(t, err)
	defer client.Close()

	// Create stream first
	streamConfig := &types.StreamConfig{
		Name:     "test-delete-stream",
		Subjects: []string{"delete.>"},
	}
	err = client.CreateStream(streamConfig)
	require.NoError(t, err)

	// Delete the stream
	err = client.DeleteStream("test-delete-stream")
	assert.NoError(t, err)

	// Verify stream was deleted
	_, err = client.GetStreamInfo("test-delete-stream")
	assert.Error(t, err)
}