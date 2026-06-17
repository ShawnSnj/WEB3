// Empty string = same origin (embedded at /crm on the Go server :8082).
const API = process.env.NEXT_PUBLIC_API_URL ?? "";

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
    cache: "no-store",
  });
  if (!res.ok) {
    const body = await res.text();
    throw new Error(body || res.statusText);
  }
  return res.json() as Promise<T>;
}

export type FitDecision = "apply" | "maybe" | "skip";

export type FitTier = "A" | "B" | "C";

export type JobMatch = {
  fit_score: number;
  final_score?: number;
  fit_tier?: FitTier;
  title_score?: number;
  skill_score?: number;
  backend_score?: number;
  infra_score?: number;
  remote_score?: number;
  salary_score?: number;
  seniority_score?: number;
  web3_score?: number;
  negative_score?: number;
  pros: string[];
  risks: string[];
  summary: string;
  company_score?: number;
  growth_score?: number;
  career_roi_score?: number;
  interview_probability?: number;
  domain_match_score?: number;
  decision?: FitDecision;
  why_this_matches_me?: string;
  missing_skills?: string[];
  missing_keywords?: string[];
  resume_keywords_to_add?: string[];
  suggested_resume_angle?: string;
  application_priority?: string;
  resume_version_recommendation?: string;
  user_action?: string;
};

export type Job = {
  id: string;
  external_id?: string;
  source?: string;
  title: string;
  company_name: string;
  location?: string;
  remote: boolean;
  web3: boolean;
  application_url: string;
  required_skills: string[];
  posted_at?: string;
  match?: JobMatch;
};

export type SourceFetchStats = {
  source: string;
  fetched: number;
  filtered?: number;
  error?: string;
};

export type FetchRunResult = {
  run_id: string;
  status: "running" | "ok" | "partial" | "error";
  jobs_fetched: number;
  jobs_inserted: number;
  jobs_updated: number;
  jobs_skipped: number;
  jobs_failed: number;
  jobs_scored: number;
  source_stats: SourceFetchStats[];
  error_message?: string;
  started_at: string;
  finished_at?: string;
  total_in_db: number;
  jobs_by_source: Record<string, number>;
};

export type FetchStatus = {
  last_fetch_at?: string;
  last_fetch_status?: "running" | "ok" | "partial" | "error";
  jobs_inserted_count: number;
  jobs_fetched_count: number;
  jobs_scored_count: number;
  fetch_error_message?: string;
  total_jobs_in_db: number;
  scored_jobs_in_db: number;
  apply_decisions: number;
  maybe_decisions: number;
  skip_decisions: number;
  jobs_by_source: Record<string, number>;
  source_stats?: SourceFetchStats[];
};

export type DailyBrief = {
  apply_job?: Job;
  apply_jobs?: Job[];
  apply_summary?: Record<string, unknown>;
  outreach_targets: {
    contact: { full_name: string; title: string; company_name: string; role_type: string };
    suggested_dm?: string;
  }[];
  learning_skill: string;
  learning_topic?: string;
  learning_reason: string;
  resume_improvement?: string;
  interview_topic?: string;
  interview_context?: string;
  estimated_minutes?: number;
  automation_tasks: {
    kind: string;
    description: string;
    target: number;
    done: number;
    completed: boolean;
  }[];
};

export type Application = {
  id: string;
  job_id?: string;
  company_name: string;
  role_title: string;
  status: string;
  applied_at?: string;
};

export type SkillGap = {
  skill: string;
  user_level: number;
  demand_pct: number;
  gap_score: number;
  roi: number;
  rank?: number;
};

export type SkillAnalysis = {
  top_demanded: { skill: string; count: number }[];
  missing_skills: { skill: string; count: number }[];
  learning_priority: { skill: string; count: number; rank: number }[];
  skill_gaps?: SkillGap[];
};

export type MarketSnapshot = {
  skill_demand: { skill: string; demand_pct: number; job_count: number }[];
  interview_topics: { skill: string; count: number }[];
  jobs_analyzed: number;
};

export type InterviewReadiness = {
  company_name: string;
  readiness_score: number;
  target_score: number;
  missing_topics: string[];
  study_topics: string[];
};

export type OfferPrediction = {
  interview_prob: number;
  offer_prob: number;
  bottleneck: string;
  recommendations: string[];
};

export type CandidateProfile = {
  id: string;
  full_name: string;
  location: string;
  email: string;
  years_of_experience: number;
  seniority_level: string;
  target_roles: string[];
  target_regions: string[];
  target_compensation_min_usd: number;
  strongest_skills: string[];
  medium_skills: string[];
  weak_skills: string[];
  backend_skills: string[];
  web3_skills: string[];
  infra_skills: string[];
  major_achievements: string[];
  domains: string[];
  last_parsed_at?: string;
};

export type ParsedResumeSummary = {
  full_name?: string;
  strongest_skills?: string[];
  medium_skills?: string[];
  major_achievements?: string[];
  domains?: string[];
};

