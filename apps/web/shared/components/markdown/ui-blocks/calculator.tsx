"use client";

import { RotateCcw } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

import { Skeleton } from "@/components/ui/skeleton";
import { Slider } from "@/components/ui/slider";
import { cn } from "@/lib/utils";
import { type CompiledExpression, compileExpression } from "@/shared/lib/safe-expression";
import type { UIBlockDefinition, UIBlockRenderProps } from "./block";
import { s } from "./schema";
import { UIBlockFrame } from "./ui-block-frame";

export type CalculatorProps = {
  title?: string;
  description?: string;
  inputs: { key: string; label: string; min: number; max: number; step?: number; default: number; unit?: string }[];
  outputs: { label: string; expr: string; unit?: string; precision?: number; highlight?: boolean }[];
};

type CompiledOutput = { label: string; unit?: string; precision: number; highlight: boolean; compiled: CompiledExpression | null; error?: string };
type OutputResult = { index: number; output: CompiledOutput; value: number | null; error?: string };

function formatNumber(value: number, precision: number): string {
  if (!Number.isFinite(value)) {
    return "—";
  }
  return new Intl.NumberFormat("en-US", { maximumFractionDigits: precision, minimumFractionDigits: 0 }).format(value);
}

function Calculator({ id, props }: UIBlockRenderProps<CalculatorProps>) {
  const t = useTranslations("chat.markdown.uiBlock");
  const defaults = React.useMemo(() => Object.fromEntries(props.inputs.map((input) => [input.key, input.default])), [props.inputs]);
  const [values, setValues] = React.useState<Record<string, number>>(defaults);
  const outputs = React.useMemo<CompiledOutput[]>(
    () =>
      props.outputs.map((output) => {
        try {
          return { label: output.label, unit: output.unit, precision: output.precision ?? 2, highlight: output.highlight ?? false, compiled: compileExpression(output.expr) };
        } catch (error) {
          return { label: output.label, unit: output.unit, precision: 2, highlight: false, compiled: null, error: error instanceof Error ? error.message : String(error) };
        }
      }),
    [props.outputs],
  );
  const dirty = props.inputs.some((input) => values[input.key] !== input.default);
  const results = React.useMemo<OutputResult[]>(
    () =>
      outputs.map((output, index) => {
        if (!output.compiled) {
          return { index, output, value: null, error: output.error };
        }
        try {
          return { index, output, value: output.compiled.evaluate(values), error: undefined };
        } catch (evalError) {
          return { index, output, value: null, error: evalError instanceof Error ? evalError.message : String(evalError) };
        }
      }),
    [outputs, values],
  );

  return (
    <div className="px-4 pb-4 pt-5">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          {props.title ? <h3 className="text-lg font-semibold leading-7">{props.title}</h3> : null}
          {props.description ? <p className="mt-1 text-xs leading-5 text-muted-foreground">{props.description}</p> : null}
        </div>
        {dirty ? (
          <button type="button" onClick={() => setValues(defaults)} className="inline-flex shrink-0 items-center gap-1 text-xs text-muted-foreground hover:text-foreground">
            <RotateCcw className="size-3" strokeWidth={1.8} />
            {t("reset")}
          </button>
        ) : null}
      </div>

      <div className="mt-4 grid grid-cols-1 gap-5 md:grid-cols-[3fr_2fr]">
        <div className="space-y-4">
          {props.inputs.map((input) => {
            const value = values[input.key] ?? input.default;
            const step = input.step ?? (input.max - input.min) / 100;
            return (
              <div key={`${id}-${input.key}`}>
                <div className="mb-1.5 flex items-baseline justify-between gap-2 text-xs">
                  <label htmlFor={`${id}-${input.key}`} className="text-muted-foreground">
                    {input.label}
                  </label>
                  <span className="font-medium tabular-nums">
                    {formatNumber(value, 4)}
                    {input.unit ? <span className="ml-0.5 font-normal text-muted-foreground">{input.unit}</span> : null}
                  </span>
                </div>
                <Slider
                  id={`${id}-${input.key}`}
                  min={input.min}
                  max={input.max}
                  step={step > 0 ? step : 1}
                  value={[value]}
                  onValueChange={([next]) => setValues((current) => ({ ...current, [input.key]: next ?? value }))}
                />
              </div>
            );
          })}
        </div>

        <div className="grid content-start gap-2">
          {results.filter((entry) => entry.output.highlight).map((entry) => (
            <div key={`${id}-out-${entry.index}`} className="rounded-lg border-[0.5px] border-primary/40 bg-primary/5 px-3 py-2.5">
              <p className="text-[11px] text-muted-foreground">{entry.output.label}</p>
              <OutputValue entry={entry} className="mt-0.5 text-xl leading-7" />
            </div>
          ))}
          {results.some((entry) => !entry.output.highlight) ? (
            <dl className="divide-y-[0.5px] divide-border rounded-lg border-[0.5px] border-border bg-background px-3">
              {results.filter((entry) => !entry.output.highlight).map((entry) => (
                <div key={`${id}-out-${entry.index}`} className="flex items-baseline justify-between gap-3 py-2">
                  <dt className="min-w-0 text-xs text-muted-foreground">{entry.output.label}</dt>
                  <OutputValue entry={entry} as="dd" className="shrink-0 text-right text-sm leading-5" />
                </div>
              ))}
            </dl>
          ) : null}
        </div>
      </div>
    </div>
  );
}

function OutputValue({ entry, as: Tag = "p", className }: { entry: OutputResult; as?: "p" | "dd"; className?: string }) {
  if (entry.error) {
    return <Tag className={cn("font-mono text-[11px] text-destructive", className)}>{entry.error}</Tag>;
  }
  return (
    <Tag className={cn("font-semibold tabular-nums", className)}>
      {formatNumber(entry.value ?? Number.NaN, entry.output.precision)}
      {entry.output.unit ? <span className="ml-1 text-xs font-normal text-muted-foreground">{entry.output.unit}</span> : null}
    </Tag>
  );
}

function CalculatorSkeleton() {
  return (
    <UIBlockFrame className="p-4">
      <Skeleton className="h-5 w-1/3 rounded-full" />
      <div className="mt-4 grid grid-cols-1 gap-5 md:grid-cols-[3fr_2fr]">
        <div className="space-y-4">
          {Array.from({ length: 3 }).map((_, index) => (
            <Skeleton key={`calc-in-${index}`} className="h-8 rounded-md" />
          ))}
        </div>
        <Skeleton className="h-24 rounded-lg" />
      </div>
    </UIBlockFrame>
  );
}

export const calculatorDefinition: UIBlockDefinition<CalculatorProps> = {
  name: "calculator",
  version: 1,
  schema: s.object(
    {
      title: s.string(),
      description: s.string(),
      inputs: s.array(
        s.object(
          { key: s.string(), label: s.string(), min: s.number(), max: s.number(), step: s.number(), default: s.number(), unit: s.string() },
          ["key", "label", "min", "max", "default"],
        ),
        { minItems: 1 },
      ),
      outputs: s.array(
        s.object({ label: s.string(), expr: s.string(), unit: s.string(), precision: s.number(), highlight: s.boolean() }, ["label", "expr"]),
        { minItems: 1 },
      ),
    },
    ["inputs", "outputs"],
  ),
  Component: Calculator,
  Skeleton: CalculatorSkeleton,
};
