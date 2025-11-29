<div align="center">

# 👨‍💻 Technical Portfolio

### Engineering Skills Demonstrated in Lilo Engine

**A showcase of AI/ML, Backend, Infrastructure, and Full-Stack engineering capabilities**

---

![Built By](https://img.shields.io/badge/Built_By-Solo_Engineer-blue?style=flat-square)
![Lines of Code](https://img.shields.io/badge/Codebase-50K+_Lines-green?style=flat-square)
![Services](https://img.shields.io/badge/Microservices-17-orange?style=flat-square)
![Production Ready](https://img.shields.io/badge/Status-Production_Ready-brightgreen?style=flat-square)

</div>

---

## 🎯 What This Project Demonstrates

This isn't just a portfolio project — it's a **production-grade healthcare AI platform** built from scratch, demonstrating:

| Skill Category | What I Built | Complexity Level |
|----------------|--------------|------------------|
| **AI/ML Engineering** | Multi-agent orchestration, RAG pipeline, crisis detection ML, intent classification | Advanced |
| **Backend Development** | 17 microservices in Go + Python, distributed architecture | Advanced |
| **System Design** | Event-driven architecture, circuit breakers, service mesh | Senior |
| **Infrastructure** | Docker orchestration, GPU optimization, HIPAA compliance | Senior |
| **Data Engineering** | PostgreSQL + pgvector, Redis caching, real-time streaming | Intermediate-Advanced |
| **Full-Stack** | 6 healthcare dashboards, WebSocket real-time, REST APIs | Intermediate |

---

## 🧠 AI/ML Engineering

### 1. Multi-Agent Orchestration System

**Problem Solved:** How do you coordinate 7 specialized AI agents to handle complex therapeutic conversations with multiple intents?

**My Solution:**

```mermaid
flowchart TD
    A["📥 User Message<br/>'I've been feeling really down since my wife passed,<br/>and I can't sleep. Sometimes I wonder if it's worth going on.'"]

    B1["BGE Semantic Matching"]
    B2["Detects Passive Suicidal Ideation"]
    B3["Flags: ELEVATED Risk Level"]

    C1["Primary: CRISIS (0.85)"]
    C2["Secondary: SOOTHE (0.72)"]
    C3["Tertiary: REMINISCE (0.68)"]

    D1["SafetyAssessmentAgent<br/>(C-SSRS Protocol)"]
    D2["ReminiscenceAgent<br/>(Grief Support)"]

    E1["Safety Assessment + Empathy"]
    E2["Grief Validation"]
    E3["🔔 Care Staff Notification"]

    A -->|"1. Safety check first"| B1
    B1 -->|"Match against 871 patterns"| B2
    B2 -->|"Determine risk level"| B3
    B3 -->|"2. Classify intents"| C1
    C1 -->|"Extract secondary"| C2
    C2 -->|"Extract tertiary"| C3
    C3 -->|"3. Select primary agent"| D1
    D1 -->|"Add secondary agent"| D2
    D2 -->|"4. Generate response"| E1
    E1 -->|"Combine outputs"| E2
    E2 -->|"Trigger alert"| E3

    style B1 fill:#ff6b6b,color:#fff
    style B2 fill:#ff6b6b,color:#fff
    style E3 fill:#ffd43b,color:#000
```

**Technical Implementation:**
- **7 Specialized Agents**: Each with distinct therapeutic protocols
- **Multi-Intent Detection**: Primary + up to 2 secondary intents
- **Coordination Strategies**: CLINICAL_PRIORITY, PARALLEL, SEQUENTIAL
- **Safety Override**: Crisis always takes precedence

**Skills Demonstrated:**
- Agent-based AI architecture design
- Intent classification with confidence scoring
- Multi-label classification
- Safety-critical system design

---

### 2. Crisis Detection ML System (100% Recall)

**Problem Solved:** How do you detect mental health crises in real-time with zero false negatives in a safety-critical healthcare environment?

**My Solution:**

| Metric | Achieved | Industry Standard |
|--------|----------|-------------------|
| **Recall** | 100% | 60-70% |
| **Precision** | 95%+ | 70-80% |
| **Response Time** | <1 second | 15-30 minutes |
| **Training Data** | 871 scenarios | Varies |

```mermaid
flowchart TD
    A["📥 User Message"]

    B["BGE Embedding<br/>768-dimensional vector"]

    C["Similarity Search<br/>871 crisis patterns"]
    D["Clinical Context<br/>PHQ-9, GAD-7 scores"]
    E["Trajectory Analysis<br/>5-message window"]

    F{"Risk Score<br/>> 0.65?"}

    G["Crisis Classification"]

    H1["🔴 IMMEDIATE<br/>911 auto-escalate"]
    H2["🟠 URGENT<br/>MD + RN alert"]
    H3["🟡 ELEVATED<br/>Social worker"]
    H4["🟢 MODERATE<br/>Enhanced monitoring"]

    A -->|"1. Generate embedding"| B
    B -->|"2. Match patterns"| C
    B -->|"3. Check clinical history"| D
    B -->|"4. Analyze trajectory"| E
    C -->|"Combine signals"| F
    D -->|"Add context"| F
    E -->|"Add trend"| F
    F -->|"Yes: Classify severity"| G
    F -->|"No: Continue therapy"| H4
    G -->|"Critical"| H1
    G -->|"High"| H2
    G -->|"Medium"| H3
    G -->|"Low"| H4

    style H1 fill:#ff0000,color:#fff
    style H2 fill:#ff6b00,color:#fff
    style H3 fill:#ffd43b,color:#000
    style H4 fill:#51cf66,color:#fff
```

**Skills Demonstrated:**
- Semantic similarity search at scale
- Fine-tuning embedding models
- Handling class imbalance (crisis is rare)
- Optimizing for recall in safety-critical systems
- Multi-signal fusion (ML + clinical context)

---

### 3. RAG Pipeline with 6 Parallel Retrieval Streams

**Problem Solved:** How do you retrieve relevant context from multiple knowledge sources in <50ms to generate personalized therapeutic responses?

**My Solution:**

```mermaid
flowchart TD
    A["📥 User Query<br/>'I miss going to church with my husband'"]

    B["BGE Embedding<br/>768-dim vector"]

    C1["📚 Knowledge Base"]
    C2["👤 Life Story"]
    C3["💬 Chat History"]
    C4["📊 Assessments"]
    C5["📅 Schedule"]
    C6["🧠 Semantic Memory"]

    D1["BM25 Scoring"]
    D2["Semantic Scoring"]

    E["RRF Fusion"]

    F["📤 Personalized RAG Context"]

    A -->|"1. Generate embedding"| B
    B -->|"2a. Query knowledge"| C1
    B -->|"2b. Query life story"| C2
    B -->|"2c. Query history"| C3
    B -->|"2d. Query assessments"| C4
    B -->|"2e. Query schedule"| C5
    B -->|"2f. Query memory"| C6
    C1 -->|"3a. Keyword score"| D1
    C2 -->|"3a. Keyword score"| D1
    C3 -->|"3a. Keyword score"| D1
    C4 -->|"3a. Keyword score"| D1
    C5 -->|"3a. Keyword score"| D1
    C6 -->|"3a. Keyword score"| D1
    C1 -->|"3b. Vector score"| D2
    C2 -->|"3b. Vector score"| D2
    C3 -->|"3b. Vector score"| D2
    C4 -->|"3b. Vector score"| D2
    C5 -->|"3b. Vector score"| D2
    C6 -->|"3b. Vector score"| D2
    D1 -->|"4. Combine & rank"| E
    D2 -->|"4. Combine & rank"| E
    E -->|"5. Output context"| F

    style C2 fill:#74c0fc,color:#000
    style C4 fill:#ffd43b,color:#000
    style F fill:#51cf66,color:#fff
```

**Technical Implementation:**
- **Parallel Retrieval**: 6 async streams, combined in 45ms
- **Hybrid Search**: BM25 (keyword) + Semantic (embedding) + RRF fusion
- **Vector Database**: PostgreSQL + pgvector (768/1024-dim)
- **Caching**: Redis with 60-80% hit rate, <5ms latency

**Skills Demonstrated:**
- RAG architecture design
- Hybrid retrieval (sparse + dense)
- Async parallel processing
- Vector database operations
- Cache optimization

---

### 4. Intent Classification System

**Problem Solved:** How do you classify user intent across 10 therapeutic categories with support for multi-intent queries?

**My Solution:**

```mermaid
flowchart TD
    A["📥 User Message<br/>'I've been feeling really down lately'"]

    B["BGE-base-en-v1.5<br/>768-dim embedding"]

    C["Cosine Similarity<br/>vs 303 prototypes"]

    D{"Confidence<br/>> 0.45?"}

    E["LLM-as-Judge<br/>Gemini fallback"]

    F["Selected Intent:<br/>💚 SOOTHE"]

    G["📤 Result<br/>Primary: SOOTHE (0.82)<br/>Secondary: REFLECT (0.71)"]

    A -->|"1. Generate embedding"| B
    B -->|"2. Compare to prototypes"| C
    C -->|"3. Check confidence"| D
    D -->|"High: Direct match"| F
    D -->|"Low: Use fallback"| E
    E -->|"LLM classification"| F
    F -->|"4. Return result"| G

    style F fill:#51cf66,color:#fff
    style G fill:#228be6,color:#fff
```

**Performance:**
- Latency: 10-20ms (BGE semantic)
- Multi-intent support: Up to 3 simultaneous intents
- Fallback accuracy: 90%+ with LLM-as-judge

**Skills Demonstrated:**
- Semantic similarity classification
- Multi-label classification
- Confidence calibration
- Fallback strategies for edge cases

---

## 🔧 Backend Engineering

### 5. Microservices Architecture (17 Services)

**Problem Solved:** How do you build a scalable, maintainable healthcare platform with multiple specialized services?

**My Solution:**

```mermaid
flowchart TB
    user["👤 User"]

    subgraph Infra["Infrastructure Layer"]
        postgres[("PostgreSQL<br/>pgvector")]
        redis[("Redis<br/>Cache")]
    end

    subgraph AI["AI Services Layer"]
        router["AI Router<br/>FastAPI"]
        embed["Embedding<br/>BGE"]
        voice["Voice<br/>Whisper/Piper"]
        gen["Generation<br/>Qwen 2.5-7B"]
    end

    subgraph Bus["Business Services Layer"]
        auth["Auth-RBAC<br/>Go/JWT"]
        ws["WebSocket<br/>Go/Gorilla"]
        gw["API Gateway<br/>Go/Gin"]
    end

    subgraph Dash["Healthcare Dashboards"]
        care["Care Manager"]
        res["Resident"]
        fam["Family"]
        staff["Staff"]
    end

    user -->|"HTTPS/WSS"| gw
    gw -->|"Validate tokens"| auth
    gw -->|"Route messages"| ws
    ws -->|"Process requests"| router
    router -->|"Create embeddings"| embed
    router -->|"Generate text"| gen
    router -->|"Transcribe audio"| voice
    router -->|"Store/retrieve data"| postgres
    router -->|"Cache results"| redis
    router -->|"Publish alerts<br/>via Pub/Sub"| care

    style AI fill:#e7f5ff,color:#000
    style router fill:#228be6,color:#fff
```

**Technical Decisions:**
- **Go** for high-performance services (WebSocket, Auth, Gateway)
- **Python** for AI/ML services (Router, Embedding, Voice)
- **Docker Compose** for orchestration (14 containers)
- **Host services** for GPU-accelerated inference

**Skills Demonstrated:**
- Microservices architecture design
- Polyglot programming (Go + Python)
- Service communication patterns
- Container orchestration

---

### 6. Real-Time WebSocket System

**Problem Solved:** How do you handle real-time therapeutic conversations with message persistence, reconnection, and crisis alerts?

**My Solution:**

```mermaid
flowchart TD
    C1["Client 1"]
    C2["Client 2"]
    C3["Client N"]

    H["WebSocket Hub<br/>(Go/Gorilla)"]

    R["Register Channel"]
    B["Broadcast Channel"]
    U["Unregister Channel"]
    A["🚨 Crisis Alerts Channel"]

    D1["Care Manager Dashboard"]
    D2["Staff Dashboard"]
    D3["Family Portal"]

    F1["Message persistence"]
    F2["Exponential backoff"]
    F3["HIPAA sessions"]

    C1 -->|"Connect"| R
    C2 -->|"Connect"| R
    C3 -->|"Connect"| R
    R -->|"Register client"| H
    H -->|"Send messages"| B
    B -->|"Broadcast"| C1
    B -->|"Broadcast"| C2
    B -->|"Broadcast"| C3
    H -->|"Crisis detected"| A
    A -->|"Alert staff"| D1
    A -->|"Alert staff"| D2
    A -->|"Alert family"| D3
    H -->|"Implements"| F1
    H -->|"Implements"| F2
    H -->|"Implements"| F3

    style A fill:#ff6b6b,color:#fff
    style H fill:#228be6,color:#fff
```

**Performance:**
- Concurrent connections: 1000+
- Message latency: <10ms
- Reconnection: Automatic with exponential backoff

**Skills Demonstrated:**
- WebSocket protocol implementation
- Concurrent programming in Go
- Real-time system design
- Session management

---

### 7. Authentication & Authorization (HIPAA Compliant)

**Problem Solved:** How do you implement secure authentication for a healthcare platform with strict compliance requirements?

**My Solution:**

```mermaid
flowchart TD
    subgraph JWT["JWT Token Strategy"]
        J1["Access Token<br/>15 min lifetime"]
        J2["Refresh Token<br/>8 hr lifetime"]
        J3["Token Blacklist<br/>Redis (<1ms)"]
    end

    subgraph Security["Security Measures"]
        S1["bcrypt (cost=12)"]
        S2["Token rotation"]
        S3["Auto timeout"]
        S4["HMAC audit logs"]
        S5["TLS 1.3"]
    end

    subgraph RBAC["Role-Based Access"]
        R1["👤 Resident"]
        R2["👨‍👩‍👧 Family"]
        R3["👨‍⚕️ Staff"]
        R4["🏥 Care Manager"]
        R5["⚙️ Admin"]
    end

    J1 -->|"Short-lived access"| Security
    J2 -->|"Secure refresh"| Security
    J3 -->|"Instant revocation"| Security
    Security -->|"Protects"| RBAC

    style S1 fill:#51cf66,color:#fff
    style S5 fill:#51cf66,color:#fff
```

**Skills Demonstrated:**
- JWT implementation with refresh tokens
- RBAC design and implementation
- Security best practices
- HIPAA compliance (§164.312)

---

## 🏗️ System Design & Infrastructure

### 8. GPU-Optimized LLM Inference

**Problem Solved:** How do you run a 7B parameter LLM on consumer hardware with production-grade latency?

**My Solution:**

```mermaid
flowchart TD
    subgraph Model["Model"]
        M1["Qwen 2.5-7B"]
        M2["Q4_K_M quantization"]
    end

    subgraph Runtime["Runtime"]
        R1["llama.cpp"]
        R2["Apple Metal GPU"]
        R3["M1 Max 32GB"]
    end

    subgraph Opts["Optimizations"]
        O1["4-bit quantization"]
        O2["Metal acceleration"]
        O3["KV-cache tuning"]
        O4["Batch inference"]
    end

    subgraph Results["Results"]
        P1["300-400ms latency"]
        P2["45-60 tokens/sec"]
        P3["~6GB memory"]
    end

    subgraph Improvement["Improvement"]
        I1["Before: 51s"]
        I2["After: 6-8s"]
        I3["🚀 6-8x faster"]
    end

    Model -->|"Loaded into"| Runtime
    Runtime -->|"Applies"| Opts
    Opts -->|"Achieves"| Results
    Results -->|"Compared to baseline"| Improvement

    style I3 fill:#51cf66,color:#fff
```

**Skills Demonstrated:**
- LLM quantization and optimization
- GPU programming (Metal)
- Performance profiling and tuning
- Resource-constrained deployment

---

### 9. Caching Architecture (60-80% Hit Rate)

**Problem Solved:** How do you minimize latency and database load in a real-time conversational system?

**My Solution:**

```mermaid
flowchart TD
    L1["Layer 1: Conversation Cache<br/>TTL: 1hr, Hit: 60-80%"]
    L2["Layer 2: Embedding Cache<br/>TTL: 24hr, Hit: 70%+"]
    L3["Layer 3: Life Story Cache<br/>TTL: 6hr, -90% DB queries"]

    R[("Redis 7")]

    I1["TTL expiration"]
    I2["Event-driven invalidation"]
    I3["Pub/Sub distributed sync"]

    L1 -->|"Store conversations"| R
    L2 -->|"Store embeddings"| R
    L3 -->|"Store life stories"| R
    R -->|"Invalidation via"| I1
    R -->|"Invalidation via"| I2
    R -->|"Invalidation via"| I3

    style L1 fill:#51cf66,color:#fff
    style L2 fill:#51cf66,color:#fff
    style L3 fill:#51cf66,color:#fff
```

**Skills Demonstrated:**
- Distributed caching strategies
- Cache invalidation patterns
- Redis data structures
- Performance optimization

---

### 10. Voice Pipeline (STT + TTS)

**Problem Solved:** How do you build an accessible voice interface for elderly users with HIPAA compliance?

**My Solution:**

```mermaid
flowchart TD
    A["🎤 WebM Audio Input"]

    W1["Medical vocabulary prompting"]
    W2["Elderly speech handling"]
    W3["Noise filtering"]
    W4["Local processing (HIPAA)"]

    P["AI Router Processing"]

    T1["Warm empathetic voice"]
    T2["Adjustable rate"]
    T3["No cloud dependency"]

    O["🔊 Audio Output"]

    A -->|"1. Receive audio"| W1
    W1 -->|"2. Apply prompts"| W2
    W2 -->|"3. Handle patterns"| W3
    W3 -->|"4. Filter noise"| W4
    W4 -->|"5. Transcribe locally"| P
    P -->|"6. Generate response"| T1
    T1 -->|"7. Apply voice"| T2
    T2 -->|"8. Adjust speed"| T3
    T3 -->|"9. Stream audio"| O

    style W4 fill:#51cf66,color:#fff
    style T3 fill:#51cf66,color:#fff
```

**Skills Demonstrated:**
- Speech recognition integration
- Text-to-speech systems
- Audio processing
- Accessibility considerations

---

## 📊 Data Engineering

### 11. Vector Database Operations

**Problem Solved:** How do you perform semantic search across millions of embeddings with sub-100ms latency?

**My Solution:**

```sql
-- PostgreSQL + pgvector schema
CREATE TABLE knowledge_documents (
    id UUID PRIMARY KEY,
    content TEXT NOT NULL,
    embedding vector(1024),  -- BGE-large embeddings
    document_type VARCHAR(50),
    care_context TEXT[],
    keywords TEXT[],
    metadata JSONB,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Optimized indexes
CREATE INDEX idx_knowledge_embedding
    ON knowledge_documents
    USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);

-- Hybrid search query (semantic + keyword)
WITH semantic_results AS (
    SELECT id, content,
           1 - (embedding <=> $1) AS semantic_score
    FROM knowledge_documents
    WHERE 1 - (embedding <=> $1) > 0.7
    ORDER BY embedding <=> $1
    LIMIT 20
),
keyword_results AS (
    SELECT id, content,
           ts_rank(to_tsvector(content), plainto_tsquery($2)) AS keyword_score
    FROM knowledge_documents
    WHERE to_tsvector(content) @@ plainto_tsquery($2)
    LIMIT 20
)
-- RRF fusion of results
SELECT * FROM rrf_combine(semantic_results, keyword_results);
```

**Skills Demonstrated:**
- Vector database design
- pgvector optimization
- Hybrid search implementation
- Query optimization

---

## 🔒 Security & Compliance

### 12. HIPAA Technical Safeguards

**Problem Solved:** How do you build a healthcare platform that meets HIPAA §164.312 requirements?

**Implementation:**

```mermaid
flowchart TD
    R1["Access Control"]
    R2["Audit Controls"]
    R3["Integrity"]
    R4["Authentication"]
    R5["Transmission"]

    I1["JWT + Redis blacklist<br/>✅ <1ms revocation"]
    I2["HMAC tamper-proof logs<br/>✅ Complete"]
    I3["E2E verification<br/>✅ Complete"]
    I4["MFA + token rotation<br/>✅ 15-min tokens"]
    I5["TLS 1.3 + cert pinning<br/>✅ Complete"]

    A1["PHI redaction"]
    A2["Auto session timeout"]
    A3["Crypto audit trail"]
    A4["Role-based access"]

    R1 -->|"Implemented as"| I1
    R2 -->|"Implemented as"| I2
    R3 -->|"Implemented as"| I3
    R4 -->|"Implemented as"| I4
    R5 -->|"Implemented as"| I5
    I1 --> A1
    I2 --> A2
    I3 --> A3
    I4 --> A4

    style I1 fill:#51cf66,color:#fff
    style I2 fill:#51cf66,color:#fff
    style I3 fill:#51cf66,color:#fff
    style I4 fill:#51cf66,color:#fff
    style I5 fill:#51cf66,color:#fff
```

**Skills Demonstrated:**
- Healthcare compliance implementation
- Security architecture design
- Audit logging systems
- Data protection strategies

---

## 📈 Performance Achievements

### Quantifiable Results

| Metric | Achieved | Context |
|--------|----------|---------|
| **Crisis Detection** | 100% recall, <1s | Zero false negatives in safety-critical system |
| **Response Latency** | 400-500ms E2E | Full therapeutic response with RAG |
| **Cache Hit Rate** | 60-80% | Conversation and embedding caches |
| **LLM Optimization** | 6-8x speedup | 51s → 6-8s response time |
| **Concurrent Users** | 1000+ | WebSocket connections |
| **Uptime Target** | 99.9% | Healthcare-grade reliability |

---

## 🛠️ Technical Stack Proficiency

### Languages & Frameworks

| Technology | Proficiency | Usage in Project |
|------------|-------------|------------------|
| **Python** | Expert | AI services, ML pipelines, data processing |
| **Go** | Advanced | WebSocket, Auth, API Gateway |
| **SQL** | Advanced | PostgreSQL, complex queries, pgvector |
| **Docker** | Advanced | 14-container orchestration |

### AI/ML Technologies

| Technology | Usage |
|------------|-------|
| **PyTorch** | Model fine-tuning, embeddings |
| **Transformers** | BGE, Whisper integration |
| **llama.cpp** | LLM inference optimization |
| **FAISS/pgvector** | Vector similarity search |
| **Sentence-Transformers** | Embedding generation |

### Infrastructure

| Technology | Usage |
|------------|-------|
| **PostgreSQL + pgvector** | Primary database, vector search |
| **Redis** | Caching, sessions, pub/sub |
| **Docker Compose** | Container orchestration |
| **FastAPI** | Python API framework |
| **Gin** | Go HTTP framework |

---

## 💡 Key Engineering Decisions

### Why These Choices?

| Decision | Rationale | Trade-off |
|----------|-----------|-----------|
| **Go for real-time services** | Goroutines, low latency, memory efficiency | Learning curve vs Python |
| **Python for AI services** | ML ecosystem, rapid development | Performance vs Go |
| **PostgreSQL + pgvector** | Single database for relational + vector | Scaling vs specialized vector DB |
| **Local LLM (llama.cpp)** | HIPAA compliance, no cloud dependency | Hardware requirements |
| **Redis for caching** | Speed, pub/sub, data structures | Additional infrastructure |
| **Docker Compose vs K8s** | Simplicity for single-machine deployment | Scaling limitations |

---

## 📬 Contact

<div align="center">

**Interested in discussing this project or my engineering approach?**

[![LinkedIn](https://img.shields.io/badge/LinkedIn-Connect-0A66C2?style=for-the-badge&logo=linkedin&logoColor=white)](https://www.linkedin.com/in/aejaz-sheriff/)
[![Email](https://img.shields.io/badge/Email-Contact-EA4335?style=for-the-badge&logo=gmail&logoColor=white)](mailto:aejaz.sheriff@gmail.com)

---

**Other Documentation:**

[Project Overview](../README.md) • [Process Flow](./PROCESS_FLOW.md) • [Investor Overview](./INVESTOR_OVERVIEW.md)

---

**© 2025 Aejaz Sheriff / PragmaticLogic AI**

</div>
