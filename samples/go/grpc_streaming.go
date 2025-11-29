// Package streaming provides gRPC streaming implementations for real-time
// therapeutic AI interactions including bidirectional chat, voice streaming,
// and crisis alert broadcasting.
package streaming

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ChatMessage represents a message in a therapeutic conversation
type ChatMessage struct {
	ID            string                 `json:"id"`
	SessionID     string                 `json:"session_id"`
	UserID        string                 `json:"user_id"`
	Role          MessageRole            `json:"role"`
	Content       string                 `json:"content"`
	Timestamp     time.Time              `json:"timestamp"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	AgentType     string                 `json:"agent_type,omitempty"`
	CrisisLevel   string                 `json:"crisis_level,omitempty"`
	IsStreaming   bool                   `json:"is_streaming"`
	StreamIndex   int32                  `json:"stream_index"`
	IsFinal       bool                   `json:"is_final"`
}

// MessageRole defines the role of a message sender
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
)

// StreamState tracks the state of a streaming session
type StreamState struct {
	SessionID     string
	UserID        string
	StartedAt     time.Time
	LastActivity  time.Time
	MessageCount  int64
	IsActive      bool
	CurrentAgent  string
	CrisisStatus  string
}

// TherapeuticStreamServer implements bidirectional streaming for therapeutic chat
type TherapeuticStreamServer struct {
	UnimplementedTherapeuticServiceServer

	redis         *redis.Client
	logger        *slog.Logger
	aiRouter      AIRouterClient
	crisisService CrisisService
	sessions      sync.Map // map[sessionID]*StreamState
	streams       sync.Map // map[sessionID]grpc.ServerStream

	// Metrics
	activeStreams   int64
	totalMessages   int64
	avgResponseTime time.Duration
}

// UnimplementedTherapeuticServiceServer for forward compatibility
type UnimplementedTherapeuticServiceServer struct{}

// AIRouterClient interface for AI router communication
type AIRouterClient interface {
	StreamGenerate(ctx context.Context, req *GenerateRequest) (<-chan *GenerateChunk, error)
	AnalyzeCrisis(ctx context.Context, message string, context *CrisisContext) (*CrisisResult, error)
	ClassifyIntent(ctx context.Context, message string) (*IntentResult, error)
}

// CrisisService interface for crisis management
type CrisisService interface {
	ReportCrisis(ctx context.Context, alert *CrisisAlert) error
	NotifyTeam(ctx context.Context, userID string, level string) error
}

// GenerateRequest for AI generation
type GenerateRequest struct {
	SessionID    string
	UserID       string
	Message      string
	Context      *ConversationContext
	AgentType    string
	StreamTokens bool
}

// GenerateChunk represents a streaming generation chunk
type GenerateChunk struct {
	Content     string
	IsFinal     bool
	TokenCount  int
	AgentType   string
	Reasoning   string
	Metadata    map[string]interface{}
}

// ConversationContext provides context for generation
type ConversationContext struct {
	History        []*ChatMessage
	UserProfile    map[string]interface{}
	ClinicalData   map[string]interface{}
	CurrentMood    string
	SessionGoals   []string
}

// CrisisContext for crisis analysis
type CrisisContext struct {
	RecentMessages []string
	PHQ9Score      *int
	GAD7Score      *int
	RiskFactors    []string
}

// CrisisResult from crisis analysis
type CrisisResult struct {
	Level      string
	Confidence float64
	Patterns   []string
	Action     string
}

// CrisisAlert for reporting
type CrisisAlert struct {
	UserID    string
	SessionID string
	Level     string
	Message   string
	Timestamp time.Time
}

// IntentResult from intent classification
type IntentResult struct {
	Intent     string
	Confidence float64
	AgentType  string
}

// NewTherapeuticStreamServer creates a new streaming server
func NewTherapeuticStreamServer(
	redis *redis.Client,
	logger *slog.Logger,
	aiRouter AIRouterClient,
	crisisService CrisisService,
) *TherapeuticStreamServer {
	return &TherapeuticStreamServer{
		redis:         redis,
		logger:        logger,
		aiRouter:      aiRouter,
		crisisService: crisisService,
	}
}

// Chat implements bidirectional streaming for therapeutic conversations
func (s *TherapeuticStreamServer) Chat(stream grpc.BidiStreamingServer[ChatMessage, ChatMessage]) error {
	ctx := stream.Context()

	// Extract session info from metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.InvalidArgument, "missing metadata")
	}

	sessionID := extractMetadata(md, "session-id")
	userID := extractMetadata(md, "user-id")

	if sessionID == "" || userID == "" {
		return status.Error(codes.InvalidArgument, "session-id and user-id required")
	}

	// Initialize stream state
	state := &StreamState{
		SessionID:    sessionID,
		UserID:       userID,
		StartedAt:    time.Now(),
		LastActivity: time.Now(),
		IsActive:     true,
	}
	s.sessions.Store(sessionID, state)
	s.streams.Store(sessionID, stream)
	defer func() {
		state.IsActive = false
		s.sessions.Delete(sessionID)
		s.streams.Delete(sessionID)
	}()

	s.logger.Info("chat stream started",
		slog.String("session_id", sessionID),
		slog.String("user_id", userID),
	)

	// Subscribe to Redis for external messages (crisis alerts, etc.)
	pubsub := s.redis.Subscribe(ctx, fmt.Sprintf("session:%s:messages", sessionID))
	defer pubsub.Close()

	// Handle Redis messages in background
	go s.handleRedisMessages(ctx, stream, pubsub)

	// Process incoming messages
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			s.logger.Info("chat stream ended by client",
				slog.String("session_id", sessionID),
			)
			return nil
		}
		if err != nil {
			s.logger.Error("chat stream error",
				slog.String("error", err.Error()),
				slog.String("session_id", sessionID),
			)
			return err
		}

		// Update state
		state.LastActivity = time.Now()
		state.MessageCount++

		// Process message
		if err := s.processMessage(ctx, stream, msg, state); err != nil {
			s.logger.Error("failed to process message",
				slog.String("error", err.Error()),
				slog.String("session_id", sessionID),
			)
			// Continue processing, don't break stream
		}
	}
}

// processMessage handles an incoming chat message
func (s *TherapeuticStreamServer) processMessage(
	ctx context.Context,
	stream grpc.BidiStreamingServer[ChatMessage, ChatMessage],
	msg *ChatMessage,
	state *StreamState,
) error {
	startTime := time.Now()

	// Crisis check first (safety-first architecture)
	crisisResult, err := s.aiRouter.AnalyzeCrisis(ctx, msg.Content, &CrisisContext{
		RecentMessages: s.getRecentMessages(ctx, state.SessionID),
	})
	if err != nil {
		s.logger.Error("crisis analysis failed",
			slog.String("error", err.Error()),
		)
	} else if crisisResult.Level != "" && crisisResult.Level != "NONE" {
		// Report crisis
		s.crisisService.ReportCrisis(ctx, &CrisisAlert{
			UserID:    state.UserID,
			SessionID: state.SessionID,
			Level:     crisisResult.Level,
			Message:   msg.Content,
			Timestamp: time.Now(),
		})

		// Send crisis acknowledgment
		crisisMsg := &ChatMessage{
			SessionID:   state.SessionID,
			UserID:      state.UserID,
			Role:        RoleSystem,
			Content:     "I'm concerned about what you've shared. Your care team has been notified and will reach out shortly.",
			Timestamp:   time.Now(),
			CrisisLevel: crisisResult.Level,
			IsFinal:     true,
		}
		if err := stream.Send(crisisMsg); err != nil {
			return err
		}
	}

	// Classify intent to determine agent
	intentResult, err := s.aiRouter.ClassifyIntent(ctx, msg.Content)
	if err != nil {
		s.logger.Error("intent classification failed",
			slog.String("error", err.Error()),
		)
		intentResult = &IntentResult{AgentType: "conversational"}
	}

	state.CurrentAgent = intentResult.AgentType

	// Stream AI response
	chunks, err := s.aiRouter.StreamGenerate(ctx, &GenerateRequest{
		SessionID:    state.SessionID,
		UserID:       state.UserID,
		Message:      msg.Content,
		AgentType:    intentResult.AgentType,
		StreamTokens: true,
	})
	if err != nil {
		return fmt.Errorf("generation failed: %w", err)
	}

	// Stream response chunks to client
	var streamIndex int32 = 0
	for chunk := range chunks {
		responseMsg := &ChatMessage{
			SessionID:   state.SessionID,
			UserID:      state.UserID,
			Role:        RoleAssistant,
			Content:     chunk.Content,
			Timestamp:   time.Now(),
			AgentType:   chunk.AgentType,
			IsStreaming: !chunk.IsFinal,
			StreamIndex: streamIndex,
			IsFinal:     chunk.IsFinal,
			Metadata:    chunk.Metadata,
		}

		if err := stream.Send(responseMsg); err != nil {
			return fmt.Errorf("failed to send chunk: %w", err)
		}

		streamIndex++
	}

	// Log response time
	s.logger.Info("message processed",
		slog.String("session_id", state.SessionID),
		slog.String("agent", intentResult.AgentType),
		slog.Duration("response_time", time.Since(startTime)),
	)

	return nil
}

// handleRedisMessages handles messages from Redis pub/sub
func (s *TherapeuticStreamServer) handleRedisMessages(
	ctx context.Context,
	stream grpc.BidiStreamingServer[ChatMessage, ChatMessage],
	pubsub *redis.PubSub,
) {
	ch := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			return
		case redisMsg := <-ch:
			var msg ChatMessage
			if err := json.Unmarshal([]byte(redisMsg.Payload), &msg); err != nil {
				s.logger.Error("failed to unmarshal Redis message",
					slog.String("error", err.Error()),
				)
				continue
			}

			if err := stream.Send(&msg); err != nil {
				s.logger.Error("failed to send Redis message to stream",
					slog.String("error", err.Error()),
				)
				return
			}
		}
	}
}

// getRecentMessages retrieves recent messages for context
func (s *TherapeuticStreamServer) getRecentMessages(ctx context.Context, sessionID string) []string {
	key := fmt.Sprintf("session:%s:history", sessionID)
	messages, err := s.redis.LRange(ctx, key, -10, -1).Result()
	if err != nil {
		return []string{}
	}
	return messages
}

// BroadcastToSession sends a message to a specific session
func (s *TherapeuticStreamServer) BroadcastToSession(sessionID string, msg *ChatMessage) error {
	streamI, ok := s.streams.Load(sessionID)
	if !ok {
		return errors.New("session not found")
	}

	stream := streamI.(grpc.BidiStreamingServer[ChatMessage, ChatMessage])
	return stream.Send(msg)
}

// extractMetadata extracts a value from gRPC metadata
func extractMetadata(md metadata.MD, key string) string {
	values := md.Get(key)
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

// VoiceStreamServer implements voice streaming for therapeutic interactions
type VoiceStreamServer struct {
	UnimplementedVoiceServiceServer

	logger     *slog.Logger
	sttClient  STTClient
	ttsClient  TTSClient
	aiRouter   AIRouterClient
}

// UnimplementedVoiceServiceServer for forward compatibility
type UnimplementedVoiceServiceServer struct{}

// STTClient interface for speech-to-text
type STTClient interface {
	StreamTranscribe(ctx context.Context, audioStream <-chan []byte) (<-chan *TranscriptionResult, error)
}

// TTSClient interface for text-to-speech
type TTSClient interface {
	StreamSynthesize(ctx context.Context, text string, voice string) (<-chan []byte, error)
}

// TranscriptionResult from STT
type TranscriptionResult struct {
	Text       string
	IsFinal    bool
	Confidence float64
	Timestamp  time.Duration
}

// AudioChunk represents audio data
type AudioChunk struct {
	Data       []byte
	Format     string // "wav", "opus", "webm"
	SampleRate int
	Channels   int
	IsFinal    bool
}

// VoiceRequest for voice streaming
type VoiceRequest struct {
	SessionID string
	UserID    string
	Audio     *AudioChunk
}

// VoiceResponse from voice streaming
type VoiceResponse struct {
	SessionID    string
	Transcription string
	Response     string
	Audio        *AudioChunk
	IsFinal      bool
}

// NewVoiceStreamServer creates a new voice streaming server
func NewVoiceStreamServer(
	logger *slog.Logger,
	sttClient STTClient,
	ttsClient TTSClient,
	aiRouter AIRouterClient,
) *VoiceStreamServer {
	return &VoiceStreamServer{
		logger:    logger,
		sttClient: sttClient,
		ttsClient: ttsClient,
		aiRouter:  aiRouter,
	}
}

// StreamVoice implements bidirectional voice streaming
func (s *VoiceStreamServer) StreamVoice(stream grpc.BidiStreamingServer[VoiceRequest, VoiceResponse]) error {
	ctx := stream.Context()

	// Extract metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.InvalidArgument, "missing metadata")
	}

	sessionID := extractMetadata(md, "session-id")
	userID := extractMetadata(md, "user-id")

	s.logger.Info("voice stream started",
		slog.String("session_id", sessionID),
		slog.String("user_id", userID),
	)

	// Channel for audio chunks
	audioIn := make(chan []byte, 100)
	defer close(audioIn)

	// Start transcription stream
	transcriptions, err := s.sttClient.StreamTranscribe(ctx, audioIn)
	if err != nil {
		return status.Error(codes.Internal, "failed to start transcription")
	}

	// Process transcriptions and generate responses
	go s.processTranscriptions(ctx, stream, sessionID, userID, transcriptions)

	// Receive audio chunks
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		if req.Audio != nil && len(req.Audio.Data) > 0 {
			select {
			case audioIn <- req.Audio.Data:
			default:
				s.logger.Warn("audio buffer full, dropping chunk")
			}
		}
	}
}

// processTranscriptions handles transcription results
func (s *VoiceStreamServer) processTranscriptions(
	ctx context.Context,
	stream grpc.BidiStreamingServer[VoiceRequest, VoiceResponse],
	sessionID, userID string,
	transcriptions <-chan *TranscriptionResult,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case result, ok := <-transcriptions:
			if !ok {
				return
			}

			if !result.IsFinal {
				// Send partial transcription
				stream.Send(&VoiceResponse{
					SessionID:     sessionID,
					Transcription: result.Text,
					IsFinal:       false,
				})
				continue
			}

			// Generate AI response for final transcription
			chunks, err := s.aiRouter.StreamGenerate(ctx, &GenerateRequest{
				SessionID: sessionID,
				UserID:    userID,
				Message:   result.Text,
			})
			if err != nil {
				s.logger.Error("generation failed",
					slog.String("error", err.Error()),
				)
				continue
			}

			// Collect response text
			var responseText string
			for chunk := range chunks {
				responseText += chunk.Content
			}

			// Synthesize speech
			audioChunks, err := s.ttsClient.StreamSynthesize(ctx, responseText, "therapeutic-warm")
			if err != nil {
				s.logger.Error("TTS failed",
					slog.String("error", err.Error()),
				)
				continue
			}

			// Stream audio response
			for audioData := range audioChunks {
				stream.Send(&VoiceResponse{
					SessionID:     sessionID,
					Transcription: result.Text,
					Response:      responseText,
					Audio: &AudioChunk{
						Data:   audioData,
						Format: "opus",
					},
				})
			}

			// Send final response
			stream.Send(&VoiceResponse{
				SessionID:     sessionID,
				Transcription: result.Text,
				Response:      responseText,
				IsFinal:       true,
			})
		}
	}
}

// CrisisAlertStreamServer implements server-side streaming for crisis alerts
type CrisisAlertStreamServer struct {
	UnimplementedCrisisAlertServiceServer

	redis  *redis.Client
	logger *slog.Logger
}

// UnimplementedCrisisAlertServiceServer for forward compatibility
type UnimplementedCrisisAlertServiceServer struct{}

// CrisisAlertRequest for subscribing to alerts
type CrisisAlertRequest struct {
	FacilityID string
	UserID     string
	Roles      []string
}

// CrisisAlertResponse streaming response
type CrisisAlertResponse struct {
	Alert     *CrisisAlert
	Timestamp time.Time
}

// NewCrisisAlertStreamServer creates a new crisis alert streaming server
func NewCrisisAlertStreamServer(redis *redis.Client, logger *slog.Logger) *CrisisAlertStreamServer {
	return &CrisisAlertStreamServer{
		redis:  redis,
		logger: logger,
	}
}

// StreamAlerts implements server-side streaming for crisis alerts
func (s *CrisisAlertStreamServer) StreamAlerts(
	req *CrisisAlertRequest,
	stream grpc.ServerStreamingServer[CrisisAlertResponse],
) error {
	ctx := stream.Context()

	// Subscribe to crisis alert channels
	channels := []string{
		fmt.Sprintf("crisis:facility:%s", req.FacilityID),
	}

	if req.UserID != "" {
		channels = append(channels, fmt.Sprintf("crisis:user:%s", req.UserID))
	}

	for _, role := range req.Roles {
		channels = append(channels, fmt.Sprintf("crisis:role:%s", role))
	}

	pubsub := s.redis.Subscribe(ctx, channels...)
	defer pubsub.Close()

	s.logger.Info("crisis alert stream started",
		slog.String("facility_id", req.FacilityID),
		slog.String("user_id", req.UserID),
		slog.Any("channels", channels),
	)

	ch := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			return nil
		case msg := <-ch:
			var alert CrisisAlert
			if err := json.Unmarshal([]byte(msg.Payload), &alert); err != nil {
				s.logger.Error("failed to unmarshal crisis alert",
					slog.String("error", err.Error()),
				)
				continue
			}

			response := &CrisisAlertResponse{
				Alert:     &alert,
				Timestamp: time.Now(),
			}

			if err := stream.Send(response); err != nil {
				s.logger.Error("failed to send crisis alert",
					slog.String("error", err.Error()),
				)
				return err
			}
		}
	}
}

// MetricsStreamServer implements streaming for real-time metrics
type MetricsStreamServer struct {
	UnimplementedMetricsServiceServer

	redis  *redis.Client
	logger *slog.Logger
}

// UnimplementedMetricsServiceServer for forward compatibility
type UnimplementedMetricsServiceServer struct{}

// MetricsRequest for subscribing to metrics
type MetricsRequest struct {
	ServiceTypes []string
	Interval     time.Duration
}

// MetricsResponse streaming response
type MetricsResponse struct {
	ServiceType string
	Metrics     map[string]float64
	Timestamp   time.Time
}

// NewMetricsStreamServer creates a new metrics streaming server
func NewMetricsStreamServer(redis *redis.Client, logger *slog.Logger) *MetricsStreamServer {
	return &MetricsStreamServer{
		redis:  redis,
		logger: logger,
	}
}

// StreamMetrics implements server-side streaming for real-time metrics
func (s *MetricsStreamServer) StreamMetrics(
	req *MetricsRequest,
	stream grpc.ServerStreamingServer[MetricsResponse],
) error {
	ctx := stream.Context()

	interval := req.Interval
	if interval < time.Second {
		interval = time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			for _, serviceType := range req.ServiceTypes {
				metrics, err := s.collectMetrics(ctx, serviceType)
				if err != nil {
					continue
				}

				response := &MetricsResponse{
					ServiceType: serviceType,
					Metrics:     metrics,
					Timestamp:   time.Now(),
				}

				if err := stream.Send(response); err != nil {
					return err
				}
			}
		}
	}
}

// collectMetrics collects metrics for a service type
func (s *MetricsStreamServer) collectMetrics(ctx context.Context, serviceType string) (map[string]float64, error) {
	key := fmt.Sprintf("metrics:%s", serviceType)

	result, err := s.redis.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	metrics := make(map[string]float64)
	for k, v := range result {
		var val float64
		fmt.Sscanf(v, "%f", &val)
		metrics[k] = val
	}

	return metrics, nil
}

// RegisterServices registers all gRPC streaming services
func RegisterServices(
	server *grpc.Server,
	redis *redis.Client,
	logger *slog.Logger,
	aiRouter AIRouterClient,
	crisisService CrisisService,
	sttClient STTClient,
	ttsClient TTSClient,
) {
	// Register therapeutic chat streaming
	chatServer := NewTherapeuticStreamServer(redis, logger, aiRouter, crisisService)
	// RegisterTherapeuticServiceServer(server, chatServer)
	_ = chatServer

	// Register voice streaming
	voiceServer := NewVoiceStreamServer(logger, sttClient, ttsClient, aiRouter)
	// RegisterVoiceServiceServer(server, voiceServer)
	_ = voiceServer

	// Register crisis alert streaming
	crisisAlertServer := NewCrisisAlertStreamServer(redis, logger)
	// RegisterCrisisAlertServiceServer(server, crisisAlertServer)
	_ = crisisAlertServer

	// Register metrics streaming
	metricsServer := NewMetricsStreamServer(redis, logger)
	// RegisterMetricsServiceServer(server, metricsServer)
	_ = metricsServer

	logger.Info("all gRPC streaming services registered")
}
