"use client";

import type { ResolvedNode, UIValue } from "@deeix/core";
import { readAction } from "@deeix/core";
import { Copy } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Slider } from "@/components/ui/slider";
import { Switch } from "@/components/ui/switch";
import { cn } from "@/lib/utils";
import { useCopyAction } from "@/shared/components/copy-action";
import { useUIBlockActions } from "../ui-block-actions";
import { UIBlockAsk } from "../ui-block-ask";
import { ChipButton, displayValue, optionIndex, readEnum, readNumber, readOptions, readText, useLiveBlock } from "./live-context";

// Controls write to the state named by `bind`; one without a binding has
// nothing to change and is not shown.

const INPUT_MAX_LENGTH = 200;

function ControlLabel({ htmlFor, label, value }: { htmlFor?: string; label?: string; value?: React.ReactNode }) {
  if (!label && value === undefined) {
    return null;
  }
  return (
    <div className="mb-1.5 flex items-baseline justify-between gap-2 text-xs">
      <label htmlFor={htmlFor} className="min-w-0 truncate text-muted-foreground">
        {label ?? ""}
      </label>
      {value !== undefined ? <span className="shrink-0 font-medium tabular-nums">{value}</span> : null}
    </div>
  );
}

export function LiveSlider({ node }: { node: ResolvedNode }) {
  const { state, update } = useLiveBlock();
  const id = React.useId();
  const bind = node.bind;
  if (!bind) {
    return null;
  }
  const min = readNumber(node.props, "min") ?? 0;
  const declaredMax = readNumber(node.props, "max") ?? 100;
  const max = declaredMax > min ? declaredMax : min + 1;
  const declaredStep = readNumber(node.props, "step");
  const step = declaredStep !== undefined && declaredStep > 0 ? declaredStep : max - min >= 10 ? 1 : (max - min) / 100;
  const current = state[bind];
  const value = typeof current === "number" && Number.isFinite(current) ? Math.min(max, Math.max(min, current)) : min;
  const unit = readText(node.props, "unit");
  return (
    <div className="min-w-0">
      <ControlLabel
        htmlFor={id}
        label={readText(node.props, "label")}
        value={
          <>
            {displayValue(value)}
            {unit ? <span className="ml-0.5 font-normal text-muted-foreground">{unit}</span> : null}
          </>
        }
      />
      <Slider id={id} min={min} max={max} step={step} value={[value]} onValueChange={([next]) => update({ [bind]: next ?? value })} />
    </div>
  );
}

