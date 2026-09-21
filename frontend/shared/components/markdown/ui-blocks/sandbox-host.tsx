"use client";

import * as React from "react";

import { useTheme } from "@/shared/components/theme-provider";
import { captureHTMLVisualThemeSnapshot, type HTMLVisualThemeSnapshot } from "@/shared/lib/html-visual-theme";
import { SANDBOX_IFRAME_PERMISSIONS, sandboxDocumentHead } from "@/shared/lib/sandbox-document";
import type { UIBlockRenderProps } from "./block";

const HOST_MESSAGE_SOURCE = "deeix-ui-block-host";
const FRAME_MESSAGE_SOURCE = "deeix-ui-block-frame";
const MIN_FRAME_HEIGHT = 96;
const MAX_FRAME_HEIGHT = 720;

type HostMessage =
  | { source: typeof HOST_MESSAGE_SOURCE; type: "props"; props: unknown }
  | { source: typeof HOST_MESSAGE_SOURCE; type: "theme"; variables: HTMLVisualThemeSnapshot["variables"]; colorScheme: string };

type FrameMessage = { source: typeof FRAME_MESSAGE_SOURCE; type: "ready" } | { source: typeof FRAME_MESSAGE_SOURCE; type: "resize"; height: number };

// Injected before the component source. Exposes window.deeix with the P1 subset
// (props and theme in, resize out). emit/setState arrive with actions in P2.
function runtimeScript(): string {
  return `<script>
(() => {
  const HOST = ${JSON.stringify(HOST_MESSAGE_SOURCE)};
  const FRAME = ${JSON.stringify(FRAME_MESSAGE_SOURCE)};
  const propsListeners = new Set();
  const themeListeners = new Set();
  let currentProps;
  let currentTheme;
  const post = (message) => window.parent.postMessage({ source: FRAME, ...message }, "*");
  window.addEventListener("message", (event) => {
    const data = event.data;
    if (!data || data.source !== HOST) return;
    if (data.type === "props") {
      currentProps = data.props;
      for (const listener of propsListeners) listener(currentProps);
    } else if (data.type === "theme") {
      currentTheme = data;
      const root = document.documentElement;
      root.style.colorScheme = data.colorScheme;
      for (const [name, value] of data.variables) root.style.setProperty(name, value);
      for (const listener of themeListeners) listener(Object.fromEntries(data.variables));
    }
  });
  const reportHeight = () => post({ type: "resize", height: Math.ceil(document.documentElement.scrollHeight) });
  new ResizeObserver(reportHeight).observe(document.documentElement);
  window.deeix = Object.freeze({
    onProps(listener) { propsListeners.add(listener); if (currentProps !== undefined) listener(currentProps); return () => propsListeners.delete(listener); },
    onTheme(listener) { themeListeners.add(listener); if (currentTheme) listener(Object.fromEntries(currentTheme.variables)); return () => themeListeners.delete(listener); },
  });
  window.addEventListener("DOMContentLoaded", () => { post({ type: "ready" }); reportHeight(); });
})();
</script>`;
}

// The source is the document body verbatim; srcdoc needs no escaping and the
// CSP in the head governs what it may do.
function sandboxDocument(source: string, theme: HTMLVisualThemeSnapshot): string {
  return `<!doctype html><html><head>${sandboxDocumentHead("Component", theme)}${runtimeScript()}</head><body>${source}</body></html>`;
}

export function SandboxComponent({ id, props, definition }: UIBlockRenderProps<unknown>) {
  const source = definition.sandbox?.source ?? "";
  const title = definition.sandbox?.title ?? definition.name;
  const { resolvedTheme } = useTheme();
  const frameRef = React.useRef<HTMLIFrameElement | null>(null);
  const [ready, setReady] = React.useState(false);
  const [height, setHeight] = React.useState(MIN_FRAME_HEIGHT);
  const [theme, setTheme] = React.useState<HTMLVisualThemeSnapshot | null>(null);
  // srcdoc is built once per source with the first theme captured after mount
  // (getComputedStyle must not run during render); later theme changes go over
  // postMessage so a toggle does not remount the component.
  const [initialTheme, setInitialTheme] = React.useState<HTMLVisualThemeSnapshot | null>(null);

  React.useEffect(() => {
    const snapshot = captureHTMLVisualThemeSnapshot(resolvedTheme);
    setInitialTheme((current) => current ?? snapshot);
    setTheme(snapshot);
  }, [resolvedTheme]);

  const documentHTML = React.useMemo(
    () => (initialTheme ? sandboxDocument(source, initialTheme) : ""),
    [initialTheme, source],
  );

  const post = React.useCallback((message: HostMessage) => {
    frameRef.current?.contentWindow?.postMessage(message, "*");
  }, []);

  React.useEffect(() => {
    const handle = (event: MessageEvent<FrameMessage>) => {
      if (event.source !== frameRef.current?.contentWindow) {
        return;
      }
      const data = event.data;
      if (!data || data.source !== FRAME_MESSAGE_SOURCE) {
        return;
      }
      if (data.type === "ready") {
        setReady(true);
      } else if (data.type === "resize" && Number.isFinite(data.height)) {
        setHeight(Math.min(MAX_FRAME_HEIGHT, Math.max(MIN_FRAME_HEIGHT, data.height)));
      }
    };
    window.addEventListener("message", handle);
    return () => window.removeEventListener("message", handle);
  }, []);

  React.useEffect(() => {
    if (ready) {
      post({ source: HOST_MESSAGE_SOURCE, type: "props", props });
    }
  }, [post, props, ready]);

  React.useEffect(() => {
    if (ready && theme) {
      post({ source: HOST_MESSAGE_SOURCE, type: "theme", variables: theme.variables, colorScheme: theme.colorScheme });
    }
  }, [post, ready, theme]);

  return (
    <iframe
      ref={frameRef}
      title={title}
      allow={SANDBOX_IFRAME_PERMISSIONS}
      sandbox="allow-scripts"
      referrerPolicy="no-referrer"
      srcDoc={documentHTML}
      style={{ height }}
      className="block w-full border-0 bg-transparent"
      data-ui-block-id={id}
    />
  );
}
