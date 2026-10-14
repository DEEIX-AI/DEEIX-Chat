"use client";

import type { BlockIssue, BlockSource, ResolvedNode, UIValue } from "@deeix/core";
import {
  compileBlock,
  expandPointProps,
  isUIContainer,
  isUIPrimitive,
  mergeState,
  resolveBlock,
  stateDiff,
  takeWhere,
  WHERE_LIST_PROP,
} from "@deeix/core";
import { useTranslations } from "next-intl";
import * as React from "react";

import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { LIVE_COMPILE_OPTIONS, resolveLatestDefinition } from "../block";
import { useUIBlockRegistry } from "../registry";
import { UIBlockErrorBoundary } from "../ui-block-error-boundary";
import { deepEquals, type UIBlockState, UIBlockLocalState, useUIBlockStateChanges, useUIBlockStatePersistence } from "../ui-block-state";
import { LiveBlockContext, type LiveBlockContextValue, useLiveBlock } from "./live-context";
import { LivePrimitive } from "./live-primitives";

// A live block: state, computed values and a tree of primitives and catalog
// components, re-resolved on every change. The model writes data and formulas;
// everything here is interpretation, no model code runs.

const NO_OVERRIDES: UIBlockState = {};

const SHOW_ISSUES = process.env.NODE_ENV !== "production";

export function LiveBlock({ block, streaming }: { block: BlockSource; streaming: boolean }) {
  const compiled = React.useMemo(() => compileBlock(block, LIVE_COMPILE_OPTIONS), [block]);
  const title = typeof block.root.props.title === "string" ? block.root.props.title : undefined;
  const { state, update, changed, reset } = useLiveBlockState(block.key, compiled.initialState, title);
  const resolved = React.useMemo(() => resolveBlock(compiled, state), [compiled, state]);
  const stateNames = React.useMemo(() => new Set(Object.keys(compiled.initialState)), [compiled.initialState]);

  const context = React.useMemo<LiveBlockContextValue>(
    () => ({
      blockKey: block.key || "live",
      streaming,
      state,
      values: resolved.values,
      stateNames,
      update,
      changed,
      reset,
      renderNode: (node) => <LiveNode node={node} />,
    }),
    [block.key, changed, reset, resolved.values, state, stateNames, streaming, update],
  );

  const issues = SHOW_ISSUES ? [...compiled.issues, ...resolved.issues] : [];
  return (
    <LiveBlockContext.Provider value={context}>
      {resolved.root.component === "panel" ? <LiveNode node={resolved.root} /> : <div className="p-4"><LiveNode node={resolved.root} /></div>}
      {issues.length > 0 && !streaming ? <LiveIssues issues={issues} /> : null}
    </LiveBlockContext.Provider>
  );
}

// Only what the user changed is kept, as a diff against the declared state;
// it is merged over the latest declaration on every render, so a block that
// is still streaming in (or was edited) picks up new defaults.
function useLiveBlockState(key: string, initialState: Readonly<Record<string, UIValue>>, title: string | undefined) {
  const { initial, save } = useUIBlockStatePersistence<UIBlockState>(key, {
    defaults: NO_OVERRIDES,
    restore: (saved) => saved ?? NO_OVERRIDES,
    title,
  });
  const [overrides, setOverrides] = React.useState<UIBlockState>(initial);
  const changedRef = React.useRef(false);
  const initialRef = React.useRef(initialState);
  initialRef.current = initialState;
  // The same block open in another view (the side panel) changed it.
  useUIBlockStateChanges(key, (saved) => {
    const next = saved ?? NO_OVERRIDES;
    setOverrides((current) => (deepEquals(current, next) ? current : next));
  });

  React.useEffect(() => {
    if (changedRef.current) {
      save(overrides);
    }
  }, [overrides, save]);

  const state = React.useMemo(() => mergeState(initialState, overrides), [initialState, overrides]);
  const update = React.useCallback((patch: Record<string, UIValue>) => {
    changedRef.current = true;
    setOverrides((current) => {
      const declared = initialRef.current;
      return stateDiff(declared, mergeState(declared, { ...current, ...patch })) ?? NO_OVERRIDES;
    });
  }, []);
  const reset = React.useCallback(() => {
    changedRef.current = true;
    setOverrides(NO_OVERRIDES);
  }, []);
  // Overrides that no longer differ from a re-declared state do not count.
  const changed = React.useMemo(() => stateDiff(initialState, state) !== undefined, [initialState, state]);
  return { state, update, changed, reset };
}

