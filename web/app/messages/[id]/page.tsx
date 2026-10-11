"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useParams } from "next/navigation";
import { Shell } from "@/components/Shell";
import { api, getToken, problemTitle } from "@/lib/api";
import { check, MODEL } from "@/lib/guardian";
import { imageURL } from "@/lib/profile";
import { readSince, registerDevice, sendPayload, threadDetail, type Shown, type ThreadView } from "@/lib/threads";
import "../../profile/profile.css";
import "../messages.css";

function bannerFor(kind: string) {
  if (kind === "fraud") return "This message asks for money, gift cards or an investment. Members never need to send money to meet. Don't send any.";
  if (kind === "extortion") return "This looks like a threat. We've paused the conversation for our safety team. Don't pay. You can report it and we'll help.";
  return "We've paused this conversation for our safety team.";
}

export default function Conversation() {
  const { id } = useParams<{ id: string }>();
  const [thread, setThread] = useState<ThreadView | null>(null);
  const [items, setItems] = useState<Shown[]>([]);
  const [text, setText] = useState("");
  const [error, setError] = useState("");
  const [banners, setBanners] = useState<string[]>([]);
  const [photo, setPhoto] = useState<string | null>(null);
  const [missing, setMissing] = useState(false);
  const last = useRef(0);
  const flagged = useRef(new Set<number>());

  const refresh = useCallback(async () => {
    const d = await threadDetail(id);
    if (d.status !== 200) return setMissing(true);
    setThread(d.data.thread);
    const fresh = await readSince(id, last.current);
    if (fresh.length) {
      last.current = fresh[fresh.length - 1].seq;
      setItems((prev) => [...prev, ...fresh]);
      for (const m of fresh) {
        if (m.mine || !m.payload || m.payload.type !== "text" || flagged.current.has(m.seq)) continue;
        for (const s of check(m.payload.text)) {
          flagged.current.add(m.seq);
          setBanners((b) => (b.includes(bannerFor(s.kind)) ? b : [...b, bannerFor(s.kind)]));
          await api(`/v1/threads/${id}/signal`, { method: "POST", token: getToken(), body: { kind: s.kind, model: MODEL, about: "received" } });
        }
      }
    }
  }, [id]);

  useEffect(() => {
    registerDevice().then(refresh);
    imageURL(`/v1/threads/${id}/photo`).then(setPhoto);
    const t = setInterval(refresh, 3000);
    return () => clearInterval(t);
  }, [id, refresh]);

  async function send() {
    const body = text.trim();
    if (!body) return;
    const s = check(body).find((x) => x.kind === "child_safety");
    if (s) {
      setError("This message can't be sent.");
      await api(`/v1/threads/${id}/signal`, { method: "POST", token: getToken(), body: { kind: s.kind, model: MODEL, about: "blocked_send" } });
      return;
    }
    const r = await sendPayload(id, { v: 1, type: "text", text: body });
    if (!r.ok) return setError(r.error ?? "Not sent.");
    setText("");
    setError("");
    refresh();
  }

  async function block() {
    if (!confirm("Block and close this conversation? They won't be told why, and you won't see each other again.")) return;
    const r = await api<{ thread: ThreadView }>(`/v1/threads/${id}/block`, { method: "POST", token: getToken() });
    if (r.status === 200) setThread(r.data.thread);
    else setError(problemTitle(r.data));
  }

  async function retention(days: number) {
    const r = await api<{ thread: ThreadView }>(`/v1/threads/${id}/retention`, { method: "PUT", token: getToken(), body: { days } });
    if (r.status === 200) setThread(r.data.thread);
  }

  if (missing) return <Shell><h1>Conversation not available</h1><a href="/messages">All messages</a></Shell>;
  if (!thread) return <Shell><p>Loading…</p></Shell>;
  const open = thread.state === "normal";

  return (
    <Shell wide>
      <header className="ms-head">
        <div className="ms-face">{photo ? <img src={photo} alt={`${thread.name}, main photo`} /> : <div className="pf-placeholder" />}</div>
        <div>
          <p className="eyebrow">Private conversation</p>
          <h1 className="ms-title">{thread.name}{thread.age !== null && <span>, {thread.age}</span>}</h1>
          <p className="note">Encrypted on your devices. Messages disappear after {thread.retention_days} days.</p>
        </div>
      </header>

      {banners.map((b) => <p key={b} role="alert" className="ms-banner">{b}</p>)}
      {thread.state === "frozen" && <p role="status" className="ms-banner">This conversation is paused while our safety team takes a look.</p>}
      {thread.state === "closed" && <p role="status" className="ms-banner">This conversation has ended.</p>}

      <ol className="ms-thread" data-testid="messages">
        {items.length === 0 && <li className="ms-empty">You both said yes. Say hello, and agree somewhere public for a first meet.</li>}
        {items.map((m) => (
          <li key={m.seq} className={m.mine ? "is-mine" : ""} data-testid="message">
            {m.payload === null ? (
              <span className="ms-locked">Sent before this device was linked.</span>
            ) : m.payload.type === "text" ? (
              <span>{m.payload.text}</span>
            ) : (
              <span className="ms-meet">Meet: {m.payload.place}, {m.payload.time}</span>
            )}
          </li>
        ))}
      </ol>

      {open && (
        <form className="ms-compose" onSubmit={(e) => { e.preventDefault(); send(); }}>
          <label htmlFor="msg" className="ms-sr">Message</label>
          <textarea id="msg" className="input ms-input" rows={2} value={text} maxLength={2000} onChange={(e) => setText(e.target.value)} placeholder="Write a message" />
          <button type="submit" className="button inline">Send</button>
        </form>
      )}
      {error && <p role="alert">{error}</p>}

      <section className="pf-section ms-tools">
        <a className="button ghost inline" href={`/messages/${id}/meet`}>Plan a safe meet</a>
        <label className="ms-retain">Keep messages for
          <select className="input pf-select" value={thread.retention_days} onChange={(e) => retention(Number(e.target.value))}>
            {[7, 30, 90].map((d) => <option key={d} value={d}>{d} days</option>)}
          </select>
        </label>
        {open && <button type="button" className="ms-block" onClick={block}>Block</button>}
      </section>
      <p><a href="/messages">All messages</a></p>
    </Shell>
  );
}
