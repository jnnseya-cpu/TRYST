"use client";

import { useEffect, useState } from "react";
import { Shell } from "@/components/Shell";
import { api, getToken, problemTitle } from "@/lib/api";
import { imageURL } from "@/lib/profile";
import "../profile/profile.css";
import "./tonight.css";

type Person = {
  handle: string;
  display_name: string;
  age: number | null;
  area: string;
  distance: string;
  photos: number;
  blurred: number;
  interested: boolean;
  mutual: boolean;
};

type View = {
  on: boolean;
  ready: boolean;
  tier_ok: boolean;
  until?: string;
  radius_km?: number;
  interests_left?: number;
  max_hours: number;
  people: Person[];
  mutual: Person[];
};

function Face({ handle, name }: { handle: string; name: string }) {
  const [src, setSrc] = useState<string | null>(null);
  useEffect(() => {
    let url: string | null = null;
    imageURL(`/v1/tonight/photos/${handle}/0`).then((u) => {
      url = u;
      setSrc(u);
    });
    return () => {
      if (url) URL.revokeObjectURL(url);
    };
  }, [handle]);
  return src ? <img src={src} alt={`${name}, main photo`} /> : <div className="pf-placeholder" />;
}

function untilText(iso?: string) {
  if (!iso) return "";
  return new Date(iso).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

export default function Tonight() {
  const [view, setView] = useState<View | null>(null);
  const [signedOut, setSignedOut] = useState(false);
  const [hours, setHours] = useState(4);
  const [radius, setRadius] = useState(10);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function load() {
    const r = await api<View>("/v1/tonight", { token: getToken() });
    if (r.status === 200) setView(r.data);
    else setSignedOut(true);
  }
  useEffect(() => {
    load();
    const t = setInterval(load, 30_000);
    return () => clearInterval(t);
  }, []);

  async function call(path: string, method: string, body?: unknown) {
    setBusy(true);
    const r = await api<View>(path, { method, body, token: getToken() });
    setBusy(false);
    if (r.status === 200) {
      setView(r.data);
      setError("");
    } else setError(problemTitle(r.data));
  }

  if (signedOut) {
    return (
      <Shell>
        <h1>Signed out</h1>
        <a className="button inline" href="/sign-in">Sign in</a>
      </Shell>
    );
  }
  if (!view) return <Shell><p>Loading…</p></Shell>;

  return (
    <Shell wide>
      <p className="eyebrow">Spark · tonight</p>
      <h1>Free tonight?</h1>
      <p className="lede">Say you&rsquo;re free and see who nearby is too. Only people who are also free tonight can see you, and only until you stop or your time runs out.</p>

      {error && <p role="alert">{error}</p>}

      {!view.ready || !view.tier_ok ? (
        <section className="pf-section">
          <p>{!view.tier_ok ? "Finish your ID check to use Tonight." : "Finish your profile first: a main photo, a name and your area."}</p>
          <a className="button inline" href="/profile">Your profile</a>
        </section>
      ) : !view.on ? (
        <section className="pf-section tn-start" aria-labelledby="start-h">
          <h2 id="start-h">Go visible for tonight</h2>
          <div className="tn-controls">
            <div>
              <label htmlFor="hours">For</label>
              <select id="hours" className="input pf-select" value={hours} onChange={(e) => setHours(Number(e.target.value))}>
                {Array.from({ length: view.max_hours }, (_, i) => i + 1).map((h) => (
                  <option key={h} value={h}>{h} hour{h === 1 ? "" : "s"}</option>
                ))}
              </select>
            </div>
            <div>
              <label htmlFor="radius">Within</label>
              <select id="radius" className="input pf-select" value={radius} onChange={(e) => setRadius(Number(e.target.value))}>
                {[5, 10, 25, 50].map((k) => <option key={k} value={k}>about {k} km</option>)}
              </select>
            </div>
          </div>
          <button type="button" className="button inline" disabled={busy} onClick={() => call("/v1/tonight", "PUT", { hours, radius_km: radius })}>
            I&rsquo;m free tonight
          </button>
        </section>
      ) : (
        <>
          <section className="pf-section tn-status" data-testid="tonight-on">
            <p>You&rsquo;re visible to people who are also free, within about {view.radius_km} km, until {untilText(view.until)}.</p>
            <button type="button" className="button ghost inline" disabled={busy} onClick={() => call("/v1/tonight", "DELETE")}>Stop now</button>
          </section>

          <aside className="tn-safety">
            <p><strong>Before you go:</strong> meet somewhere public first, set your check-in and duress PIN, and use the door check when you arrive.</p>
          </aside>

          {view.mutual.length > 0 && (
            <section className="pf-section" aria-labelledby="mutual-h">
              <h2 id="mutual-h">It&rsquo;s mutual</h2>
              <ul className="tn-list">
                {view.mutual.map((p) => (
                  <li key={p.handle} className="tn-person is-mutual" data-testid="mutual">
                    <div className="tn-photo"><Face handle={p.handle} name={p.display_name} /></div>
                    <div className="tn-info">
                      <p className="tn-name">{p.display_name}{p.age !== null && <span>, {p.age}</span>}</p>
                      <p className="tn-place">{p.area} · {p.distance}</p>
                      <p className="tn-flag">You both said yes for tonight.</p>
                    </div>
                  </li>
                ))}
              </ul>
            </section>
          )}

          <section className="pf-section" aria-labelledby="near-h">
            <div className="pf-head">
              <h2 id="near-h">Free nearby</h2>
              <span className="pf-count">{view.interests_left} of 10 left tonight</span>
            </div>
            {view.people.length === 0 ? (
              <p className="pf-empty">Nobody nearby is free right now. We&rsquo;ll keep looking while you&rsquo;re visible.</p>
            ) : (
              <ul className="tn-list">
                {view.people.map((p) => (
                  <li key={p.handle} className="tn-person" data-testid="person">
                    <div className="tn-photo"><Face handle={p.handle} name={p.display_name} /></div>
                    <div className="tn-info">
                      <p className="tn-name">{p.display_name}{p.age !== null && <span>, {p.age}</span>}</p>
                      <p className="tn-place">{p.area} · {p.distance}</p>
                      <p className="tn-meta">{p.photos} photo{p.photos === 1 ? "" : "s"}{p.blurred > 0 && ` · ${p.blurred} blurred`}</p>
                      {p.interested ? (
                        <p className="tn-flag">Interest sent. They&rsquo;ll only know if they say yes too.</p>
                      ) : (
                        <button type="button" className="button inline" disabled={busy}
                          onClick={() => call(`/v1/tonight/interest/${p.handle}`, "POST")}>Interested</button>
                      )}
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </section>
        </>
      )}
    </Shell>
  );
}
