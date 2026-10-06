"use client";

import { Braces, CircleAlert } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type { JsonEditorController, JsonEditorTheme } from "@/shared/components/json-code-editor-runtime";
import { useTheme } from "@/shared/components/theme-provider";

export type JsonCodeEditorProps = {
  id?: string;
  value: string;
  placeholder?: string;
  disabled?: boolean;
  readOnly?: boolean;
  autoFocus?: boolean;
  height?: number | string;
  wordWrap?: "on" | "off";
  className?: string;
  actions?: React.ReactNode;
  showFormatAction?: boolean;
  onChange?: (value: string) => void;
};

type JsonEditorRuntime = typeof import("@/shared/components/json-code-editor-runtime");
type EditorStatus = "loading" | "ready" | "failed";

const BASE_EDITOR_FONT_SIZE = 12;
const AUTO_FOCUS_DELAY_MS = 50;

let runtimePromise: Promise<JsonEditorRuntime> | null = null;

// The editor runtime is a separate chunk shared by every JsonCodeEditor instance.
function loadJsonEditorRuntime(): Promise<JsonEditorRuntime> {
  runtimePromise ??= import("@/shared/components/json-code-editor-runtime").catch((error: unknown) => {
    // Let a later mount retry after a transient chunk-load failure.
    runtimePromise = null;
    throw error;
  });
  return runtimePromise;
}

function readUIFontScale() {
  if (typeof window === "undefined") {
    return 1;
  }

  const rawScale = window.getComputedStyle(document.documentElement).getPropertyValue("--ui-font-scale").trim();
  const scale = Number.parseFloat(rawScale);
  return Number.isFinite(scale) && scale > 0 ? scale : 1;
}

function getEditorFontSize() {
  return BASE_EDITOR_FONT_SIZE * readUIFontScale();
}

