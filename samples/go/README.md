# Go Microservices Code Samples

This directory contains production code samples from the Lilo Engine's Go microservices layer, demonstrating enterprise patterns for healthcare AI systems.

## Files

| File | Description | Key Patterns |
|------|-------------|--------------|
| `websocket_hub.go` | Real-time therapeutic chat | WebSocket hub, Redis pub/sub, presence management |
| `auth_middleware.go` | HIPAA-compliant authentication | JWT + RBAC, session management, audit logging |
| `crisis_service.go` | Crisis detection and response | gRPC streaming, escalation workflows, care team coordination |
| `service_mesh.go` | Microservices infrastructure | Service discovery, circuit breakers, load balancing |
| `grpc_streaming.go` | Bidirectional streaming | Voice pipeline, real-time metrics, crisis alerts |

## Architecture Highlights

### Service Mesh Pattern
- **Service Discovery**: Redis-based registration with health checks
- **Load Balancing**: Round-robin, weighted, least-connections strategies
- **Circuit Breakers**: Automatic failure isolation and recovery
- **Sidecar Proxy**: Request routing and observability

### Real-Time Communication
- **WebSocket**: Therapeutic chat with crisis detection
- **gRPC Streaming**: Voice pipeline, metrics, alerts
- **Redis Pub/Sub**: Cross-instance message delivery

### Healthcare Compliance
- **HIPAA**: PHI protection, audit logging, session management
- **Crisis Response**: <30s detection, automatic escalation, 911 integration
- **RBAC**: Role-based access control for resident, family, staff, provider, admin

## Technology Stack

- **Go 1.25** with workspace pattern
- **Gin** for HTTP routing
- **gRPC** for inter-service communication
- **Redis** for caching and pub/sub
- **PostgreSQL** for persistence

## Integration Points

These services integrate with:
- Python AI Router (port 8100)
- Embedding Service (port 8005)
- Generation Service (port 8006)
- Voice Service (port 8007)

## Performance Targets

| Metric | Target | Achieved |
|--------|--------|----------|
| Crisis Detection | <30s | <1s |
| WebSocket Latency | <100ms | <50ms |
| Health Check Interval | 10s | 10s |
| Circuit Breaker Recovery | 30s | 30s |

