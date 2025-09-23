# SMTS Deployment Guide

This guide covers the deployment of SMTS (Secure Message Transport System) for both EXT and INT network environments.

## Prerequisites

### System Requirements

- **Operating System**: Linux, macOS, or Windows
- **Go Version**: 1.25 (for source deployment)
- **Docker**: 20.10 or later (for container deployment)
- **Memory**: Minimum 512MB, Recommended 1GB
- **Storage**: Minimum 1GB for NATS data storage

### Network Requirements

- **EXT Network**: Outbound access to corporate API endpoints
- **INT Network**: Outbound access to corporate API and DLP endpoints
- **ArtemisMQ**: Required for INT deployment (port 61616)
- **Health Checks**: Port 8080 for health endpoints

## Deployment Methods

### 1. Source Deployment

#### Building from Source

```bash
# Clone the repository
git clone <repository-url>
cd smts

# Build both deployments
make build

# Verify builds
ls -la bin/
```

#### Running EXT Deployment

```bash
# Set required environment variables
export API_KEY=your_corporate_api_key

# Run EXT SMTS
./bin/ext-smts --config configs/ext-config.yaml
```

#### Running INT Deployment

```bash
# Set required environment variables
export API_KEY=your_corporate_api_key
export ARTEMIS_USER=artemis_username
export ARTEMIS_PASSWORD=artemis_password

# Run INT SMTS
./bin/int-smts --config configs/int-config.yaml
```

### 2. Docker Deployment

#### Building Docker Images

```bash
# Build both images
make docker-build

# Or build individually
docker build -t smts-ext:latest .
docker build -t smts-int:latest .
```

#### Running EXT Docker Container

```bash
docker run -d \
  --name smts-ext \
  -p 8080:8080 \
  -e API_KEY=your_corporate_api_key \
  -v $(pwd)/configs:/app/configs \
  smts-ext:latest
```

#### Running INT Docker Container

```bash
docker run -d \
  --name smts-int \
  -p 8080:8080 \
  -e API_KEY=your_corporate_api_key \
  -e ARTEMIS_USER=artemis_username \
  -e ARTEMIS_PASSWORD=artemis_password \
  -v $(pwd)/configs:/app/configs \
  smts-int:latest
```

### 3. Kubernetes Deployment

#### Namespace Setup

```yaml
# k8s/namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: smts
```

#### EXT Deployment

```yaml
# k8s/ext-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: smts-ext
  namespace: smts
spec:
  replicas: 2
  selector:
    matchLabels:
      app: smts-ext
  template:
    metadata:
      labels:
        app: smts-ext
    spec:
      containers:
      - name: smts-ext
        image: smts-ext:latest
        ports:
        - containerPort: 8080
        env:
        - name: API_KEY
          valueFrom:
            secretKeyRef:
              name: smts-secrets
              key: api-key
        volumeMounts:
        - name: config-volume
          mountPath: /app/configs
        livenessProbe:
          httpGet:
            path: /live
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: config-volume
        configMap:
          name: smts-config
```

#### INT Deployment

```yaml
# k8s/int-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: smts-int
  namespace: smts
spec:
  replicas: 2
  selector:
    matchLabels:
      app: smts-int
  template:
    metadata:
      labels:
        app: smts-int
    spec:
      containers:
      - name: smts-int
        image: smts-int:latest
        ports:
        - containerPort: 8080
        env:
        - name: API_KEY
          valueFrom:
            secretKeyRef:
              name: smts-secrets
              key: api-key
        - name: ARTEMIS_USER
          valueFrom:
            secretKeyRef:
              name: smts-secrets
              key: artemis-user
        - name: ARTEMIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: smts-secrets
              key: artemis-password
        volumeMounts:
        - name: config-volume
          mountPath: /app/configs
        livenessProbe:
          httpGet:
            path: /live
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: config-volume
        configMap:
          name: smts-config
```

## Configuration Management

### Environment-Specific Configuration

#### Development Configuration

```yaml
# configs/ext-config-dev.yaml
deployment:
  type: "ext"
  name: "smts-ext-dev"
  environment: "development"

nats:
  embedded: true
  host: "localhost"
  port: 4222

logging:
  level: "debug"
  format: "console"
```

#### Production Configuration

