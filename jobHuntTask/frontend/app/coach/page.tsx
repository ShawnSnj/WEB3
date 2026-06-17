"use client";

import { useEffect, useState } from "react";
import { api, type OfferPrediction } from "@/lib/api";
import { Card, CardTitle } from "@/components/ui/card";

export default function CoachPage() {
  const [weekly, setWeekly] = useState<Record<string, unknown> | null>(null);
  const [coach, setCoach] = useState<Record<string, unknown> | null>(null);
  const [offer, setOffer] = useState<OfferPrediction | null>(null);

  useEffect(() => {
    api.weekly().then(setWeekly).catch(() => setWeekly(null));
    api.coach().then(setCoach).catch(() => setCoach(null));
    api.offerPrediction().then(setOffer).catch(() => setOffer(null));
  }, []);

  const recs = [
    ...(offer?.recommendations ?? []),
    ...((coach?.recommendations as string[]) ?? []),
  ].filter((r, i, a) => a.indexOf(r) === i);

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <header>
        <p className="text-sm text-muted">Weekly Review</p>
        <h2 className="text-2xl font-semibold">Career coach</h2>
      </header>

      {offer && (
        <Card>
          <CardTitle>Offer prediction</CardTitle>
          <dl className="mt-4 grid grid-cols-2 gap-4 text-sm">
            <div>
              <dt className="text-muted">Interview probability</dt>
              <dd className="text-xl font-semibold">{offer.interview_prob}%</dd>
            </div>
            <div>
              <dt className="text-muted">Offer probability</dt>
              <dd className="text-xl font-semibold">{offer.offer_prob}%</dd>
            </div>
          </dl>
          {offer.bottleneck && (
            <p className="mt-4 rounded-lg border border-border bg-background/50 px-4 py-3 text-sm">
              <span className="font-medium">Bottleneck: </span>
              {offer.bottleneck}
            </p>
          )}
        </Card>
      )}

      <Card>
        <CardTitle>This week</CardTitle>
        <dl className="mt-4 grid grid-cols-2 gap-4 text-sm">
          <div>
            <dt className="text-muted">Jobs found</dt>
            <dd className="text-xl font-semibold">{String(weekly?.jobs_found ?? "—")}</dd>
          </div>
          <div>
            <dt className="text-muted">Applications sent</dt>
            <dd className="text-xl font-semibold">{String(weekly?.applications_sent ?? "—")}</dd>
          </div>
          <div>
            <dt className="text-muted">Response rate</dt>
            <dd className="text-xl font-semibold">
              {weekly?.response_rate != null ? `${weekly.response_rate}%` : "—"}
            </dd>
          </div>
          <div>
            <dt className="text-muted">Interview rate</dt>
            <dd className="text-xl font-semibold">
              {weekly?.interview_rate != null ? `${weekly.interview_rate}%` : "—"}
            </dd>
          </div>
        </dl>
        {weekly?.coach_summary ? (
          <p className="mt-4 text-sm leading-relaxed text-foreground/80">
            {String(weekly.coach_summary)}
          </p>
        ) : null}
      </Card>

      <Card>
        <CardTitle>What to do next</CardTitle>
        <ul className="mt-4 space-y-2 text-sm">
          {recs.map((r) => (
            <li key={r} className="text-foreground/80">
              → {r}
            </li>
          ))}
        </ul>
      </Card>
    </div>
  );
}
