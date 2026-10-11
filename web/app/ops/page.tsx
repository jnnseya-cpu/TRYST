"use client";

import { useCallback, useEffect, useState } from "react";
import { Shell } from "@/components/Shell";
import { ops, setStaffToken, staffToken } from "@/lib/safety";
import "../profile/profile.css";
import "./ops.css";

type Row = { id: string; category: string; label: string; source: string; opened: string; due: string; overdue: boolean; state: string; appeal_pending: boolean };

function dueText(due: string) {
  const mins = Math.round((new Date(due).getTime() - Date.now()) / 60000);
  if (mins < 0) return `${-mins} min overdue`;
  if (mins < 120) return `${mins} min`;
  return `${Math.round(mins / 60)} h`;
}

export default function Console() {
  const [rows, setRows] = useState<Row[] | null>(null);
  const [needsSignIn, setNeedsSignIn] = useState(false);
  const [token, setToken] = useState("");

  const load = useCallback(async () => {
    const r = await ops<{ cases: Row[] }>("/ops/v1/cases");
    if (r.status === 401) return setNeedsSignIn(true);
    setNeedsSignIn(false);
    setRows(r.data.cases ?? []);
  }, []);

  useEffect(() => {
    load();
    const t = setInterval(load, 15000);
    return () => clearInterval(t);
  }, [load]);

  if (needsSignIn) {
    return (
      <Shell>
        <p className="eyebrow">Trust &amp; Safety</p>
        <h1>Staff sign-in</h1>
        <p className="note">In production the console sits behind the company sign-in. This development build takes a staff token.</p>
        <form onSubmit={(e) => { e.preventDefault(); setStaffToken(token.trim()); load(); }}>
          <label htmlFor="staff-token">Staff token</label>
          <input id="staff-token" className="input" type="password" autoComplete="off" value={token} onChange={(e) => setToken(e.target.value)} />
          <button type="submit" className="button inline">Sign in</button>
        </form>
      </Shell>
    );
  }
  if (!rows) return <Shell><p>Loading…</p></Shell>;
  return (
    <Shell wide>
      <div className="op-bar">
        <div>
          <p className="eyebrow">Trust &amp; Safety</p>
          <h1>Case queue</h1>
        </div>
        {staffToken() && <button type="button" className="ms-block" onClick={() => { setStaffToken(null); setRows(null); load(); }}>Sign out</button>}
      </div>
      <p className="note">Most urgent first. Reporters are never shown. Every action is logged.</p>
      {rows.length === 0 ? (
        <p className="pf-empty">Nothing waiting.</p>
      ) : (
        <table className="op-table" data-testid="queue">
          <thead><tr><th>Due</th><th>What</th><th>From</th><th>State</th></tr></thead>
          <tbody>
            {rows.map((c) => (
              <tr key={c.id} data-testid="case-row">
                <td className={c.overdue ? "op-due-late" : ""}>{dueText(c.due)}</td>
                <td><a href={`/ops/cases/${c.id}`}>{c.label}</a></td>
                <td>{c.source === "guardian" ? "On-device signal" : "Member report"}</td>
                <td>{c.appeal_pending ? "Appeal" : c.state === "pending_second" ? "Needs second moderator" : "Open"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </Shell>
  );
}
