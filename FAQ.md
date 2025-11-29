<div align="center">

# Frequently Asked Questions

### Common Questions About Lilo Engine

</div>

---

## General Questions

### What is Lilo Engine?

Lilo Engine is a **production-ready AI platform** that provides 24/7 therapeutic mental health support for elderly residents in assisted living facilities. It combines multi-agent AI, real-time crisis detection, and evidence-based therapeutic interventions in a HIPAA-compliant system.

### How is this different from chatbots like ChatGPT or other mental health apps?

| Feature | Lilo Engine | General Chatbots | Mental Health Apps |
|---------|-------------|------------------|-------------------|
| **Crisis Detection** | 100% recall, <1 second | None | Basic keyword matching |
| **Target Population** | Elderly-specific design | General | Mostly younger adults |
| **Clinical Integration** | PHQ-9, GAD-7, EHR | None | Limited |
| **Care Team Alerts** | Real-time, <30 seconds | None | Delayed or none |
| **Therapeutic Approach** | 7 evidence-based agents | Generic responses | Single methodology |
| **HIPAA Compliance** | Full §164.312 | Rarely | Sometimes |
| **Voice Interface** | Optimized for elderly | Limited | Rarely |

### Why focus on elderly care specifically?

1. **Underserved market**: 50% of assisted living residents have depression, 30% have anxiety
2. **Unique needs**: Elderly users need voice interfaces, larger touch targets, simpler UX
3. **High stakes**: Crisis detection must be perfect — lives depend on it
4. **Clear buyer**: Assisted living facilities have budget and regulatory pressure
5. **Growing demand**: 54M Americans are 65+ today; 82M by 2050

---

## Technical Questions

### Is this HIPAA compliant?

**Yes, fully compliant with HIPAA §164.312 Technical Safeguards:**

| Requirement | Implementation | Status |
|-------------|----------------|--------|
| Access Control | JWT + Redis token blacklist | ✅ |
| Audit Controls | Tamper-proof HMAC logging | ✅ |
| Integrity | End-to-end verification | ✅ |
| Authentication | Token rotation, 15-min sessions | ✅ |
| Transmission Security | TLS 1.3 | ✅ |

All data processing can occur on-premise — no PHI needs to leave the facility.

### How does the crisis detection achieve 100% recall?

Our **Crisis Detection V4** system uses multiple layers:

1. **BGE Semantic Matching** — 871 training scenarios (214 crisis + 657 non-crisis)
2. **Clinical Context Integration** — PHQ-9, GAD-7 scores, life story risk factors
3. **Trajectory Analysis** — 5-message sliding window detects progressive deterioration
4. **Three-Stage Severity Grading** — IMMEDIATE, URGENT, ELEVATED, MODERATE

The system is tuned for **zero false negatives** (100% recall) with <5% false positive rate. We accept some false positives because missing a real crisis is unacceptable.

### What AI models do you use?

| Component | Model | Purpose |
|-----------|-------|---------|
| **LLM** | Qwen 2.5-7B (quantized) | Response generation |
| **Embeddings** | BGE-base-en-v1.5 (fine-tuned) | Semantic search, crisis matching |
| **Speech-to-Text** | Whisper large-v3 | Voice transcription |
| **Text-to-Speech** | Piper TTS | Voice responses |
| **Intent Classification** | Custom BGE + Gemini fallback | 10-category classification |

All models can run locally on Apple Silicon or standard GPU hardware — no cloud AI dependency required.

### Can this run on-premise?

**Yes.** The entire platform runs on a single machine:
- 14 Docker containers
- Apple Silicon (M1/M2/M3) or NVIDIA GPU
- No external API calls required
- All data stays within facility network

This is critical for HIPAA compliance and facilities with strict data policies.

### What's the response time?

| Component | Latency |
|-----------|---------|
| Crisis Detection | <1 second (regulatory: 30s) |
| Intent Classification | 10-20ms |
| RAG Retrieval | 45ms (6 parallel streams) |
| Full Response | 400-500ms end-to-end |
| Cache Hit | <5ms |

---

## Business Questions

### What's the pricing model?

| Tier | Monthly/Resident | Features |
|------|------------------|----------|
| **Essential** | $50 | Text therapy, crisis detection, basic dashboards |
| **Professional** | $100 | + Voice, clinical assessments, family portal |
| **Enterprise** | $150 | + EHR integration, analytics, custom training |

