"use client";

import { useCallback, useEffect, useState } from "react";
import { api, type CandidateProfile, type CandidateProfileResponse } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardTitle } from "@/components/ui/card";

function TagList({ items }: { items: string[] }) {
  if (!items?.length) return <span className="text-sm text-muted">—</span>;
  return (
    <div className="flex flex-wrap gap-1.5">
      {items.map((s) => (
        <span key={s} className="rounded-md bg-white/5 px-2 py-0.5 text-xs">
          {s}
        </span>
      ))}
    </div>
  );
}

export default function ProfilePage() {
  const [data, setData] = useState<CandidateProfileResponse | null>(null);
  const [enText, setEnText] = useState("");
  const [zhText, setZhText] = useState("");
  const [edit, setEdit] = useState<Partial<CandidateProfile>>({});
  const [busy, setBusy] = useState("");
  const [msg, setMsg] = useState("");

  const load = useCallback(() => {
    api.candidateProfile().then(setData).catch(() => setData(null));
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    if (data?.profile) setEdit(data.profile);
  }, [data]);

  async function uploadFile(lang: "en" | "zh", file: File | null | undefined) {
    if (!file) {
      setMsg("Choose a file first (.doc, .docx, .pdf, or .txt)");
      return;
    }
    setBusy(`upload-${lang}`);
    setMsg("");
    try {
      await api.uploadResumeFile(lang, file);
      setMsg(`${file.name} uploaded (${lang.toUpperCase()})`);
      load();
    } catch (e) {
      setMsg(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy("");
    }
  }

  async function upload(lang: "en" | "zh") {
    const text = lang === "en" ? enText : zhText;
    if (!text.trim()) {
      setMsg(`Paste ${lang.toUpperCase()} resume text first`);
      return;
    }
    setBusy(`upload-${lang}`);
    setMsg("");
    try {
      await api.uploadResume(lang, text, `${lang}-resume.txt`);
      setMsg(`${lang.toUpperCase()} resume uploaded`);
      if (lang === "en") setEnText("");
      else setZhText("");
      load();
    } catch (e) {
      setMsg(String(e));
    } finally {
      setBusy("");
    }
  }

  async function parse() {
    setBusy("parse");
    setMsg("");
    try {
      const out = await api.parseResumes();
      setData(out);
      setEdit(out.profile);
      setMsg("Profile parsed and merged");
    } catch (e) {
      setMsg(String(e));
    } finally {
      setBusy("");
    }
  }

  async function save() {
    setBusy("save");
    setMsg("");
    try {
      const out = await api.updateCandidateProfile(edit);
      setData(out);
      setMsg("Profile saved");
    } catch (e) {
      setMsg(String(e));
    } finally {
      setBusy("");
    }
  }

  const p = data?.profile;

  return (
    <div className="mx-auto max-w-4xl space-y-6">
      <header>
        <p className="text-sm text-muted">Resume Intelligence Engine</p>
        <h2 className="text-2xl font-semibold">Candidate Master Profile</h2>
        <p className="mt-1 text-sm text-muted">
          Upload .doc, .docx, .pdf, or .txt — or paste text. English + Chinese merge into one profile.
        </p>
      </header>

      {msg ? (
        <p className="rounded-lg border border-border bg-white/5 px-4 py-2 text-sm">{msg}</p>
      ) : null}

      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardTitle>English resume</CardTitle>
          <textarea
            className="mt-3 min-h-[140px] w-full rounded-lg border border-border bg-transparent p-3 text-sm"
            placeholder="Paste English master resume…"
            value={enText}
            onChange={(e) => setEnText(e.target.value)}
          />
          <Button className="mt-2" disabled={!!busy} onClick={() => upload("en")}>
            Upload pasted text
          </Button>
          <label className="mt-3 flex cursor-pointer flex-col gap-1 text-sm">
            <span className="text-muted">Or choose file</span>
            <input
              type="file"
              accept=".doc,.docx,.pdf,.txt,.md,application/msword,application/vnd.openxmlformats-officedocument.wordprocessingml.document,application/pdf,text/plain"
              className="text-xs file:mr-2 file:rounded-md file:border-0 file:bg-white/10 file:px-3 file:py-1.5 file:text-sm"
              disabled={!!busy}
              onChange={(e) => {
                void uploadFile("en", e.target.files?.[0]);
                e.target.value = "";
              }}
            />
          </label>
          {data?.resumes?.en ? (
            <p className="mt-2 text-xs text-muted">
              On file: {data.resumes.en.filename || "en-resume"}
              {data.resumes.en.parsed_at ? " · parsed" : ""}
            </p>
          ) : null}
        </Card>
        <Card>
          <CardTitle>Chinese resume</CardTitle>
          <textarea
            className="mt-3 min-h-[140px] w-full rounded-lg border border-border bg-transparent p-3 text-sm"
            placeholder="Paste Chinese master resume…"
            value={zhText}
            onChange={(e) => setZhText(e.target.value)}
          />
          <Button className="mt-2" disabled={!!busy} onClick={() => upload("zh")}>
            Upload pasted text
          </Button>
          <label className="mt-3 flex cursor-pointer flex-col gap-1 text-sm">
            <span className="text-muted">Or choose file</span>
            <input
              type="file"
              accept=".doc,.docx,.pdf,.txt,.md,application/msword,application/vnd.openxmlformats-officedocument.wordprocessingml.document,application/pdf,text/plain"
              className="text-xs file:mr-2 file:rounded-md file:border-0 file:bg-white/10 file:px-3 file:py-1.5 file:text-sm"
              disabled={!!busy}
              onChange={(e) => {
                void uploadFile("zh", e.target.files?.[0]);
                e.target.value = "";
              }}
            />
          </label>
          {data?.resumes?.zh ? (
            <p className="mt-2 text-xs text-muted">
              On file: {data.resumes.zh.filename || "zh-resume"}
              {data.resumes.zh.parsed_at ? " · parsed" : ""}
            </p>
          ) : null}
        </Card>
      </div>

      {data?.parse_summary ? (
        <Card>
          <CardTitle>Merged from both resumes</CardTitle>
          <p className="mt-1 text-sm text-muted">
            English and Chinese are parsed separately, then combined. The edit form below is the merged profile.
          </p>
          <div className="mt-4 grid gap-4 md:grid-cols-2">
            <div className="rounded-lg border border-border p-3">
              <p className="text-sm font-medium">From English</p>
              {data.parse_summary.en ? (
                <>
                  <p className="mt-1 text-xs text-muted">{data.parse_summary.en.full_name || "—"}</p>
                  <p className="mt-2 text-xs font-medium text-muted">Skills</p>
                  <TagList items={data.parse_summary.en.strongest_skills ?? []} />
                </>
              ) : (
                <p className="mt-2 text-xs text-muted">No English resume parsed yet</p>
              )}
            </div>
            <div className="rounded-lg border border-border p-3">
              <p className="text-sm font-medium">From Chinese</p>
              {data.parse_summary.zh ? (
                <>
                  <p className="mt-1 text-xs text-muted">{data.parse_summary.zh.full_name || "—"}</p>
                  <p className="mt-2 text-xs font-medium text-muted">Skills</p>
                  <TagList items={data.parse_summary.zh.strongest_skills ?? []} />
                </>
              ) : (
                <p className="mt-2 text-xs text-muted">No Chinese resume parsed yet</p>
              )}
            </div>
          </div>
        </Card>
      ) : null}

      <div className="flex gap-2">
        <Button disabled={!!busy} onClick={parse}>
          {busy === "parse" ? "Parsing…" : "Parse & merge resumes"}
        </Button>
        {p?.last_parsed_at ? (
          <span className="self-center text-xs text-muted">
            Last parsed: {new Date(p.last_parsed_at).toLocaleString()}
          </span>
        ) : null}
      </div>

      {p ? (
        <Card>
          <CardTitle>Edit profile</CardTitle>
          <div className="mt-4 grid gap-4 md:grid-cols-2">
            <label className="block text-sm">
              <span className="text-muted">Full name</span>
              <input
                className="mt-1 w-full rounded-lg border border-border bg-transparent px-3 py-2"
                value={edit.full_name ?? ""}
                onChange={(e) => setEdit({ ...edit, full_name: e.target.value })}
              />
            </label>
            <label className="block text-sm">
              <span className="text-muted">Email</span>
              <input
                className="mt-1 w-full rounded-lg border border-border bg-transparent px-3 py-2"
                value={edit.email ?? ""}
                onChange={(e) => setEdit({ ...edit, email: e.target.value })}
              />
            </label>
            <label className="block text-sm">
              <span className="text-muted">Years of experience</span>
              <input
                type="number"
                className="mt-1 w-full rounded-lg border border-border bg-transparent px-3 py-2"
                value={edit.years_of_experience ?? 0}
                onChange={(e) => setEdit({ ...edit, years_of_experience: Number(e.target.value) })}
              />
            </label>
            <label className="block text-sm">
              <span className="text-muted">Min compensation (USD)</span>
              <input
                type="number"
                className="mt-1 w-full rounded-lg border border-border bg-transparent px-3 py-2"
                value={edit.target_compensation_min_usd ?? 0}
                onChange={(e) =>
                  setEdit({ ...edit, target_compensation_min_usd: Number(e.target.value) })
                }
              />
            </label>
          </div>

          <div className="mt-6 space-y-4">
            <div>
              <p className="mb-2 text-sm font-medium">Strongest skills</p>
              <TagList items={p.strongest_skills} />
            </div>
            <div>
              <p className="mb-2 text-sm font-medium">Medium skills</p>
              <TagList items={p.medium_skills} />
            </div>
            <div>
              <p className="mb-2 text-sm font-medium">Weak / developing</p>
              <TagList items={p.weak_skills} />
            </div>
            <div>
              <p className="mb-2 text-sm font-medium">Domains</p>
              <TagList items={p.domains} />
            </div>
            <div>
              <p className="mb-2 text-sm font-medium">Major achievements</p>
              <ul className="list-inside list-disc space-y-1 text-sm text-muted">
                {(p.major_achievements ?? []).slice(0, 6).map((a) => (
                  <li key={a}>{a}</li>
                ))}
              </ul>
            </div>
          </div>

          <Button className="mt-6" disabled={!!busy} onClick={save}>
            Save manual edits
          </Button>
        </Card>
      ) : null}
    </div>
  );
}
