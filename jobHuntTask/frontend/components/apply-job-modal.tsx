"use client";

import { useEffect } from "react";
import { type Job } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { ExternalLink, Loader2, X } from "lucide-react";

type ApplyJobModalProps = {
  job: Job | null;
  open: boolean;
  applying: boolean;
  applied: boolean;
  error: string;
  onClose: () => void;
  onSubmit: () => void;
};

export function ApplyJobModal({
  job,
  open,
  applying,
  applied,
  error,
  onClose,
  onSubmit,
}: ApplyJobModalProps) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open || !job) return null;

  const fit = job.match?.fit_score;

  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center p-4 sm:items-center">
      <button
        type="button"
        aria-label="Close"
        className="absolute inset-0 bg-black/60"
        onClick={onClose}
      />
      <div className="relative z-10 w-full max-w-lg rounded-xl border border-border bg-card p-6 shadow-xl">
        <div className="flex items-start justify-between gap-4">
          <div>
            <p className="text-xs font-medium uppercase tracking-wider text-muted">
              Submit application
            </p>
            <h3 className="mt-1 text-xl font-semibold">{job.title}</h3>
            <p className="text-sm text-muted">{job.company_name}</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-md p-1 text-muted hover:bg-white/5 hover:text-foreground"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {fit != null && (
          <div className="mt-4 inline-flex rounded-full bg-success/15 px-3 py-1 text-sm font-medium text-success">
            Fit score {fit}
          </div>
        )}

        {job.match?.pros?.length ? (
          <ul className="mt-4 space-y-1 text-sm text-muted">
            {job.match.pros.map((p) => (
              <li key={p}>+ {p}</li>
            ))}
          </ul>
        ) : null}

        {job.match?.risks?.length ? (
          <p className="mt-3 text-sm text-muted">
            Risks: {job.match.risks.join(", ")}
          </p>
        ) : null}

        {error && (
          <p className="mt-4 rounded-lg border border-danger/30 bg-danger/10 px-3 py-2 text-sm text-danger">
            {error}
          </p>
        )}

        {applied ? (
          <div className="mt-6 space-y-3">
            <p className="rounded-lg border border-success/30 bg-success/10 px-4 py-3 text-sm text-success">
              Application recorded. Track it on the Pipeline page.
            </p>
            <div className="flex gap-2">
              <a
                href="/crm/applications/"
                className="inline-flex h-9 flex-1 items-center justify-center rounded-lg border border-border text-sm hover:bg-white/5"
              >
                View pipeline
              </a>
              <Button variant="outline" onClick={onClose}>
                Close
              </Button>
            </div>
          </div>
        ) : (
          <div className="mt-6 flex flex-col gap-2 sm:flex-row">
            {job.application_url ? (
              <a
                href={job.application_url}
                target="_blank"
                rel="noreferrer"
                className="inline-flex h-9 flex-1 items-center justify-center gap-2 rounded-lg border border-border text-sm hover:bg-white/5"
              >
                <ExternalLink className="h-4 w-4" />
                Open job posting
              </a>
            ) : null}
            <Button className="flex-1" onClick={onSubmit} disabled={applying}>
              {applying ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Submitting…
                </>
              ) : (
                "Submit application"
              )}
            </Button>
          </div>
        )}
      </div>
    </div>
  );
}