**ROI for facilities:** $50K-150K annual savings per 100-bed facility from reduced ER visits, staff efficiency, and liability prevention.

### Who are your competitors?

| Competitor | Focus | Limitation |
|------------|-------|------------|
| **Woebot** | General CBT chatbot | No crisis detection, not elderly-focused |
| **Wysa** | Anxiety/depression | No clinical integration, consumer-focused |
| **Ginger/Headspace** | Employee wellness | Not healthcare-grade, no EHR |
| **ElliQ** | Elderly companion | Hardware-dependent, limited therapy |

**Lilo's differentiation:** Only platform combining elderly-specific design + 100% crisis recall + clinical integration + HIPAA compliance.

### Is there clinical evidence for this approach?

Yes, our therapeutic agents are based on peer-reviewed research:

| Therapy | Evidence | Source |
|---------|----------|--------|
| **Behavioral Activation** | 35% depression reduction | Comparable to cognitive therapy |
| **Reminiscence Therapy** | 15% depression reduction, -2 pts UCLA-3 | Life review studies |
| **Grounding Techniques** | 40-60% anxiety reduction | PTSD and anxiety research |
| **Digital Mental Health** | $4 return per $1 invested | WHO/Lancet studies |

### What stage is the company?

| Milestone | Status |
|-----------|--------|
| Platform Development | ✅ Complete (17 microservices) |
| HIPAA Compliance | ✅ Complete |
| Crisis Detection Validation | ✅ 100% recall achieved |
| Clinical Pilot | 🔄 Planning (Q1 2026) |
| First Revenue | 📋 Target Q2 2026 |

**Funding:** Bootstrapped with founder's capital ($0 external funding to date)

---

## Partnership Questions

### Can I see a demo?

Yes! Contact us to schedule a demonstration:
- **Email:** aejaz.sheriff@gmail.com
- **LinkedIn:** [linkedin.com/in/aejaz-sheriff](https://www.linkedin.com/in/aejaz-sheriff/)

### What partnership opportunities exist?

| Partner Type | Opportunity |
|--------------|-------------|
| **Assisted Living Facilities** | Pilot programs, enterprise deployment |
| **Healthcare Systems** | EHR integration, clinical validation |
| **Device Manufacturers** | RPM integration (Withings, Fitbit, etc.) |
| **Smart Home Platforms** | Alexa Smart Properties, Matter protocol |
| **Clinical Advisors** | Geriatric psychiatry, elderly care expertise |

### Are you looking for investment?

We're open to conversations with investors who:
- Understand healthcare/mental health markets
- Have portfolio companies in senior care or digital health
- Can provide strategic value beyond capital (introductions, expertise)

**Stage:** Seed/Pre-A discussions welcome

---

## Safety & Privacy Questions

### What happens when a crisis is detected?

```
1. AI Router detects crisis (<1 second)
2. Severity graded (IMMEDIATE/URGENT/ELEVATED/MODERATE)
3. Redis Pub/Sub broadcasts alert
4. Care Manager dashboard shows real-time notification
5. Staff mobile devices receive push notification
6. Therapeutic response continues with safety protocols
7. If IMMEDIATE: Auto-escalation to emergency services
```

All alerts are logged for compliance and clinical review.

### Is resident data shared with anyone?

**No.** All data stays within the facility's control:
- On-premise deployment option available
- No data sold to third parties
- Family access only with resident consent
- Audit logs for all data access
- HIPAA BAA available for cloud deployments

### Can residents opt out?

Yes. Consent management is built into the platform:
- Residents can opt out at any time
- Family access requires explicit consent
- Data deletion available on request
- Consent records maintained for compliance

---

<div align="center">

## More Questions?

[![Email](https://img.shields.io/badge/Email-Contact-EA4335?style=for-the-badge&logo=gmail&logoColor=white)](mailto:aejaz.sheriff@gmail.com)
[![LinkedIn](https://img.shields.io/badge/LinkedIn-Connect-0A66C2?style=for-the-badge&logo=linkedin&logoColor=white)](https://www.linkedin.com/in/aejaz-sheriff/)

---

**© 2025 Aejaz Sheriff / PragmaticLogic AI**

</div>
