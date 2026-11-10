// Growth intelligence console helpers (D-73). Staff only; nothing here reaches members.
import { ops } from "@/lib/safety";

export type Source = { source_id: string; family: string; access_path: string; kind: string; context_only: boolean; observations: number; last_window_end?: string; age_hours?: number; stale?: boolean };
export type PageRow = { asset_id: string; slug: string; created: boolean; active_revision?: number; fields?: Record<string, string> };
export type BriefRow = { id: string; status: string; topic_id: string; brief_id: string; brief_version: number; expires_at: string };
export type Overview = { frozen: boolean; mode: string; profile_revision: string; inference_route: string; platform_connected: boolean; pages: PageRow[]; sources: Source[]; briefs: BriefRow[]; roles: string[]; claims: { claim_id: string; text: string; source: string }[] };
export type Opportunity = { id: string; topic_id: string; status: string; score: number; confidence: number; components: Record<string, number>; vetoes: string[]; reasons: string[]; notes: string[]; families: string[][] };

export function message(data: unknown, fallback = "Something went wrong. Try again."): string {
  const d = data as { message?: string; details?: string[] } | null;
  if (!d || typeof d.message !== "string") return fallback;
  return d.details?.length ? `${d.message} ${d.details.join("; ")}` : d.message;
}

export function key(): string {
  return `k_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 10)}`;
}

export const growth = ops;

/** One CSV row per week: source_id,term,week_start (YYYY-MM-DD, a Monday),value */
export function parseCsv(text: string, sources: Source[]): { items: unknown[]; errors: string[] } {
  const items: unknown[] = [];
  const errors: string[] = [];
  text.split(/\r?\n/).map((l) => l.trim()).filter((l) => l && !l.startsWith("source_id")).forEach((line, i) => {
    const [source_id, term, week, value] = line.split(",").map((x) => x.trim());
    const src = sources.find((s) => s.source_id === source_id);
    const start = new Date(`${week}T00:00:00Z`);
    if (!src || src.access_path !== "file_import" || !term || Number.isNaN(start.getTime()) || !(Number(value) >= 0)) {
      errors.push(`Line ${i + 1}: expected source_id,term,week_start,value for an import source`);
      return;
    }
    const end = new Date(start.getTime() + 7 * 86400000);
    const slug = term.toLowerCase().replace(/[^a-z0-9]+/g, "_").slice(0, 60);
    items.push({
      source_id, source_record_id: `${source_id}-${slug}-${week.replaceAll("-", "")}`, term, market: "GB", locale: "en-GB",
      metric: src.kind === "index" ? "interest" : "clicks", value: Number(value), unit: src.kind, coverage: 1,
      window_start: start.toISOString().replace(".000", ""), window_end: end.toISOString().replace(".000", ""), observed_at: end.toISOString().replace(".000", ""),
    });
  });
  return { items, errors };
}
