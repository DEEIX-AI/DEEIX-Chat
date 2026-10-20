"use client";

import { BLOCK_LIMITS } from "@deeix/core";
import * as React from "react";

import { useTheme } from "@/shared/components/theme-provider";
import { captureHTMLVisualThemeSnapshot, type HTMLVisualThemeSnapshot } from "@/shared/lib/html-visual-theme";
import { SANDBOX_IFRAME_PERMISSIONS, sandboxDocumentHead } from "@/shared/lib/sandbox-document";
import { isFiniteNumber, isRecord } from "@/shared/lib/type-guards";
import type { UIBlockRenderProps } from "./block";
import { useUIBlockActions } from "./ui-block-actions";
import {
  deepEquals,
  isUIBlockState,
  sanitizeUIBlockState,
  type UIBlockState,
  useUIBlockStateChanges,
  useUIBlockStatePersistence,
} from "./ui-block-state";

const HOST_MESSAGE_SOURCE = "deeix-ui-block-host";
const FRAME_MESSAGE_SOURCE = "deeix-ui-block-frame";
const MIN_FRAME_HEIGHT = 96;
const MAX_FRAME_HEIGHT = 720;
// The frame cannot prove a click came from the user, so `ask` only fills the
// composer, at most once a second, and never replaces what the user typed.
const ASK_INTERVAL_MS = 1000;
const ASK_MAX_LENGTH = 2000;

const NO_STATE: UIBlockState = {};

type HostMessage =
  | { source: typeof HOST_MESSAGE_SOURCE; type: "props"; props: unknown }
  | { source: typeof HOST_MESSAGE_SOURCE; type: "state"; state: UIBlockState }
  | { source: typeof HOST_MESSAGE_SOURCE; type: "theme"; variables: HTMLVisualThemeSnapshot["variables"]; colorScheme: string };

// Frame → host messages: { source, type: "ready" }, { type: "resize", height },
// { type: "state", state } (the whole state after a setState), { type: "ask", text }
// and { type: "suggest", text }.

// Injected before the component source. window.deeix:
//   onProps(fn) / onTheme(fn)   props and theme from the host
//   getState() / onState(fn)    the block's stored state (restored on reload)
//   setState(patch)             merge into the state; a null value removes a key
//   ask(text)                   put text in the composer for the user to send
//   suggest(text)               illustrations: the follow-up the host shows under
//                               the frame ("" removes it)
function runtimeScript(): string {
  return `<script>
(() => {
  const HOST = ${JSON.stringify(HOST_MESSAGE_SOURCE)};
  const FRAME = ${JSON.stringify(FRAME_MESSAGE_SOURCE)};
  const propsListeners = new Set();
  const themeListeners = new Set();
  const stateListeners = new Set();
  let currentProps;
  let currentTheme;
  let currentState = {};
  const post = (message) => window.parent.postMessage({ source: FRAME, ...message }, "*");
  window.addEventListener("message", (event) => {
    const data = event.data;
    if (!data || data.source !== HOST) return;
    if (data.type === "props") {
      currentProps = data.props;
      for (const listener of propsListeners) listener(currentProps);
    } else if (data.type === "state") {
      currentState = data.state && typeof data.state === "object" ? data.state : {};
      for (const listener of stateListeners) listener(currentState);
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
    getState() { return currentState; },
    onState(listener) { stateListeners.add(listener); listener(currentState); return () => stateListeners.delete(listener); },
    setState(patch) {
      if (!patch || typeof patch !== "object" || Array.isArray(patch)) return;
      const next = { ...currentState };
      for (const [key, value] of Object.entries(patch)) {
        if (value === null || value === undefined) delete next[key]; else next[key] = value;
      }
      currentState = next;
      post({ type: "state", state: next });
    },
    ask(text) { if (typeof text === "string" && text.trim()) post({ type: "ask", text }); },
    suggest(text) { post({ type: "suggest", text: typeof text === "string" ? text : "" }); },
  });
  window.addEventListener("DOMContentLoaded", () => { post({ type: "ready" }); reportHeight(); });
})();
</script>`;
}

// Inline frames sit on the block's card: no page background of their own.
const TRANSPARENT_STYLE = "<style>html, body { background: transparent; }</style>";

