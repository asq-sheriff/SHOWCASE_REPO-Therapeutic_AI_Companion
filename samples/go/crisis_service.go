// Package crisis provides real-time crisis detection and response coordination
// for the Lilo Engine therapeutic AI platform with <30 second response time.
package crisis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CrisisLevel defines severity levels for crisis detection
type CrisisLevel string

const (
	CrisisLevelImmediate CrisisLevel = "IMMEDIATE" // <30s response, 911 auto-escalation
	CrisisLevelUrgent    CrisisLevel = "URGENT"    // <5min, physician + nurse
	CrisisLevelElevated  CrisisLevel = "ELEVATED"  // <1hr, physician + social worker
	CrisisLevelModerate  CrisisLevel = "MODERATE"  // <24hr, routine monitoring
	CrisisLevelNone      CrisisLevel = "NONE"      // No crisis detected
)

// CrisisAlert represents a detected crisis event
type CrisisAlert struct {
	ID              string                 `json:"id"`
	UserID          string                 `json:"user_id"`
	SessionID       string                 `json:"session_id"`
	Level           CrisisLevel            `json:"level"`
	ConfidenceScore float64                `json:"confidence_score"`
	TriggerMessage  string                 `json:"trigger_message"`
	DetectedPatterns []string              `json:"detected_patterns"`
	ClinicalContext map[string]interface{} `json:"clinical_context"`
	Timestamp       time.Time              `json:"timestamp"`
	ResponseDeadline time.Time             `json:"response_deadline"`
	Status          AlertStatus            `json:"status"`
	AssignedTo      []string               `json:"assigned_to"`
	Acknowledgments []Acknowledgment       `json:"acknowledgments"`
	Escalations     []Escalation           `json:"escalations"`
}

// AlertStatus represents the current state of a crisis alert
type AlertStatus string

const (
	AlertStatusActive       AlertStatus = "ACTIVE"
	AlertStatusAcknowledged AlertStatus = "ACKNOWLEDGED"
	AlertStatusInProgress   AlertStatus = "IN_PROGRESS"
	AlertStatusResolved     AlertStatus = "RESOLVED"
	AlertStatusEscalated    AlertStatus = "ESCALATED"
)

// Acknowledgment records when a care team member acknowledges an alert
type Acknowledgment struct {
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"`
	Timestamp time.Time `json:"timestamp"`
	Notes     string    `json:"notes,omitempty"`
}

// Escalation records when an alert is escalated
type Escalation struct {
	FromLevel    CrisisLevel `json:"from_level"`
	ToLevel      CrisisLevel `json:"to_level"`
	Reason       string      `json:"reason"`
	Timestamp    time.Time   `json:"timestamp"`
	TriggeredBy  string      `json:"triggered_by"` // "auto" or user_id
}

// CrisisServiceConfig contains configuration for the crisis service
type CrisisServiceConfig struct {
	ResponseTimeouts  map[CrisisLevel]time.Duration
	EscalationDelays  map[CrisisLevel]time.Duration
	MaxRetries        int
	RetryDelay        time.Duration
	Enable911AutoCall bool
}

// DefaultCrisisConfig returns regulatory-compliant default configuration
func DefaultCrisisConfig() *CrisisServiceConfig {
	return &CrisisServiceConfig{
		ResponseTimeouts: map[CrisisLevel]time.Duration{
			CrisisLevelImmediate: 30 * time.Second,
			CrisisLevelUrgent:    5 * time.Minute,
			CrisisLevelElevated:  1 * time.Hour,
			CrisisLevelModerate:  24 * time.Hour,
		},
		EscalationDelays: map[CrisisLevel]time.Duration{
			CrisisLevelImmediate: 15 * time.Second,
			CrisisLevelUrgent:    2 * time.Minute,
			CrisisLevelElevated:  30 * time.Minute,
			CrisisLevelModerate:  12 * time.Hour,
		},
		MaxRetries:        3,
		RetryDelay:        5 * time.Second,
		Enable911AutoCall: true,
	}
}