export type CandidateProfileResponse = {
  profile: CandidateProfile;
  skills: { skill_name: string; level: number; strength: string; category: string }[];
  parse_summary?: { en?: ParsedResumeSummary; zh?: ParsedResumeSummary };
  resumes: { en?: { id: string; filename: string; language: string; parsed_at?: string }; zh?: { id: string; filename: string; language: string; parsed_at?: string } };
};

export type OutreachData = {
  contacts: { id: string; full_name: string; title: string; company_name: string }[];
  daily_targets: {
    contact: { full_name: string; title: string; company_name: string };
    suggested_dm?: string;
  }[];
};

export const api = {
  dashboard: () => request<DailyBrief>("/api/v1/crm/dashboard"),
  outreach: () => request<OutreachData>("/api/v1/crm/outreach"),
  runPipeline: () => request<{ status: string }>("/api/v1/crm/pipeline/run", { method: "POST" }),
  fitJobs: (filters: {
    tier?: FitTier | "AB";
    limit?: number;
    hide_frontend?: boolean;
    hide_junior?: boolean;
    hide_non_remote?: boolean;
    hide_marketing?: boolean;
    q?: string;
  } = {}) => {
    const params = new URLSearchParams({ limit: String(filters.limit ?? 30) });
    if (filters.tier) params.set("tier", filters.tier);
    if (filters.hide_frontend === false) params.set("hide_frontend", "false");
    if (filters.hide_junior === false) params.set("hide_junior", "false");
    if (filters.hide_non_remote === false) params.set("hide_non_remote", "false");
    if (filters.hide_marketing === false) params.set("hide_marketing", "false");
    if (filters.q) params.set("q", filters.q);
    return request<{ jobs: Job[] }>(`/api/v1/crm/jobs?${params.toString()}`);
  },
  jobAction: (id: string, action: "apply" | "skip" | "save") =>
    request<{ status: string }>(`/api/v1/crm/jobs/${id}/action`, {
      method: "POST",
      body: JSON.stringify({ action }),
    }),
  rescoreJobs: (limit = 300) =>
    request<{ rescored: number }>(`/api/v1/crm/jobs/rescore?limit=${limit}`, { method: "POST" }),
  fetchJobs: () =>
    request<FetchRunResult>("/api/v1/crm/jobs/fetch", { method: "POST" }),
  fetchStatus: () => request<FetchStatus>("/api/v1/crm/jobs/fetch/status"),
  jobFit: (id: string) => request<Job>(`/api/v1/crm/jobs/${id}/fit`),
  recommendedJobs: (limit = 5) =>
    request<{ jobs: Job[] }>(`/api/v1/crm/jobs/recommended?limit=${limit}`),
  apply: (id: string) =>
    request<Application>(`/api/v1/crm/jobs/${id}/apply`, { method: "POST" }),
  applications: () => request<{ applications: Application[] }>("/api/v1/crm/applications"),
  skills: () => request<SkillAnalysis>("/api/v1/crm/skills"),
  market: () => request<MarketSnapshot>("/api/v1/crm/market/trends"),
  interviewReadiness: () =>
    request<{ companies: InterviewReadiness[] }>("/api/v1/crm/interview/readiness").then(
      (r) => r.companies
    ),
  offerPrediction: () => request<OfferPrediction>("/api/v1/crm/offers/predictions"),
  weekly: () => request<Record<string, unknown>>("/api/v1/crm/weekly"),
  coach: () => request<Record<string, unknown>>("/api/v1/crm/coach"),
  completeOutreach: () => request("/api/v1/crm/outreach/complete", { method: "POST" }),
  candidateProfile: () => request<CandidateProfileResponse>("/api/v1/crm/candidate-profile"),
  updateCandidateProfile: (profile: Partial<CandidateProfile>) =>
    request<CandidateProfileResponse>("/api/v1/crm/candidate-profile", {
      method: "PUT",
      body: JSON.stringify(profile),
    }),
  uploadResume: (language: "en" | "zh", text: string, filename = "") =>
    request<{ id: string; language: string; filename: string }>("/api/v1/crm/resumes/upload", {
      method: "POST",
      body: JSON.stringify({ language, text, filename }),
    }),
  uploadResumeFile: (language: "en" | "zh", file: File) => {
    const fd = new FormData();
    fd.append("language", language);
    fd.append("file", file);
    return fetch(`${API}/api/v1/crm/resumes/upload`, {
      method: "POST",
      body: fd,
    }).then(async (res) => {
      if (!res.ok) {
        const body = await res.text();
        let msg = body;
        try {
          const j = JSON.parse(body) as { error?: string };
          if (j.error) msg = j.error;
        } catch {
          /* use raw body */
        }
        throw new Error(msg || res.statusText);
      }
      return res.json() as Promise<{ id: string; language: string; filename: string }>;
    });
  },
  parseResumes: () =>
    request<CandidateProfileResponse>("/api/v1/crm/resumes/parse", { method: "POST" }),
};
