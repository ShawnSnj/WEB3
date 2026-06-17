# AI Career Operating System — Redesign Plan

> **Status:** Phase 1 implemented (Resume Intelligence + Candidate Master Profile)  
> **Principle:** Optimize interview rate, response rate, offer rate — not dashboards.

---

## 1. Product redesign

### What exists today
- Working heuristic MVP: job collection, fit scoring, daily brief, 6 intelligence layers
- Split identity: `user_profile` + `user_skills` (not unified)
- No resume parsing; empty `resume_text` seed
- Homepage shows missions but jobs page still lists raw jobs first

### Target state
Single **Candidate Master Profile** drives every engine:

| Question | Engine | Phase |
|----------|--------|-------|
| Which jobs suit me? | Job Fit | 2 |
| What should I apply to today? | Daily Mission | 4 |
| Which companies to prioritize? | Target Company | 3 |
| Which skill gap matters? | Skill Gap | 5 |
| What to prepare for interviews? | Interview Readiness | 5 |
| Am I making progress? | Reality Check | 6 |

### Phase 1 ✅ (this PR)
- Upload EN + ZH resumes
- Parse → merge → `candidate_profiles` + `candidate_skills`
- Manual edit UI at `/crm/profile`
- Sync to legacy `user_profile` + `user_skills` for existing matchers

---

## 2. System architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Next.js CRM (/crm) — action-first UI                       │
└───────────────────────────┬─────────────────────────────────┘
                            │ REST /api/v1/crm/*
┌───────────────────────────▼─────────────────────────────────┐
│  CRM Service (orchestrator)                                  │
│  ┌──────────────┐ ┌────────────┐ ┌─────────────────────────┐ │
│  │ Resume Intel │ │ Job Fit    │ │ Daily Mission (L6)     │ │
│  │ (Phase 1)    │ │ (Phase 2)  │ │ (Phase 4)              │ │
│  └──────┬───────┘ └─────┬──────┘ └───────────┬─────────────┘ │
│         │               │                     │               │
│  candidate_profiles ◄───┴─────────────────────┘               │
└───────────────────────────┬─────────────────────────────────┘
                            │
              PostgreSQL + optional Kafka + OpenAI
```

**Package layout (Phase 1):**
- `internal/crm/engine/resume/` — parse + merge
- `internal/crm/model/candidate_profile.go` — domain types
- `internal/crm/repository/candidate.go` — persistence
- `internal/crm/service/candidate.go` — upload, parse, sync

---

## 3. Database migration plan

| Migration | Tables | Command |
|-----------|--------|---------|
| `crm.sql` | jobs, applications, user_profile | `make migrate-all` |
| `career_os.sql` | user_skills, market, interview | `make migrate-career-os` |
| **`resume_intelligence.sql`** | resume_documents, candidate_profiles, candidate_skills | **`make migrate-resume-intelligence`** |

**Phase 2+ tables (planned):**
- `job_fit_analysis` — extend `job_matches` with decision, ROI, missing_keywords
- `target_companies` — extend `company_profiles` with career URLs
- `daily_missions` — extend `daily_briefs` with per-item completion
- `reality_checks` — weekly funnel diagnosis

---

## 4. Backend service design

### Resume Intelligence Engine
1. `UploadResume(lang, text)` → `resume_documents`
2. `ParseResumes()` → heuristic parse EN + ZH → `Merge()` → upsert profile
3. Optional OpenAI enrichment when `OPENAI_API_KEY` set
4. `syncUserProfileFromCandidate()` — backward compat for matcher

### Engine dependency graph (future)
```
candidate_profiles
    → Job Fit Engine (Phase 2)
    → Target Company Engine (Phase 3)
    → Daily Mission Generator (Phase 4)
    → Skill Gap Engine (Phase 5, filtered to fit>70 jobs)
    → Interview Readiness (Phase 5)
    → Reality Check (Phase 6)
```

---

## 5. API design

Base: `/api/v1/crm`

| Method | Route | Phase | Status |
|--------|-------|-------|--------|
| POST | `/resumes/upload` | 1 | ✅ |
| POST | `/resumes/parse` | 1 | ✅ |
| GET | `/candidate-profile` | 1 | ✅ |
| PUT | `/candidate-profile` | 1 | ✅ |
| GET | `/jobs/recommended` | 2 | partial |
| GET | `/jobs/:id/fit` | 2 | planned |
| GET | `/companies/watchlist` | 3 | planned |
| GET | `/daily-mission/today` | 4 | via `/dashboard` |
| GET | `/skill-gaps/latest` | 5 | via `/skills` |
| GET | `/reality-check/latest` | 6 | via `/offers/predictions` |

---

## 6. Frontend page redesign

| Page | Current | Target |
|------|---------|--------|
| **Today** | Mission cards | Keep — action first |
| **Profile** | — | ✅ Upload + parse + edit |
| **Jobs** | Raw list + recommended | Ranked Apply/Maybe/Skip only |
| **Skills** | Top 3 gaps | Keep — candidate-filtered gaps (Phase 5) |
| **Companies** | — | Watchlist + open roles (Phase 3) |
| **Reality** | Coach page | Diagnosis + next action (Phase 6) |

---

## 7. Implementation tasks

### Phase 1 ✅
- [x] Migration: resume_documents, candidate_profiles, candidate_skills
- [x] Resume parser (heuristic + OpenAI optional)
- [x] EN/ZH merge logic
- [x] API: upload, parse, get/put profile
- [x] UI: `/crm/profile`
- [x] Sync to user_profile + user_skills

### Phase 2 — Job Fit Engine
- [x] `job_matches` extended with decision, career_roi_score, missing_keywords
- [x] Matcher reads `candidate_profiles` via `UserProfileFromCandidate`
- [x] Jobs page: Apply / Maybe / Skip tabs (ranked by ROI)
- [x] Homepage: apply picks only (no raw unranked lists)

### Phase 3 — Target Company Engine
- [ ] Seed 30-company watchlist from spec
- [ ] Career page fetchers (Greenhouse/Lever/Ashby)
- [ ] Open/closed job tracking

### Phase 4 — Daily Mission Generator
- [ ] `daily_missions` completion tracking
- [ ] Homepage: single 30-min mission layout

### Phase 5 — Skill Gap + Interview
- [ ] Filter gaps to jobs with fit > 70
- [ ] Link interview questions to target companies

### Phase 6 — Reality Check
- [ ] Weekly diagnosis rules (volume, mismatch, outreach, interview)
- [ ] Dedicated `/reality` page

---

## 8. Testing plan

| Layer | Tests |
|-------|-------|
| Resume parser | `internal/crm/engine/resume/parser_test.go` |
| Merge logic | unit tests EN+ZH |
| Repository | integration with postgres (existing pattern) |
| API | upload → parse → get profile flow |
| E2E | manual: upload resumes at `/crm/profile`, verify matcher skills update |

---

## 9. Execution order

```bash
make migrate-career-os          # if not done
make migrate-resume-intelligence
make run                        # rebuild CRM + server
# Open http://localhost:8082/crm/profile
# Upload EN + ZH resumes → Parse & merge
```

Then Phase 2: wire Job Fit Engine to `candidate_profiles`.

---

## Product principles (non-negotiable)

1. **Candidate profile is source of truth** — no engine uses ad-hoc skill lists
2. **Homepage answers one question:** What should I do today?
3. **No raw job dumps** on primary surfaces
4. **Missed days are not punished** — continue next day
5. **Every feature must improve interview/response/offer rate**
