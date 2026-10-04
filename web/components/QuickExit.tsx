"use client";

import { useEffect, useRef } from "react";
import { quickExit } from "@/lib/quickExit";

/** Always-visible exit; pressing Escape twice also leaves (FR-029). */
export function QuickExit() {
  const last = useRef(0);
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      const now = Date.now();
      if (now - last.current < 600) quickExit(window);
      last.current = now;
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);
  return (
    <button type="button" className="quick-exit" onClick={() => quickExit(window)} aria-label="Leave this site now">
      Leave
    </button>
  );
}