// Defaults for illustrations, so model-written markup looks like the app
// without restyling every control: an inset from the card's edge, the app's
// type, and theme-coloured buttons, selects, inputs and tables. `:where()` keeps
// them at zero specificity, so anything the illustration sets wins. Classes:
// .controls (a row of controls), .num (digits that do not shift as they
// change), .muted, .ask (for illustrations that draw their own follow-up) and
// .stable: every variant of a changing text stacked in one cell, only the one
// marked .on visible, so the cell is as tall as the longest and nothing below
// it moves.
// The frame is already a card, so groups inside it are .card tiles (a muted
// fill, no border; a selected one is lifted, not painted in --primary), and
// .row / .stack / .grid lay them out. .label / .value pair a caption with a
// figure, .badge and .chip are the app's pills, and --positive / --negative /
// --warning are the good/bad tones: the theme has none, and chart colours change
// hue per theme. They mix toward the foreground, so they darken in light mode
// and lighten in dark mode like the stat cards' green and red.
const WIDGET_BASE_STYLE = `<style data-deeix-widget-base>
:where(:root) { --positive: color-mix(in oklab, #16a34a 78%, var(--foreground)); --negative: color-mix(in oklab, #dc2626 80%, var(--foreground)); --warning: color-mix(in oklab, #d97706 80%, var(--foreground)); scrollbar-width: thin; scrollbar-color: color-mix(in oklab, var(--foreground) 22%, transparent) transparent; }
:where(body) { padding: 16px; font: 13px/1.6 var(--font-sans); color: var(--foreground); }
:where(h1, h2, h3, h4) { margin: 0 0 4px; font-size: 15px; line-height: 1.4; font-weight: 600; }
:where(h4) { margin: 0 0 8px; font-size: 13px; }
:where(p) { margin: 0 0 8px; }
:where(hr) { margin: 16px 0; border: 0; border-top: 1px solid var(--border); }
:where(button, select, input, textarea) { font: inherit; font-size: 12px; color: inherit; }
:where(button) { height: 28px; padding: 0 12px; border: 1px solid var(--border); border-radius: 8px; background: var(--card); cursor: pointer; white-space: nowrap; transition: background-color .15s, border-color .15s; }
:where(button:hover) { background: var(--muted); }
:where(button[aria-pressed="true"], button.primary) { background: var(--primary); border-color: var(--primary); color: var(--primary-foreground); }
:where(button:disabled) { opacity: .5; cursor: not-allowed; }
:where(select) { height: 28px; padding: 0 8px; border: 1px solid var(--border); border-radius: 8px; background: var(--card); }
:where(input[type="text"], input[type="number"]) { height: 28px; padding: 0 8px; border: 1px solid var(--border); border-radius: 8px; background: var(--card); }
:where(input.num) { width: 96px; text-align: right; }
:where(input[type="range"], input[type="checkbox"], input[type="radio"]) { accent-color: var(--primary); }
:where(label) { color: var(--muted-foreground); font-size: 12px; }
:where(:focus-visible) { outline: 2px solid color-mix(in oklch, var(--ring) 45%, transparent); outline-offset: 2px; border-radius: 8px; }
:where(svg, canvas, img) { max-width: 100%; }
:where(table) { width: 100%; border-collapse: collapse; }
:where(th) { padding: 6px 8px; border-bottom: 1px solid var(--border); color: var(--muted-foreground); font-size: 12px; font-weight: 500; text-align: left; }
:where(td) { padding: 7px 8px; border-bottom: 1px solid color-mix(in oklab, var(--border) 55%, transparent); }
:where(th.num, td.num) { text-align: right; }
:where(tfoot td) { border-bottom: 0; font-weight: 600; }
:where(.controls, .row) { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 12px; }
:where(.stack) { display: flex; flex-direction: column; gap: 16px; }
:where(.grid) { display: grid; grid-template-columns: repeat(auto-fit, minmax(128px, 1fr)); gap: 8px; }
:where(.card) { display: block; padding: 10px 12px; border: 1px solid transparent; border-radius: 10px; background: color-mix(in oklab, var(--muted) 65%, transparent); }
:where(button.card) { width: 100%; height: auto; font-size: 13px; text-align: left; white-space: normal; color: inherit; }
:where(button.card:hover) { border-color: var(--border); background: color-mix(in oklab, var(--muted) 65%, transparent); }
:where(.card[aria-pressed="true"], .card.on) { border-color: color-mix(in oklab, var(--foreground) 22%, transparent); background: var(--card); color: inherit; box-shadow: var(--shadow-sm); }
:where(.label) { display: block; color: var(--muted-foreground); font-size: 12px; }
:where(.value) { display: block; font-size: 20px; font-weight: 600; line-height: 1.35; font-variant-numeric: tabular-nums; }
:where(.badge) { display: inline-flex; align-items: center; gap: 4px; height: 22px; padding: 0 8px; border-radius: 999px; background: var(--muted); color: var(--muted-foreground); font-size: 11px; font-weight: 500; white-space: nowrap; }
:where(button.chip) { height: 26px; padding: 0 10px; border-radius: 999px; background: transparent; color: var(--muted-foreground); }
:where(button.chip[aria-pressed="true"]) { border-color: color-mix(in oklab, var(--primary) 45%, transparent); background: color-mix(in oklab, var(--primary) 12%, transparent); color: var(--foreground); }
:where(.positive) { color: var(--positive); }
:where(.negative) { color: var(--negative); }
:where(.warning) { color: var(--warning); }
:where(.badge.positive) { background: color-mix(in oklab, var(--positive) 12%, transparent); }
:where(.badge.negative) { background: color-mix(in oklab, var(--negative) 12%, transparent); }
:where(.badge.warning) { background: color-mix(in oklab, var(--warning) 14%, transparent); }
:where(.num) { font-variant-numeric: tabular-nums; }
:where(.muted) { color: var(--muted-foreground); }
:where(button.ask) { display: flex; align-items: center; width: 100%; height: auto; min-height: 36px; padding: 8px 12px; border-radius: 10px; background: var(--muted); text-align: left; white-space: normal; }
:where(button.ask:hover) { border-color: var(--primary); }
:where(button.ask)::after { content: "→"; margin-left: auto; padding-left: 12px; color: var(--muted-foreground); }
:where(.stable) { display: grid; }
:where(.stable > *) { grid-area: 1 / 1; }
:where(.stable > :not(.on)) { visibility: hidden; }
</style>`;

