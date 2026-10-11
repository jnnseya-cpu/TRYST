"use client";

import { useEffect, useState } from "react";
import { Shell } from "@/components/Shell";
import { api, getToken } from "@/lib/api";
import { registerDevice, type ThreadView } from "@/lib/threads";
import "../profile/profile.css";
import "./messages.css";

export default function Messages() {
  const [threads, setThreads] = useState<ThreadView[] | null>(null);
  const [signedOut, setSignedOut] = useState(false);
  const [suspended, setSuspended] = useState(false);

  useEffect(() => {
    api<{ threads: ThreadView[] }>("/v1/threads", { token: getToken() }).then((r) => {
      if (r.status === 403) return setSuspended(true);
      if (r.status !== 200) return setSignedOut(true);
      setThreads(r.data.threads);
      registerDevice().catch(() => undefined);
    });
  }, []);

  if (signedOut) return <Shell><h1>Signed out</h1><a className="button inline" href="/sign-in">Sign in</a></Shell>;
  if (suspended) return <Shell><h1>Account suspended</h1><p>You can see why, and appeal, from your account page.</p><a className="button inline" href="/account">Your account</a></Shell>;
  if (!threads) return <Shell><p>Loading…</p></Shell>;
  return (
    <Shell wide>
      <p className="eyebrow">Private</p>
      <h1>Messages</h1>
      <p className="lede">Conversations open only when you both said yes. They&rsquo;re encrypted on your device; we can&rsquo;t read them.</p>
      {threads.length === 0 ? (
        <p className="pf-empty">No conversations yet. When interest is mutual, it opens here.</p>
      ) : (
        <ul className="ms-list">
          {threads.map((t) => (
            <li key={t.id} data-testid="thread">
              <a href={`/messages/${t.id}`}>
                <span className="ms-name">{t.name}{t.age !== null && <span>, {t.age}</span>}</span>
                <span className="ms-meta">{t.state === "normal" ? (t.last_seq ? `${t.last_seq} message${t.last_seq === 1 ? "" : "s"}` : "New match") : t.state === "frozen" ? "Paused for a safety review" : "Closed"}</span>
              </a>
            </li>
          ))}
        </ul>
      )}
    </Shell>
  );
}
