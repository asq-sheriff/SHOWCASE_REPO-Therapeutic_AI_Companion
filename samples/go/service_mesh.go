// Package mesh provides service mesh infrastructure for the Lilo Engine
// microservices architecture with service discovery, load balancing,
// circuit breaking, and observability.
package mesh

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
)

// ServiceType defines the type of microservice
type ServiceType string

const (
	ServiceTypeAIRouter     ServiceType = "ai-router"
	ServiceTypeEmbedding    ServiceType = "embedding"
	ServiceTypeGeneration   ServiceType = "generation"
	ServiceTypeVoice        ServiceType = "voice"
	ServiceTypeAuth         ServiceType = "auth"
	ServiceTypeWebSocket    ServiceType = "websocket"
	ServiceTypeCrisis       ServiceType = "crisis"
	ServiceTypeCareManager  ServiceType = "care-manager"
	ServiceTypeResident     ServiceType = "resident-dashboard"
	ServiceTypeFamily       ServiceType = "family-dashboard"
	ServiceTypeStaff        ServiceType = "staff-dashboard"
	ServiceTypeAdmin        ServiceType = "admin-dashboard"
	ServiceTypeAnalytics    ServiceType = "analytics"
	ServiceTypeAudit        ServiceType = "audit"
	ServiceTypeGateway      ServiceType = "api-gateway"
	ServiceTypePostgres     ServiceType = "postgres"
	ServiceTypeRedis        ServiceType = "redis"
)