// CrisisService handles crisis detection, alerting, and response coordination
type CrisisService struct {
	config          *CrisisServiceConfig
	redis           *redis.Client
	logger          *slog.Logger
	notifier        CrisisNotifier
	detector        CrisisDetector
	careTeamService CareTeamService
	auditLogger     AuditLogger

	// Active alerts by ID
	activeAlerts sync.Map

	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc

	// gRPC client for AI router
	aiRouterClient AIRouterClient
}

// CrisisNotifier defines the interface for sending crisis notifications
type CrisisNotifier interface {
	SendPush(ctx context.Context, userIDs []string, alert *CrisisAlert) error
	SendSMS(ctx context.Context, phoneNumbers []string, message string) error
	SendEmail(ctx context.Context, emails []string, subject, body string) error
	TriggerEmergencyCall(ctx context.Context, phoneNumber string, alert *CrisisAlert) error
}

// CrisisDetector defines the interface for ML-based crisis detection
type CrisisDetector interface {
	AnalyzeMessage(ctx context.Context, message string, context *DetectionContext) (*DetectionResult, error)
	GetTrajectory(ctx context.Context, userID string, windowSize int) (*TrajectoryAnalysis, error)
}

// CareTeamService defines the interface for care team management
type CareTeamService interface {
	GetCareTeam(ctx context.Context, residentID string) (*CareTeam, error)
	GetOnCallStaff(ctx context.Context, facilityID string, role string) ([]TeamMember, error)
	GetEmergencyContacts(ctx context.Context, residentID string) ([]EmergencyContact, error)
}

// AIRouterClient defines the gRPC interface for the AI router
type AIRouterClient interface {
	AnalyzeCrisis(ctx context.Context, req *CrisisAnalysisRequest) (*CrisisAnalysisResponse, error)
	GetEmbedding(ctx context.Context, text string) ([]float32, error)
}

// AuditLogger defines the interface for HIPAA audit logging
type AuditLogger interface {
	LogCrisisEvent(ctx context.Context, event *CrisisAuditEvent) error
}

// DetectionContext provides context for crisis detection
type DetectionContext struct {
	UserID           string
	SessionID        string
	RecentMessages   []string
	PHQ9Score        *int
	GAD7Score        *int
	LifeStoryRisks   []string
	RecentAssessments map[string]interface{}
}

// DetectionResult contains the result of crisis analysis
type DetectionResult struct {
	Level            CrisisLevel
	ConfidenceScore  float64
	DetectedPatterns []string
	SemanticMatches  []SemanticMatch
	TrajectoryRisk   float64
	Reasoning        string
}

// SemanticMatch represents a matched crisis pattern
type SemanticMatch struct {
	Pattern    string
	Similarity float64
	Category   string
}

// TrajectoryAnalysis contains analysis of conversation trajectory
type TrajectoryAnalysis struct {
	TrendDirection   string  // "improving", "stable", "deteriorating"
	RiskScore        float64
	WindowSize       int
	SignificantShifts []TrajectoryShift
}

// TrajectoryShift represents a significant change in conversation
type TrajectoryShift struct {
	MessageIndex int
	FromState    string
	ToState      string
	Magnitude    float64
}

// CareTeam represents a resident's care team
type CareTeam struct {
	ResidentID  string
	FacilityID  string
	Members     []TeamMember
	Updated     time.Time
}

// TeamMember represents a care team member
type TeamMember struct {
	UserID      string
	Name        string
	Role        string
	Phone       string
	Email       string
	IsOnCall    bool
	Permissions []string
}

// EmergencyContact represents an emergency contact
type EmergencyContact struct {
	Name         string
	Relationship string
	Phone        string
	Email        string
	Priority     int
	IsLegalProxy bool
}

// CrisisAuditEvent represents a crisis-related audit event
type CrisisAuditEvent struct {
	Timestamp    time.Time
	AlertID      string
	UserID       string
	EventType    string
	Actor        string
	Details      map[string]interface{}
}

// CrisisAnalysisRequest is the gRPC request for crisis analysis
type CrisisAnalysisRequest struct {
	Message   string
	UserID    string
	SessionID string
	Context   *DetectionContext
}

