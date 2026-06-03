"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import type * as Monaco from "monaco-editor";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { useTheme } from "@/shared/components/theme-provider";

type JsonCodeEditorProps = {
  id?: string;
  value: string;
  placeholder?: string;
  disabled?: boolean;
  autoFocus?: boolean;
  height?: number | string;
  className?: string;
  actions?: React.ReactNode;
  onChange: (value: string) => void;
};

type MonacoModule = typeof Monaco;
type JsonDiagnosticsDefaults = {
  setDiagnosticsOptions: (options: {
    validate: boolean;
    allowComments: boolean;
    trailingCommas: "ignore" | "error";
  }) => void;
};

let monacoLoadPromise: Promise<MonacoModule> | null = null;
const BASE_EDITOR_FONT_SIZE = 12;

function isMonacoCanceledError(error: unknown): boolean {
  return error instanceof Error && (error.name === "Canceled" || error.message === "Canceled");
}

function disposeMonacoResource(resource: { dispose: () => void } | null | undefined) {
  if (!resource) {
    return;
  }
  try {
    resource.dispose();
  } catch (error) {
    if (!isMonacoCanceledError(error)) {
      throw error;
    }
  }
}

function readUIFontScale() {
  if (typeof window === "undefined") {
    return 1;
  }

  const rawScale = window
    .getComputedStyle(document.documentElement)
    .getPropertyValue("--ui-font-scale")
    .trim();
  const scale = Number.parseFloat(rawScale);
  return Number.isFinite(scale) && scale > 0 ? scale : 1;
}

function getEditorFontSize() {
  return BASE_EDITOR_FONT_SIZE * readUIFontScale();
}

function configureMonacoWorkers() {
  if (typeof window === "undefined") {
    return;
  }

  const browserGlobal = globalThis as typeof globalThis & {
    MonacoEnvironment?: {
      getWorker?: (workerID: string, label: string) => Worker;
    };
  };

  browserGlobal.MonacoEnvironment = {
    getWorker: (_workerID: string, label: string) => {
      if (label === "json") {
        return new Worker(
          new URL("monaco-editor/esm/vs/language/json/json.worker.js", import.meta.url),
          { type: "module" },
        );
      }

      return new Worker(
        new URL("monaco-editor/esm/vs/editor/editor.worker.js", import.meta.url),
        { type: "module" },
      );
    },
  };
}

function loadMonaco(): Promise<MonacoModule> {
  if (!monacoLoadPromise) {
    configureMonacoWorkers();
    monacoLoadPromise = import("monaco-editor");
  }
  return monacoLoadPromise;
}

