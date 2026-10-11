"use client";

import { useEffect, useState } from "react";
import { Shell } from "@/components/Shell";

type Proof = { deletion_job_id: string; completed_at: string; receipts: { part: string; erased: number; retained?: string[] }[]; signature: string };

/** Shown after deletion: the signed completion proof, which names no one. */
export default function Deleted() {
  const [job, setJob] = useState("");
  const [proof, setProof] = useState<Proof | null>(null);
  const [pending, setPending] = useState(false);

  useEffect(() => {
    const id = new URLSearchParams(window.location.search).get("job") ?? "";
    setJob(id);
    if (!id) return;
    let tries = 0;
    const load = async () => {
      const r = await fetch(`/v1/deletion-proofs/${encodeURIComponent(id)}`, { cache: "no-store" });
      if (r.ok) return setProof(await r.json());
      setPending(true);
      if (++tries < 20) setTimeout(load, 3000);
    };
    load();
  }, []);

  const retained = proof?.receipts.flatMap((r) => r.retained ?? []) ?? [];
  return (
    <Shell>
      <p className="eyebrow">Account deleted</p>
      <h1>{proof ? "Your account is gone" : "Deleting your account"}</h1>
      {proof ? (
        <>
          <p>We destroyed the key that protected your data and removed everything else. Reference <span data-testid="deletion-ref">{job}</span>, completed {new Date(proof.completed_at).toLocaleString()}.</p>
          {retained.length > 0 && (
            <>
              <p>What we had to keep, and why:</p>
              <ul>{retained.map((r) => <li key={r}>{r}</li>)}</ul>
            </>
          )}
          <a className="button ghost" href={`data:application/json,${encodeURIComponent(JSON.stringify(proof, null, 2))}`} download="deletion-proof.json">Save the signed proof</a>
        </>
      ) : (
        <p role="status">{pending ? "Finishing up. This page updates on its own." : "One moment…"}</p>
      )}
      <a className="button" href="/">Close</a>
    </Shell>
  );
}