// CrisisAnalysisResponse is the gRPC response from crisis analysis
type CrisisAnalysisResponse struct {
	Level           CrisisLevel
	Confidence      float64
	Patterns        []string
	Reasoning       string
	ProcessingTime  time.Duration
}

// NewCrisisService creates a new crisis service
func NewCrisisService(
	config *CrisisServiceConfig,
	redis *redis.Client,
	logger *slog.Logger,
	notifier CrisisNotifier,
	detector CrisisDetector,
	careTeamService CareTeamService,
	aiRouterClient AIRouterClient,
) *CrisisService {
	ctx, cancel := context.WithCancel(context.Background())

	svc := &CrisisService{
		config:          config,
		redis:           redis,
		logger:          logger,
		notifier:        notifier,
		detector:        detector,
		careTeamService: careTeamService,
		aiRouterClient:  aiRouterClient,
		ctx:             ctx,
		cancel:          cancel,
	}

	// Start background workers
	go svc.escalationMonitor()
	go svc.alertCleanup()

	return svc
}

// AnalyzeMessage analyzes a message for crisis indicators
func (s *CrisisService) AnalyzeMessage(ctx context.Context, message string, detectionCtx *DetectionContext) (*CrisisAlert, error) {
	startTime := time.Now()

	// Use AI router for crisis analysis via gRPC
	response, err := s.aiRouterClient.AnalyzeCrisis(ctx, &CrisisAnalysisRequest{
		Message:   message,
		UserID:    detectionCtx.UserID,
		SessionID: detectionCtx.SessionID,
		Context:   detectionCtx,
	})
	if err != nil {
		// Fallback to local detector if AI router is unavailable
		s.logger.Warn("AI router unavailable, using fallback detector",
			slog.String("error", err.Error()),
		)

		result, fallbackErr := s.detector.AnalyzeMessage(ctx, message, detectionCtx)
		if fallbackErr != nil {
			return nil, fmt.Errorf("crisis detection failed: %w", fallbackErr)
		}

		response = &CrisisAnalysisResponse{
			Level:      result.Level,
			Confidence: result.ConfidenceScore,
			Patterns:   result.DetectedPatterns,
			Reasoning:  result.Reasoning,
		}
	}

	// No crisis detected
	if response.Level == CrisisLevelNone {
		return nil, nil
	}

	// Create crisis alert
	alert := &CrisisAlert{
		ID:               uuid.New().String(),
		UserID:           detectionCtx.UserID,
		SessionID:        detectionCtx.SessionID,
		Level:            response.Level,
		ConfidenceScore:  response.Confidence,
		TriggerMessage:   message,
		DetectedPatterns: response.Patterns,
		ClinicalContext:  make(map[string]interface{}),
		Timestamp:        time.Now(),
		Status:           AlertStatusActive,
	}

	// Set response deadline
	if timeout, ok := s.config.ResponseTimeouts[response.Level]; ok {
		alert.ResponseDeadline = alert.Timestamp.Add(timeout)
	}

	// Add clinical context
	if detectionCtx.PHQ9Score != nil {
		alert.ClinicalContext["phq9_score"] = *detectionCtx.PHQ9Score
	}
	if detectionCtx.GAD7Score != nil {
		alert.ClinicalContext["gad7_score"] = *detectionCtx.GAD7Score
	}

	// Store alert
	if err := s.storeAlert(ctx, alert); err != nil {
		s.logger.Error("failed to store crisis alert",
			slog.String("error", err.Error()),
			slog.String("alert_id", alert.ID),
		)
	}

	// Initiate response
	go s.initiateResponse(alert)

	// Log metrics
	s.logger.Info("crisis detected",
		slog.String("alert_id", alert.ID),
		slog.String("user_id", alert.UserID),
		slog.String("level", string(alert.Level)),
		slog.Float64("confidence", alert.ConfidenceScore),
		slog.Duration("detection_time", time.Since(startTime)),
	)

	return alert, nil
}

