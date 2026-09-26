"use client";

import * as React from "react";
import Script from "next/script";

type TurnstileAPI = {
  render: (
    container: HTMLElement,
    options: {
      sitekey: string;
      theme?: "auto" | "light" | "dark";
      size?: "normal" | "compact" | "flexible";
      callback?: (token: string) => void;
      "expired-callback"?: () => void;
      "error-callback"?: () => void;
    },
  ) => string;
  reset: (widgetID?: string) => void;
  remove?: (widgetID: string) => void;
};

declare global {
  interface Window {
    turnstile?: TurnstileAPI;
  }
}

type TurnstileWidgetProps = {
  siteKey: string;
  resetSignal: number;
  onTokenChange: (token: string) => void;
};

export function TurnstileWidget({ siteKey, resetSignal, onTokenChange }: TurnstileWidgetProps) {
  const containerRef = React.useRef<HTMLDivElement | null>(null);
  const widgetIDRef = React.useRef<string | null>(null);
  const onTokenChangeRef = React.useRef(onTokenChange);
  const lastResetSignalRef = React.useRef(resetSignal);
  const [scriptReady, setScriptReady] = React.useState(() => typeof window !== "undefined" && Boolean(window.turnstile));

  React.useEffect(() => {
    onTokenChangeRef.current = onTokenChange;
  }, [onTokenChange]);

  React.useEffect(() => {
    const container = containerRef.current;
    if (!siteKey || !scriptReady || !container || !window.turnstile || widgetIDRef.current) {
      return undefined;
    }

    widgetIDRef.current = window.turnstile.render(container, {
      sitekey: siteKey,
      theme: "auto",
      size: "flexible",
      callback: (token) => onTokenChangeRef.current(token),
      "expired-callback": () => onTokenChangeRef.current(""),
      "error-callback": () => onTokenChangeRef.current(""),
    });

    return () => {
      const widgetID = widgetIDRef.current;
      if (widgetID && window.turnstile?.remove) {
        window.turnstile.remove(widgetID);
      }
      widgetIDRef.current = null;
      onTokenChangeRef.current("");
    };
  }, [scriptReady, siteKey]);

  React.useEffect(() => {
    if (lastResetSignalRef.current === resetSignal) {
      return;
    }
    lastResetSignalRef.current = resetSignal;
    onTokenChangeRef.current("");
    if (widgetIDRef.current && window.turnstile) {
      window.turnstile.reset(widgetIDRef.current);
    }
  }, [resetSignal]);

  return (
    <div className="min-h-[65px] overflow-hidden rounded-md">
      <Script
        id="cloudflare-turnstile"
        src="https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit"
        strategy="afterInteractive"
        async
        defer
        onLoad={() => setScriptReady(true)}
        onReady={() => setScriptReady(true)}
      />
      <div ref={containerRef} />
    </div>
  );
}
