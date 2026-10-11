"use client";

import { useEffect, useState } from "react";
import { Shell } from "@/components/Shell";
import { markRead, notices, type Notice } from "@/lib/notify";
import "../profile/profile.css";
import "../messages/messages.css";

export default function Notices() {
  const [list, setList] = useState<Notice[] | null>(null);
  const [signedOut, setSignedOut] = useState(false);

  useEffect(() => {
    notices().then((r) => {
      if (r.status !== 200) return setSignedOut(true);
      setList(r.data.notices);
      if (r.data.unread > 0) markRead();
    });
  }, []);

  if (signedOut) return <Shell><h1>Signed out</h1><a className="button inline" href="/sign-in">Sign in</a></Shell>;
  if (!list) return <Shell><p>Loading…</p></Shell>;
  return (
    <Shell wide>
      <p className="eyebrow">Private</p>
      <h1>Notices</h1>
      {list.length === 0 ? (
        <p className="pf-empty">Nothing new.</p>
      ) : (
        <ul className="ms-list">
          {list.map((n) => (
            <li key={n.id} data-testid="notice">
              <a href={n.link ?? "/account"}>
                <span className={n.read ? "" : "ms-name"}>{n.text}</span>
                <span className="ms-meta">{new Date(n.at).toLocaleString([], { weekday: "short", hour: "2-digit", minute: "2-digit" })}</span>
              </a>
            </li>
          ))}
        </ul>
      )}
    </Shell>
  );
}