// ServiceInstance represents a running service instance
type ServiceInstance struct {
	ID          string            `json:"id"`
	Type        ServiceType       `json:"type"`
	Host        string            `json:"host"`
	Port        int               `json:"port"`
	GRPCPort    int               `json:"grpc_port,omitempty"`
	Version     string            `json:"version"`
	Status      InstanceStatus    `json:"status"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	StartedAt   time.Time         `json:"started_at"`
	LastHealthCheck time.Time     `json:"last_health_check"`
	HealthCheckURL  string        `json:"health_check_url"`
	Weight      int               `json:"weight"` // For weighted load balancing
}

// InstanceStatus represents the health status of an instance
type InstanceStatus string

const (
	InstanceStatusHealthy   InstanceStatus = "healthy"
	InstanceStatusUnhealthy InstanceStatus = "unhealthy"
	InstanceStatusDraining  InstanceStatus = "draining"
	InstanceStatusStarting  InstanceStatus = "starting"
)

// ServiceRegistry manages service discovery and registration
type ServiceRegistry struct {
	redis       *redis.Client
	logger      *slog.Logger
	localInstance *ServiceInstance
	instances   map[ServiceType][]*ServiceInstance
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc

	// Health check configuration
	healthCheckInterval time.Duration
	healthCheckTimeout  time.Duration
	unhealthyThreshold  int
}

// RegistryConfig contains configuration for the service registry
type RegistryConfig struct {
	RedisURL            string
	HealthCheckInterval time.Duration
	HealthCheckTimeout  time.Duration
	UnhealthyThreshold  int
	RegistrationTTL     time.Duration
}

// DefaultRegistryConfig returns default configuration
func DefaultRegistryConfig() *RegistryConfig {
	return &RegistryConfig{
		HealthCheckInterval: 10 * time.Second,
		HealthCheckTimeout:  5 * time.Second,
		UnhealthyThreshold:  3,
		RegistrationTTL:     30 * time.Second,
	}
}

// NewServiceRegistry creates a new service registry
func NewServiceRegistry(redis *redis.Client, logger *slog.Logger, config *RegistryConfig) *ServiceRegistry {
	ctx, cancel := context.WithCancel(context.Background())

	registry := &ServiceRegistry{
		redis:               redis,
		logger:              logger,
		instances:           make(map[ServiceType][]*ServiceInstance),
		ctx:                 ctx,
		cancel:              cancel,
		healthCheckInterval: config.HealthCheckInterval,
		healthCheckTimeout:  config.HealthCheckTimeout,
		unhealthyThreshold:  config.UnhealthyThreshold,
	}

	// Start background workers
	go registry.syncInstances()
	go registry.healthChecker()

	return registry
}

// Register registers a service instance
func (r *ServiceRegistry) Register(instance *ServiceInstance) error {
	r.localInstance = instance
	instance.Status = InstanceStatusStarting
	instance.StartedAt = time.Now()

	key := fmt.Sprintf("service:%s:%s", instance.Type, instance.ID)
	data, err := json.Marshal(instance)
	if err != nil {
		return fmt.Errorf("failed to marshal instance: %w", err)
	}

	// Register with TTL
	if err := r.redis.Set(r.ctx, key, data, 30*time.Second).Err(); err != nil {
		return fmt.Errorf("failed to register instance: %w", err)
	}

	// Add to service set
	setKey := fmt.Sprintf("services:%s", instance.Type)
	if err := r.redis.SAdd(r.ctx, setKey, instance.ID).Err(); err != nil {
		return fmt.Errorf("failed to add to service set: %w", err)
	}

	// Start heartbeat
	go r.heartbeat(instance)

	r.logger.Info("service registered",
		slog.String("type", string(instance.Type)),
		slog.String("id", instance.ID),
		slog.String("host", instance.Host),
		slog.Int("port", instance.Port),
	)

	return nil
}

// Deregister removes a service instance
func (r *ServiceRegistry) Deregister(instance *ServiceInstance) error {
	key := fmt.Sprintf("service:%s:%s", instance.Type, instance.ID)
	if err := r.redis.Del(r.ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to deregister instance: %w", err)
	}

	setKey := fmt.Sprintf("services:%s", instance.Type)
	if err := r.redis.SRem(r.ctx, setKey, instance.ID).Err(); err != nil {
		return fmt.Errorf("failed to remove from service set: %w", err)
	}

	r.logger.Info("service deregistered",
		slog.String("type", string(instance.Type)),
		slog.String("id", instance.ID),
	)

	return nil
}

// GetInstances returns all healthy instances of a service type
func (r *ServiceRegistry) GetInstances(serviceType ServiceType) []*ServiceInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instances := r.instances[serviceType]
	healthy := make([]*ServiceInstance, 0, len(instances))

	for _, inst := range instances {
		if inst.Status == InstanceStatusHealthy {
			healthy = append(healthy, inst)
		}
	}

	return healthy
}

// GetInstance returns a single healthy instance using load balancing
func (r *ServiceRegistry) GetInstance(serviceType ServiceType, strategy LoadBalanceStrategy) (*ServiceInstance, error) {
	instances := r.GetInstances(serviceType)
	if len(instances) == 0 {
		return nil, fmt.Errorf("no healthy instances of %s available", serviceType)
	}

	switch strategy {
	case LoadBalanceRoundRobin:
		return r.roundRobin(serviceType, instances), nil
	case LoadBalanceRandom:
		return instances[rand.Intn(len(instances))], nil
	case LoadBalanceWeighted:
		return r.weightedRandom(instances), nil
	case LoadBalanceLeastConnections:
		return r.leastConnections(instances), nil
	default:
		return instances[0], nil
	}
}

// LoadBalanceStrategy defines load balancing strategies
type LoadBalanceStrategy int

const (
	LoadBalanceRoundRobin LoadBalanceStrategy = iota
	LoadBalanceRandom
	LoadBalanceWeighted
	LoadBalanceLeastConnections
)

// roundRobinCounters tracks round-robin state per service type
var roundRobinCounters sync.Map

// roundRobin implements round-robin load balancing
func (r *ServiceRegistry) roundRobin(serviceType ServiceType, instances []*ServiceInstance) *ServiceInstance {
	counterI, _ := roundRobinCounters.LoadOrStore(serviceType, new(uint64))
	counter := counterI.(*uint64)

	idx := atomic.AddUint64(counter, 1) % uint64(len(instances))
	return instances[idx]
}

// weightedRandom implements weighted random load balancing
func (r *ServiceRegistry) weightedRandom(instances []*ServiceInstance) *ServiceInstance {
	totalWeight := 0
	for _, inst := range instances {
		if inst.Weight <= 0 {
			inst.Weight = 1
		}
		totalWeight += inst.Weight
	}

	random := rand.Intn(totalWeight)
	for _, inst := range instances {
		random -= inst.Weight
		if random < 0 {
			return inst
		}
	}

	return instances[0]
}

// connectionCounts tracks connection counts per instance
var connectionCounts sync.Map

// leastConnections implements least connections load balancing
func (r *ServiceRegistry) leastConnections(instances []*ServiceInstance) *ServiceInstance {
	var minInst *ServiceInstance
	var minCount int64 = -1

	for _, inst := range instances {
		countI, _ := connectionCounts.LoadOrStore(inst.ID, new(int64))
		count := atomic.LoadInt64(countI.(*int64))

		if minCount < 0 || count < minCount {
			minCount = count
			minInst = inst
		}
	}

	return minInst
}

// heartbeat sends periodic heartbeats to maintain registration
func (r *ServiceRegistry) heartbeat(instance *ServiceInstance) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			key := fmt.Sprintf("service:%s:%s", instance.Type, instance.ID)

			instance.LastHealthCheck = time.Now()
			data, _ := json.Marshal(instance)

			if err := r.redis.Set(r.ctx, key, data, 30*time.Second).Err(); err != nil {
				r.logger.Error("heartbeat failed",
					slog.String("error", err.Error()),
				)
			}
		}
	}
}

// syncInstances synchronizes local instance cache from Redis
func (r *ServiceRegistry) syncInstances() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.refreshInstances()
		}
	}
}

// refreshInstances refreshes the local instance cache
func (r *ServiceRegistry) refreshInstances() {
	allTypes := []ServiceType{
		ServiceTypeAIRouter, ServiceTypeEmbedding, ServiceTypeGeneration,
		ServiceTypeVoice, ServiceTypeAuth, ServiceTypeWebSocket,
		ServiceTypeCrisis, ServiceTypeCareManager, ServiceTypeResident,
		ServiceTypeFamily, ServiceTypeStaff, ServiceTypeAdmin,
		ServiceTypeAnalytics, ServiceTypeAudit, ServiceTypeGateway,
	}

	newInstances := make(map[ServiceType][]*ServiceInstance)

	for _, svcType := range allTypes {
		setKey := fmt.Sprintf("services:%s", svcType)
		ids, err := r.redis.SMembers(r.ctx, setKey).Result()
		if err != nil {
			continue
		}

		instances := make([]*ServiceInstance, 0, len(ids))
		for _, id := range ids {
			key := fmt.Sprintf("service:%s:%s", svcType, id)
			data, err := r.redis.Get(r.ctx, key).Bytes()
			if err != nil {
				continue
			}

			var inst ServiceInstance
			if err := json.Unmarshal(data, &inst); err != nil {
				continue
			}

			instances = append(instances, &inst)
		}

		newInstances[svcType] = instances
	}

	r.mu.Lock()
	r.instances = newInstances
	r.mu.Unlock()
}

// healthChecker performs periodic health checks
func (r *ServiceRegistry) healthChecker() {
	ticker := time.NewTicker(r.healthCheckInterval)
	defer ticker.Stop()

	client := &http.Client{
		Timeout: r.healthCheckTimeout,
	}

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.mu.RLock()
			for _, instances := range r.instances {
				for _, inst := range instances {
					go r.checkHealth(client, inst)
				}
			}
			r.mu.RUnlock()
		}
	}
}

// checkHealth checks the health of an instance
func (r *ServiceRegistry) checkHealth(client *http.Client, inst *ServiceInstance) {
	if inst.HealthCheckURL == "" {
		inst.HealthCheckURL = fmt.Sprintf("http://%s:%d/health", inst.Host, inst.Port)
	}

	resp, err := client.Get(inst.HealthCheckURL)
	if err != nil {
		r.markUnhealthy(inst)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		inst.Status = InstanceStatusHealthy
		inst.LastHealthCheck = time.Now()
	} else {
		r.markUnhealthy(inst)
	}
}

// unhealthyCounts tracks consecutive health check failures
var unhealthyCounts sync.Map

// markUnhealthy marks an instance as unhealthy
func (r *ServiceRegistry) markUnhealthy(inst *ServiceInstance) {
	countI, _ := unhealthyCounts.LoadOrStore(inst.ID, new(int32))
	count := atomic.AddInt32(countI.(*int32), 1)

	if int(count) >= r.unhealthyThreshold {
		inst.Status = InstanceStatusUnhealthy
		r.logger.Warn("instance marked unhealthy",
			slog.String("type", string(inst.Type)),
			slog.String("id", inst.ID),
			slog.Int("consecutive_failures", int(count)),
		)
	}
}

// Stop gracefully stops the registry
func (r *ServiceRegistry) Stop() {
	if r.localInstance != nil {
		r.Deregister(r.localInstance)
	}
	r.cancel()
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name          string
	maxFailures   int
	timeout       time.Duration
	halfOpenMax   int

	failures      int32
	successes     int32
	state         CircuitState
	lastFailure   time.Time
	mu            sync.RWMutex
	logger        *slog.Logger

	onStateChange func(from, to CircuitState)
}

// CircuitState represents the circuit breaker state
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

// String returns the string representation of circuit state
func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "CLOSED"
	case CircuitOpen:
		return "OPEN"
	case CircuitHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreakerConfig contains circuit breaker configuration
type CircuitBreakerConfig struct {
	Name          string
	MaxFailures   int
	Timeout       time.Duration
	HalfOpenMax   int
	OnStateChange func(from, to CircuitState)
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config *CircuitBreakerConfig, logger *slog.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		name:          config.Name,
		maxFailures:   config.MaxFailures,
		timeout:       config.Timeout,
		halfOpenMax:   config.HalfOpenMax,
		state:         CircuitClosed,
		logger:        logger,
		onStateChange: config.OnStateChange,
	}
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if !cb.allowRequest() {
		return ErrCircuitOpen
	}

	err := fn()

	cb.recordResult(err)

	return err
}

// ErrCircuitOpen is returned when the circuit is open
var ErrCircuitOpen = errors.New("circuit breaker is open")

// allowRequest checks if a request is allowed
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// Check if timeout has elapsed
		if time.Since(cb.lastFailure) > cb.timeout {
			cb.mu.RUnlock()
			cb.transitionTo(CircuitHalfOpen)
			cb.mu.RLock()
			return true
		}
		return false
	case CircuitHalfOpen:
		// Allow limited requests in half-open state
		return atomic.LoadInt32(&cb.successes) < int32(cb.halfOpenMax)
	default:
		return false
	}
}

// recordResult records the result of a request
func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failures++
		cb.lastFailure = time.Now()
		atomic.StoreInt32(&cb.successes, 0)

		if cb.state == CircuitClosed && int(cb.failures) >= cb.maxFailures {
			cb.transitionToLocked(CircuitOpen)
		} else if cb.state == CircuitHalfOpen {
			cb.transitionToLocked(CircuitOpen)
		}
	} else {
		if cb.state == CircuitHalfOpen {
			successes := atomic.AddInt32(&cb.successes, 1)
			if int(successes) >= cb.halfOpenMax {
				cb.transitionToLocked(CircuitClosed)
			}
		}
		cb.failures = 0
	}
}

// transitionTo transitions to a new state
func (cb *CircuitBreaker) transitionTo(newState CircuitState) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.transitionToLocked(newState)
}

// transitionToLocked transitions to a new state (caller must hold lock)
func (cb *CircuitBreaker) transitionToLocked(newState CircuitState) {
	oldState := cb.state
	cb.state = newState
	cb.failures = 0
	atomic.StoreInt32(&cb.successes, 0)

	cb.logger.Info("circuit breaker state change",
		slog.String("name", cb.name),
		slog.String("from", oldState.String()),
		slog.String("to", newState.String()),
	)

	if cb.onStateChange != nil {
		go cb.onStateChange(oldState, newState)
	}
}

// State returns the current circuit state
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// ServiceClient provides a client for inter-service communication
type ServiceClient struct {
	registry       *ServiceRegistry
	circuitBreakers map[ServiceType]*CircuitBreaker
	httpClient     *http.Client
	grpcConns      map[string]*grpc.ClientConn
	logger         *slog.Logger
	mu             sync.RWMutex
}

// ServiceClientConfig contains client configuration
type ServiceClientConfig struct {
	HTTPTimeout       time.Duration
	GRPCKeepalive     time.Duration
	MaxRetries        int
	RetryBackoff      time.Duration
	CircuitBreaker    *CircuitBreakerConfig
}

// NewServiceClient creates a new service client
func NewServiceClient(registry *ServiceRegistry, logger *slog.Logger, config *ServiceClientConfig) *ServiceClient {
	client := &ServiceClient{
		registry:        registry,
		circuitBreakers: make(map[ServiceType]*CircuitBreaker),
		grpcConns:       make(map[string]*grpc.ClientConn),
		logger:          logger,
		httpClient: &http.Client{
			Timeout: config.HTTPTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}

	// Initialize circuit breakers for each service type
	allTypes := []ServiceType{
		ServiceTypeAIRouter, ServiceTypeEmbedding, ServiceTypeGeneration,
		ServiceTypeVoice, ServiceTypeAuth, ServiceTypeWebSocket,
	}

	for _, svcType := range allTypes {
		cbConfig := &CircuitBreakerConfig{
			Name:        string(svcType),
			MaxFailures: 5,
			Timeout:     30 * time.Second,
			HalfOpenMax: 3,
		}
		client.circuitBreakers[svcType] = NewCircuitBreaker(cbConfig, logger)
	}

	return client
}

// CallHTTP makes an HTTP call to a service
func (c *ServiceClient) CallHTTP(ctx context.Context, serviceType ServiceType, method, path string, body io.Reader) (*http.Response, error) {
	cb := c.circuitBreakers[serviceType]
	if cb != nil && cb.State() == CircuitOpen {
		return nil, ErrCircuitOpen
	}

	instance, err := c.registry.GetInstance(serviceType, LoadBalanceRoundRobin)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("http://%s:%d%s", instance.Host, instance.Port, path)

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	// Track connection for least connections LB
	countI, _ := connectionCounts.LoadOrStore(instance.ID, new(int64))
	counter := countI.(*int64)
	atomic.AddInt64(counter, 1)
	defer atomic.AddInt64(counter, -1)

	var resp *http.Response
	executeErr := cb.Execute(func() error {
		var reqErr error
		resp, reqErr = c.httpClient.Do(req)
		if reqErr != nil {
			return reqErr
		}
		if resp.StatusCode >= 500 {
			return fmt.Errorf("server error: %d", resp.StatusCode)
		}
		return nil
	})

	if executeErr != nil {
		return nil, executeErr
	}

	return resp, nil
}

// GetGRPCConn returns a gRPC connection to a service
func (c *ServiceClient) GetGRPCConn(ctx context.Context, serviceType ServiceType) (*grpc.ClientConn, error) {
	instance, err := c.registry.GetInstance(serviceType, LoadBalanceRoundRobin)
	if err != nil {
		return nil, err
	}

	if instance.GRPCPort == 0 {
		return nil, fmt.Errorf("service %s does not support gRPC", serviceType)
	}

	connKey := fmt.Sprintf("%s:%d", instance.Host, instance.GRPCPort)

	c.mu.RLock()
	if conn, ok := c.grpcConns[connKey]; ok {
		c.mu.RUnlock()
		return conn, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if conn, ok := c.grpcConns[connKey]; ok {
		return conn, nil
	}

	// Create new connection
	opts := []grpc.DialOption{
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             3 * time.Second,
			PermitWithoutStream: true,
		}),
	}

	// Use TLS in production
	if instance.Metadata["tls"] == "true" {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}

	conn, err := grpc.DialContext(ctx, connKey, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", serviceType, err)
	}

	c.grpcConns[connKey] = conn

	return conn, nil
}

// HealthCheck performs a health check on a gRPC service
func (c *ServiceClient) HealthCheck(ctx context.Context, serviceType ServiceType) (bool, error) {
	conn, err := c.GetGRPCConn(ctx, serviceType)
	if err != nil {
		return false, err
	}

	client := grpc_health_v1.NewHealthClient(conn)
	resp, err := client.Check(ctx, &grpc_health_v1.HealthCheckRequest{
		Service: string(serviceType),
	})
	if err != nil {
		return false, err
	}

	return resp.Status == grpc_health_v1.HealthCheckResponse_SERVING, nil
}

// Close closes all connections
func (c *ServiceClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, conn := range c.grpcConns {
		conn.Close()
	}
}

// RetryPolicy defines retry behavior
type RetryPolicy struct {
	MaxRetries  int
	InitialWait time.Duration
	MaxWait     time.Duration
	Multiplier  float64
}

// DefaultRetryPolicy returns a default retry policy
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries:  3,
		InitialWait: 100 * time.Millisecond,
		MaxWait:     5 * time.Second,
		Multiplier:  2.0,
	}
}

// Retry executes a function with retry logic
func Retry(ctx context.Context, policy *RetryPolicy, fn func() error) error {
	var lastErr error
	wait := policy.InitialWait

	for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}

		if attempt < policy.MaxRetries {
			time.Sleep(wait)
			wait = time.Duration(float64(wait) * policy.Multiplier)
			if wait > policy.MaxWait {
				wait = policy.MaxWait
			}
		}
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// Sidecar provides sidecar proxy functionality
type Sidecar struct {
	registry    *ServiceRegistry
	client      *ServiceClient
	localPort   int
	proxyPort   int
	logger      *slog.Logger
	server      *http.Server
}

// NewSidecar creates a new sidecar proxy
func NewSidecar(registry *ServiceRegistry, client *ServiceClient, localPort, proxyPort int, logger *slog.Logger) *Sidecar {
	return &Sidecar{
		registry:  registry,
		client:    client,
		localPort: localPort,
		proxyPort: proxyPort,
		logger:    logger,
	}
}

// Start starts the sidecar proxy
func (s *Sidecar) Start() error {
	mux := http.NewServeMux()

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Metrics endpoint
	mux.HandleFunc("/metrics", s.metricsHandler)

	// Proxy all other requests
	mux.HandleFunc("/", s.proxyHandler)

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.proxyPort),
		Handler: mux,
	}

	s.logger.Info("sidecar proxy starting",
		slog.Int("local_port", s.localPort),
		slog.Int("proxy_port", s.proxyPort),
	)

	return s.server.ListenAndServe()
}

// proxyHandler handles proxy requests
func (s *Sidecar) proxyHandler(w http.ResponseWriter, r *http.Request) {
	// Extract target service from header
	targetService := r.Header.Get("X-Target-Service")
	if targetService == "" {
		http.Error(w, "X-Target-Service header required", http.StatusBadRequest)
		return
	}

	serviceType := ServiceType(targetService)

	resp, err := s.client.CallHTTP(r.Context(), serviceType, r.Method, r.URL.Path, r.Body)
	if err != nil {
		s.logger.Error("proxy request failed",
			slog.String("error", err.Error()),
			slog.String("service", targetService),
		)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// metricsHandler returns Prometheus-style metrics
func (s *Sidecar) metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	// Output circuit breaker states
	for svcType, cb := range s.client.circuitBreakers {
		fmt.Fprintf(w, "circuit_breaker_state{service=\"%s\"} %d\n", svcType, cb.State())
	}

	// Output instance counts
	s.registry.mu.RLock()
	for svcType, instances := range s.registry.instances {
		healthy := 0
		for _, inst := range instances {
			if inst.Status == InstanceStatusHealthy {
				healthy++
			}
		}
		fmt.Fprintf(w, "service_instances_total{service=\"%s\"} %d\n", svcType, len(instances))
		fmt.Fprintf(w, "service_instances_healthy{service=\"%s\"} %d\n", svcType, healthy)
	}
	s.registry.mu.RUnlock()
}

// Stop gracefully stops the sidecar
func (s *Sidecar) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
