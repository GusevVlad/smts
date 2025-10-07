# SMTS Production Deployment Guide

This guide describes how to deploy SMTS (Secure Message Transfer Service) in production using separate Docker Compose files for external and internal deployments on different VMs.

## Overview

The system consists of two separate deployments:
- **SMTS-EXT**: External-facing service for receiving messages
- **SMTS-INT**: Internal service for processing messages with DLP validation

## Prerequisites

- Docker and Docker Compose installed on both VMs
- Persistent storage volumes configured
- Network connectivity between services (if needed)
- Environment variables configured

## Deployment Files

### External SMTS (SMTS-EXT)
- `docker-compose.ext-prod.yml` - Production deployment for external service
- `.env.ext-prod` - Environment variables for external service

### Internal SMTS (SMTS-INT)  
- `docker-compose.int-prod.yml` - Production deployment for internal service
- `.env.int-prod` - Environment variables for internal service

## Deployment Steps

### 1. External SMTS Deployment

On the external VM:

```bash
# Copy deployment files to external VM
scp docker-compose.ext-prod.yml .env.ext-prod user@ext-vm:/opt/smts/

# Deploy external SMTS
cd /opt/smts
docker-compose -f docker-compose.ext-prod.yml up -d

# Check service status
docker-compose -f docker-compose.ext-prod.yml ps
docker-compose -f docker-compose.ext-prod.yml logs -f
```

### 2. Internal SMTS Deployment

On the internal VM:

```bash
# Copy deployment files to internal VM
scp docker-compose.int-prod.yml .env.int-prod user@int-vm:/opt/smts/

# Deploy internal SMTS
cd /opt/smts
docker-compose -f docker-compose.int-prod.yml up -d

# Check service status
docker-compose -f docker-compose.int-prod.yml ps
docker-compose -f docker-compose.int-prod.yml logs -f
```

## Configuration

### Environment Variables

Update the environment files with your production values:

**External SMTS (.env.ext-prod):**
```bash
API_KEY=your-production-api-key-here
```

**Internal SMTS (.env.int-prod):**
```bash
API_KEY=your-production-api-key-here
ARTEMIS_USER=admin
ARTEMIS_PASSWORD=your-secure-artemis-password-here
ARTEMIS_HOST=your-artemis-host
ARTEMIS_PORT=61613
DLP_ENDPOINT=https://dlp.corporate.com/validate
```

### Persistent Volumes

Both deployments use persistent volumes for data storage:

- **External SMTS**: `smts-ext-data` - Stores NATS stream data
- **Internal SMTS**: `smts-int-data` - Stores NATS stream data

## Port Configuration

### External SMTS Ports
- `4222` - NATS server
- `8080` - Health checks
- `8082` - HTTP server (send messages)
- `8081` - Message API (receive messages)

### Internal SMTS Ports  
- `4223` - NATS server (different port to avoid conflicts)
- `8083` - Health checks
- `8084` - HTTP server
- `8085` - Message API

## Health Checks

Both services include health checks that run every 30 seconds:

```bash
# Check external SMTS health
curl http://ext-vm:8080/health

# Check internal SMTS health  
curl http://int-vm:8083/health
```

## Monitoring

### Logs
```bash
# View external SMTS logs
docker-compose -f docker-compose.ext-prod.yml logs -f

# View internal SMTS logs
docker-compose -f docker-compose.int-prod.yml logs -f
```

### Service Status
```bash
# Check external SMTS status
docker-compose -f docker-compose.ext-prod.yml ps

# Check internal SMTS status
docker-compose -f docker-compose.int-prod.yml ps
```

## Maintenance

### Updates
```bash
# Update external SMTS
docker-compose -f docker-compose.ext-prod.yml pull
docker-compose -f docker-compose.ext-prod.yml up -d

# Update internal SMTS
docker-compose -f docker-compose.int-prod.yml pull  
docker-compose -f docker-compose.int-prod.yml up -d
```

### Backups
Backup the persistent volumes regularly:
```bash
# Backup external SMTS data
docker run --rm -v smts-ext-data:/source -v /backup:/backup alpine tar czf /backup/smts-ext-data-$(date +%Y%m%d).tar.gz -C /source .

# Backup internal SMTS data
docker run --rm -v smts-int-data:/source -v /backup:/backup alpine tar czf /backup/smts-int-data-$(date +%Y%m%d).tar.gz -C /source .
```

## Troubleshooting

### Common Issues

1. **Service won't start**: Check environment variables and port availability
2. **Health checks failing**: Verify API connectivity and configuration
3. **Volume permissions**: Ensure Docker has write access to volume directories

### Debug Mode
For troubleshooting, you can run in debug mode:
```bash
docker-compose -f docker-compose.ext-prod.yml logs -f --tail=100
```

## Security Considerations

- Use strong passwords for API keys and Artemis credentials
- Configure firewall rules to restrict access to necessary ports only
- Regularly update Docker images and dependencies
- Monitor logs for suspicious activity