// initiateResponse starts the crisis response workflow
func (s *CrisisService) initiateResponse(alert *CrisisAlert) {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
	defer cancel()

	// Get care team
	careTeam, err := s.careTeamService.GetCareTeam(ctx, alert.UserID)
	if err != nil {
		s.logger.Error("failed to get care team",
			slog.String("error", err.Error()),
			slog.String("user_id", alert.UserID),
		)
		// Continue with on-call staff
	}

	// Determine notification recipients based on crisis level
	recipients := s.determineRecipients(ctx, alert, careTeam)
	alert.AssignedTo = recipients.UserIDs

	// Send notifications
	if err := s.notifier.SendPush(ctx, recipients.UserIDs, alert); err != nil {
		s.logger.Error("failed to send push notifications",
			slog.String("error", err.Error()),
		)
	}

	// For IMMEDIATE level, also send SMS and consider 911
	if alert.Level == CrisisLevelImmediate {
		// SMS to all care team
		if len(recipients.PhoneNumbers) > 0 {
			message := fmt.Sprintf(
				"CRISIS ALERT: Immediate attention required for resident. Level: %s. Please respond within 30 seconds.",
				alert.Level,
			)
			s.notifier.SendSMS(ctx, recipients.PhoneNumbers, message)
		}

		// Auto-escalate to 911 if enabled and no acknowledgment
		if s.config.Enable911AutoCall {
			go s.monitorFor911Escalation(alert)
		}
	}

	// Notify emergency contacts for IMMEDIATE and URGENT
	if alert.Level == CrisisLevelImmediate || alert.Level == CrisisLevelUrgent {
		go s.notifyEmergencyContacts(ctx, alert)
	}

	// Audit log
	if s.auditLogger != nil {
		s.auditLogger.LogCrisisEvent(ctx, &CrisisAuditEvent{
			Timestamp: time.Now(),
			AlertID:   alert.ID,
			UserID:    alert.UserID,
			EventType: "response_initiated",
			Details: map[string]interface{}{
				"level":      alert.Level,
				"recipients": recipients.UserIDs,
			},
		})
	}
}

// NotificationRecipients contains recipients for notifications
type NotificationRecipients struct {
	UserIDs      []string
	PhoneNumbers []string
	Emails       []string
}

// determineRecipients determines who should be notified based on crisis level
func (s *CrisisService) determineRecipients(ctx context.Context, alert *CrisisAlert, careTeam *CareTeam) *NotificationRecipients {
	recipients := &NotificationRecipients{
		UserIDs:      make([]string, 0),
		PhoneNumbers: make([]string, 0),
		Emails:       make([]string, 0),
	}

	if careTeam == nil {
		return recipients
	}

	// Add care team members based on role and crisis level
	for _, member := range careTeam.Members {
		switch alert.Level {
		case CrisisLevelImmediate:
			// All care team members
			recipients.UserIDs = append(recipients.UserIDs, member.UserID)
			if member.Phone != "" {
				recipients.PhoneNumbers = append(recipients.PhoneNumbers, member.Phone)
			}
			recipients.Emails = append(recipients.Emails, member.Email)

		case CrisisLevelUrgent:
			// Physicians, nurses, and social workers
			if member.Role == "physician" || member.Role == "nurse" || member.Role == "social_worker" {
				recipients.UserIDs = append(recipients.UserIDs, member.UserID)
				if member.Phone != "" {
					recipients.PhoneNumbers = append(recipients.PhoneNumbers, member.Phone)
				}
			}

		case CrisisLevelElevated:
			// Physicians and social workers
			if member.Role == "physician" || member.Role == "social_worker" {
				recipients.UserIDs = append(recipients.UserIDs, member.UserID)
			}

		case CrisisLevelModerate:
			// Care manager only
			if member.Role == "care_manager" {
				recipients.UserIDs = append(recipients.UserIDs, member.UserID)
			}
		}
	}

	return recipients
}

