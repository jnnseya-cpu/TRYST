"use client";

import { useEffect, useState } from "react";

/**
 * A button that stays disabled until the page's JavaScript has loaded. Server-rendered
 * buttons are visible before React attaches their handlers; a tap in that moment would
 * silently do nothing (found by the end-to-end tests on the sign-in page).
 */
export function Action({ onClick, children, className = "button", disabled = false }: {
  onClick: () => void;
  children: React.ReactNode;
  className?: string;
  disabled?: boolean;
}) {
  const [ready, setReady] = useState(false);
  useEffect(() => setReady(true), []);
  return (
    <button type="button" className={className} onClick={onClick} disabled={!ready || disabled} aria-busy={!ready || undefined}>
      {children}
    </button>
  );
}
