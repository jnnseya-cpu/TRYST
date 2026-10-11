"use client";

import { useEffect, useRef, useState } from "react";
import { Shell } from "@/components/Shell";
import { api, getToken, problemTitle } from "@/lib/api";
import { imageURL, uploadPhoto, type Area, type OwnerProfile, type PhotoView } from "@/lib/profile";
import "./profile.css";

function Photo({ photo, index, count, canLeft, busy, onAction }: {
  photo: PhotoView;
  index: number;
  count: number;
  canLeft: boolean;
  busy: boolean;
  onAction: (action: "main" | "toggle" | "left" | "right" | "remove", p: PhotoView) => void;
}) {
  const [src, setSrc] = useState<string | null>(null);
  useEffect(() => {
    let url: string | null = null;
    imageURL(photo.url).then((u) => {
      url = u;
      setSrc(u);
    });
    return () => {
      if (url) URL.revokeObjectURL(url);
    };
  }, [photo.url]);

  return (
    <li className={`pf-photo${photo.main ? " is-main" : ""}`} data-testid="photo">
      <div className="pf-frame">
        {src && <img src={src} alt={photo.main ? "Your main photo" : `Photo ${index + 1}`} className={photo.revealed ? "" : "is-blurred"} />}
        {photo.main && <span className="pf-tag">Main photo</span>}
        {!photo.revealed && <span className="pf-tag pf-tag-quiet">Blurred until you reveal it</span>}
        {photo.status === "checking" && <span className="pf-tag pf-tag-quiet">Being checked</span>}
      </div>
      <div className="pf-actions">
        {!photo.main && photo.can_be_main && (
          <button type="button" disabled={busy} onClick={() => onAction("main", photo)}>Make main</button>
        )}
        {!photo.main && (
          <button type="button" disabled={busy} onClick={() => onAction("toggle", photo)}>{photo.revealed ? "Blur" : "Show"}</button>
        )}
        {!photo.main && canLeft && (
          <button type="button" disabled={busy} aria-label="Move earlier" onClick={() => onAction("left", photo)}>←</button>
        )}
        {!photo.main && index < count - 1 && (
          <button type="button" disabled={busy} aria-label="Move later" onClick={() => onAction("right", photo)}>→</button>
        )}
        <button type="button" disabled={busy} onClick={() => onAction("remove", photo)}>Remove</button>
      </div>
      {!photo.main && !photo.can_be_main && photo.main_problem && <p className="pf-note">{photo.main_problem}</p>}
    </li>
  );
}

