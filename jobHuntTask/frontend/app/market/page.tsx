"use client";

import { useEffect, useState } from "react";
import { api, type MarketSnapshot } from "@/lib/api";
import { Card, CardTitle } from "@/components/ui/card";
import { Loader2 } from "lucide-react";

export default function MarketPage() {
  const [market, setMarket] = useState<MarketSnapshot | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.market().then(setMarket).finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="flex items-center gap-2 text-muted">
        <Loader2 className="h-4 w-4 animate-spin" />
        Loading market intel…
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <header>
        <p className="text-sm text-muted">Career Intelligence</p>
        <h2 className="text-2xl font-semibold tracking-tight">Market demand</h2>
        <p className="mt-1 text-sm text-muted">
          Based on {market?.jobs_analyzed ?? 0} active jobs
        </p>
      </header>

      <Card>
        <CardTitle>Most demanded skills</CardTitle>
        <div className="mt-4 space-y-3">
          {(market?.skill_demand ?? []).slice(0, 10).map((s) => (
            <div key={s.skill} className="space-y-1">
              <div className="flex justify-between text-sm">
                <span>{s.skill}</span>
                <span className="text-muted">{s.demand_pct}%</span>
              </div>
              <div className="h-1.5 overflow-hidden rounded-full bg-border">
                <div
                  className="h-full rounded-full bg-foreground/70"
                  style={{ width: `${Math.min(100, s.demand_pct)}%` }}
                />
              </div>
            </div>
          ))}
        </div>
      </Card>

      {(market?.interview_topics?.length ?? 0) > 0 && (
        <Card>
          <CardTitle>Common interview topics</CardTitle>
          <ul className="mt-4 space-y-2">
            {market!.interview_topics.slice(0, 6).map((t) => (
              <li key={t.skill} className="flex justify-between text-sm">
                <span className="capitalize">{t.skill}</span>
                <span className="text-muted">{t.count} mentions</span>
              </li>
            ))}
          </ul>
        </Card>
      )}
    </div>
  );
}
