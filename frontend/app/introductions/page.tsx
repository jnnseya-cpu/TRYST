"use client";

import { useCallback, useEffect, useState } from "react";
import { Shell } from "@/components/Shell";
import { problemTitle } from "@/lib/api";
import { imageURL } from "@/lib/profile";
import { CONSENT_COPY, setConsent } from "@/lib/consent";
import {
  approve, confirmVector, decide, envoy, intents, label, mark, reply, sendIntent, slate, vector,
  type Card, type Intents, type Vector, type Vocab,
} from "@/lib/intro";
import "../profile/profile.css";
import "../tonight/tonight.css";
import "../billing/billing.css";

function Face({ handle, name }: { handle: string; name: string }) {
  const [src, setSrc] = useState<string | null>(null);
  useEffect(() => {
    let url: string | null = null;
    imageURL(`/v1/slate/photos/${handle}/0`).then((u) => {
      url = u;
      setSrc(u);
    });
    return () => {
      if (url) URL.revokeObjectURL(url);
    };
  }, [handle]);
  return src ? <img src={src} alt={`${name}, main photo`} /> : <div className="pf-placeholder" />;
}

function Pick({ name, options, value, onChange, multi = true }: { name: string; options: string[]; value: string[]; onChange: (v: string[]) => void; multi?: boolean }) {
  return (
    <fieldset className="in-pick">
      <legend>{name}</legend>
      {options.map((o) => (
        <label key={o} className="bl-check">
          <input type={multi ? "checkbox" : "radio"} name={name} checked={value.includes(o)}
            onChange={(e) => onChange(multi ? (e.target.checked ? [...value, o] : value.filter((x) => x !== o)) : [o])} />
          {label(o)}
        </label>
      ))}
    </fieldset>
  );
}

function Intake({ vocab, initial, onDone }: { vocab: Vocab; initial: Vector | null; onDone: () => void }) {
  const [v, setV] = useState<Vector>(initial ?? { identity: "", seeking: [], intent_shape: [], hard_limits: [], interests: [] });
  const [msg, setMsg] = useState("");
  async function save() {
    const r = await confirmVector({ identity: v.identity, seeking: v.seeking, intent_shape: v.intent_shape, hard_limits: v.hard_limits, interests: v.interests });
    if (r.status !== 200) return setMsg(problemTitle(r.data));
    onDone();
  }
  return (
    <section className="pf-section" aria-label="What you're looking for" data-testid="intake">
      <h2>What you&rsquo;re looking for</h2>
      <p className="note">Only what you choose here is used to introduce you. You confirm every answer, and you can change it any time.</p>
      <Pick name="I am" options={vocab.identity} value={v.identity ? [v.identity] : []} multi={false} onChange={(x) => setV({ ...v, identity: x[0] })} />
      <Pick name="I want to meet" options={vocab.seeking} value={v.seeking} onChange={(x) => setV({ ...v, seeking: x })} />
      <Pick name="I'm looking for" options={vocab.intent_shape} value={v.intent_shape} onChange={(x) => setV({ ...v, intent_shape: x })} />
      <Pick name="Never show me" options={vocab.hard_limits.filter((h) => !v.intent_shape.includes(h))} value={v.hard_limits} onChange={(x) => setV({ ...v, hard_limits: x })} />
      <Pick name="Interests (up to 6)" options={vocab.interests} value={v.interests} onChange={(x) => setV({ ...v, interests: x.slice(0, 6) })} />
      <button type="button" className="button inline" disabled={!v.identity || !v.seeking.length || !v.intent_shape.length} onClick={save}>Confirm my answers</button>
      {msg && <p role="alert">{msg}</p>}
    </section>
  );
}

function CardView({ c, children }: { c: Card; children?: React.ReactNode }) {
  return (
    <li className="tn-person" data-testid="card">
      <div className="tn-photo"><Face handle={c.handle} name={c.display_name} /></div>
      <div className="tn-info">
        <p className="tn-name">{c.display_name}{c.age !== null && <span>, {c.age}</span>}{c.couple && <span> · couple</span>}</p>
        <p className="tn-place">{c.area} · {c.distance}</p>
        <p className="tn-meta">{c.explanation_codes.map(label).join(" · ")}{c.exploration && " · something different"}</p>
        {c.shared_interests.length > 0 && <p className="tn-meta">Both into: {c.shared_interests.map(label).join(", ")}</p>}
        {children}
      </div>
    </li>
  );
}