export function JsonCodeEditor({
  id,
  value,
  placeholder,
  disabled = false,
  autoFocus = false,
  height = 220,
  className,
  actions,
  onChange,
}: JsonCodeEditorProps) {
  const t = useTranslations("common.jsonEditor");
  const { resolvedTheme } = useTheme();
  const containerRef = React.useRef<HTMLDivElement | null>(null);
  const editorRef = React.useRef<Monaco.editor.IStandaloneCodeEditor | null>(null);
  const monacoRef = React.useRef<MonacoModule | null>(null);
  const onChangeRef = React.useRef(onChange);
  const suppressChangeRef = React.useRef(false);
  const valueRef = React.useRef(value);
  const editorValueRef = React.useRef(value);
  const mountValueRef = React.useRef(value);
  const mountDisabledRef = React.useRef(disabled);
  const mountThemeRef = React.useRef(resolvedTheme);
  const mountAutoFocusRef = React.useRef(autoFocus);
  const [loading, setLoading] = React.useState(true);
  const [markerCount, setMarkerCount] = React.useState(0);

  React.useEffect(() => {
    onChangeRef.current = onChange;
  }, [onChange]);

  React.useEffect(() => {
    valueRef.current = value;
    mountValueRef.current = value;
  }, [value]);

  const syncEditorValue = React.useCallback((nextValue: string) => {
    const editor = editorRef.current;
    if (!editor || editorValueRef.current === nextValue) {
      return;
    }

    suppressChangeRef.current = true;
    try {
      const model = editor.getModel();
      if (model) {
        editor.executeEdits("json-code-editor.external-value", [{
          range: model.getFullModelRange(),
          text: nextValue,
          forceMoveMarkers: true,
        }]);
      } else {
        editor.setValue(nextValue);
      }
      editorValueRef.current = nextValue;
    } finally {
      suppressChangeRef.current = false;
    }
  }, []);

  React.useEffect(() => {
    mountDisabledRef.current = disabled;
  }, [disabled]);

  React.useEffect(() => {
    mountThemeRef.current = resolvedTheme;
  }, [resolvedTheme]);

  React.useEffect(() => {
    mountAutoFocusRef.current = autoFocus;
  }, [autoFocus]);

  React.useEffect(() => {
    let disposed = false;
    let contentSubscription: Monaco.IDisposable | null = null;
    let markerSubscription: Monaco.IDisposable | null = null;
    let blurSubscription: Monaco.IDisposable | null = null;

    async function mountEditor() {
      const monaco = await loadMonaco();
      if (disposed || !containerRef.current) {
        return;
      }

      monacoRef.current = monaco;
      const jsonDefaults = (monaco.languages as unknown as {
        json?: { jsonDefaults?: JsonDiagnosticsDefaults };
      }).json?.jsonDefaults;
      jsonDefaults?.setDiagnosticsOptions({
        validate: true,
        allowComments: true,
        trailingCommas: "ignore",
      });

      const editor = monaco.editor.create(containerRef.current, {
        value: mountValueRef.current,
        language: "json",
        readOnly: mountDisabledRef.current,
        theme: mountThemeRef.current === "dark" ? "vs-dark" : "vs",
        automaticLayout: true,
        bracketPairColorization: { enabled: true },
        contextmenu: true,
        detectIndentation: false,
        editContext: false,
        fixedOverflowWidgets: true,
        folding: true,
        fontFamily: "var(--font-mono), ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace",
        fontSize: getEditorFontSize(),
        lineDecorationsWidth: 8,
        lineNumbersMinChars: 3,
        minimap: { enabled: false },
        overviewRulerBorder: false,
        padding: { top: 8, bottom: 8 },
        renderLineHighlight: "line",
        renderWhitespace: "selection",
        scrollBeyondLastLine: false,
        scrollbar: {
          horizontalScrollbarSize: 8,
          verticalScrollbarSize: 8,
        },
        tabSize: 2,
        tabFocusMode: false,
        wordWrap: "on",
      });

      editorRef.current = editor;
      editorValueRef.current = editor.getValue();
      contentSubscription = editor.onDidChangeModelContent(() => {
        const nextValue = editor.getValue();
        editorValueRef.current = nextValue;
        if (suppressChangeRef.current) return;
        valueRef.current = nextValue;
        onChangeRef.current(nextValue);
      });
      blurSubscription = editor.onDidBlurEditorText(() => {
        syncEditorValue(valueRef.current);
      });
      markerSubscription = monaco.editor.onDidChangeMarkers((uris) => {
        const model = editor.getModel();
        if (!model || !uris.some((uri) => uri.toString() === model.uri.toString())) {
          return;
        }
        setMarkerCount(monaco.editor.getModelMarkers({ resource: model.uri }).length);
      });
      setLoading(false);

      if (mountAutoFocusRef.current) {
        setTimeout(() => {
          if (!disposed) {
            editor.focus();
          }
        }, 50);
      }
    }

    void mountEditor();

    return () => {
      disposed = true;
      disposeMonacoResource(contentSubscription);
      disposeMonacoResource(markerSubscription);
      disposeMonacoResource(blurSubscription);
      disposeMonacoResource(editorRef.current);
      editorRef.current = null;
      monacoRef.current = null;
    };
  }, [syncEditorValue]);

  React.useEffect(() => {
    const editor = editorRef.current;
    if (!editor || editorValueRef.current === value || editor.hasTextFocus()) {
      return;
    }
    syncEditorValue(value);
  }, [syncEditorValue, value]);

  React.useEffect(() => {
    editorRef.current?.updateOptions({ readOnly: disabled });
  }, [disabled]);

  React.useEffect(() => {
    const monaco = monacoRef.current;
    if (monaco) {
      monaco.editor.setTheme(resolvedTheme === "dark" ? "vs-dark" : "vs");
    }
  }, [resolvedTheme]);

  React.useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }

    function updateEditorFontSize() {
      editorRef.current?.updateOptions({ fontSize: getEditorFontSize() });
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
    const editor = editorRef.current;
    if (!editor) {
      return;
    }
    void editor.getAction("editor.action.formatDocument")?.run();
  }, []);

  return (
    <div
      id={id}
      className={cn(
        "relative overflow-hidden rounded-md border border-input bg-background text-xs shadow-sm focus-within:border-ring/60 focus-within:ring-[1px] focus-within:ring-ring/40",
        disabled && "opacity-60",
        className,
      )}
      style={{ height }}
    >
      <div className="flex h-8 items-center justify-between border-b bg-muted/30 px-2">
        <span className="font-mono text-[11px] text-muted-foreground">JSON</span>
        <div className="flex items-center gap-2">
          {!loading && markerCount > 0 ? (
            <span className="text-[11px] text-destructive">{t("errors", { count: markerCount })}</span>
          ) : null}
          {actions}
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-6 px-2 text-[11px]"
            disabled={disabled || loading}
            onClick={formatDocument}
          >
            {t("format")}
          </Button>
        </div>
      </div>
      <div ref={containerRef} className="h-[calc(100%-2rem)] w-full" />
      {loading ? (
        <div className="absolute inset-x-0 bottom-0 top-8 flex items-center px-3 font-mono text-xs text-muted-foreground">
          {t("loading")}
        </div>
      ) : null}
      {!loading && value.trim() === "" && placeholder ? (
        <div className="pointer-events-none absolute left-[58px] top-[39px] font-mono text-xs text-muted-foreground/70">
          {placeholder}
        </div>
      ) : null}
    </div>
  );
}