// Illustrations often wrap everything in a box that caps its height and scrolls
// (max-height + overflow: auto). The frame already grows with the content and,
// past its own cap, scrolls the document right at its edge; the capped box adds a
// second scrollbar inset by the body padding and clips what is drawn just outside
// its children (selection rings, focus outlines). Only top-level boxes are
// released: a scrolling list deeper inside is the illustration's own design.
const WIDGET_LAYOUT_SCRIPT = `<script>
(() => {
  const release = () => {
    for (const element of document.body.children) {
      const style = getComputedStyle(element);
      if (style.maxHeight !== "none" && /auto|scroll/.test(style.overflowY)) {
        element.style.setProperty("max-height", "none", "important");
        element.style.setProperty("overflow", "visible", "important");
      }
    }
  };
  window.addEventListener("DOMContentLoaded", release);
  window.addEventListener("load", release);
})();
</script>`;

// The source is the document body verbatim; srcdoc needs no escaping and the
// CSP in the head governs what it may do.
function sandboxDocument(source: string, theme: HTMLVisualThemeSnapshot, widget: boolean): string {
  const base = widget ? WIDGET_BASE_STYLE + WIDGET_LAYOUT_SCRIPT : "";
  return `<!doctype html><html><head>${sandboxDocumentHead("Component", theme)}${TRANSPARENT_STYLE}${base}${runtimeScript()}</head><body>${source}</body></html>`;
}

// A frame whose runtime has not reported in by then cannot run scripts here
// (a host CSP that blocks inline scripts, as in the desktop shell).
const READY_TIMEOUT_MS = 5000;

export function SandboxComponent({ id, props, definition }: UIBlockRenderProps<unknown>) {
  return (
    <SandboxFrame
      id={id}
      source={definition.sandbox?.source ?? ""}
      title={isRecord(props) && typeof props.title === "string" ? props.title : (definition.sandbox?.title ?? definition.name)}
      props={props}
    />
  );
}

type SandboxFrameProps = {
  // Block key: state is stored under it.
  id: string;
  source: string;
  title: string;
  props?: unknown;
  maxHeight?: number;
  // An inline illustration: gets the default inset and control styles.
  widget?: boolean;
  // Shown instead of the frame when its scripts cannot run.
  fallback?: React.ReactNode;
  // Receives deeix.suggest: the follow-up the host renders itself. Without it
  // the call does nothing.
  onSuggest?: (text: string) => void;
};