// monitorFor911Escalation monitors for 911 auto-escalation
func (s *CrisisService) monitorFor911Escalation(alert *CrisisAlert) {
	// Wait for escalation delay
	delay := s.config.EscalationDelays[CrisisLevelImmediate]
	time.Sleep(delay)

	// Check if alert is still active and unacknowledged
	current, err := s.GetAlert(s.ctx, alert.ID)
	if err != nil {
		s.logger.Error("failed to get alert for 911 check",
			slog.String("error", err.Error()),
		)
		return
	}

	if current.Status == AlertStatusActive && len(current.Acknowledgments) == 0 {
		s.logger.Warn("no acknowledgment received, triggering 911 escalation",
			slog.String("alert_id", alert.ID),
			slog.String("user_id", alert.UserID),
		)

		// Get facility emergency number
		contacts, err := s.careTeamService.GetEmergencyContacts(s.ctx, alert.UserID)
		if err == nil && len(contacts) > 0 {
			s.notifier.TriggerEmergencyCall(s.ctx, "911", alert)
		}

		// Record escalation
		s.recordEscalation(alert, CrisisLevelImmediate, CrisisLevelImmediate, "No acknowledgment within deadline", "auto")
	}
}

// notifyEmergencyContacts notifies emergency contacts
func (s *CrisisService) notifyEmergencyContacts(ctx context.Context, alert *CrisisAlert) {
	contacts, err := s.careTeamService.GetEmergencyContacts(ctx, alert.UserID)
	if err != nil {
		s.logger.Error("failed to get emergency contacts",
			slog.String("error", err.Error()),
		)
		return
	}

	for _, contact := range contacts {
		// SMS notification
		if contact.Phone != "" {
			message := fmt.Sprintf(
				"Important: A crisis alert has been raised for your loved one. The care team has been notified and is responding. Please contact the facility for more information.",
			)
			s.notifier.SendSMS(ctx, []string{contact.Phone}, message)
		}

		// Email notification
		if contact.Email != "" {
			s.notifier.SendEmail(ctx, []string{contact.Email},
				"Crisis Alert Notification",
				"A crisis alert has been raised. Please contact the facility for more information.",
			)
		}
	}
}

// AcknowledgeAlert records an acknowledgment for an alert
func (s *CrisisService) AcknowledgeAlert(ctx context.Context, alertID, userID, role string, notes string) error {
	alert, err := s.GetAlert(ctx, alertID)
	if err != nil {
		return err
	}

	if alert.Status != AlertStatusActive && alert.Status != AlertStatusAcknowledged {
		return errors.New("alert is not in an acknowledgeable state")
	}

	ack := Acknowledgment{
		UserID:    userID,
		Role:      role,
		Timestamp: time.Now(),
		Notes:     notes,
	}

	alert.Acknowledgments = append(alert.Acknowledgments, ack)
	alert.Status = AlertStatusAcknowledged

	// Update stored alert
	if err := s.storeAlert(ctx, alert); err != nil {
		return fmt.Errorf("failed to update alert: %w", err)
	}

	// Audit log
	if s.auditLogger != nil {
		s.auditLogger.LogCrisisEvent(ctx, &CrisisAuditEvent{
			Timestamp: time.Now(),
			AlertID:   alertID,
			UserID:    alert.UserID,
			EventType: "acknowledged",
			Actor:     userID,
			Details: map[string]interface{}{
				"role":                role,
				"notes":               notes,
				"response_time_seconds": time.Since(alert.Timestamp).Seconds(),
			},
		})
	}

	s.logger.Info("alert acknowledged",
		slog.String("alert_id", alertID),
		slog.String("acknowledged_by", userID),
		slog.Duration("response_time", time.Since(alert.Timestamp)),
	)

	return nil
}

