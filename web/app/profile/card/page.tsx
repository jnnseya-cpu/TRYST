"use client";

import { useEffect, useState } from "react";
import { Shell } from "@/components/Shell";
import { api, getToken } from "@/lib/api";
import { imageURL, type Card } from "@/lib/profile";
import "../profile.css";

function Img({ path, alt, blurred, className }: { path: string; alt: string; blurred?: boolean; className?: string }) {
  const [src, setSrc] = useState<string | null>(null);
  useEffect(() => {
    let url: string | null = null;
    imageURL(path).then((u) => {
      url = u;
      setSrc(u);
    });
    return () => {
      if (url) URL.revokeObjectURL(url);
    };
  }, [path]);
  if (!src) return <div className={`pf-placeholder ${className ?? ""}`} />;
  return <img src={src} alt={alt} className={`${className ?? ""}${blurred ? " is-blurred" : ""}`} />;
}

/** The card exactly as another verified member sees it in their slate. */
export default function CardPreview() {
  const [card, setCard] = useState<Card | null>(null);
  const [signedOut, setSignedOut] = useState(false);

  useEffect(() => {
    api<Card>("/v1/profile/card", { token: getToken() }).then((r) => (r.status === 200 ? setCard(r.data) : setSignedOut(true)));
  }, []);

  if (signedOut) {
    return (
      <Shell>
        <h1>Signed out</h1>
        <a className="button inline" href="/sign-in">Sign in</a>
      </Shell>
    );
  }
  if (!card) return <Shell><p>Loading…</p></Shell>;

  const photos = card.photos ?? [];
  const main = photos.find((p) => p.main);
  const rest = photos.filter((p) => !p.main);

  return (
    <Shell wide>
      <p className="eyebrow">Preview</p>
      <h1>Your card</h1>
      <p className="lede">This is what a verified member sees when you appear in their introductions.</p>
      {!card.ready && (
        <p role="alert">Your profile isn&rsquo;t visible yet. <a href="/profile">Finish your profile</a>.</p>
      )}

      <article className="pf-card" data-testid="card" aria-label="Profile card">
        <div className="pf-card-hero">
          {main ? <Img path={main.url} alt={`${card.display_name || "Member"}, main photo`} className="pf-card-main" /> : <div className="pf-placeholder pf-card-main" />}
          <div className="pf-card-caption">
            <p className="pf-card-name">
              {card.display_name || "Your name"}
              {card.age !== null && <span className="pf-card-age">, {card.age}</span>}
            </p>
            <p className="pf-card-place">
              {card.area || "Your area"}
              {card.distance && <span> · {card.distance}</span>}
            </p>
          </div>
        </div>
        <div className="pf-card-body">
          {card.verified === "V2" || card.verified === "V3" ? (
            <p className="pf-card-fact">ID and face verified</p>
          ) : (
            <p className="pf-card-fact is-pending">ID check not finished yet. Members can&rsquo;t see your card until it is.</p>
          )}
          {rest.length > 0 && (
            <ul className="pf-card-thumbs" aria-label="More photos">
              {rest.map((p, i) => (
                <li key={p.url}>
                  <Img path={p.url} alt={`Photo ${i + 2}`} blurred={p.blurred} className="pf-thumb" />
                  {p.blurred && <span className="pf-thumb-lock">Blurred</span>}
                </li>
              ))}
            </ul>
          )}
          <p className="pf-card-count">{photos.length} photo{photos.length === 1 ? "" : "s"}</p>
        </div>
      </article>
      <p><a href="/profile">Edit your profile</a></p>
    </Shell>
  );
}
