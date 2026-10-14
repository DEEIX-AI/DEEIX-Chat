"use client";

import { UI_WIDGET_MAX_BYTES, widgetKey, widgetTitle } from "@deeix/core";
import { Code, PanelRight, Sparkles } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

import { HeightTransition } from "@/components/ui/height-transition";
import { cn } from "@/lib/utils";
import { CollapsibleMotionContent } from "@/shared/components/collapsible-motion-content";
import { SandboxFrame } from "./sandbox-host";
import { useUIBlockActions, useUIBlockOpen } from "./ui-block-actions";
import { UIBlockAsk } from "./ui-block-ask";
import { UIBlockFrame } from "./ui-block-frame";

// An inline illustration (`deeix-widget`): HTML, SVG and script the model wrote
// for something the block format cannot express (animation, 3D sketches, flow
// fields). It runs in the same sandbox as Artifacts and custom components: no
// same origin, no network, no forms. It is rendered once its fence is complete;
// half-written script would only throw.

const WIDGET_MAX_HEIGHT = 900;

// Which illustrations of the message on screen are complete: a fence still
// being written has no closing line yet. Provided per message by the renderer.
const UIWidgetCompletionContext = React.createContext<ReadonlySet<string> | null>(null);

export function UIWidgetCompletionProvider({ complete, children }: { complete: ReadonlySet<string>; children: React.ReactNode }) {
  return <UIWidgetCompletionContext.Provider value={complete}>{children}</UIWidgetCompletionContext.Provider>;
}

// From a Markdown code block: complete when the message is done, or when the
// message scan found this illustration's closing fence.
export function MarkdownWidgetHost({ raw, streaming }: { raw: string; streaming: boolean }) {
  const completed = React.useContext(UIWidgetCompletionContext);
  const complete = !streaming || Boolean(completed?.has(widgetKey(raw)));
  return <UIWidgetHost raw={raw} complete={complete} streaming={streaming} />;
}

// The illustration's script picks the follow-up and may change it at any time.
// While the pointer or focus is on the composer, a change waits, so the text
// cannot be swapped between reading it and pressing send.
function useWidgetFollowUp() {
  const [suggestion, setSuggestion] = React.useState("");
  const latestRef = React.useRef("");
  const hoveredRef = React.useRef(false);
  const focusedRef = React.useRef(false);
  const suggest = React.useCallback((text: string) => {
    latestRef.current = text;
    if (!hoveredRef.current && !focusedRef.current) {
      setSuggestion(text);
    }
  }, []);
  const release = () => {
    if (!hoveredRef.current && !focusedRef.current) {
      setSuggestion(latestRef.current);
    }
  };
  const holdProps = {
    onPointerEnter: () => {
      hoveredRef.current = true;
    },
    onPointerLeave: () => {
      hoveredRef.current = false;
      release();
    },
    onFocus: () => {
      focusedRef.current = true;
    },
    onBlur: (event: React.FocusEvent<HTMLDivElement>) => {
      if (!event.currentTarget.contains(event.relatedTarget)) {
        focusedRef.current = false;
        release();
      }
    },
  };
  return { suggestion, suggest, holdProps };
}

