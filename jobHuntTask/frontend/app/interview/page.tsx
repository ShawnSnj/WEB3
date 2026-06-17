"use client";

import { useEffect, useState } from "react";
import { api, type InterviewReadiness } from "@/lib/api";
import { Card, CardTitle } from "@/components/ui/card";
import { Loader2 } from "lucide-react";

export default function InterviewPage() {
  const [companies, setCompanies] = useState<InterviewReadiness[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.interviewReadiness().then(setCompanies).finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="flex items-center gap-2 text-muted">
        <Loader2 className="h-4 w-4 animate-spin" />
        Loading readiness…
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <header>
        <p className="text-sm text-muted">Interview Readiness</p>
        <h2 className="text-2xl font-semibold tracking-tight">Target companies</h2>
        <p className="mt-1 text-sm text-muted">
          Focus study on companies with lowest readiness
        </p>
      </header>

      <div className="space-y-4">
        {companies.map((c) => (
          <Card key={c.company_name}>
            <div className="flex items-start justify-between gap-4">
              <CardTitle>{c.company_name}</CardTitle>
              <span
                className={`rounded-full px-3 py-1 text-sm font-medium ${
                  c.readiness_score >= c.target_score
                    ? "bg-success/15 text-success"
                    : "bg-warning/15 text-warning"
                }`}
              >
                {c.readiness_score}/{c.target_score}
              </span>
            </div>
            {c.study_topics?.length > 0 && (
              <div className="mt-4">
                <p className="text-xs font-medium uppercase tracking-wider text-muted">
                  Study next
                </p>
                <ul className="mt-2 space-y-1 text-sm">
                  {c.study_topics.map((t) => (
                    <li key={t}>→ {t}</li>
                  ))}
                </ul>
              </div>
            )}
            {c.missing_topics?.length > 0 && (
              <div className="mt-3">
                <p className="text-xs text-muted">Gaps: {c.missing_topics.join(", ")}</p>
              </div>
            )}
          </Card>
        ))}
      </div>
    </div>
  );
}