// ResolveAlert marks an alert as resolved
func (s *CrisisService) ResolveAlert(ctx context.Context, alertID, userID, resolution string) error {
	alert, err := s.GetAlert(ctx, alertID)
	if err != nil {
		return err
	}

	alert.Status = AlertStatusResolved
	alert.ClinicalContext["resolution"] = resolution
	alert.ClinicalContext["resolved_by"] = userID
	alert.ClinicalContext["resolved_at"] = time.Now()

	if err := s.storeAlert(ctx, alert); err != nil {
		return fmt.Errorf("failed to update alert: %w", err)
	}

	// Remove from active alerts
	s.activeAlerts.Delete(alertID)

	// Audit log
	if s.auditLogger != nil {
		s.auditLogger.LogCrisisEvent(ctx, &CrisisAuditEvent{
			Timestamp: time.Now(),
			AlertID:   alertID,
			UserID:    alert.UserID,
			EventType: "resolved",
			Actor:     userID,
			Details: map[string]interface{}{
				"resolution":           resolution,
				"total_time_seconds":   time.Since(alert.Timestamp).Seconds(),
				"acknowledgment_count": len(alert.Acknowledgments),
			},
		})
	}

	return nil
}

// GetAlert retrieves an alert by ID
func (s *CrisisService) GetAlert(ctx context.Context, alertID string) (*CrisisAlert, error) {
	key := fmt.Sprintf("crisis:alert:%s", alertID)
	data, err := s.redis.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, errors.New("alert not found")
		}
		return nil, fmt.Errorf("failed to get alert: %w", err)
	}

	var alert CrisisAlert
	if err := json.Unmarshal(data, &alert); err != nil {
		return nil, fmt.Errorf("failed to unmarshal alert: %w", err)
	}

	return &alert, nil
}

// GetActiveAlerts retrieves all active alerts for a facility or user
func (s *CrisisService) GetActiveAlerts(ctx context.Context, facilityID, userID string) ([]*CrisisAlert, error) {
	var pattern string
	if userID != "" {
		pattern = fmt.Sprintf("crisis:alert:*:user:%s", userID)
	} else {
		pattern = "crisis:alert:*"
	}

	keys, err := s.redis.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get alert keys: %w", err)
	}

	alerts := make([]*CrisisAlert, 0)
	for _, key := range keys {
		data, err := s.redis.Get(ctx, key).Bytes()
		if err != nil {
			continue
		}

		var alert CrisisAlert
		if err := json.Unmarshal(data, &alert); err != nil {
			continue
		}

		if alert.Status == AlertStatusActive || alert.Status == AlertStatusAcknowledged {
			alerts = append(alerts, &alert)
		}
	}

	return alerts, nil
}

// storeAlert stores an alert in Redis
func (s *CrisisService) storeAlert(ctx context.Context, alert *CrisisAlert) error {
	key := fmt.Sprintf("crisis:alert:%s", alert.ID)
	data, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("failed to marshal alert: %w", err)
	}

	// Store with 7-day TTL
	if err := s.redis.Set(ctx, key, data, 7*24*time.Hour).Err(); err != nil {
		return fmt.Errorf("failed to store alert: %w", err)
	}

	// Add to active alerts map
	s.activeAlerts.Store(alert.ID, alert)

	return nil
}

// recordEscalation records an escalation event
func (s *CrisisService) recordEscalation(alert *CrisisAlert, from, to CrisisLevel, reason, triggeredBy string) {
	escalation := Escalation{
		FromLevel:   from,
		ToLevel:     to,
		Reason:      reason,
		Timestamp:   time.Now(),
		TriggeredBy: triggeredBy,
	}

	alert.Escalations = append(alert.Escalations, escalation)
	alert.Status = AlertStatusEscalated

	s.storeAlert(s.ctx, alert)

	if s.auditLogger != nil {
		s.auditLogger.LogCrisisEvent(s.ctx, &CrisisAuditEvent{
			Timestamp: time.Now(),
			AlertID:   alert.ID,
			UserID:    alert.UserID,
			EventType: "escalated",
			Actor:     triggeredBy,
			Details: map[string]interface{}{
				"from_level": from,
				"to_level":   to,
				"reason":     reason,
			},
		})
	}
}

// escalationMonitor monitors alerts for escalation
func (s *CrisisService) escalationMonitor() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.checkEscalations()
		}
	}
}

