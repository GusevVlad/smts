package nats_test

import (
	"context"
	"testing"
	"time"

	"github.com/corporate/smts/internal/nats"
	"github.com/corporate/smts/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

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
				Port:     4222,
			},
			wantErr:      false,
			wantEmbedded: true,
		},
		{
			name: "success with external server",
			config: &types.NATSConfig{
				Embedded: false,
				Host:     "localhost",
				Port:     4222,
			},
			wantErr:      false,
			wantEmbedded: false,
		},
		{
			name: "invalid port",
			config: &types.NATSConfig{
				Embedded: true,
				Host:     "localhost",
				Port:     0, // invalid port
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
		Port:     4222,
	}
	
	client, err := nats.NewClient(config, logger)
	require.NoError(t, err)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.HealthCheck(ctx)
	assert.NoError(t, err)
}

func TestClient_CreateStream(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	config := &types.NATSConfig{
		Embedded: true,
		Host:     "localhost",
		Port:     4222,
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
		Port:     4222,
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