/** Daily introductions: slate, private intents, handshakes and ENVOY proposals. */
export default function Introductions() {
  const [vocab, setVocab] = useState<Vocab | null>(null);
  const [vec, setVec] = useState<Vector | null>(null);
  const [editing, setEditing] = useState(false);
  const [cards, setCards] = useState<Card[] | null>(null);
  const [blocked, setBlocked] = useState("");
  const [blockedCode, setBlockedCode] = useState("");
  const [agree, setAgree] = useState(false);
  const [inbox, setInbox] = useState<Intents | null>(null);
  const [props, setProps] = useState<{ card: Card; rationale: string[] }[] | null>(null);
  const [msg, setMsg] = useState("");
  const [signedOut, setSignedOut] = useState(false);

  const load = useCallback(async () => {
    const v = await vector();
    if (v.status === 401) return setSignedOut(true);
    if (v.status !== 200) return;
    setVocab(v.data.vocab);
    setVec(v.data.vector);
    const s = await slate();
    if (s.status === 200) {
      setCards(s.data.cards);
      setBlocked("");
    } else {
      setCards(null);
      setBlocked(problemTitle(s.data));
      setBlockedCode((s.data as { code?: string }).code ?? "");
    }
    const i = await intents();
    if (i.status === 200) setInbox(i.data);
    const e = await envoy();
    setProps(e.status === 200 ? e.data.proposals : null);
  }, []);
  useEffect(() => {
    load();
  }, [load]);

  async function act(run: () => Promise<{ status: number; data: unknown }>, done: string) {
    setMsg("");
    const r = await run();
    if (r.status >= 300) return setMsg(problemTitle(r.data));
    setMsg(done);
    load();
  }

  if (signedOut) return <Shell><h1>Signed out</h1><a className="button inline" href="/sign-in">Sign in</a></Shell>;
  if (!vocab) return <Shell><p>Loading…</p></Shell>;
  const left = inbox?.unlimited ? "Unlimited intents" : `${inbox?.free_left ?? 0} free intents left this week`;

  return (
    <Shell wide>
      <p className="eyebrow">Introductions</p>
      <h1>Today&rsquo;s introductions</h1>
      <p className="lede">A short list chosen for you each day. Saying you&rsquo;d like to meet someone is private; they only find out if they reply.</p>
      {msg && <p role="status">{msg}</p>}

      {(!vec || editing) ? (
        <Intake vocab={vocab} initial={vec} onDone={() => { setEditing(false); load(); }} />
      ) : (
        <p className="note">Looking for: {vec.intent_shape.map(label).join(", ")} · <button type="button" className="ms-block" onClick={() => setEditing(true)}>Change my answers</button></p>
      )}

      {inbox && inbox.handshakes.length > 0 && (
        <section className="pf-section" aria-label="Introductions to answer">
          <h2>Both of you want to meet</h2>
          <ul className="tn-list">
            {inbox.handshakes.map((h) => (
              <li key={h.id} className="tn-person is-mutual" data-testid="handshake">
                <div className="tn-info">
                  <p className="tn-name">{h.brief?.display_name}</p>
                  <p className="tn-place">{h.brief?.area} · {h.brief?.distance}</p>
                  {h.brief?.shared_intent && <p className="tn-meta">You both want: {h.brief.shared_intent.map(label).join(", ")}</p>}
                  {h.partner_decision && <p className="tn-meta">Your partner: {h.partner_decision}</p>}
                  {h.state === "accepted" && h.thread_id ? <a className="button inline" href={`/messages/${h.thread_id}`}>Message</a>
                    : h.state === "declined" ? <p className="tn-flag">This introduction has closed.</p>
                    : h.your_decision ? <p className="tn-flag">You said {h.your_decision}. Waiting for the other side.</p>
                    : (
                      <>
                        <button type="button" className="button inline" onClick={() => act(() => decide(h.id, "accept"), "Accepted.")}>Accept</button>
                        <button type="button" className="button ghost inline" onClick={() => act(() => decide(h.id, "decline"), "Declined. They won't be told why.")}>Decline</button>
                      </>
                    )}
                </div>
              </li>
            ))}
          </ul>
        </section>
      )}

      {inbox && inbox.inbound.length > 0 && (
        <section className="pf-section" aria-label="Someone would like to meet you">
          <h2>Would like to meet you</h2>
          <ul className="tn-list">
            {inbox.inbound.map((c) => (
              <CardView key={c.id} c={c}>
                <button type="button" className="button inline" onClick={() => act(() => reply(c.id, "accept"), "Accepted. Now you each decide on the introduction.")}>Yes, introduce us</button>
                <button type="button" className="button ghost inline" onClick={() => act(() => reply(c.id, "decline"), "Declined. They won't be told.")}>No thanks</button>
              </CardView>
            ))}
          </ul>
        </section>
      )}

      {props && (
        <section className="pf-section" aria-label="ENVOY" data-testid="envoy">
          <h2>ENVOY suggests</h2>
          <p className="note">Nothing is sent until you approve it.</p>
          <ul className="tn-list">
            {props.map((p) => (
              <CardView key={p.card.handle} c={p.card}>
                <p className="tn-meta">Why: {p.rationale.join("; ")}</p>
                <button type="button" className="button inline" onClick={() => act(() => approve(p.card.handle), "Sent.")}>Approve and send</button>
              </CardView>
            ))}
          </ul>
        </section>
      )}

      <section className="pf-section" aria-label="Slate">
        <div className="pf-head">
          <h2>For you today</h2>
          <span className="pf-count" data-testid="free-left">{left}</span>
        </div>
        {blockedCode === "consent_required" ? (
          <div data-testid="art9-consent">
            <p>{CONSENT_COPY.art9_processing.body}</p>
            <p className="note">Separate from our Terms; you can withdraw it any time in your account.</p>
            <label className="bl-check"><input type="checkbox" checked={agree} onChange={(e) => setAgree(e.target.checked)} /> I give my explicit consent</label>
            <button type="button" className="button inline" disabled={!agree} onClick={() => act(() => setConsent("art9_processing", true), "Thank you.")}>Continue</button>
          </div>
        ) : blocked ? <p className="pf-empty" data-testid="slate-blocked">{blocked}</p>
          : !cards || cards.length === 0 ? <p className="pf-empty">No new introductions today. We&rsquo;ll look again tomorrow.</p>
          : (
            <ul className="tn-list">
              {cards.map((c) => (
                <CardView key={c.handle} c={c}>
                  {c.intent_sent ? <p className="tn-flag">Sent. They&rsquo;ll see it privately.</p> : (
                    <>
                      <button type="button" className="button inline" onClick={() => act(() => sendIntent(c.handle), "Sent. They'll see it privately.")}>I&rsquo;d like to meet</button>
                      <button type="button" className="button ghost inline" onClick={() => act(() => mark("pass", c.handle), "Passed. You won't see them again.")}>Pass</button>
                    </>
                  )}
                </CardView>
              ))}
            </ul>
          )}
      </section>
      <a className="button ghost" href="/couple">Couple link</a>
    </Shell>
  );
}
