"use client";

import { useEffect, useState } from "react";
import { api, type SkillAnalysis } from "@/lib/api";
import { Card, CardTitle } from "@/components/ui/card";

export default function SkillsPage() {
  const [analysis, setAnalysis] = useState<SkillAnalysis | null>(null);

  useEffect(() => {
    api.skills().then(setAnalysis).catch(() => setAnalysis(null));
  }, []);

  const gaps = analysis?.skill_gaps ?? [];

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <header>
        <p className="text-sm text-muted">Skill Gap Engine</p>
        <h2 className="text-2xl font-semibold">What to learn next</h2>
        <p className="mt-1 text-sm text-muted">
          Ranked by ROI — demand × level gap
        </p>
      </header>

      {gaps.length > 0 ? (
        <Card>
          <CardTitle>Top 3 priorities</CardTitle>
          <div className="mt-4 space-y-4">
            {gaps.slice(0, 3).map((g) => (
              <div key={g.skill} className="rounded-lg border border-border p-4">
                <div className="flex items-start justify-between">
                  <div>
                    <p className="font-medium">{g.skill}</p>
                    <p className="text-sm text-muted">
                      Your level: {g.user_level}/10 · Market: {g.demand_pct}%
                    </p>
                  </div>
                  <span className="rounded-full bg-warning/15 px-3 py-1 text-sm font-medium text-warning">
                    Gap {g.gap_score}
                  </span>
                </div>
                <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-border">
                  <div
                    className="h-full rounded-full bg-warning/70"
                    style={{ width: `${Math.min(100, g.gap_score)}%` }}
                  />
                </div>
              </div>
            ))}
          </div>
        </Card>
      ) : null}

      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardTitle>Market demand</CardTitle>
          <ol className="mt-4 space-y-2">
            {(analysis?.top_demanded ?? []).slice(0, 8).map((s, i) => (
              <li key={s.skill} className="flex justify-between text-sm">
                <span>
                  {i + 1}. {s.skill}
                </span>
                <span className="text-muted">{s.count} jobs</span>
              </li>
            ))}
          </ol>
        </Card>
        <Card>
          <CardTitle>All gaps</CardTitle>
          <ol className="mt-4 space-y-2">
            {gaps.slice(0, 8).map((g) => (
              <li key={g.skill} className="flex justify-between text-sm">
                <span>
                  {g.rank}. {g.skill}
                </span>
                <span className="text-muted">ROI {g.roi}</span>
              </li>
            ))}
          </ol>
        </Card>
      </div>
    </div>
  );
}
