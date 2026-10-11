"use client";

import { useEffect, useState } from "react";
import { problemTitle } from "@/lib/api";
import { actionText, appeal, standing, type StandingDecision } from "@/lib/safety";

/** Shows enforcement decisions about the member, with the reason and a way to appeal. */
export function Standing() {
  const [state, setState] = useState("good");
  const [review, setReview] = useState(false);
  const [decisions, setDecisions] = useState<StandingDecision[]>([]);
  const [writing, setWriting] = useState<string | null>(null);
  const [text, setText] = useState("");
  const [msg, setMsg] = useState("");

  async function load() {
    const r = await standing();
    if (r.status === 200) {
      setState(r.data.standing);
      setDecisions(r.data.decisions ?? []);
      setReview(r.data.under_review);
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function send(id: string) {
    const r = await appeal(id, text.trim());
    if (r.status !== 201) return setMsg(problemTitle(r.data));
    setMsg(r.data.message);
    setWriting(null);
    setText("");
    load();
  }

  if (!decisions.length && !review) return null;
  return (
    <section className="card" data-testid="standing" aria-label="Account standing">
      {review && state !== "permanent" ? (
        <p role="alert"><strong>Paused for a safety review.</strong> Your account is paused while our safety team looks at a report. This usually takes under an hour. We&rsquo;ll show the outcome here.</p>
      ) : state === "suspend" || state === "permanent" ? (
        <p role="alert"><strong>{actionText[state]}.</strong> You can't use messages, Tonight or your profile while this applies.</p>
      ) : (
        <p>Your account is {state === "good" ? "in good standing" : actionText[state]?.toLowerCase()}.</p>
      )}
      <ul className="st-list">
        {decisions.map((d) => (
          <li key={d.decision_id}>
            <p><strong>{actionText[d.action] ?? d.action}</strong> · {new Date(d.at).toLocaleDateString()}</p>
            <p className="note">{d.reason}</p>
            {d.appeal ? (
              <p className="note">
                {d.appeal.outcome === "overturned" ? "Your appeal succeeded. The decision was reversed."
                  : d.appeal.outcome === "upheld" ? "Your appeal was reviewed. The decision stands."
                  : `Appeal received. A different reviewer will decide by ${new Date(d.appeal.due).toLocaleDateString()}.`}
              </p>
            ) : writing === d.decision_id ? (
              <div>
                <label htmlFor={`appeal-${d.decision_id}`}>Why is this decision wrong?</label>
                <textarea id={`appeal-${d.decision_id}`} className="input" rows={4} maxLength={4000} value={text} onChange={(e) => setText(e.target.value)} />
                <button type="button" className="button inline" onClick={() => send(d.decision_id)}>Send appeal</button>
              </div>
            ) : (
              <button type="button" className="button ghost inline" onClick={() => { setWriting(d.decision_id); setMsg(""); }}>Appeal</button>
            )}
          </li>
        ))}
      </ul>
      {msg && <p role="status">{msg}</p>}
    </section>
  );
}
