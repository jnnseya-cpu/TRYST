"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { Shell } from "@/components/Shell";
import { api, getToken, problemTitle } from "@/lib/api";
import { doorWords } from "@/lib/doorwords";
import { b64, unb64 } from "@/lib/e2ee";
import { imageURL } from "@/lib/profile";
import { readSince, registerDevice, sendPayload, threadDetail, type MeetPayload, type ThreadView } from "@/lib/threads";
import "../../../profile/profile.css";
import "../../messages.css";
import "./meet.css";

type Meet = { id: string; state: "proposed" | "confirmed" | "cancelled" | "done"; mine: boolean; start: string; end: string; safety_active?: boolean };

const when = (iso: string) => new Date(iso).toLocaleString([], { weekday: "long", hour: "2-digit", minute: "2-digit" });

export default function MeetPage() {
  const { id } = useParams<{ id: string }>();
  const [thread, setThread] = useState<ThreadView | null>(null);
  const [plan, setPlan] = useState<MeetPayload | null>(null);
  const [meet, setMeet] = useState<Meet | null>(null);
  const [words, setWords] = useState<string[] | null>(null);
  const [photo, setPhoto] = useState<string | null>(null);
  const [msg, setMsg] = useState("");
  const [err, setErr] = useState("");
  const [panic, setPanic] = useState(false);
  // proposal form
  const [place, setPlace] = useState("");
  const [date, setDate] = useState(() => new Date().toISOString().slice(0, 10));
  const [from, setFrom] = useState("20:00");
  const [to, setTo] = useState("23:00");
  // safety form
  const [every, setEvery] = useState(60);
  const [pin, setPin] = useState("");
  const [duress, setDuress] = useState("");
  const [trusted, setTrusted] = useState("");
  const [line, setLine] = useState(true);
  const [checkPin, setCheckPin] = useState("");
  const [checking, setChecking] = useState(false);

  const load = useCallback(async () => {
    await registerDevice();
    const d = await threadDetail(id);
    if (d.status !== 200) return setErr("This conversation isn't available.");
    setThread(d.data.thread);
    const all = await readSince(id, 0);
    const latest = [...all].reverse().find((m) => m.payload?.type === "meet")?.payload as MeetPayload | undefined;
    setPlan(latest ?? null);
    if (latest) {
      const r = await api<Meet>(`/v1/meets/${latest.meet_id}`, { token: getToken() });
      if (r.status === 200) setMeet(r.data);
    }
  }, [id]);

  useEffect(() => {
    load();
    imageURL(`/v1/threads/${id}/photo`).then(setPhoto);
  }, [id, load]);

  async function propose() {
    if (!place.trim()) return setErr("Say where. Somewhere public for a first meet.");
    const start = new Date(`${date}T${from}`);
    const end = new Date(`${date}T${to}`);
    if (end <= start) end.setDate(end.getDate() + 1);
    const r = await api<Meet>(`/v1/threads/${id}/meets`, { method: "POST", token: getToken(), body: { start: start.toISOString(), end: end.toISOString() } });
    if (r.status !== 201) return setErr(problemTitle(r.data));
    const door = b64(crypto.getRandomValues(new Uint8Array(16)));
    const sent = await sendPayload(id, { v: 1, type: "meet", meet_id: r.data.id, place: place.trim(), start: start.toISOString(), end: end.toISOString(), door });
    if (!sent.ok) return setErr(sent.error ?? "Not sent.");
    setErr("");
    load();
  }

  async function decide(decision: "confirm" | "cancel") {
    if (!meet) return;
    const r = await api<Meet>(`/v1/meets/${meet.id}/${decision}`, { method: "POST", token: getToken() });
    if (r.status === 200) setMeet(r.data);
    else setErr(problemTitle(r.data));
  }

  async function showDoor() {
    if (!plan) return;
    setWords(await doorWords(unb64(plan.door), plan.meet_id));
  }

  async function startSafety() {
    if (!meet) return;
    const r = await api<Meet>(`/v1/meets/${meet.id}/safety`, {
      method: "POST", token: getToken(),
      body: { every_minutes: every, pin, duress_pin: duress, trusted_contact: trusted, safety_line: line },
    });
    if (r.status !== 200) return setErr(problemTitle(r.data));
    setMeet(r.data);
    setErr("");
    setPin("");
    setDuress("");
  }

  async function checkIn(end: boolean) {
    if (!meet || checking) return;
    // The PIN is cleared as it's sent, not when the answer comes back: on a slow connection
    // a reply arriving late would otherwise wipe the next PIN the member has typed.
    const pin = checkPin;
    setCheckPin("");
    setChecking(true);
    const r = await api<{ message: string; ended?: boolean }>(`/v1/meets/${meet.id}/checkin`, { method: "POST", token: getToken(), body: { pin, end } });
    setChecking(false);
    if (r.status !== 200) return setErr(problemTitle(r.data));
    setErr("");
    setMsg(r.data.message);
    if (r.data.ended) load();
  }

  async function sharePlan() {
    if (!plan) return;
    const text = `I'm meeting someone at ${plan.place}, ${when(plan.start)}. If you haven't heard from me by ${new Date(plan.end).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}, please call me.`;
    if (navigator.share) {
      try {
        await navigator.share({ text });
        return;
      } catch {
        /* cancelled */
      }
    }
    await navigator.clipboard?.writeText(text);
    setMsg("Copied. Paste it to someone you trust.");
  }

  async function raisePanic() {
    setPanic(true);
    let coords: { lat?: number; lon?: number } = {};
    try {
      const pos = await new Promise<GeolocationPosition>((res, rej) => navigator.geolocation.getCurrentPosition(res, rej, { timeout: 3000 }));
      coords = { lat: pos.coords.latitude, lon: pos.coords.longitude };
    } catch {
      /* location not available: escalate without it */
    }
    await api("/v1/safety/panic", { method: "POST", token: getToken(), body: { meet_id: meet?.id, ...coords } });
  }

  if (panic) {
    return (
      <Shell>
        <section className="mt-panic" role="alert" data-testid="panic">
          <h1>Call 999 now</h1>
          <a className="button mt-call" href="tel:999">Call 999</a>
          <p>If you can&rsquo;t speak, dial 999 and press <strong>55</strong> when asked. The police will know it&rsquo;s a real emergency.</p>
          <p className="note">We&rsquo;ve alerted the safety line{meet?.safety_active ? " and your trusted contact" : ""} with the location you shared.</p>
        </section>
      </Shell>
    );
  }
  if (!thread) return <Shell><p>{err || "Loading…"}</p></Shell>;

  return (
    <Shell wide>
      <p className="eyebrow">Meet {thread.name}</p>
      <h1>Plan a safe meet</h1>
      <button type="button" className="mt-panic-btn" onClick={raisePanic}>Panic</button>
      {err && <p role="alert">{err}</p>}
      {msg && <p role="status" className="ms-banner">{msg}</p>}

      {(!plan || meet?.state === "cancelled") && (
        <section className="pf-section" aria-labelledby="propose-h">
          <h2 id="propose-h">Suggest a first meet</h2>
          <p className="note">Somewhere public and busy. The place is only in your private conversation; we never see it.</p>
          <div className="pf-details">
            <div><label htmlFor="place">Where</label><input id="place" className="input" value={place} onChange={(e) => setPlace(e.target.value)} placeholder="The bar by the station" /></div>
            <div><label htmlFor="date">Day</label><input id="date" type="date" className="input" value={date} onChange={(e) => setDate(e.target.value)} /></div>
            <div className="mt-times">
              <div><label htmlFor="from">From</label><input id="from" type="time" className="input" value={from} onChange={(e) => setFrom(e.target.value)} /></div>
              <div><label htmlFor="to">Until</label><input id="to" type="time" className="input" value={to} onChange={(e) => setTo(e.target.value)} /></div>
            </div>
          </div>
          <button type="button" className="button inline" onClick={propose}>Send suggestion</button>
        </section>
      )}

      {plan && meet && meet.state !== "cancelled" && (
        <section className="pf-section" aria-labelledby="plan-h" data-testid="plan">
          <h2 id="plan-h">{plan.place}</h2>
          <p className="mt-when">{when(plan.start)} to {new Date(plan.end).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}</p>
          {meet.state === "proposed" && meet.mine && <p>Waiting for {thread.name} to confirm. <button type="button" className="ms-block" onClick={() => decide("cancel")}>Cancel</button></p>}
          {meet.state === "proposed" && !meet.mine && (
            <div className="mt-row">
              <button type="button" className="button inline" onClick={() => decide("confirm")}>Confirm</button>
              <button type="button" className="button ghost inline" onClick={() => decide("cancel")}>Decline</button>
            </div>
          )}
          {meet.state === "confirmed" && <p className="tn-flag">Confirmed by both of you.</p>}
        </section>
      )}

      {meet?.state === "confirmed" && plan && (
        <>
          <section className="pf-section mt-door" aria-labelledby="door-h">
            <h2 id="door-h">Door check</h2>
            <p className="note">When you arrive, you&rsquo;ll both see the same three words and each other&rsquo;s verified photo. If the person in front of you doesn&rsquo;t match, leave.</p>
            {words ? (
              <div className="mt-door-card" data-testid="door">
                <div className="mt-door-photo">{photo ? <img src={photo} alt={`${thread.name}, verified photo`} /> : <div className="pf-placeholder" />}</div>
                <div>
                  <p className="mt-words">{words.join(" · ")}</p>
                  <p className="note">Ask them to read their three words. They must match.</p>
                  <button type="button" className="ms-block" onClick={raisePanic}>This isn&rsquo;t them</button>
                </div>
              </div>
            ) : (
              <button type="button" className="button ghost inline" onClick={showDoor}>I&rsquo;m here</button>
            )}
          </section>

          <section className="pf-section" aria-labelledby="safe-h">
            <h2 id="safe-h">Check-ins</h2>
            {!meet.safety_active ? (
              <>
                <p className="note">If you don&rsquo;t check in, we tell your trusted contact, then the 24/7 safety line. Your duress PIN looks like a normal check-in but quietly raises the alarm.</p>
                <div className="pf-details">
                  <div><label htmlFor="every">Check in every</label>
                    <select id="every" className="input pf-select" value={every} onChange={(e) => setEvery(Number(e.target.value))}>
                      {[30, 60, 90, 120].map((m) => <option key={m} value={m}>{m} minutes</option>)}
                    </select></div>
                  <div><label htmlFor="pin">Check-in PIN</label><input id="pin" className="input" inputMode="numeric" type="password" value={pin} onChange={(e) => setPin(e.target.value)} /></div>
                  <div><label htmlFor="duress">Duress PIN</label><input id="duress" className="input" inputMode="numeric" type="password" value={duress} onChange={(e) => setDuress(e.target.value)} /></div>
                  <div><label htmlFor="trusted">Trusted contact (this meet only)</label><input id="trusted" className="input" value={trusted} onChange={(e) => setTrusted(e.target.value)} placeholder="+44… or email" /></div>
                </div>
                <label className="pf-check"><input type="checkbox" checked={line} onChange={(e) => setLine(e.target.checked)} /> Use the 24/7 safety line</label>
                <div className="mt-row">
                  <button type="button" className="button inline" onClick={startSafety}>Start check-ins</button>
                  <button type="button" className="button ghost inline" onClick={sharePlan}>Share my plan</button>
                </div>
              </>
            ) : (
              <div data-testid="checkins">
                <label htmlFor="checkpin">PIN</label>
                <input id="checkpin" className="input" inputMode="numeric" type="password" value={checkPin} onChange={(e) => setCheckPin(e.target.value)} />
                <div className="mt-row">
                  <button type="button" className="button inline" disabled={checking} onClick={() => checkIn(false)}>Check in</button>
                  <button type="button" className="button ghost inline" disabled={checking} onClick={() => checkIn(true)}>I&rsquo;m home safe</button>
                </div>
              </div>
            )}
          </section>
        </>
      )}
      <p><a href={`/messages/${id}`}>Back to the conversation</a></p>
    </Shell>
  );
}