// checkEscalations checks all active alerts for escalation
func (s *CrisisService) checkEscalations() {
	s.activeAlerts.Range(func(key, value interface{}) bool {
		alert := value.(*CrisisAlert)

		// Skip if not active
		if alert.Status != AlertStatusActive {
			return true
		}

		// Check if response deadline passed
		if time.Now().After(alert.ResponseDeadline) && len(alert.Acknowledgments) == 0 {
			s.logger.Warn("alert response deadline passed",
				slog.String("alert_id", alert.ID),
				slog.String("level", string(alert.Level)),
			)

			// Escalate to next level
			s.escalateAlert(alert)
		}

		return true
	})
}

// escalateAlert escalates an alert to the next level
func (s *CrisisService) escalateAlert(alert *CrisisAlert) {
	var nextLevel CrisisLevel

	switch alert.Level {
	case CrisisLevelModerate:
		nextLevel = CrisisLevelElevated
	case CrisisLevelElevated:
		nextLevel = CrisisLevelUrgent
	case CrisisLevelUrgent:
		nextLevel = CrisisLevelImmediate
	case CrisisLevelImmediate:
		// Already at highest level, trigger emergency services
		nextLevel = CrisisLevelImmediate
	}

	s.recordEscalation(alert, alert.Level, nextLevel, "Response deadline exceeded", "auto")

	// Re-initiate response with higher level
	alert.Level = nextLevel
	if timeout, ok := s.config.ResponseTimeouts[nextLevel]; ok {
		alert.ResponseDeadline = time.Now().Add(timeout)
	}

	go s.initiateResponse(alert)
}

// alertCleanup removes old resolved alerts from memory
func (s *CrisisService) alertCleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.activeAlerts.Range(func(key, value interface{}) bool {
				alert := value.(*CrisisAlert)
				if alert.Status == AlertStatusResolved && time.Since(alert.Timestamp) > 24*time.Hour {
					s.activeAlerts.Delete(key)
				}
				return true
			})
		}
	}
}

// Stop gracefully stops the crisis service
func (s *CrisisService) Stop() {
	s.cancel()
}

// gRPC Service Implementation

// CrisisGRPCServer implements the gRPC crisis service
type CrisisGRPCServer struct {
	service *CrisisService
	UnimplementedCrisisServiceServer
}

// UnimplementedCrisisServiceServer is a placeholder for gRPC
type UnimplementedCrisisServiceServer struct{}

// NewCrisisGRPCServer creates a new gRPC server
func NewCrisisGRPCServer(service *CrisisService) *CrisisGRPCServer {
	return &CrisisGRPCServer{service: service}
}

// AnalyzeCrisis implements the gRPC AnalyzeCrisis method
func (s *CrisisGRPCServer) AnalyzeCrisis(ctx context.Context, req *CrisisAnalysisRequest) (*CrisisAnalysisResponse, error) {
	if req.Message == "" {
		return nil, status.Error(codes.InvalidArgument, "message is required")
	}

	alert, err := s.service.AnalyzeMessage(ctx, req.Message, req.Context)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if alert == nil {
		return &CrisisAnalysisResponse{
			Level:      CrisisLevelNone,
			Confidence: 0,
		}, nil
	}

	return &CrisisAnalysisResponse{
		Level:      alert.Level,
		Confidence: alert.ConfidenceScore,
		Patterns:   alert.DetectedPatterns,
	}, nil
}

// StreamAlerts implements streaming crisis alerts
func (s *CrisisGRPCServer) StreamAlerts(req *StreamAlertsRequest, stream grpc.ServerStream) error {
	ctx := stream.Context()

	// Subscribe to Redis pub/sub for real-time alerts
	pubsub := s.service.redis.Subscribe(ctx, "crisis:alerts:"+req.FacilityID)
	defer pubsub.Close()

	ch := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			return nil
		case msg := <-ch:
			var alert CrisisAlert
			if err := json.Unmarshal([]byte(msg.Payload), &alert); err != nil {
				continue
			}

			if err := stream.SendMsg(&alert); err != nil {
				return status.Error(codes.Internal, err.Error())
			}
		}
	}
}

// StreamAlertsRequest is the request for streaming alerts
type StreamAlertsRequest struct {
	FacilityID string
	UserID     string
}