export function UIWidgetHost({ raw, complete, streaming = false }: { raw: string; complete: boolean; streaming?: boolean }) {
  const t = useTranslations("chat.markdown.uiBlock");
  const open = useUIBlockOpen();
  const actions = useUIBlockActions();
  const followUp = useWidgetFollowUp();
  const [sourceOpen, setSourceOpen] = React.useState(false);
  const sourceID = React.useId();
  const source = raw.trim();
  const key = widgetKey(source);
  const tooLarge = new TextEncoder().encode(source).length > UI_WIDGET_MAX_BYTES;

  if (!complete) {
    // Same loading surface as a Mermaid diagram, inside the frame the
    // illustration will occupy. `<title>` comes first, so it can be named early.
    const pendingTitle = widgetTitle(source);
    return (
      <HeightTransition className="my-3">
        <UIBlockFrame>
          <div role="status" aria-live="polite" className="relative flex h-40 items-center justify-center overflow-hidden bg-muted/30 px-6">
            <span
              aria-hidden="true"
              className="pointer-events-none absolute inset-0 [background-image:radial-gradient(var(--border)_1px,transparent_1px)] [background-size:14px_14px] [mask-image:radial-gradient(ellipse_at_center,black_20%,transparent_75%)]"
            />
            <span className="relative inline-flex min-w-0 items-center gap-1.5 text-[12px] font-medium text-muted-foreground">
              <Sparkles aria-hidden="true" className="size-3.5 shrink-0" strokeWidth={1.8} />
              <span className="shimmer truncate">
                {pendingTitle ? t("widgetGeneratingTitled", { title: pendingTitle }) : t("widgetGenerating")}
              </span>
            </span>
          </div>
        </UIBlockFrame>
      </HeightTransition>
    );
  }

  const sourceView = (
    <div className="max-h-80 overflow-auto px-4 py-3">
      <pre className="w-max min-w-full whitespace-pre font-mono text-[11px] leading-5 text-foreground/90">{source}</pre>
    </div>
  );
  if (tooLarge) {
    return (
      <div className="my-3">
        <UIBlockFrame>
          <p className="border-b-[0.5px] border-border px-4 py-2.5 text-xs text-muted-foreground">{t("widgetTooLarge")}</p>
          {sourceView}
        </UIBlockFrame>
      </div>
    );
  }

  const unavailable = (
    <div>
      <p className="border-b-[0.5px] border-border px-4 py-2.5 text-xs text-muted-foreground">{t("widgetUnavailable")}</p>
      {sourceView}
    </div>
  );
  return (
    <div className="group/ui-block my-3">
      <UIBlockFrame>
        {/* The title also names the illustration's state for the model on the next turn. */}
        <SandboxFrame
          id={key}
          source={source}
          title={widgetTitle(source) || t("widgetTitle")}
          maxHeight={WIDGET_MAX_HEIGHT}
          widget
          fallback={unavailable}
          onSuggest={followUp.suggest}
        />
        {/* The follow-up is the host's own composer, as under a block's `ask`: the
            frame only proposes the text, and a send is a click the host saw. */}
        {followUp.suggestion ? (
          <div className="px-4 pb-4" {...followUp.holdProps}>
            <UIBlockAsk suggestion={followUp.suggestion} busy={Boolean(actions.busy) || streaming} send={actions.ask} />
          </div>
        ) : null}
      </UIBlockFrame>
      <div className="mt-1 flex justify-end gap-1">
        <WidgetAction onClick={() => setSourceOpen((value) => !value)} ariaExpanded={sourceOpen} ariaControls={sourceID}>
          <Code className="size-3" strokeWidth={1.8} />
          {sourceOpen ? t("hideSource") : t("viewSource")}
        </WidgetAction>
        {open ? (
          <WidgetAction onClick={() => open(key)}>
            <PanelRight className="size-3" strokeWidth={1.8} />
            {t("openInPanel")}
          </WidgetAction>
        ) : null}
      </div>
      <CollapsibleMotionContent id={sourceID} open={sourceOpen}>
        <UIBlockFrame className="mt-1">{sourceView}</UIBlockFrame>
      </CollapsibleMotionContent>
    </div>
  );
}

// Under the block, appearing on hover on pointer devices (always shown on touch).
function WidgetAction({
  onClick,
  ariaExpanded,
  ariaControls,
  children,
}: {
  onClick: () => void;
  ariaExpanded?: boolean;
  ariaControls?: string;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-expanded={ariaExpanded}
      aria-controls={ariaControls}
      className={cn(
        "inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-[11px] text-muted-foreground transition-[opacity,color,background-color] hover:bg-accent hover:text-accent-foreground focus-visible:opacity-100",
        !ariaExpanded && "[@media(hover:hover)]:opacity-0 [@media(hover:hover)]:group-hover/ui-block:opacity-100",
      )}
    >
      {children}
    </button>
  );
}
