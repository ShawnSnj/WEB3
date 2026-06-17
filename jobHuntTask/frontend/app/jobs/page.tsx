"use client";

import { useCallback, useEffect, useState } from "react";
import { api, type FitTier, type Job } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { CheckCircle2, Loader2, RefreshCw, XCircle, Bookmark } from "lucide-react";

const FILTERS = {
  hide_frontend: true,
  hide_junior: true,
  hide_non_remote: true,
  hide_marketing: true,
};

export default function JobsPage() {
  const [tier, setTier] = useState<FitTier | "AB">("AB");
  const [jobs, setJobs] = useState<Job[]>([]);
  const [loading, setLoading] = useState(true);
  const [rescoring, setRescoring] = useState(false);
  const [actingId, setActingId] = useState<string | null>(null);
  const [message, setMessage] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const r = await api.fitJobs({ tier, ...FILTERS, limit: 40 });
      setJobs(r.jobs ?? []);
    } catch {
      setJobs([]);
    } finally {
      setLoading(false);
    }
  }, [tier]);

  useEffect(() => {
    load();
  }, [load]);

  const rescore = async () => {
    setRescoring(true);
    setMessage("");
    try {
      const r = await api.rescoreJobs();
      setMessage(`Rescored ${r.rescored} jobs against your master resume.`);
      await load();
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "Rescore failed");
    } finally {
      setRescoring(false);
    }
  };

  const act = async (jobId: string, action: "apply" | "skip" | "save") => {
    setActingId(jobId);
    try {
      await api.jobAction(jobId, action);
      setJobs((prev) => prev.filter((j) => j.id !== jobId));
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "Action failed");
    } finally {
      setActingId(null);
    }
  };

  const aCount = jobs.filter((j) => j.match?.fit_tier === "A").length;
  const bCount = jobs.filter((j) => j.match?.fit_tier === "B").length;

  return (
    <div className="mx-auto max-w-4xl space-y-6">
      <header className="flex items-start justify-between gap-4">
        <div>
          <p className="text-sm text-muted">Personalized Job Fit Engine</p>
          <h2 className="text-2xl font-semibold">Your matches</h2>
          <p className="mt-1 text-sm text-muted">
            Ranked from your master resume — Go backend, Web3 infra, Kafka/PostgreSQL. No raw job dumps.
          </p>
        </div>
        <Button size="sm" variant="outline" onClick={rescore} disabled={rescoring}>
          {rescoring ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
          Rescore
        </Button>
      </header>

      {message && (
        <p className="rounded-lg border border-border bg-card px-4 py-3 text-sm">{message}</p>
      )}

      <div className="flex gap-1 rounded-lg border border-border bg-card p-1">
        {(["AB", "A", "B"] as const).map((t) => (
          <button
            key={t}
            type="button"
            onClick={() => setTier(t)}
            className={`flex-1 rounded-md px-4 py-2 text-sm font-medium ${
              tier === t ? "bg-white/10 text-foreground" : "text-muted hover:bg-white/5"
            }`}
          >
            {t === "AB" ? `All matches (${aCount + bCount})` : t === "A" ? `A — Apply now (${aCount})` : `B — Consider (${bCount})`}
          </button>
        ))}
      </div>

      <Card className="text-xs text-muted">
        Active filters: hide frontend-heavy · hide junior · remote/async only · hide marketing/content ·
        prefer Go, Kafka, PostgreSQL, Solidity, DeFi, indexing
      </Card>

      {loading ? (
        <div className="flex items-center gap-2 text-muted">
          <Loader2 className="h-4 w-4 animate-spin" /> Loading personalized matches…
        </div>
      ) : (
        <div className="space-y-4">
          {jobs.map((job, i) => {
            const m = job.match;
            const isA = m?.fit_tier === "A";
            return (
              <Card key={job.id} className={`space-y-3 ${isA ? "border-success/30" : ""}`}>
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-muted">#{i + 1}</span>
                      <span
                        className={`rounded-full px-2 py-0.5 text-xs font-semibold ${
                          isA ? "bg-success/20 text-success" : "bg-amber-500/15 text-amber-400"
                        }`}
                      >
                        {m?.fit_tier}-level · {m?.final_score ?? m?.fit_score}/100
                      </span>
                      <span className="text-xs text-muted capitalize">{m?.application_priority} priority</span>
                    </div>
                    <p className="mt-1 text-lg font-medium">{job.title}</p>
                    <p className="text-sm text-muted">
                      {job.company_name}
                      {job.remote && " · Remote"}
                    </p>
                  </div>
                </div>

                {m?.why_this_matches_me && (
                  <div>
                    <p className="text-xs font-medium uppercase text-muted">Why this matches you</p>
                    <p className="mt-1 text-sm">{m.why_this_matches_me}</p>
                  </div>
                )}

                {m?.missing_skills?.length ? (
                  <div>
                    <p className="text-xs font-medium uppercase text-muted">Missing skills</p>
                    <p className="mt-1 text-sm text-amber-400/90">{m.missing_skills.join(", ")}</p>
                  </div>
                ) : null}

                {m?.suggested_resume_angle && (
                  <p className="text-xs text-muted">Resume angle: {m.suggested_resume_angle}</p>
                )}

                <div className="flex flex-wrap gap-2 pt-1">
                  <Button
                    size="sm"
                    onClick={() => act(job.id, "apply")}
                    disabled={actingId === job.id}
                  >
                    {actingId === job.id ? <Loader2 className="h-4 w-4 animate-spin" /> : <CheckCircle2 className="h-4 w-4" />}
                    Apply
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => act(job.id, "save")}
                    disabled={actingId === job.id}
                  >
                    <Bookmark className="h-4 w-4" /> Save
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => act(job.id, "skip")}
                    disabled={actingId === job.id}
                  >
                    <XCircle className="h-4 w-4" /> Skip
                  </Button>
                  {job.application_url && (
                    <a
                      href={job.application_url}
                      target="_blank"
                      rel="noreferrer"
                      className="inline-flex h-8 items-center rounded-lg border border-border px-3 text-xs hover:bg-white/5"
                    >
                      Open posting
                    </a>
                  )}
                </div>
              </Card>
            );
          })}
          {!jobs.length && (
            <p className="text-muted">
              No A/B matches yet. Upload your resume at Profile, then click Rescore to rank jobs against
              your master profile.
            </p>
          )}
        </div>
      )}
    </div>
  );
}
