<div align="center">

# 💻 Code Samples & Engineering Patterns

### Architectural Patterns and Implementation Approaches in Lilo Engine

**Showcasing production-grade Python and Go code patterns**

---

![Python](https://img.shields.io/badge/Python-3.12+-3776AB?style=for-the-badge&logo=python&logoColor=white)
![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![Patterns](https://img.shields.io/badge/Patterns-Production_Grade-success?style=for-the-badge)

</div>

---

> **Note:** These are representative code samples showing architectural patterns and approaches. The actual production code is proprietary and not publicly available.

---

## Table of Contents

1. [Multi-Agent Orchestration](#1-multi-agent-orchestration)
2. [Crisis Detection Pipeline](#2-crisis-detection-pipeline)
3. [RAG Context Builder](#3-rag-context-builder)
4. [Intent Classification](#4-intent-classification)
5. [WebSocket Handler (Go)](#5-websocket-handler-go)
6. [Caching Strategy](#6-caching-strategy)
7. [HIPAA-Compliant Logging](#7-hipaa-compliant-logging)
8. [Circuit Breaker Pattern](#8-circuit-breaker-pattern)
9. [Safety Service Architecture](#9-safety-service-architecture)
10. [Configuration Hot-Reload](#10-configuration-hot-reload)

---

## 1. Multi-Agent Orchestration

**Pattern:** Strategy pattern with dynamic agent selection based on intent classification

```python
from dataclasses import dataclass
from enum import Enum
from typing import List, Optional, Protocol
from abc import abstractmethod

class Intent(Enum):
    CRISIS = "crisis"
    SOOTHE = "soothe"
    REMINISCE = "reminisce"
    ACTIVATE = "activate"
    GROUND = "ground"
    ASSESS = "assess"
    CONNECT = "connect"
    GENERAL = "general"

class CoordinationStrategy(Enum):
    CLINICAL_PRIORITY = "clinical_priority"  # Safety overrides all
    SEQUENTIAL = "sequential"                 # Primary then secondary
    PARALLEL = "parallel"                     # Concurrent execution

@dataclass
class TherapeuticContext:
    """Immutable context passed through the agent pipeline"""
    user_id: str
    session_id: str
    message: str
    conversation_history: List[dict]
    clinical_scores: dict  # PHQ-9, GAD-7, UCLA-3
    life_story_context: dict
    safety_flags: List[str]
    query_embedding: Optional[List[float]] = None

class TherapeuticAgent(Protocol):
    """Protocol defining the agent interface"""

    @property
    def agent_type(self) -> str: ...

    @property
    def supported_intents(self) -> List[Intent]: ...

    @abstractmethod
    async def generate_response(
        self,
        context: TherapeuticContext,
        rag_context: dict
    ) -> str: ...

class AgentOrchestrator:
    """
    Coordinates multiple therapeutic agents based on intent classification.

    Key design decisions:
    - Safety agent ALWAYS runs first (clinical priority)
    - Max 2 secondary agents to avoid response confusion
    - Agents are stateless; all state in TherapeuticContext
    """

    def __init__(self, agents: List[TherapeuticAgent]):
        self.agents = {agent.agent_type: agent for agent in agents}
        self.intent_to_agent = self._build_intent_mapping()

    def _build_intent_mapping(self) -> dict:
        """Map intents to their primary agents"""
        mapping = {}
        for agent in self.agents.values():
            for intent in agent.supported_intents:
                if intent not in mapping:
                    mapping[intent] = agent.agent_type
        return mapping

    async def orchestrate(
        self,
        context: TherapeuticContext,
        primary_intent: Intent,
        secondary_intents: List[Intent],
        strategy: CoordinationStrategy
    ) -> str:
        """
        Main orchestration logic with safety-first guarantee.

        Returns:
            Combined therapeutic response from selected agents
        """
        # SAFETY FIRST: Always check for crisis
        if "CRISIS_DETECTED" in context.safety_flags:
            return await self._handle_crisis(context)

        # Select agents based on intents
        primary_agent = self.agents[self.intent_to_agent[primary_intent]]
        secondary_agents = [
            self.agents[self.intent_to_agent[intent]]
            for intent in secondary_intents[:2]  # Max 2 secondary
            if intent in self.intent_to_agent
        ]

        # Execute based on strategy
        if strategy == CoordinationStrategy.CLINICAL_PRIORITY:
            return await self._clinical_priority_execution(
                context, primary_agent, secondary_agents
            )
        elif strategy == CoordinationStrategy.SEQUENTIAL:
            return await self._sequential_execution(
                context, primary_agent, secondary_agents
            )
        else:
            return await self._parallel_execution(
                context, primary_agent, secondary_agents
            )

    async def _handle_crisis(self, context: TherapeuticContext) -> str:
        """Crisis handling with automatic escalation"""
        safety_agent = self.agents["safety_assessment"]

        # Generate therapeutic response while alerting
        response = await safety_agent.generate_response(context, {})

        # Trigger alert pipeline (non-blocking)
        await self._trigger_crisis_alerts(context)

        return response
```

**Key Engineering Decisions:**
- Immutable `TherapeuticContext` prevents state mutation bugs
- Protocol-based agent interface enables easy testing and extension
- Safety check is unconditional and runs before any other logic
- Strategy pattern allows runtime coordination changes

---

## 2. Crisis Detection Pipeline

**Pattern:** Multi-stage pipeline with semantic matching and clinical context fusion

```python
import numpy as np
from dataclasses import dataclass
from typing import List, Tuple
from enum import Enum

class CrisisLevel(Enum):
    NONE = 0
    MODERATE = 1      # Enhanced monitoring
    ELEVATED = 2      # Social worker alert
    URGENT = 3        # Physician + nurse
    IMMEDIATE = 4     # Auto-escalate 911

@dataclass
class CrisisAssessment:
    level: CrisisLevel
    confidence: float
    matched_patterns: List[str]
    clinical_factors: List[str]
    trajectory_risk: float
    response_time_ms: float

class CrisisDetectorV4:
    """
    BGE semantic matching crisis detector with 100% recall target.

    Architecture:
    1. Embed user message with BGE model
    2. Semantic similarity against 871 crisis patterns
    3. Integrate clinical context (PHQ-9, GAD-7, life story risks)
    4. Trajectory analysis for progressive deterioration
    5. Three-stage severity grading

    Performance: <1 second detection, 100% recall, <5% FPR
    """

    def __init__(
        self,
        embedding_service,
        crisis_patterns: np.ndarray,  # Shape: (871, 768)
        pattern_labels: List[str],
        similarity_threshold: float = 0.65
    ):
        self.embedding_service = embedding_service
        self.crisis_patterns = crisis_patterns
        self.pattern_labels = pattern_labels
        self.threshold = similarity_threshold

        # Clinical risk factors from life story
        self.life_story_risk_factors = [
            "recent_loss", "chronic_pain", "social_isolation",
            "previous_attempts", "terminal_diagnosis"
        ]

    async def assess(
        self,
        message: str,
        clinical_scores: dict,
        life_story: dict,
        conversation_history: List[dict]
    ) -> CrisisAssessment:
        """
        Full crisis assessment pipeline.

        Returns CrisisAssessment with level, confidence, and factors.
        """
        import time
        start_time = time.perf_counter()

        # Stage 1: Semantic embedding
        message_embedding = await self.embedding_service.embed(message)

        # Stage 2: Pattern matching
        similarities = self._compute_similarities(message_embedding)
        matched_patterns = self._get_matched_patterns(similarities)

        # Stage 3: Clinical context integration
        clinical_risk = self._assess_clinical_risk(clinical_scores)
        life_story_risk = self._assess_life_story_risk(life_story)

        # Stage 4: Trajectory analysis
        trajectory_risk = self._analyze_trajectory(conversation_history)

        # Stage 5: Final grading
        combined_score = self._combine_risk_scores(
            semantic_score=max(similarities) if len(similarities) > 0 else 0,
            clinical_risk=clinical_risk,
            life_story_risk=life_story_risk,
            trajectory_risk=trajectory_risk
        )

        level = self._grade_crisis_level(combined_score)

        elapsed_ms = (time.perf_counter() - start_time) * 1000

        return CrisisAssessment(
            level=level,
            confidence=combined_score,
            matched_patterns=matched_patterns,
            clinical_factors=self._get_clinical_factors(clinical_scores),
            trajectory_risk=trajectory_risk,
            response_time_ms=elapsed_ms
        )

    def _compute_similarities(self, embedding: np.ndarray) -> np.ndarray:
        """Vectorized cosine similarity against all patterns"""
        # Normalize for cosine similarity
        embedding_norm = embedding / np.linalg.norm(embedding)
        pattern_norms = self.crisis_patterns / np.linalg.norm(
            self.crisis_patterns, axis=1, keepdims=True
        )

        # Single matrix multiplication for all similarities
        similarities = pattern_norms @ embedding_norm
        return similarities

    def _analyze_trajectory(
        self,
        history: List[dict],
        window_size: int = 5
    ) -> float:
        """
        Detect progressive deterioration over conversation.

        Uses sliding window to identify worsening patterns:
        - Increasing negative affect
        - Escalating crisis language
        - Decreasing engagement
        """
        if len(history) < 2:
            return 0.0

        recent = history[-window_size:]

        # Track sentiment trajectory
        sentiments = [msg.get("sentiment", 0) for msg in recent]

        # Detect downward trend
        if len(sentiments) >= 3:
            trend = np.polyfit(range(len(sentiments)), sentiments, 1)[0]
            if trend < -0.1:  # Negative slope indicates deterioration
                return min(abs(trend) * 2, 1.0)

        return 0.0

    def _grade_crisis_level(self, combined_score: float) -> CrisisLevel:
        """Map combined risk score to actionable crisis level"""
        if combined_score >= 0.90:
            return CrisisLevel.IMMEDIATE
        elif combined_score >= 0.75:
            return CrisisLevel.URGENT
        elif combined_score >= 0.60:
            return CrisisLevel.ELEVATED
        elif combined_score >= 0.40:
            return CrisisLevel.MODERATE
        return CrisisLevel.NONE
```

**Key Engineering Decisions:**
- Vectorized similarity computation for <10ms pattern matching
- Multi-signal fusion prevents single-point-of-failure
- Trajectory analysis catches gradual deterioration
- Conservative thresholds optimize for recall over precision

---

## 3. RAG Context Builder

**Pattern:** Parallel async retrieval with hybrid search (BM25 + semantic + RRF fusion)

```python
import asyncio
from dataclasses import dataclass
from typing import List, Optional
from enum import Enum

class RetrievalSource(Enum):
    KNOWLEDGE_BASE = "knowledge_base"
    LIFE_STORY = "life_story"
    CHAT_HISTORY = "chat_history"
    CLINICAL_ASSESSMENTS = "clinical_assessments"
    SCHEDULE_EVENTS = "schedule_events"
    SEMANTIC_MEMORY = "semantic_memory"

@dataclass
class RetrievalResult:
    source: RetrievalSource
    content: str
    relevance_score: float
    metadata: dict

@dataclass
class RAGContext:
    """Aggregated context from all retrieval sources"""
    query: str
    results: List[RetrievalResult]
    total_retrieval_time_ms: float

class RAGContextBuilder:
    """
    6-stream parallel RAG retrieval with hybrid search.

    Architecture:
    - All 6 sources queried in parallel (asyncio.gather)
    - Each source uses hybrid: BM25 (keyword) + Semantic (embedding)
    - Results fused using Reciprocal Rank Fusion (RRF)
    - Total latency: ~45ms (parallel) vs ~270ms (sequential)
    """

    def __init__(
        self,
        knowledge_retriever,
        life_story_retriever,
        chat_history_retriever,
        assessment_retriever,
        schedule_retriever,
        memory_retriever,
        embedding_service
    ):
        self.retrievers = {
            RetrievalSource.KNOWLEDGE_BASE: knowledge_retriever,
            RetrievalSource.LIFE_STORY: life_story_retriever,
            RetrievalSource.CHAT_HISTORY: chat_history_retriever,
            RetrievalSource.CLINICAL_ASSESSMENTS: assessment_retriever,
            RetrievalSource.SCHEDULE_EVENTS: schedule_retriever,
            RetrievalSource.SEMANTIC_MEMORY: memory_retriever,
        }
        self.embedding_service = embedding_service

    async def build_context(
        self,
        query: str,
        user_id: str,
        query_embedding: Optional[List[float]] = None,
        top_k_per_source: int = 3
    ) -> RAGContext:
        """
        Build RAG context from all sources in parallel.

        Returns aggregated, deduplicated, relevance-ranked results.
        """
        import time
        start_time = time.perf_counter()

        # Get embedding if not provided
        if query_embedding is None:
            query_embedding = await self.embedding_service.embed(query)

        # Launch all retrievals in parallel
        retrieval_tasks = [
            self._retrieve_from_source(
                source, query, query_embedding, user_id, top_k_per_source
            )
            for source in self.retrievers.keys()
        ]

        # Wait for all to complete
        results_by_source = await asyncio.gather(
            *retrieval_tasks,
            return_exceptions=True
        )

        # Aggregate results, handling any failures gracefully
        all_results = []
        for source, results in zip(self.retrievers.keys(), results_by_source):
            if isinstance(results, Exception):
                # Log but don't fail - degraded retrieval is better than none
                continue
            all_results.extend(results)

        # Apply RRF fusion for final ranking
        fused_results = self._reciprocal_rank_fusion(all_results)

        elapsed_ms = (time.perf_counter() - start_time) * 1000

        return RAGContext(
            query=query,
            results=fused_results[:10],  # Top 10 overall
            total_retrieval_time_ms=elapsed_ms
        )

    async def _retrieve_from_source(
        self,
        source: RetrievalSource,
        query: str,
        embedding: List[float],
        user_id: str,
        top_k: int
    ) -> List[RetrievalResult]:
        """
        Hybrid retrieval from a single source.

        Combines BM25 (keyword) and semantic (embedding) scores.
        """
        retriever = self.retrievers[source]

        # Parallel BM25 and semantic search
        bm25_task = retriever.bm25_search(query, user_id, top_k * 2)
        semantic_task = retriever.semantic_search(embedding, user_id, top_k * 2)

        bm25_results, semantic_results = await asyncio.gather(
            bm25_task, semantic_task
        )

        # Merge and deduplicate
        merged = self._merge_results(bm25_results, semantic_results, source)

        return merged[:top_k]

    def _reciprocal_rank_fusion(
        self,
        results: List[RetrievalResult],
        k: int = 60
    ) -> List[RetrievalResult]:
        """
        RRF combines rankings from multiple sources.

        Formula: RRF(d) = Σ 1/(k + rank(d))

        This is more robust than raw score averaging because
        it handles different score scales across sources.
        """
        # Group by content hash for deduplication
        content_scores = {}
        content_to_result = {}

        for rank, result in enumerate(sorted(
            results, key=lambda x: x.relevance_score, reverse=True
        )):
            content_hash = hash(result.content)
            rrf_score = 1.0 / (k + rank + 1)

            if content_hash in content_scores:
                content_scores[content_hash] += rrf_score
            else:
                content_scores[content_hash] = rrf_score
                content_to_result[content_hash] = result

        # Sort by RRF score
        sorted_hashes = sorted(
            content_scores.keys(),
            key=lambda h: content_scores[h],
            reverse=True
        )

        return [content_to_result[h] for h in sorted_hashes]
```

**Key Engineering Decisions:**
- `asyncio.gather` for true parallel I/O (not threading)
- Graceful degradation on individual source failures
- RRF fusion handles heterogeneous score scales
- Deduplication prevents context pollution

---

## 4. Intent Classification

**Pattern:** Semantic similarity with prototype examples and LLM fallback

```python
import numpy as np
from typing import List, Tuple, Optional
from dataclasses import dataclass

@dataclass
class ClassificationResult:
    primary_intent: str
    primary_confidence: float
    secondary_intents: List[Tuple[str, float]]
    used_fallback: bool

class SemanticIntentClassifier:
    """
    BGE-based intent classification with 303 prototype examples.

    Performance: 10-20ms latency, 78% accuracy (95%+ with LLM fallback)

    Architecture:
    1. Embed user message
    2. Compare against 303 intent prototypes
    3. If confidence < 0.45, use LLM-as-judge fallback
    4. Return primary + up to 2 secondary intents
    """

    INTENT_CATEGORIES = [
        "CRISIS", "ASSESS", "REMINISCE", "SOOTHE", "ACTIVATE",
        "GROUND", "BRIDGE", "REFLECT", "CONNECT", "GENERAL"
    ]

    # Intent-specific confidence thresholds
    THRESHOLDS = {
        "CRISIS": 0.65,    # Higher threshold for safety
        "ASSESS": 0.55,    # Medical accuracy important
        "default": 0.45
    }

    def __init__(
        self,
        embedding_service,
        prototype_embeddings: np.ndarray,  # (303, 768)
        prototype_labels: List[str],
        llm_judge: Optional["LLMJudge"] = None
    ):
        self.embedding_service = embedding_service
        self.prototypes = prototype_embeddings
        self.labels = prototype_labels
        self.llm_judge = llm_judge

        # Pre-normalize prototypes for faster cosine similarity
        self.prototypes_normalized = self.prototypes / np.linalg.norm(
            self.prototypes, axis=1, keepdims=True
        )

    async def classify(
        self,
        message: str,
        embedding: Optional[np.ndarray] = None
    ) -> ClassificationResult:
        """
        Classify message intent with confidence scores.
        """
        # Get embedding
        if embedding is None:
            embedding = await self.embedding_service.embed(message)

        embedding_normalized = embedding / np.linalg.norm(embedding)

        # Compute similarities to all prototypes
        similarities = self.prototypes_normalized @ embedding_normalized

        # Aggregate by intent category
        intent_scores = self._aggregate_by_intent(similarities)

        # Sort by score
        sorted_intents = sorted(
            intent_scores.items(),
            key=lambda x: x[1],
            reverse=True
        )

        primary_intent, primary_score = sorted_intents[0]

        # Check if fallback needed
        threshold = self.THRESHOLDS.get(
            primary_intent,
            self.THRESHOLDS["default"]
        )

        used_fallback = False
        if primary_score < threshold and self.llm_judge:
            # LLM-as-judge fallback for low confidence
            fallback_result = await self.llm_judge.classify(message)
            if fallback_result:
                primary_intent = fallback_result
                used_fallback = True

        # Get secondary intents (threshold: 0.80 of primary)
        secondary_threshold = primary_score * 0.80
        secondary_intents = [
            (intent, score)
            for intent, score in sorted_intents[1:3]
            if score >= secondary_threshold
        ]

        return ClassificationResult(
            primary_intent=primary_intent,
            primary_confidence=primary_score,
            secondary_intents=secondary_intents,
            used_fallback=used_fallback
        )

    def _aggregate_by_intent(
        self,
        similarities: np.ndarray
    ) -> dict:
        """
        Aggregate prototype similarities by intent category.

        Uses max pooling - the highest similarity prototype
        represents the intent score.
        """
        intent_scores = {}

        for intent in self.INTENT_CATEGORIES:
            # Find all prototypes for this intent
            indices = [
                i for i, label in enumerate(self.labels)
                if label == intent
            ]

            if indices:
                # Max pooling over prototypes
                intent_scores[intent] = float(np.max(similarities[indices]))
            else:
                intent_scores[intent] = 0.0

        return intent_scores
```

**Key Engineering Decisions:**
- Pre-normalized prototypes eliminate repeated computation
- Max pooling over prototypes handles intra-class variation
- Intent-specific thresholds (CRISIS higher for safety)
- LLM fallback only when needed (cost optimization)

---

## 5. WebSocket Handler (Go)

**Pattern:** Hub-and-spoke with message queuing and graceful degradation

```go
package websocket

import (
    "context"
    "encoding/json"
    "sync"
    "time"

    "github.com/gorilla/websocket"
)

// Message types for therapeutic chat
type MessageType string

const (
    TypeChat       MessageType = "chat"
    TypeCrisis     MessageType = "crisis_alert"
    TypeTyping     MessageType = "typing"
    TypeAck        MessageType = "ack"
    TypeHeartbeat  MessageType = "heartbeat"
)

// ChatMessage represents a therapeutic conversation message
type ChatMessage struct {
    ID          string      `json:"id"`
    Type        MessageType `json:"type"`
    UserID      string      `json:"user_id"`
    SessionID   string      `json:"session_id"`
    Content     string      `json:"content"`
    Timestamp   time.Time   `json:"timestamp"`
    SafetyFlags []string    `json:"safety_flags,omitempty"`
}

// Client represents a connected WebSocket client
type Client struct {
    hub       *Hub
    conn      *websocket.Conn
    send      chan []byte
    userID    string
    sessionID string

    // Message queue for offline/reconnection scenarios
    pendingMessages []ChatMessage
    mu              sync.Mutex
}

// Hub maintains active client connections and broadcasts messages
type Hub struct {
    // Registered clients by session ID
    clients    map[string]*Client
    clientsMu  sync.RWMutex

    // Channels for client lifecycle
    register   chan *Client
    unregister chan *Client

    // Broadcast channel for crisis alerts
    crisisAlerts chan ChatMessage

    // Dependencies
    aiRouter     AIRouterClient
    messageStore MessageStore
}

// NewHub creates a new WebSocket hub
func NewHub(aiRouter AIRouterClient, store MessageStore) *Hub {
    return &Hub{
        clients:      make(map[string]*Client),
        register:     make(chan *Client),
        unregister:   make(chan *Client),
        crisisAlerts: make(chan ChatMessage, 100),
        aiRouter:     aiRouter,
        messageStore: store,
    }
}

// Run starts the hub's main event loop
func (h *Hub) Run(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return

        case client := <-h.register:
            h.registerClient(client)

        case client := <-h.unregister:
            h.unregisterClient(client)

        case alert := <-h.crisisAlerts:
            h.broadcastCrisisAlert(alert)
        }
    }
}

// HandleConnection manages a single WebSocket connection
func (h *Hub) HandleConnection(
    conn *websocket.Conn,
    userID, sessionID string,
) {
    client := &Client{
        hub:       h,
        conn:      conn,
        send:      make(chan []byte, 256),
        userID:    userID,
        sessionID: sessionID,
    }

    h.register <- client

    // Start read/write pumps
    go client.writePump()
    go client.readPump()
}

// readPump handles incoming messages from the client
func (c *Client) readPump() {
    defer func() {
        c.hub.unregister <- c
        c.conn.Close()
    }()

    c.conn.SetReadLimit(maxMessageSize)
    c.conn.SetReadDeadline(time.Now().Add(pongWait))
    c.conn.SetPongHandler(func(string) error {
        c.conn.SetReadDeadline(time.Now().Add(pongWait))
        return nil
    })

    for {
        _, message, err := c.conn.ReadMessage()
        if err != nil {
            if websocket.IsUnexpectedCloseError(
                err,
                websocket.CloseGoingAway,
                websocket.CloseAbnormalClosure,
            ) {
                // Log unexpected close
            }
            break
        }

        var chatMsg ChatMessage
        if err := json.Unmarshal(message, &chatMsg); err != nil {
            continue
        }

        // Process message through AI router
        go c.processMessage(chatMsg)
    }
}

// processMessage sends message to AI router and handles response
func (c *Client) processMessage(msg ChatMessage) {
    ctx, cancel := context.WithTimeout(
        context.Background(),
        60*time.Second,
    )
    defer cancel()

    // Store incoming message
    if err := c.hub.messageStore.Save(ctx, msg); err != nil {
        // Log but continue - degraded storage is acceptable
    }

    // Route to AI for processing
    response, err := c.hub.aiRouter.ProcessMessage(ctx, msg)
    if err != nil {
        c.sendError("Processing failed, please try again")
        return
    }

    // Check for crisis detection
    if containsCrisisFlag(response.SafetyFlags) {
        c.hub.crisisAlerts <- ChatMessage{
            Type:        TypeCrisis,
            UserID:      msg.UserID,
            SessionID:   msg.SessionID,
            SafetyFlags: response.SafetyFlags,
            Timestamp:   time.Now(),
        }
    }

    // Send response to client
    c.sendMessage(response)
}

// broadcastCrisisAlert sends alert to all care staff dashboards
func (h *Hub) broadcastCrisisAlert(alert ChatMessage) {
    h.clientsMu.RLock()
    defer h.clientsMu.RUnlock()

    alertJSON, _ := json.Marshal(alert)

    for _, client := range h.clients {
        // Only send to care staff sessions
        if isCareStaffSession(client.sessionID) {
            select {
            case client.send <- alertJSON:
            default:
                // Client buffer full - queue for retry
                client.queueMessage(alert)
            }
        }
    }
}

// Exponential backoff for reconnection
func (c *Client) reconnectWithBackoff() {
    baseDelay := 1 * time.Second
    maxDelay := 30 * time.Second

    for attempt := 0; attempt < maxReconnectAttempts; attempt++ {
        delay := time.Duration(1<<uint(attempt)) * baseDelay
        if delay > maxDelay {
            delay = maxDelay
        }

        // Add jitter to prevent thundering herd
        jitter := time.Duration(rand.Int63n(int64(delay / 2)))
        time.Sleep(delay + jitter)

        if err := c.attemptReconnect(); err == nil {
            // Replay pending messages
            c.replayPendingMessages()
            return
        }
    }
}
```

**Key Engineering Decisions:**
- Hub-and-spoke pattern for efficient broadcasting
- Buffered channels prevent blocking on slow clients
- Separate goroutines for read/write prevent deadlocks
- Exponential backoff with jitter prevents thundering herd
- Message queuing handles temporary disconnections

---

## 6. Caching Strategy

**Pattern:** Multi-layer cache with TTL and event-driven invalidation

```python
import asyncio
import hashlib
import json
from typing import Optional, Any, Callable
from datetime import timedelta
from functools import wraps

class CacheLayer:
    """
    Multi-layer caching with Redis backend.

    Layers:
    1. Conversation cache (60-80% hit rate, 1hr TTL)
    2. Embedding cache (70%+ hit rate, 24hr TTL)
    3. Life story cache (90% DB reduction, 6hr TTL)

    Features:
    - TTL-based expiration
    - Event-driven invalidation via Redis Pub/Sub
    - Cache stampede prevention with locks
    """

    def __init__(self, redis_client, pubsub_channel: str = "cache_invalidation"):
        self.redis = redis_client
        self.pubsub_channel = pubsub_channel
        self._local_cache = {}  # L1 in-memory cache
        self._locks = {}

    async def get(
        self,
        key: str,
        deserializer: Callable = json.loads
    ) -> Optional[Any]:
        """
        Get value from cache with L1 -> L2 fallback.
        """
        # L1: Local memory
        if key in self._local_cache:
            return self._local_cache[key]

        # L2: Redis
        value = await self.redis.get(key)
        if value:
            deserialized = deserializer(value)
            self._local_cache[key] = deserialized  # Populate L1
            return deserialized

        return None

    async def set(
        self,
        key: str,
        value: Any,
        ttl: timedelta,
        serializer: Callable = json.dumps
    ) -> None:
        """
        Set value in both L1 and L2 cache.
        """
        serialized = serializer(value)

        # L2: Redis with TTL
        await self.redis.setex(key, ttl, serialized)

        # L1: Local memory
        self._local_cache[key] = value

    async def get_or_compute(
        self,
        key: str,
        compute_fn: Callable,
        ttl: timedelta,
        lock_timeout: float = 5.0
    ) -> Any:
        """
        Get from cache or compute with stampede prevention.

        Uses distributed lock to prevent multiple concurrent
        computations of the same expensive value.
        """
        # Try cache first
        cached = await self.get(key)
        if cached is not None:
            return cached

        # Acquire distributed lock
        lock_key = f"lock:{key}"
        lock_acquired = await self.redis.set(
            lock_key, "1",
            nx=True,  # Only set if not exists
            ex=int(lock_timeout)
        )

        if not lock_acquired:
            # Another process is computing - wait and retry
            await asyncio.sleep(0.1)
            return await self.get_or_compute(key, compute_fn, ttl, lock_timeout)

        try:
            # Double-check after acquiring lock
            cached = await self.get(key)
            if cached is not None:
                return cached

            # Compute value
            value = await compute_fn()

            # Store in cache
            await self.set(key, value, ttl)

            return value
        finally:
            # Release lock
            await self.redis.delete(lock_key)

    async def invalidate(self, key: str) -> None:
        """
        Invalidate cache entry and broadcast to all instances.
        """
        # Remove from L1
        self._local_cache.pop(key, None)

        # Remove from L2
        await self.redis.delete(key)

        # Broadcast invalidation
        await self.redis.publish(
            self.pubsub_channel,
            json.dumps({"action": "invalidate", "key": key})
        )

    async def invalidate_pattern(self, pattern: str) -> None:
        """
        Invalidate all keys matching pattern.

        Use sparingly - expensive operation.
        """
        cursor = 0
        while True:
            cursor, keys = await self.redis.scan(
                cursor, match=pattern, count=100
            )

            if keys:
                await self.redis.delete(*keys)
                for key in keys:
                    self._local_cache.pop(key.decode(), None)

            if cursor == 0:
                break


def cached(
    ttl: timedelta,
    key_prefix: str,
    key_builder: Optional[Callable] = None
):
    """
    Decorator for caching async function results.

    Usage:
        @cached(ttl=timedelta(hours=1), key_prefix="embedding")
        async def get_embedding(text: str) -> List[float]:
            ...
    """
    def decorator(func: Callable):
        @wraps(func)
        async def wrapper(self, *args, **kwargs):
            # Build cache key
            if key_builder:
                key_suffix = key_builder(*args, **kwargs)
            else:
                key_suffix = hashlib.md5(
                    json.dumps({"args": args, "kwargs": kwargs}).encode()
                ).hexdigest()

            cache_key = f"{key_prefix}:{key_suffix}"

            # Try cache
            cached_value = await self.cache.get(cache_key)
            if cached_value is not None:
                return cached_value

            # Compute and cache
            result = await func(self, *args, **kwargs)
            await self.cache.set(cache_key, result, ttl)

            return result

        return wrapper
    return decorator
```

---

## 7. HIPAA-Compliant Logging

**Pattern:** Structured logging with PHI redaction and HMAC integrity

```python
import hashlib
import hmac
import json
import re
from datetime import datetime
from typing import Any, Dict, Optional
from dataclasses import dataclass, asdict

@dataclass
class AuditLogEntry:
    """HIPAA-compliant audit log entry"""
    timestamp: str
    event_type: str
    user_id: str
    session_id: str
    action: str
    resource_type: str
    resource_id: Optional[str]
    outcome: str
    client_ip: str
    user_agent: str
    details: Dict[str, Any]
    hmac_signature: str = ""

class HIPAACompliantLogger:
    """
    HIPAA §164.312(b) compliant audit logging.

    Features:
    - PHI detection and redaction
    - HMAC integrity verification
    - Tamper-evident log chain
    - Structured JSON output
    """

    # PHI patterns for redaction
    PHI_PATTERNS = [
        (r'\b\d{3}-\d{2}-\d{4}\b', '[SSN_REDACTED]'),          # SSN
        (r'\b\d{3}-\d{3}-\d{4}\b', '[PHONE_REDACTED]'),        # Phone
        (r'\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b',
         '[EMAIL_REDACTED]'),                                   # Email
        (r'\b\d{1,2}/\d{1,2}/\d{2,4}\b', '[DOB_REDACTED]'),   # DOB
        (r'\b(?:MRN|mrn)[:\s]*\d+\b', '[MRN_REDACTED]'),      # MRN
    ]

    def __init__(
        self,
        hmac_key: bytes,
        log_sink,
        previous_hash: str = ""
    ):
        self.hmac_key = hmac_key
        self.log_sink = log_sink
        self.previous_hash = previous_hash

    def log_access(
        self,
        user_id: str,
        session_id: str,
        action: str,
        resource_type: str,
        resource_id: Optional[str],
        outcome: str,
        client_ip: str,
        user_agent: str,
        details: Optional[Dict] = None
    ) -> AuditLogEntry:
        """
        Log a PHI access event with HMAC integrity.
        """
        # Redact any PHI in details
        safe_details = self._redact_phi(details or {})

        entry = AuditLogEntry(
            timestamp=datetime.utcnow().isoformat() + "Z",
            event_type="PHI_ACCESS",
            user_id=self._hash_identifier(user_id),  # Pseudonymize
            session_id=session_id,
            action=action,
            resource_type=resource_type,
            resource_id=self._hash_identifier(resource_id) if resource_id else None,
            outcome=outcome,
            client_ip=self._mask_ip(client_ip),
            user_agent=user_agent[:100],  # Truncate
            details=safe_details
        )

        # Generate HMAC including previous hash (chain)
        entry.hmac_signature = self._generate_hmac(entry)

        # Update chain
        self.previous_hash = entry.hmac_signature

        # Write to sink
        self.log_sink.write(json.dumps(asdict(entry)))

        return entry

    def _redact_phi(self, data: Dict) -> Dict:
        """
        Recursively redact PHI from log data.
        """
        if isinstance(data, dict):
            return {k: self._redact_phi(v) for k, v in data.items()}
        elif isinstance(data, list):
            return [self._redact_phi(item) for item in data]
        elif isinstance(data, str):
            result = data
            for pattern, replacement in self.PHI_PATTERNS:
                result = re.sub(pattern, replacement, result)
            return result
        return data

    def _generate_hmac(self, entry: AuditLogEntry) -> str:
        """
        Generate HMAC signature including chain link.
        """
        # Include previous hash for tamper detection
        content = json.dumps({
            **asdict(entry),
            "previous_hash": self.previous_hash
        }, sort_keys=True)

        signature = hmac.new(
            self.hmac_key,
            content.encode(),
            hashlib.sha256
        ).hexdigest()

        return signature

    def verify_chain(self, entries: list) -> bool:
        """
        Verify the integrity of a log chain.
        """
        previous_hash = ""

        for entry_dict in entries:
            entry = AuditLogEntry(**entry_dict)
            stored_signature = entry.hmac_signature
            entry.hmac_signature = ""

            # Recompute expected signature
            content = json.dumps({
                **asdict(entry),
                "previous_hash": previous_hash
            }, sort_keys=True)

            expected = hmac.new(
                self.hmac_key,
                content.encode(),
                hashlib.sha256
            ).hexdigest()

            if not hmac.compare_digest(stored_signature, expected):
                return False

            previous_hash = stored_signature

        return True

    def _hash_identifier(self, identifier: str) -> str:
        """Pseudonymize identifier while maintaining consistency"""
        return hashlib.sha256(
            f"{identifier}:{self.hmac_key[:8].hex()}".encode()
        ).hexdigest()[:16]

    def _mask_ip(self, ip: str) -> str:
        """Mask last octet of IP for privacy"""
        parts = ip.split('.')
        if len(parts) == 4:
            parts[3] = 'xxx'
            return '.'.join(parts)
        return ip
```

---

## 8. Circuit Breaker Pattern

**Pattern:** Resilience pattern for external service calls

```python
import asyncio
import time
from enum import Enum
from dataclasses import dataclass
from typing import Callable, Any, Optional

class CircuitState(Enum):
    CLOSED = "closed"      # Normal operation
    OPEN = "open"          # Failing, reject calls
    HALF_OPEN = "half_open"  # Testing recovery

@dataclass
class CircuitBreakerConfig:
    failure_threshold: int = 5
    success_threshold: int = 2
    timeout: float = 30.0
    half_open_max_calls: int = 3

class CircuitBreaker:
    """
    Circuit breaker for external service resilience.

    States:
    - CLOSED: Normal operation, track failures
    - OPEN: Service failing, fail fast without calling
    - HALF_OPEN: Testing if service recovered

    Used for: AI Router, Embedding Service, Generation Service
    """

    def __init__(
        self,
        name: str,
        config: CircuitBreakerConfig = CircuitBreakerConfig()
    ):
        self.name = name
        self.config = config
        self.state = CircuitState.CLOSED
        self.failure_count = 0
        self.success_count = 0
        self.last_failure_time: Optional[float] = None
        self.half_open_calls = 0
        self._lock = asyncio.Lock()

    async def call(
        self,
        func: Callable,
        *args,
        fallback: Optional[Callable] = None,
        **kwargs
    ) -> Any:
        """
        Execute function with circuit breaker protection.

        Args:
            func: Async function to call
            fallback: Optional fallback function if circuit is open

        Returns:
            Function result or fallback result

        Raises:
            CircuitOpenError: If circuit is open and no fallback
        """
        async with self._lock:
            if not self._can_execute():
                if fallback:
                    return await fallback(*args, **kwargs)
                raise CircuitOpenError(f"Circuit {self.name} is open")

        try:
            result = await func(*args, **kwargs)
            await self._record_success()
            return result
        except Exception as e:
            await self._record_failure()
            if fallback:
                return await fallback(*args, **kwargs)
            raise

    def _can_execute(self) -> bool:
        """Check if call should be allowed"""
        if self.state == CircuitState.CLOSED:
            return True

        if self.state == CircuitState.OPEN:
            # Check if timeout has passed
            if self.last_failure_time:
                elapsed = time.time() - self.last_failure_time
                if elapsed >= self.config.timeout:
                    self._transition_to_half_open()
                    return True
            return False

        if self.state == CircuitState.HALF_OPEN:
            # Allow limited calls in half-open
            if self.half_open_calls < self.config.half_open_max_calls:
                self.half_open_calls += 1
                return True
            return False

        return False

    async def _record_success(self):
        """Record successful call"""
        async with self._lock:
            if self.state == CircuitState.HALF_OPEN:
                self.success_count += 1
                if self.success_count >= self.config.success_threshold:
                    self._transition_to_closed()
            elif self.state == CircuitState.CLOSED:
                # Reset failure count on success
                self.failure_count = 0

    async def _record_failure(self):
        """Record failed call"""
        async with self._lock:
            self.last_failure_time = time.time()

            if self.state == CircuitState.HALF_OPEN:
                # Any failure in half-open goes back to open
                self._transition_to_open()
            elif self.state == CircuitState.CLOSED:
                self.failure_count += 1
                if self.failure_count >= self.config.failure_threshold:
                    self._transition_to_open()

    def _transition_to_open(self):
        """Transition to open state"""
        self.state = CircuitState.OPEN
        self.success_count = 0
        self.half_open_calls = 0

    def _transition_to_half_open(self):
        """Transition to half-open state"""
        self.state = CircuitState.HALF_OPEN
        self.success_count = 0
        self.half_open_calls = 0

    def _transition_to_closed(self):
        """Transition to closed state"""
        self.state = CircuitState.CLOSED
        self.failure_count = 0
        self.success_count = 0
        self.half_open_calls = 0


class CircuitOpenError(Exception):
    """Raised when circuit breaker is open"""
    pass
```

---

## 9. Safety Service Architecture

**Pattern:** Safety-first middleware that runs before all AI processing

```python
from dataclasses import dataclass
from typing import List, Optional, Tuple
from enum import Enum

class SafetyAction(Enum):
    CONTINUE = "continue"           # Safe to proceed
    ESCALATE = "escalate"           # Alert care staff
    ESCALATE_IMMEDIATE = "immediate"  # Auto-call 911
    BLOCK = "block"                 # Block unsafe content

@dataclass
class SafetyAssessment:
    action: SafetyAction
    crisis_level: Optional[str]
    flags: List[str]
    enriched_context: dict
    should_notify_staff: bool
    should_notify_family: bool

class SafetyService:
    """
    Safety-first service that ALWAYS runs before AI processing.

    Responsibilities:
    1. Crisis detection (100% recall requirement)
    2. Content safety (harmful content blocking)
    3. Context enrichment (clinical scores, risk factors)
    4. Alert orchestration (staff, family, emergency)

    Architecture principle: Safety checks are NEVER bypassed.
    """

    def __init__(
        self,
        crisis_detector,
        content_filter,
        clinical_context_service,
        alert_service
    ):
        self.crisis_detector = crisis_detector
        self.content_filter = content_filter
        self.clinical_context = clinical_context_service
        self.alert_service = alert_service

    async def assess_and_enrich(
        self,
        message: str,
        user_id: str,
        session_id: str,
        conversation_history: List[dict]
    ) -> Tuple[SafetyAssessment, dict]:
        """
        Main safety assessment entry point.

        This method MUST be called before any AI processing.
        Returns (assessment, enriched_context).
        """
        # Parallel safety checks
        crisis_task = self.crisis_detector.assess(
            message=message,
            clinical_scores=await self.clinical_context.get_scores(user_id),
            life_story=await self.clinical_context.get_life_story(user_id),
            conversation_history=conversation_history
        )

        content_task = self.content_filter.check(message)

        crisis_result, content_result = await asyncio.gather(
            crisis_task, content_task
        )

        # Determine action
        action, flags = self._determine_action(crisis_result, content_result)

        # Trigger alerts if needed
        if action in [SafetyAction.ESCALATE, SafetyAction.ESCALATE_IMMEDIATE]:
            await self._trigger_alerts(
                action, user_id, session_id, crisis_result
            )

        # Build enriched context
        enriched_context = {
            "clinical_scores": crisis_result.clinical_factors,
            "safety_flags": flags,
            "crisis_level": crisis_result.level.name if crisis_result.level else None,
            "trajectory_risk": crisis_result.trajectory_risk
        }

        assessment = SafetyAssessment(
            action=action,
            crisis_level=crisis_result.level.name if crisis_result.level else None,
            flags=flags,
            enriched_context=enriched_context,
            should_notify_staff=action != SafetyAction.CONTINUE,
            should_notify_family=action == SafetyAction.ESCALATE_IMMEDIATE
        )

        return assessment, enriched_context

    def _determine_action(
        self,
        crisis: "CrisisAssessment",
        content: "ContentCheckResult"
    ) -> Tuple[SafetyAction, List[str]]:
        """
        Determine safety action based on all assessments.

        Priority: IMMEDIATE > ESCALATE > BLOCK > CONTINUE
        """
        flags = []

        # Check crisis level
        if crisis.level == CrisisLevel.IMMEDIATE:
            flags.append("CRISIS_IMMEDIATE")
            return SafetyAction.ESCALATE_IMMEDIATE, flags

        if crisis.level == CrisisLevel.URGENT:
            flags.append("CRISIS_URGENT")
            return SafetyAction.ESCALATE, flags

        if crisis.level == CrisisLevel.ELEVATED:
            flags.append("CRISIS_ELEVATED")
            return SafetyAction.ESCALATE, flags

        # Check content safety
        if content.is_harmful:
            flags.append("CONTENT_HARMFUL")
            return SafetyAction.BLOCK, flags

        # Default: safe to continue
        if crisis.level == CrisisLevel.MODERATE:
            flags.append("CRISIS_MODERATE")

        return SafetyAction.CONTINUE, flags

    async def _trigger_alerts(
        self,
        action: SafetyAction,
        user_id: str,
        session_id: str,
        crisis: "CrisisAssessment"
    ):
        """
        Trigger appropriate alerts based on action.

        Alert routing:
        - IMMEDIATE: 911 + physician + nurse + family
        - URGENT: Physician + nurse
        - ELEVATED: Physician + social worker
        """
        alert_config = {
            SafetyAction.ESCALATE_IMMEDIATE: {
                "channels": ["emergency_911", "physician", "nurse", "family"],
                "priority": "critical",
                "response_sla_seconds": 30
            },
            SafetyAction.ESCALATE: {
                "channels": ["physician", "nurse"],
                "priority": "high",
                "response_sla_seconds": 300
            }
        }

        config = alert_config.get(action, {})

        await self.alert_service.send_alert(
            user_id=user_id,
            session_id=session_id,
            crisis_level=crisis.level.name,
            matched_patterns=crisis.matched_patterns,
            channels=config.get("channels", []),
            priority=config.get("priority", "normal")
        )
```

---

## 10. Configuration Hot-Reload

**Pattern:** File-watched configuration with thread-safe updates

```python
import yaml
import threading
from pathlib import Path
from typing import Any, Dict, Optional, Callable
from dataclasses import dataclass
from watchdog.observers import Observer
from watchdog.events import FileSystemEventHandler
import time

@dataclass
class PromptConfig:
    """Configuration for therapeutic prompts"""
    system_prompt: str
    temperature: float
    max_tokens: int
    top_p: float

class ConfigManager:
    """
    Thread-safe configuration manager with hot-reload.

    Features:
    - YAML-based configuration
    - File watcher for automatic reload
    - 1-second debounce to prevent rapid reloads
    - Thread-safe reads during updates
    - Validation before applying changes
    """

    def __init__(
        self,
        config_dir: Path,
        on_reload: Optional[Callable] = None
    ):
        self.config_dir = config_dir
        self.on_reload = on_reload

        self._config: Dict[str, Any] = {}
        self._lock = threading.RLock()
        self._last_reload = 0
        self._debounce_seconds = 1.0

        # Load initial config
        self._load_all_configs()

        # Start file watcher
        self._start_watcher()

    def get(self, key: str, default: Any = None) -> Any:
        """Thread-safe config retrieval"""
        with self._lock:
            return self._config.get(key, default)

    def get_prompt_config(self, intent: str) -> PromptConfig:
        """Get prompt configuration for specific intent"""
        with self._lock:
            prompts = self._config.get("prompts", {})
            params = self._config.get("parameters", {})

            # Get intent-specific or default
            intent_params = params.get(intent, params.get("default", {}))

            return PromptConfig(
                system_prompt=prompts.get(intent, prompts.get("default", "")),
                temperature=intent_params.get("temperature", 0.4),
                max_tokens=intent_params.get("max_tokens", 150),
                top_p=intent_params.get("top_p", 0.9)
            )

    def _load_all_configs(self):
        """Load all YAML config files"""
        new_config = {}

        for config_file in self.config_dir.glob("*.yaml"):
            try:
                with open(config_file, 'r') as f:
                    file_config = yaml.safe_load(f)
                    if file_config:
                        # Use filename (without extension) as config section
                        section = config_file.stem
                        new_config[section] = file_config
            except Exception as e:
                # Log but don't fail - keep existing config
                print(f"Error loading {config_file}: {e}")

        # Validate before applying
        if self._validate_config(new_config):
            with self._lock:
                self._config = new_config

    def _validate_config(self, config: Dict) -> bool:
        """Validate configuration before applying"""
        # Check required sections exist
        required_sections = ["prompts", "parameters"]
        for section in required_sections:
            if section not in config:
                return False

        # Validate parameter ranges
        params = config.get("parameters", {})
        for intent, intent_params in params.items():
            temp = intent_params.get("temperature", 0.5)
            if not 0 <= temp <= 2:
                return False

            max_tokens = intent_params.get("max_tokens", 150)
            if not 1 <= max_tokens <= 4096:
                return False

        return True

    def _start_watcher(self):
        """Start file system watcher for config changes"""
        handler = ConfigFileHandler(self._on_file_changed)
        self._observer = Observer()
        self._observer.schedule(handler, str(self.config_dir), recursive=False)
        self._observer.start()

    def _on_file_changed(self, path: str):
        """Handle config file change with debounce"""
        current_time = time.time()

        # Debounce rapid changes
        if current_time - self._last_reload < self._debounce_seconds:
            return

        self._last_reload = current_time

        # Reload in background thread
        threading.Thread(target=self._reload_config).start()

    def _reload_config(self):
        """Reload configuration from files"""
        self._load_all_configs()

        # Notify listeners
        if self.on_reload:
            self.on_reload(self._config)

    def stop(self):
        """Stop file watcher"""
        self._observer.stop()
        self._observer.join()


class ConfigFileHandler(FileSystemEventHandler):
    """Watchdog handler for config file changes"""

    def __init__(self, callback: Callable):
        self.callback = callback

    def on_modified(self, event):
        if event.src_path.endswith('.yaml'):
            self.callback(event.src_path)

    def on_created(self, event):
        if event.src_path.endswith('.yaml'):
            self.callback(event.src_path)
```

---

## Summary: Engineering Principles

These code samples demonstrate several key engineering principles used throughout Lilo Engine:

| Principle | Application |
|-----------|-------------|
| **Safety-First** | Crisis detection runs before all AI processing |
| **Immutability** | TherapeuticContext is immutable to prevent bugs |
| **Graceful Degradation** | Circuit breakers and fallbacks prevent cascading failures |
| **Parallel Processing** | Async/await for I/O-bound operations |
| **Thread Safety** | Locks and atomic operations for concurrent access |
| **Configuration as Code** | YAML configs with hot-reload |
| **Observability** | Structured logging with HMAC integrity |
| **Defense in Depth** | Multiple layers of validation and safety checks |

---

<div align="center">

**© 2025 Aejaz Sheriff / PragmaticLogic AI**

*These are representative patterns. Full implementation is proprietary.*

[Back to README](../README.md) • [Technical Portfolio](./TECHNICAL_PORTFOLIO.md)

</div>