export default function ProfileEditor() {
  const [profile, setProfile] = useState<OwnerProfile | null>(null);
  const [areas, setAreas] = useState<Area[]>([]);
  const [signedOut, setSignedOut] = useState(false);
  const [busy, setBusy] = useState(false);
  const [errors, setErrors] = useState<string[]>([]);
  const [name, setName] = useState("");
  const fileInput = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const token = getToken();
    api<OwnerProfile>("/v1/profile", { token }).then((r) => {
      if (r.status !== 200) return setSignedOut(true);
      setProfile(r.data);
      setName(r.data.display_name);
    });
    api<{ areas: Area[] }>("/v1/profile/areas", { token }).then((r) => r.status === 200 && setAreas(r.data.areas));
  }, []);

  async function call(path: string, method: string, body?: unknown) {
    setBusy(true);
    const r = await api<OwnerProfile>(path, { method, body, token: getToken() });
    setBusy(false);
    if (r.status === 200) {
      setProfile(r.data);
      setErrors([]);
      return true;
    }
    setErrors([problemTitle(r.data)]);
    return false;
  }

  async function onFiles(files: FileList | null) {
    if (!files || !profile) return;
    const room = profile.max_photos - profile.photos.length;
    const chosen = Array.from(files);
    const problems: string[] = [];
    if (chosen.length > room) problems.push(`You can add ${room} more photo${room === 1 ? "" : "s"}. The rest were not uploaded.`);
    setBusy(true);
    for (const file of chosen.slice(0, Math.max(room, 0))) {
      const res = await uploadPhoto(file);
      if (res.ok) setProfile(res.profile);
      else problems.push(`${file.name}: ${res.reason}`);
    }
    setBusy(false);
    setErrors(problems);
    if (fileInput.current) fileInput.current.value = "";
  }

  function onAction(action: "main" | "toggle" | "left" | "right" | "remove", p: PhotoView) {
    if (!profile) return;
    if (action === "main") call(`/v1/profile/photos/${p.id}/main`, "POST");
    if (action === "toggle") call(`/v1/profile/photos/${p.id}`, "PATCH", { revealed: !p.revealed });
    if (action === "remove") call(`/v1/profile/photos/${p.id}`, "DELETE");
    if (action === "left" || action === "right") {
      const ids = profile.photos.map((x) => x.id);
      const i = ids.indexOf(p.id);
      const j = action === "left" ? i - 1 : i + 1;
      [ids[i], ids[j]] = [ids[j], ids[i]];
      call("/v1/profile/photos/order", "PUT", { ids });
    }
  }

  if (signedOut) {
    return (
      <Shell>
        <h1>Signed out</h1>
        <p className="lede">Sign in with both checks to edit your profile.</p>
        <a className="button inline" href="/sign-in">Sign in</a>
      </Shell>
    );
  }
  if (!profile) return <Shell><p>Loading…</p></Shell>;

  const count = profile.photos.length;
  const cities = Array.from(new Set(areas.map((a) => a.city)));

  return (
    <Shell wide>
      <p className="eyebrow">Your profile</p>
      <h1>How other members see you</h1>
      <p className="lede">Your main photo, your age and your area are shown to verified members who can see you. People you&rsquo;ve shielded never see you at all.</p>

      <section className="pf-section" aria-labelledby="photos-h">
        <div className="pf-head">
          <h2 id="photos-h">Photos <span className="pf-count">{count} of {profile.max_photos}</span></h2>
          <label className={`button inline${busy || count >= profile.max_photos ? " is-disabled" : ""}`}>
            Add photos
            <input ref={fileInput} type="file" accept="image/jpeg,image/png" multiple hidden
              disabled={busy || count >= profile.max_photos} onChange={(e) => onFiles(e.target.files)} data-testid="file-input" />
          </label>
        </div>
        <ul className="pf-rules">
          <li>Your main photo must clearly show your face. It is always shown, never blurred.</li>
          <li>Real photos of you only: nothing computer-generated or edited with an image generator, no one else in the picture, nothing found online.</li>
          <li>Location and camera details are removed from your photos before they leave this device.</li>
        </ul>
        {errors.length > 0 && (
          <div role="alert" className="pf-errors">
            {errors.map((e) => <p key={e}>{e}</p>)}
          </div>
        )}
        {count === 0 ? (
          <p className="pf-empty">No photos yet. Start with one that clearly shows your face.</p>
        ) : (
          <ul className="pf-grid">
            {profile.photos.map((p, i) => (
              <Photo key={p.id} photo={p} index={i} count={count} canLeft={i > 0 && !profile.photos[i - 1].main} busy={busy} onAction={onAction} />
            ))}
          </ul>
        )}
      </section>

      <section className="pf-section" aria-labelledby="details-h">
        <h2 id="details-h">Details</h2>
        <div className="pf-details">
          <div>
            <label htmlFor="name">Name shown to members</label>
            <input id="name" className="input" value={name} maxLength={24} autoComplete="off"
              onChange={(e) => setName(e.target.value)} onBlur={() => name !== profile.display_name && call("/v1/profile", "PUT", { display_name: name })} />
            <p className="note">Not your real name. No contact details, links or prices.</p>
          </div>
          <div>
            <label htmlFor="age">Age</label>
            <p id="age" className="pf-fixed">{profile.age ?? "Shown after your ID check"}</p>
            <p className="note">From your verified ID. It can&rsquo;t be edited.</p>
          </div>
          <div>
            <label htmlFor="area">Area</label>
            <select id="area" className="input pf-select" value={profile.area.id ?? ""} disabled={busy}
              onChange={(e) => call("/v1/profile", "PUT", { area_id: e.target.value })}>
              <option value="" disabled>Choose your area</option>
              {cities.map((c) => (
                <optgroup key={c} label={c}>
                  {areas.filter((a) => a.city === c).map((a) => <option key={a.id} value={a.id}>{a.district}</option>)}
                </optgroup>
              ))}
            </select>
            <label className="pf-check">
              <input type="checkbox" checked={profile.city_only} disabled={busy}
                onChange={(e) => call("/v1/profile", "PUT", { city_only: e.target.checked })} />
              Show only my city
            </label>
            <p className="note">Members see an area and an approximate distance, never your exact location.</p>
          </div>
        </div>
      </section>

      <section className="pf-section pf-status" aria-live="polite">
        {profile.ready ? (
          <p data-testid="ready">Your profile is ready to be seen.</p>
        ) : (
          <>
            <p>Before members can see you:</p>
            <ul>{(profile.problems ?? []).map((p) => <li key={p}>{p}</li>)}</ul>
          </>
        )}
        <a className="button ghost inline" href="/profile/card">Preview your card</a>
      </section>
    </Shell>
  );
}
