"use client";

import { useEffect, useState } from "react";
import { api, type OutreachData } from "@/lib/api";
import { Card } from "@/components/ui/card";

export default function OutreachPage() {
  const [data, setData] = useState<OutreachData | null>(null);

  useEffect(() => {
    api.outreach().then(setData).catch(() => setData(null));
  }, []);

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <h2 className="text-2xl font-semibold">Outreach</h2>
      <p className="text-sm text-muted">
        Personalized messages for recruiters and engineering managers.
      </p>
      <div className="space-y-4">
        {(data?.daily_targets ?? []).map((t, i) => (
          <Card key={i}>
            <p className="font-medium">{t.contact.full_name}</p>
            <p className="text-sm text-muted">
              {t.contact.title} · {t.contact.company_name}
            </p>
            {t.suggested_dm && (
              <pre className="mt-3 whitespace-pre-wrap rounded-lg bg-background p-3 text-sm leading-relaxed">
                {t.suggested_dm}
              </pre>
            )}
          </Card>
        ))}
      </div>
    </div>
  );
}
