"use client";

import { useEffect, useState } from "react";
import { api, type Application } from "@/lib/api";
import { Card } from "@/components/ui/card";

const PIPELINE = [
  "saved",
  "applied",
  "interview",
  "technical",
  "final_round",
  "offer",
  "rejected",
] as const;

export default function ApplicationsPage() {
  const [apps, setApps] = useState<Application[]>([]);

  useEffect(() => {
    api.applications().then((r) => setApps(r.applications ?? [])).catch(() => setApps([]));
  }, []);

  const byStatus = (status: string) => apps.filter((a) => a.status === status);

  return (
    <div className="mx-auto max-w-5xl space-y-6">
      <h2 className="text-2xl font-semibold">Application pipeline</h2>
      <div className="grid gap-4 md:grid-cols-4 lg:grid-cols-7">
        {PIPELINE.map((status) => (
          <div key={status} className="space-y-2">
            <p className="text-xs font-medium uppercase tracking-wide text-muted">
              {status.replace("_", " ")}
            </p>
            {byStatus(status).map((a) => (
              <Card key={a.id} className="p-3">
                <p className="text-sm font-medium leading-tight">{a.role_title}</p>
                <p className="text-xs text-muted">{a.company_name}</p>
              </Card>
            ))}
            {!byStatus(status).length && (
              <p className="text-xs text-muted/50">—</p>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
