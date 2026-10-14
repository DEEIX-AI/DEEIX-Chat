"use client";

import type { ResolvedNode, UIValue } from "@deeix/core";
import { formatTemplateNumber, valuesEqual } from "@deeix/core";
import * as React from "react";

import { cn } from "@/lib/utils";
import { isRecord } from "@/shared/lib/type-guards";

// What every node of one live block shares. Containers render their children
// through `renderNode`, so primitives and the block host do not import each
// other.
export type LiveBlockContextValue = {
  // Stable part of the ids handed to nested catalog components.
  blockKey: string;
  streaming: boolean;
  // The block's state after the user's changes; controls read their binding here.
  state: Readonly<Record<string, UIValue>>;
  // State plus computed values, for primitives that show a formula's inputs.
  values: Readonly<Record<string, UIValue>>;
  stateNames: ReadonlySet<string>;
  update: (patch: Record<string, UIValue>) => void;
  // The user changed something; the root panel then offers a reset.
  changed: boolean;
  reset: () => void;
  renderNode: (node: ResolvedNode) => React.ReactNode;
};

export const LiveBlockContext = React.createContext<LiveBlockContextValue | null>(null);

export function useLiveBlock(): LiveBlockContextValue {
  const value = React.useContext(LiveBlockContext);
  if (!value) {
    throw new Error("live block nodes must render inside a LiveBlock");
  }
  return value;
}

export function LiveChildren({ node }: { node: ResolvedNode }) {
  const { renderNode } = useLiveBlock();
  return node.children.map((child) => <React.Fragment key={child.path}>{renderNode(child)}</React.Fragment>);
}

// ---- Reading resolved props ----
// Props are model output after formula evaluation: anything may be missing or
// of the wrong type, and every reader falls back instead of throwing.

export function readText(props: Record<string, unknown>, key: string): string | undefined {
  const value = props[key];
  if (typeof value === "string") {
    return value;
  }
  return typeof value === "number" ? formatTemplateNumber(value) : undefined;
}

export function readNumber(props: Record<string, unknown>, key: string): number | undefined {
  const value = props[key];
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

export function readBoolean(props: Record<string, unknown>, key: string): boolean | undefined {
  const value = props[key];
  return typeof value === "boolean" ? value : undefined;
}

export function readEnum<T extends string>(props: Record<string, unknown>, key: string, allowed: readonly T[]): T | undefined {
  const value = props[key];
  return allowed.find((item) => item === value);
}

// How a resolved value reads as text: numbers grouped with at most two
// decimals, lists and objects as JSON, null as nothing.
export function displayValue(value: unknown): string {
  if (typeof value === "number") {
    return formatTemplateNumber(value);
  }
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "boolean") {
    return String(value);
  }
  if (value === null || value === undefined) {
    return "";
  }
  return JSON.stringify(value);
}

type LiveOption = { label: string; value: UIValue };

// Options written as plain values (`["a", "b"]`) or as `{label, value}`.
export function readOptions(props: Record<string, unknown>, key: string): LiveOption[] {
  const value = props[key];
  if (!Array.isArray(value)) {
    return [];
  }
  const options: LiveOption[] = [];
  for (const item of value) {
    if (isRecord(item)) {
      const optionValue = toOptionValue(item.value ?? item.label);
      if (optionValue !== undefined) {
        options.push({ label: displayValue(item.label ?? item.value), value: optionValue });
      }
      continue;
    }
    const optionValue = toOptionValue(item);
    if (optionValue !== undefined) {
      options.push({ label: displayValue(item), value: optionValue });
    }
  }
  return options;
}

function toOptionValue(value: unknown): UIValue | undefined {
  if (typeof value === "string" || typeof value === "boolean" || value === null) {
    return value;
  }
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

export function optionIndex(options: readonly LiveOption[], value: UIValue | undefined): number {
  return value === undefined ? -1 : options.findIndex((option) => valuesEqual(option.value, value));
}

export type LiveTone = "default" | "muted" | "positive" | "negative" | "warning" | "info";

export const LIVE_TONES: readonly LiveTone[] = ["default", "muted", "positive", "negative", "warning", "info"];

export const TONE_TEXT_CLASS: Record<LiveTone, string> = {
  default: "text-foreground",
  muted: "text-muted-foreground",
  positive: "text-green-600 dark:text-green-400",
  negative: "text-red-600 dark:text-red-400",
  warning: "text-amber-600 dark:text-amber-400",
  info: "text-sky-600 dark:text-sky-400",
};

// A pill toggle, shared by tabs and segmented controls.
export function ChipButton({ active, onClick, disabled, children }: { active: boolean; onClick: () => void; disabled?: boolean; children: React.ReactNode }) {
  return (
    <button
      type="button"
      role="tab"
      aria-selected={active}
      disabled={disabled}
      onClick={onClick}
      className={cn(
        "rounded-full border-[0.5px] px-3 py-1 text-xs transition-colors disabled:cursor-not-allowed disabled:opacity-50",
        active ? "border-primary/40 bg-primary/10 text-foreground" : "border-border text-muted-foreground hover:bg-accent hover:text-accent-foreground",
      )}
    >
      {children}
    </button>
  );
}
