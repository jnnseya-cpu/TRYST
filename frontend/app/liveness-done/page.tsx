"use client";

import { useEffect } from "react";
import { Shell } from "@/components/Shell";
import { LIVENESS_CHANNEL } from "@/lib/liveness";

/** Where the provider's hosted face check ends: tells the TRYST tab, then closes. */
export default function LivenessDone() {
  useEffect(() => {
    if (typeof BroadcastChannel !== "undefined") {
      const ch = new BroadcastChannel(LIVENESS_CHANNEL);
      ch.postMessage("done");
      ch.close();
    }
    window.close();
  }, []);
  return (
    <Shell>
      <h1>Live check finished</h1>
      <p className="lede" data-testid="liveness-done">You can close this tab and go back to TRYST to finish.</p>
    </Shell>
  );
}