// Each node is its own error boundary: a component that throws disappears
// alone. While the message streams, a leaf waits for its closing brace (a
// placeholder keeps the layout); containers show as soon as they open.
function LiveNode({ node }: { node: ResolvedNode }) {
  const { streaming } = useLiveBlock();
  if (streaming && !node.closed && !isUIContainer(node.component)) {
    return <Skeleton className="h-8 w-full rounded-md" />;
  }
  return (
    // Every resolution yields new node objects, so a node that threw is retried
    // as soon as the state or the streamed source changes.
    <UIBlockErrorBoundary resetKey={node} fallback={null}>
      {isUIPrimitive(node.component) ? <LivePrimitive node={node} /> : <LiveCatalogNode node={node} />}
    </UIBlockErrorBoundary>
  );
}

// A catalog component inside a live block: per-point formulas are expanded,
// `where` filters its list, then it validates and renders as usual. Its own
// state (sort, filters…) stays local: the block's state is what is stored.
function LiveCatalogNode({ node }: { node: ResolvedNode }) {
  const t = useTranslations("chat.markdown.uiBlock");
  const registry = useUIBlockRegistry();
  const { blockKey } = useLiveBlock();
  const definition = resolveLatestDefinition(registry, node.component);
  if (!definition) {
    return <LiveNotice text={t("unknownComponent", { component: node.component })} />;
  }
  // Per-point problems already show as gaps or a notice; they are not listed.
  const issues: BlockIssue[] = [];
  const { props, where } = takeWhere(node.component, expandPointProps(node.component, node.props, issues), issues);
  let finalProps = props;
  if (where) {
    const listProp = WHERE_LIST_PROP[node.component] ?? "";
    const list = props[listProp];
    const kept = Array.isArray(list) ? list.filter((row, index) => where(row, index)) : [];
    if (kept.length === 0) {
      return <LiveNotice text={t("empty")} />;
    }
    finalProps = { ...props, [listProp]: kept };
  }
  // Ids must stay valid CSS identifiers: charts use them in selectors.
  const rendered = definition.render(`${blockKey}-n${node.path.replaceAll(".", "-") || "root"}`, finalProps);
  if (!rendered.ok) {
    return <LiveNotice text={t("invalidProps", { component: node.component })} />;
  }
  return (
    <div className={cn("min-w-0", node.path !== "" && "overflow-hidden rounded-lg border-[0.5px] border-border bg-background")}>
      <UIBlockLocalState>{rendered.element}</UIBlockLocalState>
    </div>
  );
}

function LiveNotice({ text }: { text: string }) {
  return <p className="rounded-lg border-[0.5px] border-dashed border-border px-3 py-2 text-xs text-muted-foreground">{text}</p>;
}

// Development aid: what the engine dropped or could not compute.
function LiveIssues({ issues }: { issues: readonly BlockIssue[] }) {
  const t = useTranslations("chat.markdown.uiBlock");
  return (
    <details className="border-t-[0.5px] border-border px-4 py-2 text-[11px] text-muted-foreground">
      <summary className="cursor-pointer select-none">{t("formulaIssues", { count: issues.length })}</summary>
      <ul className="mt-1 font-mono leading-5">
        {issues.slice(0, 12).map((issue, index) => (
          <li key={`${issue.path}-${index}`}>
            {issue.path || "$"}: {issue.message}
          </li>
        ))}
      </ul>
    </details>
  );
}
