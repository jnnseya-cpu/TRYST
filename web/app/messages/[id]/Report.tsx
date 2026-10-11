"use client";

import { useEffect, useState } from "react";
import { problemTitle } from "@/lib/api";
import { categories, report, type Category } from "@/lib/safety";
import type { Shown } from "@/lib/threads";

/**
 * Report form. Messages are end-to-end encrypted, so TRYST sees only what the member ticks
 * here; nothing else from the conversation leaves their device.
 */
export function Report({ threadId, name, items, onBlock, canBlock, onClose }: {
  threadId: string;
  name: string;
  items: Shown[];
  onBlock: () => Promise<void>;
  canBlock: boolean;
  onClose: () => void;
}) {
  const [cats, setCats] = useState<Category[]>([]);
  const [cat, setCat] = useState("");
  const [picked, setPicked] = useState<Set<number>>(new Set());
  const [note, setNote] = useState("");
  const [alsoBlock, setAlsoBlock] = useState(true);
  const [error, setError] = useState("");
  const [done, setDone] = useState("");
  const [ref, setRef] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    categories().then(setCats);
  }, []);

  const theirs = items.filter((m) => !m.mine && m.payload?.type === "text").slice(-20);

  async function submit() {
    if (!cat) return setError("Choose what happened.");
    setBusy(true);
    const disclosed = theirs.filter((m) => picked.has(m.seq)).map((m) => (m.payload?.type === "text" ? m.payload.text : ""));
    const r = await report(threadId, cat, disclosed, note.trim());
    setBusy(false);
    if (r.status !== 201) return setError(problemTitle(r.data));
    if (alsoBlock && canBlock) await onBlock();
    setRef(r.data.reference);
    setDone(r.data.message);
  }

  if (done) {
    return (
      <section className="pf-section ms-report" aria-label="Report sent">
        <p role="status">{done}</p>
        <p className="note">Reference: <span data-testid="report-ref">{ref}</span></p>
        <p className="note">If you're in danger now, call 999.</p>
        <button type="button" className="button ghost inline" onClick={onClose}>Close</button>
      </section>
    );
  }

  return (
    <section className="pf-section ms-report" aria-label={`Report ${name}`}>
      <h2>Report {name}</h2>
      <p className="note">They won't be told. If you're in danger now, call 999.</p>
      <fieldset className="ms-cats">
        <legend>What happened?</legend>
        {cats.map((c) => (
          <label key={c.key}>
            <input type="radio" name="category" value={c.key} checked={cat === c.key} onChange={() => setCat(c.key)} /> {c.label}
          </label>
        ))}
      </fieldset>
      {theirs.length > 0 && (
        <fieldset className="ms-cats">
          <legend>Share messages with our safety team (optional)</legend>
          <p className="note">Your messages are encrypted, so we can only see the ones you tick.</p>
          {theirs.map((m) => (
            <label key={m.seq}>
              <input
                type="checkbox"
                checked={picked.has(m.seq)}
                onChange={() => setPicked((p) => { const n = new Set(p); if (n.has(m.seq)) n.delete(m.seq); else n.add(m.seq); return n; })}
              />{" "}
              {m.payload?.type === "text" ? m.payload.text : ""}
            </label>
          ))}
        </fieldset>
      )}
      <label htmlFor="report-note">Anything else we should know (optional)</label>
      <textarea id="report-note" className="input" rows={3} maxLength={2000} value={note} onChange={(e) => setNote(e.target.value)} />
      {canBlock && (
        <label className="ms-check">
          <input type="checkbox" checked={alsoBlock} onChange={(e) => setAlsoBlock(e.target.checked)} /> Also block {name}
        </label>
      )}
      {error && <p role="alert">{error}</p>}
      <div className="ms-tools">
        <button type="button" className="button inline" disabled={busy} onClick={submit}>Send report</button>
        <button type="button" className="ms-block" onClick={onClose}>Cancel</button>
      </div>
    </section>
  );
}
