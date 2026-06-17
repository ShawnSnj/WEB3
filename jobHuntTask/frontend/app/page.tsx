"use client";

import { useCallback, useEffect, useState } from "react";
import { api, type DailyBrief, type Job } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardTitle } from "@/components/ui/card";
import { CheckCircle2, Clock, Loader2, RefreshCw, Target } from "lucide-react";

export default function TodayPage() {
  const [brief, setBrief] = useState<DailyBrief | null>(null);
  const [appliedIds, setAppliedIds] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(true);
  const [running, setRunning] = useState(false);
  const [applyingId, setApplyingId] = useState<string | null>(null);
  const [error, setError] = useState("");

  const loadApplied = useCallback(async () => {
    try {
      const { applications } = await api.applications();
      setAppliedIds(
        new Set(applications.map((a) => a.job_id).filter(Boolean) as string[])
      );
    } catch {
      setAppliedIds(new Set());
    }
  }, []);

  const load = async () => {
    setLoading(true);
    setError("");
    try {
      const dashboard = await api.dashboard();
      await loadApplied();
      setBrief(dashboard);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const runPipeline = async () => {
    setRunning(true);
    try {
      await api.rescoreJobs();
      await api.runPipeline();
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Pipeline failed");
    } finally {
      setRunning(false);
    }
  };

  const applyToJob = async (job: Job) => {
    setApplyingId(job.id);
    try {
      await api.jobAction(job.id, "apply");
      setAppliedIds((prev) => new Set(prev).add(job.id));
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to apply");
    } finally {
      setApplyingId(null);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center gap-2 text-muted">
        <Loader2 className="h-4 w-4 animate-spin" />
        Loading your mission…
      </div>
    );
  }

  const applyJobs =
    brief?.apply_jobs?.length
      ? brief.apply_jobs
      : brief?.apply_job
        ? [brief.apply_job]
        : [];
  const minutes = brief?.estimated_minutes ?? 30;

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <header className="flex items-start justify-between gap-4">
        <div>
          <p className="text-sm text-muted">Daily Mission</p>
          <h2 className="text-2xl font-semibold tracking-tight">What should I do today?</h2>
          <p className="mt-1 flex items-center gap-1.5 text-sm text-muted">
            <Clock className="h-3.5 w-3.5" />
            {minutes} minutes · precision over volume
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={runPipeline} disabled={running}>
          {running ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
          Sync
        </Button>
      </header>

      {error && (
        <p className="rounded-lg border border-danger/30 bg-danger/10 px-4 py-3 text-sm text-danger">
          {error}
        </p>
      )}

      <Card className="border-success/20 bg-success/5">
        <CardTitle className="flex items-center gap-2">
          <Target className="h-4 w-4 text-success" />
          Apply — top {Math.min(applyJobs.length, 3)} A/B matches
        </CardTitle>
        {applyJobs.length > 0 ? (
          <div className="mt-4 space-y-3">
            {applyJobs.slice(0, 3).map((job, i) => {
              const isApplied = appliedIds.has(job.id);
              const m = job.match;
              return (
                <div key={job.id} className="rounded-lg border border-border bg-background/50 p-4">
                  <div className="flex items-start justify-between gap-4">
                    <div>
                      <p className="text-xs text-muted">
                        #{i + 1} · {m?.fit_tier ?? "?"}-level
                      </p>
                      <p className="font-medium">{job.title}</p>
                      <p className="text-sm text-muted">{job.company_name}</p>
                    </div>
                    {m && (
                      <span className="rounded-full bg-success/15 px-3 py-1 text-sm font-medium text-success">
                        {m.final_score ?? m.fit_score}/100
                      </span>
                    )}
                  </div>
                  {m?.why_this_matches_me && (
                    <p className="mt-2 text-sm text-muted">{m.why_this_matches_me}</p>
                  )}
                  <div className="mt-3 flex gap-2">
                    {isApplied ? (
                      <span className="inline-flex items-center gap-1 text-sm text-success">
                        <CheckCircle2 className="h-4 w-4" /> Applied
                      </span>
                    ) : (
                      <Button size="sm" onClick={() => applyToJob(job)} disabled={applyingId === job.id}>
                        {applyingId === job.id ? <Loader2 className="h-4 w-4 animate-spin" /> : "Apply"}
                      </Button>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        ) : (
          <p className="mt-4 text-sm text-muted">
            No A/B matches yet. Upload resume at Profile, then Sync to rescore jobs.
          </p>
        )}
      </Card>

      <Card>
        <CardTitle>Outreach — top 2 people</CardTitle>
        <div className="mt-4 space-y-4">
          {(brief?.outreach_targets ?? []).slice(0, 2).map((t, i) => (
            <div key={i} className="rounded-lg border border-border bg-background/50 p-4">
              <p className="font-medium">{t.contact.full_name}</p>
              <p className="text-sm text-muted">
                {t.contact.title} · {t.contact.company_name}
              </p>
              {t.suggested_dm && <p className="mt-3 text-sm leading-relaxed">{t.suggested_dm}</p>}
            </div>
          ))}
        </div>
      </Card>

      <div className="grid gap-4 sm:grid-cols-2">
        <Card>
          <CardTitle>Resume — today&apos;s improvement</CardTitle>
          <p className="mt-4 text-sm">{brief?.resume_improvement || "—"}</p>
        </Card>
        <Card>
          <CardTitle>Skill gap — study task</CardTitle>
          <p className="mt-4 text-lg font-medium">{brief?.learning_skill || "—"}</p>
          <p className="mt-2 text-sm text-muted">{brief?.learning_reason}</p>
        </Card>
      </div>
    </div>
  );
}