export function LiveSelect({ node }: { node: ResolvedNode }) {
  const { state, update } = useLiveBlock();
  const id = React.useId();
  const bind = node.bind;
  const options = readOptions(node.props, "options");
  if (!bind || options.length === 0) {
    return null;
  }
  const index = optionIndex(options, state[bind]);
  return (
    <div className="min-w-0">
      <ControlLabel htmlFor={id} label={readText(node.props, "label")} />
      {/* Radix needs string values; the option index stands in for the real value. */}
      <Select value={index >= 0 ? String(index) : undefined} onValueChange={(next) => selectOption(options, next, bind, update)}>
        <SelectTrigger id={id} size="sm" className="h-8 w-full text-xs">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map((option, optionIndexValue) => (
            <SelectItem key={`${node.path}-${optionIndexValue}`} value={String(optionIndexValue)}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

function selectOption(options: ReturnType<typeof readOptions>, next: string, bind: string, update: (patch: Record<string, UIValue>) => void) {
  const option = options[Number.parseInt(next, 10)];
  if (option) {
    update({ [bind]: option.value });
  }
}

export function LiveSegmented({ node }: { node: ResolvedNode }) {
  const { state, update } = useLiveBlock();
  const bind = node.bind;
  const options = readOptions(node.props, "options");
  if (!bind || options.length === 0) {
    return null;
  }
  const index = optionIndex(options, state[bind]);
  const label = readText(node.props, "label");
  return (
    <div className="min-w-0">
      {label ? <p className="mb-1.5 truncate text-xs text-muted-foreground">{label}</p> : null}
      <div role="tablist" aria-label={label} className="flex flex-wrap gap-1.5">
        {options.map((option, optionIndexValue) => (
          <ChipButton key={`${node.path}-${optionIndexValue}`} active={optionIndexValue === index} onClick={() => update({ [bind]: option.value })}>
            {option.label}
          </ChipButton>
        ))}
      </div>
    </div>
  );
}

export function LiveSwitch({ node }: { node: ResolvedNode }) {
  const { state, update } = useLiveBlock();
  const id = React.useId();
  const bind = node.bind;
  if (!bind) {
    return null;
  }
  return (
    <div className="flex min-w-0 items-center justify-between gap-3">
      <label htmlFor={id} className="min-w-0 truncate text-sm">
        {readText(node.props, "label") ?? ""}
      </label>
      <Switch id={id} checked={state[bind] === true} onCheckedChange={(checked) => update({ [bind]: checked })} />
    </div>
  );
}

const INPUT_TYPES = ["text", "number"] as const;

export function LiveInput({ node }: { node: ResolvedNode }) {
  const { state, update } = useLiveBlock();
  const id = React.useId();
  const bind = node.bind;
  const numeric = readEnum(node.props, "type", INPUT_TYPES) === "number";
  const current = bind ? state[bind] : undefined;
  // A number field keeps what is typed ("1." or "-") until it parses.
  const [draft, setDraft] = React.useState(() => displayInput(current));
  const committed = displayInput(current);
  const [lastCommitted, setLastCommitted] = React.useState(committed);
  if (committed !== lastCommitted) {
    setLastCommitted(committed);
    setDraft(committed);
  }
  if (!bind) {
    return null;
  }
  const onChange = (text: string) => {
    setDraft(text);
    if (!numeric) {
      update({ [bind]: text });
      return;
    }
    const value = Number(text);
    if (text.trim() !== "" && Number.isFinite(value)) {
      update({ [bind]: value });
    }
  };
  return (
    <div className="min-w-0">
      <ControlLabel htmlFor={id} label={readText(node.props, "label")} />
      <Input
        id={id}
        className="h-8 text-sm"
        inputMode={numeric ? "decimal" : undefined}
        maxLength={INPUT_MAX_LENGTH}
        placeholder={readText(node.props, "placeholder")}
        value={draft}
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  );
}

function displayInput(value: UIValue | undefined): string {
  if (typeof value === "number") {
    return Number.isFinite(value) ? String(value) : "";
  }
  return typeof value === "string" ? value : "";
}

const BUTTON_VARIANTS = ["default", "outline"] as const;
const COPY_PREVIEW_LENGTH = 120;

function truncate(text: string, length: number): string {
  const characters = Array.from(text);
  return characters.length > length ? `${characters.slice(0, length).join("")}…` : text;
}

// `ask` becomes a follow-up composer, `set` changes block state, `copy` copies
// text; a button without an action renders disabled. Set and copy buttons stay
// neutral so the accent colour marks the one thing that leaves the block.
export function LiveButton({ node }: { node: ResolvedNode }) {
  const t = useTranslations("chat.markdown.uiBlock");
  const { stateNames, streaming, update } = useLiveBlock();
  const actions = useUIBlockActions();
  const { copy } = useCopyAction({ messages: { copied: t("copied"), failed: t("copyFailed") } });
  const action = readAction(node.props, stateNames);
  const variant = readEnum(node.props, "variant", BUTTON_VARIANTS) ?? "default";
  const className = cn(
    "inline-flex min-h-8 w-fit max-w-full items-center gap-1.5 rounded-md px-3 py-1.5 text-left text-xs font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50",
    variant === "outline"
      ? "border-[0.5px] border-border bg-background hover:bg-accent hover:text-accent-foreground"
      : "bg-secondary text-secondary-foreground hover:bg-secondary/80",
  );

  if (action?.kind === "ask") {
    // `variant` does not apply: an ask is a small composer, not a button.
    return <UIBlockAsk suggestion={action.text} busy={Boolean(actions.busy) || streaming} send={actions.ask} />;
  }

  const label = readText(node.props, "label") ?? "";
  if (action?.kind === "copy") {
    return (
      // The toast repeats what was copied, so a block cannot fill the clipboard unseen.
      <button type="button" className={className} onClick={() => void copy(action.text, { copiedDescription: truncate(action.text, COPY_PREVIEW_LENGTH) })}>
        <Copy className="size-3.5 shrink-0" strokeWidth={1.8} />
        <span className="min-w-0 truncate">{label || t("copy")}</span>
      </button>
    );
  }
  if (action?.kind === "set") {
    const patch = action.patch;
    return (
      <button type="button" className={className} onClick={() => update(patch)}>
        <span className="min-w-0 truncate">{label}</span>
      </button>
    );
  }
  return label ? (
    <button type="button" className={className} disabled>
      <span className="min-w-0 truncate">{label}</span>
    </button>
  ) : null;
}
