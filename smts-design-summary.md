# SMTS Design Summary and Implementation Approval

## Project Overview

**SMTS (Secure Message Transport System)** is a Golang service designed for secure message transport between isolated corporate networks (EXT and INT) using NATS JetStream as the message backbone.

## Key Design Decisions

### 1. Unified Codebase with Configuration-Driven Behavior
- Single codebase for both EXT and INT deployments
- Behavior determined by configuration files
- Reduces maintenance overhead and ensures consistency

### 2. Embedded NATS JetStream
- Self-contained message broker within each deployment
- No external NATS dependency required
- Simplified deployment and operation

### 3. JSON Raw Type for Message Body
- Flexible message format that preserves original structure
- Supports various message types without schema changes
- Base64 encoding for binary data support

### 4. Configuration-Based Security
- Topics and privileges defined in YAML configuration
- Role-based access control for message operations
- Easy management without code changes

## Architecture Highlights

### Message Flow Patterns

**EXT Network:**
```
NATS JetStream → EXT SMTS → Corporate API (/topic_name)
```

**INT Network:**
```
NATS JetStream → INT SMTS → DLP Validation → Corporate API (/topic_name)
```

**Cross-Network:**
```
Corporate API → ArtemisMQ → INT SMTS → NATS JetStream
```

### Technology Stack
- **Language**: Go 1.25
- **Message Broker**: NATS JetStream (embedded)
- **Queue**: ArtemisMQ (STOMP protocol)
- **Configuration**: YAML with Viper
- **Logging**: Structured logging with Zap
- **Testing**: Go testing framework with Testify

## Implementation Approach

### Phase-Based Development
1. **Core Infrastructure** (Week 1-2): NATS, configuration, message schema
2. **EXT SMTS** (Week 2-3): Corporate API integration, basic flow
3. **INT SMTS** (Week 3-4): DLP validation, ArtemisMQ integration
4. **Advanced Features** (Week 4-5): Error handling, monitoring, security
5. **Testing & Documentation** (Week 5-6): Comprehensive testing, deployment guides

### Key Features Implemented
- ✅ Embedded NATS JetStream with stream management
- ✅ Configuration-driven deployment behavior
- ✅ Corporate API integration with authentication
- ✅ DLP validation for INT network messages
- ✅ ArtemisMQ consumer for cross-network flow
- ✅ Topic-based permission system
- ✅ Comprehensive error handling and logging
- ✅ Health checks and monitoring

## Configuration Management

### Deployment-Specific Configs
- `ext-config.yaml`: EXT network settings (no DLP, no Artemis)
- `int-config.yaml`: INT network settings (with DLP and Artemis)
- `topics.yaml`: Unified topic and privilege definitions

### Environment Variables
- API keys for corporate API and DLP service
- ArtemisMQ credentials for INT deployment
- Logging and monitoring settings

## Security Considerations

### Network Security
- EXT and INT networks physically isolated
- All external calls use TLS encryption
- API key authentication for corporate endpoints

### Data Security
- DLP validation for all INT-bound messages
- No sensitive data in logs
- Message content base64 encoded for transport

### Access Control
- Topic-level permission enforcement
- Role-based access via configuration
- Audit logging for security events

## Performance Targets
- **Throughput**: 1000+ messages/second
- **Latency**: < 100ms (EXT), < 500ms (INT with DLP)
- **Availability**: 99.9% uptime
- **Memory**: < 512MB per instance

## Next Steps for Implementation

The design is complete and ready for implementation. The next phase involves switching to Code mode to begin developing the core infrastructure.

### Immediate Implementation Tasks
1. Set up Go module and project structure
2. Implement NATS JetStream integration
3. Create message schema and configuration system
4. Build EXT SMTS deployment with corporate API integration

## Approval Request

This comprehensive design addresses all requirements:
- ✅ Secure message transport between isolated networks
- ✅ Unified codebase for EXT and INT deployments  
- ✅ NATS JetStream with embedded server
- ✅ JSON raw type for message bodies
- ✅ Configuration-based topics and privileges
- ✅ DLP validation for INT network
- ✅ ArtemisMQ integration for cross-network flow

**Ready to proceed with implementation?**