export function JsonCodeEditor({
  id,
  value,
  placeholder,
  disabled = false,
  readOnly = false,
  autoFocus = false,
  height = 220,
  wordWrap = "on",
  className,
  actions,
  showFormatAction = true,
  onChange,
}: JsonCodeEditorProps) {
  const t = useTranslations("common.jsonEditor");
  const { resolvedTheme } = useTheme();
  const containerRef = React.useRef<HTMLDivElement | null>(null);
  const editorRef = React.useRef<JsonEditorController | null>(null);
  const [status, setStatus] = React.useState<EditorStatus>("loading");
  const [diagnosticCount, setDiagnosticCount] = React.useState(0);
  const effectiveReadOnly = disabled || readOnly;
  const theme: JsonEditorTheme = resolvedTheme === "dark" ? "dark" : "light";

  // Latest props, read when the lazily loaded editor mounts and from editor callbacks,
  // so prop changes never recreate the editor.
  const latestPropsRef = React.useRef({ value, placeholder, readOnly: effectiveReadOnly, wordWrap, theme, autoFocus, onChange });
  React.useEffect(() => {
    latestPropsRef.current = { value, placeholder, readOnly: effectiveReadOnly, wordWrap, theme, autoFocus, onChange };
  });

  React.useEffect(() => {
    let disposed = false;

    loadJsonEditorRuntime()
      .then(({ createJsonEditor }) => {
        const parent = containerRef.current;
        if (disposed || !parent) {
          return;
        }
        const props = latestPropsRef.current;
        const editor = createJsonEditor({
          parent,
          value: props.value,
          placeholder: props.placeholder,
          readOnly: props.readOnly,
          wordWrap: props.wordWrap === "on",
          theme: props.theme,
          fontSize: getEditorFontSize(),
          onChange: (nextValue) => latestPropsRef.current.onChange?.(nextValue),
          // A controlled parent may reject or normalize edits; leaving the editor shows its value again.
          onBlur: () => editorRef.current?.setValue(latestPropsRef.current.value),
          onDiagnosticsChange: setDiagnosticCount,
        });
        editorRef.current = editor;
        setStatus("ready");

        if (props.autoFocus) {
          window.setTimeout(() => {
            if (!disposed) {
              editor.focus();
            }
          }, AUTO_FOCUS_DELAY_MS);
        }
      })
      .catch(() => {
        if (!disposed) {
          setStatus("failed");
        }
      });

    return () => {
      disposed = true;
      editorRef.current?.destroy();
      editorRef.current = null;
    };
  }, []);

  React.useEffect(() => {
    editorRef.current?.setValue(value);
  }, [value]);

  React.useEffect(() => {
    editorRef.current?.setReadOnly(effectiveReadOnly);
  }, [effectiveReadOnly]);

  React.useEffect(() => {
    editorRef.current?.setWordWrap(wordWrap === "on");
  }, [wordWrap]);

  React.useEffect(() => {
    editorRef.current?.setTheme(theme);
  }, [theme]);

  React.useEffect(() => {
    editorRef.current?.setPlaceholder(placeholder);
  }, [placeholder]);

  React.useEffect(() => {
    if (!autoFocus) {
      return;
    }
    const timer = window.setTimeout(() => {
      editorRef.current?.focus();
    }, AUTO_FOCUS_DELAY_MS);
    return () => window.clearTimeout(timer);
  }, [autoFocus]);

  React.useEffect(() => {
    function updateEditorFontSize() {
      editorRef.current?.setFontSize(getEditorFontSize());
    }

    const observer = new MutationObserver(updateEditorFontSize);
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-font-size"],
    });

    updateEditorFontSize();
    return () => observer.disconnect();
  }, []);

  const formatDocument = React.useCallback(() => {
    editorRef.current?.format();
  }, []);

  return (
    <div
      id={id}
      className={cn(
        "relative resize-y overflow-hidden rounded-md border border-input/40 bg-transparent text-xs shadow-none transition-[color,box-shadow] focus-within:border-ring/60 focus-within:ring-[1px] focus-within:ring-ring/40 dark:bg-input/30",
        disabled && "opacity-60",
        className,
      )}
      style={{ height }}
    >
      <div className="flex h-8 items-center justify-between gap-2 border-b border-input/30 pl-2.5 pr-1">
        <span className="inline-flex items-center gap-1.5 text-[11px] font-medium text-muted-foreground">
          <Braces className="size-3.5" aria-hidden="true" />
          JSON
        </span>
        <div className="flex min-w-0 items-center gap-1">
          {status === "ready" && diagnosticCount > 0 ? (
            <span className="inline-flex items-center gap-1 px-1 text-[11px] text-destructive" role="status">
              <CircleAlert className="size-3" aria-hidden="true" />
              {t("errors", { count: diagnosticCount })}
            </span>
          ) : null}
          {actions}
          {showFormatAction ? (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-6 px-2 text-[11px]"
              disabled={effectiveReadOnly || status !== "ready"}
              onClick={formatDocument}
            >
              {t("format")}
            </Button>
          ) : null}
        </div>
      </div>
      {status === "failed" ? (
        // Plain-text fallback keeps the field usable when the editor chunk cannot be loaded.
        <textarea
          value={value}
          placeholder={placeholder}
          readOnly={effectiveReadOnly}
          spellCheck={false}
          onChange={(event) => onChange?.(event.target.value)}
          className="block h-[calc(100%-2rem)] w-full resize-none bg-transparent px-3 py-2 font-mono text-xs leading-5 text-foreground outline-none placeholder:text-muted-foreground"
        />
      ) : (
        <div ref={containerRef} className="h-[calc(100%-2rem)] w-full" />
      )}
      {status === "loading" ? (
        <div className="absolute inset-x-0 bottom-0 top-8 flex items-center px-3 font-mono text-xs text-muted-foreground">
          {t("loading")}
        </div>
      ) : null}
    </div>
  );
}