// Model-written HTML and script in a sandboxed frame (no same origin, no
// network): props and theme in; size, state, ask and suggest out. Shared by custom
// components and inline illustrations.
export function SandboxFrame({ id, source, title, props, maxHeight = MAX_FRAME_HEIGHT, widget = false, fallback = null, onSuggest }: SandboxFrameProps) {
  const actions = useUIBlockActions();
  const actionsRef = React.useRef(actions);
  actionsRef.current = actions;
  const onSuggestRef = React.useRef(onSuggest);
  onSuggestRef.current = onSuggest;
  const lastAskRef = React.useRef(0);
  const frameStateRef = React.useRef<UIBlockState>(NO_STATE);
  // The component's own state, stored like any block's. It decides the shape;
  // the host only enforces the shared limits (names, depth, 4 KB).
  const { initial: initialState, save: saveState } = useUIBlockStatePersistence<UIBlockState>(id, {
    defaults: NO_STATE,
    restore: (saved) => saved ?? NO_STATE,
    title,
  });
  const { resolvedTheme } = useTheme();
  const frameRef = React.useRef<HTMLIFrameElement | null>(null);
  const [ready, setReady] = React.useState(false);
  const [unavailable, setUnavailable] = React.useState(false);
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
    () => (initialTheme ? sandboxDocument(source, initialTheme, widget) : ""),
    [initialTheme, source, widget],
  );

  const post = React.useCallback((message: HostMessage) => {
    frameRef.current?.contentWindow?.postMessage(message, "*");
  }, []);

  React.useEffect(() => {
    // The frame runs model-authored code, so its messages are narrowed field by field.
    const handle = (event: MessageEvent<unknown>) => {
      if (event.source !== frameRef.current?.contentWindow) {
        return;
      }
      const data = event.data;
      if (!isRecord(data) || data.source !== FRAME_MESSAGE_SOURCE) {
        return;
      }
      if (data.type === "ready") {
        setReady(true);
      } else if (data.type === "resize" && isFiniteNumber(data.height)) {
        setHeight(Math.min(maxHeight, Math.max(MIN_FRAME_HEIGHT, data.height)));
      } else if (data.type === "state" && isUIBlockState(data.state)) {
        const next = sanitizeUIBlockState(data.state) ?? NO_STATE;
        frameStateRef.current = next;
        saveState(next);
      } else if (data.type === "ask" && typeof data.text === "string") {
        const now = Date.now();
        const text = data.text.trim().slice(0, ASK_MAX_LENGTH);
        if (text && now - lastAskRef.current >= ASK_INTERVAL_MS) {
          lastAskRef.current = now;
          actionsRef.current.prefill?.(text);
        }
      } else if (data.type === "suggest" && typeof data.text === "string") {
        // Same length as an `ask` in a block: a suggestion is one question.
        onSuggestRef.current?.(data.text.trim().slice(0, BLOCK_LIMITS.askLength));
      }
    };
    window.addEventListener("message", handle);
    return () => window.removeEventListener("message", handle);
  }, [maxHeight, saveState]);

  React.useEffect(() => {
    if (!documentHTML || ready) {
      return undefined;
    }
    const timer = window.setTimeout(() => setUnavailable(true), READY_TIMEOUT_MS);
    return () => window.clearTimeout(timer);
  }, [documentHTML, ready]);

  // Another view of the same block changed its state: pass it in, unless it is
  // what this frame just wrote.
  useUIBlockStateChanges(id, (saved) => {
    const next = saved ?? NO_STATE;
    if (ready && !deepEquals(next, frameStateRef.current)) {
      frameStateRef.current = next;
      post({ source: HOST_MESSAGE_SOURCE, type: "state", state: next });
    }
  });

  // Restored state goes in once the frame is listening, before props; later state is the
  // frame's own, so it is not echoed back.
  React.useEffect(() => {
    if (ready) {
      frameStateRef.current = initialState;
      post({ source: HOST_MESSAGE_SOURCE, type: "state", state: initialState });
    }
  }, [initialState, post, ready]);

  React.useEffect(() => {
    if (ready && props !== undefined) {
      post({ source: HOST_MESSAGE_SOURCE, type: "props", props });
    }
  }, [post, props, ready]);

  React.useEffect(() => {
    if (ready && theme) {
      post({ source: HOST_MESSAGE_SOURCE, type: "theme", variables: theme.variables, colorScheme: theme.colorScheme });
    }
  }, [post, ready, theme]);

  if (unavailable && !ready && fallback) {
    return fallback;
  }
  // The frame is created only once its document exists. Chrome can ignore a
  // srcdoc that changes while the frame's first (empty) document is loading,
  // which left the frame blank; setting it once at creation avoids that.
  if (!documentHTML) {
    return <div style={{ height }} aria-hidden="true" />;
  }
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
