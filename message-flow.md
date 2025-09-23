# SMTS Message Flow Diagrams

## System Message Flow

```mermaid
flowchart TD
    subgraph EXT Network
        EXT_NATS[EXT NATS JetStream]
        EXT_SMTS[EXT SMTS Service]
    end

    subgraph INT Network  
        INT_NATS[INT NATS JetStream]
        INT_SMTS[INT SMTS Service]
        DLP[DLP Validation]
    end

    subgraph Corporate API
        API[Corporate API Server]
        ARTEMIS[ArtemisMQ]
    end

    %% EXT Network Flow
    EXT_NATS --> EXT_SMTS
    EXT_SMTS --> API

    %% INT Network Flow
    INT_NATS --> INT_SMTS
    INT_SMTS --> DLP
    DLP --> API

    %% Cross-Network Flow
    API --> ARTEMIS
    ARTEMIS --> INT_SMTS
    INT_SMTS --> INT_NATS
```

## Detailed EXT SMTS Flow

```mermaid
sequenceDiagram
    participant C as EXT Client
    participant N as EXT NATS
    participant S as EXT SMTS
    participant A as Corporate API

    C->>N: Publish message to topic
    N->>S: Consumer pulls message
    S->>S: Validate topic permissions
    S->>A: POST /topic_name with message
    A->>S: 200 OK
    S->>N: Acknowledge message
```

## Detailed INT SMTS Flow

```mermaid
sequenceDiagram
    participant C as INT Client
    participant N as INT NATS
    participant S as INT SMTS
    participant D as DLP Service
    participant A as Corporate API
    participant M as ArtemisMQ

    C->>N: Publish message to topic
    N->>S: Consumer pulls message
    S->>S: Validate topic permissions
    S->>D: POST /validate for DLP check
    D->>S: Validation result
    alt Validation Passed
        S->>A: POST /topic_name with message
        A->>S: 200 OK
        S->>N: Acknowledge message
    else Validation Failed
        S->>S: Log rejection
        S->>N: Acknowledge message (discard)
    end

    Note over A,M: Cross-network message flow
    A->>M: Push message to ArtemisMQ
    M->>S: INT SMTS consumes from Artemis
    S->>N: Publish to INT NATS stream
```

## Error Handling Flow

```mermaid
flowchart TD
    Start[Process Message] --> Validate{Validate Permissions}
    Validate -->|Invalid| LogError[Log Permission Error]
    Validate -->|Valid| Process
    
    subgraph Process
        Direction{Network Type}
        Direction -->|EXT| SendAPI[Send to Corporate API]
        Direction -->|INT| DLPCheck[DLP Validation]
        
        DLPCheck -->|Passed| SendAPI
        DLPCheck -->|Failed| LogDLPError[Log DLP Rejection]
    end

    SendAPI --> APIResult{API Response}
    APIResult -->|Success| Ack[Acknowledge Message]
    APIResult -->|Temporary Error| Retry[Retry with Backoff]
    APIResult -->|Permanent Error| DeadLetter[Dead Letter Queue]
    
    Retry -->|Max Retries| DeadLetter
    Retry -->|Success| Ack
    
    LogError --> Discard[Discard Message]
    LogDLPError --> Discard
    DeadLetter --> Finish[Finish Processing]
    Ack --> Finish
    Discard --> Finish
```

## Configuration-Driven Behavior

The system uses configuration to determine behavior:

### EXT SMTS Configuration
```yaml
deployment: "ext"
nats:
  stream: "SMTS_EXT"
  consumer: "SMTS_EXT_CONSUMER"
api:
  endpoint: "https://api.corporate.com"
  auth_type: "api_key"
flow:
  dlp_validation: false
  artemis_consumer: false
```

### INT SMTS Configuration
```yaml
deployment: "int" 
nats:
  stream: "SMTS_INT"
  consumer: "SMTS_INT_CONSUMER"
api:
  endpoint: "https://api.corporate.com"
  auth_type: "api_key"
dlp:
  endpoint: "https://dlp.corporate.com/validate"
flow:
  dlp_validation: true
  artemis_consumer: true
```

## Security Considerations

1. **Network Isolation**: EXT and INT networks are physically separated
2. **API Authentication**: All corporate API calls use API key authentication
3. **Message Encryption**: Messages are base64 encoded for transport
4. **Access Control**: Topic-level permissions enforced via configuration
5. **DLP Validation**: All INT messages undergo data loss prevention checks