```yaml
# configs/ext-config-prod.yaml
deployment:
  type: "ext"
  name: "smts-ext-prod"
  environment: "production"

nats:
  embedded: true
  host: "nats.prod.internal"
  port: 4222

logging:
  level: "info"
  format: "json"
```

### Secrets Management

#### Kubernetes Secrets

```bash
# Create secrets
kubectl create secret generic smts-secrets \
  --namespace=smts \
  --from-literal=api-key=your_api_key \
  --from-literal=artemis-user=artemis_user \
  --from-literal=artemis-password=artemis_password
```

#### Docker Secrets

```bash
# Use Docker secrets or environment files
echo "API_KEY=your_api_key" > .env
docker run --env-file .env smts-ext:latest
```

## Monitoring and Health Checks

### Health Endpoints

- `GET /health` - Comprehensive health status
- `GET /ready` - Readiness probe (for Kubernetes)
- `GET /live` - Liveness probe (for Kubernetes)
- `GET /metrics` - Prometheus metrics (future)

### Health Check Script

```bash
#!/bin/bash
# health-check.sh

URL="http://localhost:8080/health"
response=$(curl -s -w "%{http_code}" $URL)
http_code=$(tail -n1 <<< "$response")
content=$(sed '$ d' <<< "$response")

if [ $http_code -eq 200 ]; then
    echo "Health check PASSED"
    exit 0
else
    echo "Health check FAILED: HTTP $http_code"
    echo "$content"
    exit 1
fi
```

### Log Monitoring

```bash
# Tail logs for debugging
docker logs -f smts-ext

# Search for errors
docker logs smts-ext | grep -i error

# Structured log analysis (JSON format)
docker logs smts-ext | jq '. | select(.level == "error")'
```

## Performance Tuning

### NATS Configuration

```yaml
nats:
  stream:
    max_age: "24h"  # Adjust based on retention needs
    storage: "file" # Use "memory" for better performance
    replicas: 3     # For high availability
```

### Resource Limits

#### Docker Resource Limits

```bash
docker run -d \
  --name smts-ext \
  --memory=1g \
  --cpus=2 \
  smts-ext:latest
```

#### Kubernetes Resource Limits

```yaml
resources:
  requests:
    memory: "512Mi"
    cpu: "500m"
  limits:
    memory: "1Gi"
    cpu: "2"
```

## High Availability

### EXT Deployment HA

```yaml
# Run multiple instances with load balancing
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
```

### INT Deployment HA

```yaml
# Multiple instances with shared Artemis queue
spec:
  replicas: 3
```

## Backup and Recovery

### NATS Data Backup

```bash
# Backup NATS data directory
tar -czf nats-backup-$(date +%Y%m%d).tar.gz /app/data/nats

# Restore from backup
tar -xzf nats-backup-20250101.tar.gz -C /app/data/
```

### Configuration Backup

```bash
# Backup configuration files
tar -czf config-backup-$(date +%Y%m%d).tar.gz configs/
```

## Troubleshooting

### Common Issues

1. **NATS Connection Issues**
   ```bash
   # Check NATS server
   nc -zv localhost 4222
   
   # Check embedded NATS logs
   docker logs smts-ext | grep -i nats
   ```

2. **API Authentication Failures**
   ```bash
   # Verify API key
   echo $API_KEY
   
   # Test API connectivity
   curl -H "X-API-Key: $API_KEY" https://api.corporate.com/health
   ```

3. **ArtemisMQ Connection Issues**
   ```bash
   # Test Artemis connectivity
   nc -zv artemis.corporate.com 61616
   ```

### Debug Mode

Enable debug logging for troubleshooting:

```yaml
logging:
  level: "debug"
  format: "console"
```

## Security Considerations

### Network Security

- Use TLS for all external communications
- Implement network segmentation
- Use firewall rules to restrict access

### Application Security

- Regularly rotate API keys
- Use secure secret management
- Implement audit logging
- Regular security updates

## Maintenance

### Regular Tasks

- Monitor disk usage for NATS data
- Review application logs for errors
- Update configuration as needed
- Perform health checks regularly

### Upgrade Procedure

1. Backup current deployment
2. Deploy new version alongside old version
3. Verify health and functionality
4. Switch traffic to new version
5. Decommission old version

## Support

For deployment issues:
1. Check the troubleshooting section
2. Review application logs
3. Verify configuration files
4. Contact support team if needed