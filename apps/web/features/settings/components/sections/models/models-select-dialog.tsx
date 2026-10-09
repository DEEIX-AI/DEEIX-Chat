"use client";

import { ChevronRight, Image as ImageIcon, MessageSquare, RefreshCw, Search, Video } from "lucide-react";
import * as React from "react";
import { useTranslations } from "next-intl";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogHeightTransition,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableEmptyRow, TableHead, TableHeader, TableLoadingRow, TableRow } from "@/components/ui/table";
import {
  describeModelProtocols,
  type ModelProtocolChoice,
  modelProtocolChoiceGroups,
  type ModelProtocolKind,
} from "@/features/settings/model/model-protocol-choices";
import { cn } from "@/lib/utils";

/** A model the provider lists, with the protocols the server suggests for it. */
export type ModelOption = { name: string; suggestedProtocols: readonly string[] };

/** Enabled models and the protocols each one runs on. */
export type ModelSelection = ReadonlyMap<string, readonly string[]>;

/** Chips in the field before the rest collapse into "+N". */
const FIELD_VISIBLE_CHIPS = 2;

const FILTER_CHIP_CLASS =
  "inline-flex h-6 shrink-0 items-center gap-1 rounded-md px-2 text-xs text-muted-foreground outline-none transition-colors hover:bg-muted/60 focus-visible:bg-muted/60";
const FILTER_CHIP_ACTIVE_CLASS = "bg-muted font-medium text-foreground hover:bg-muted";

/**
 * Form field for a provider's enabled models: the chosen ones as chips, opening
 * the picker on click. Styled as an input so it lines up with the other fields.
 */
export function ModelsSelectField({
  id,
  models,
  selected,
  disabled = false,
  onOpen,
}: {
  id?: string;
  models: readonly ModelOption[];
  selected: ModelSelection;
  disabled?: boolean;
  onOpen: () => void;
}) {
  const t = useTranslations("settings.modelsPage.selectDialog");
  const chosen = models.map((model) => model.name).filter((name) => selected.has(name));
  const visible = chosen.slice(0, FIELD_VISIBLE_CHIPS);
  const hidden = chosen.length - visible.length;

  return (
    <button
      id={id}
      type="button"
      disabled={disabled}
      title={chosen.join("\n") || undefined}
      onClick={onOpen}
      className={cn(
        "flex h-8 w-full min-w-0 items-center gap-2 rounded-md border border-input/40 bg-transparent px-3 text-left text-xs shadow-none outline-none transition-[color,box-shadow,background-color]",
        "hover:bg-muted/40 focus-visible:border-ring/60 focus-visible:ring-[1px] focus-visible:ring-ring/40 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-input/30 dark:hover:bg-input/50",
      )}
    >
      {chosen.length === 0 ? (
        <span className="min-w-0 flex-1 truncate text-muted-foreground">{t("placeholder")}</span>
      ) : (
        <span className="flex min-w-0 flex-1 items-center gap-1 overflow-hidden">
          {visible.map((model) => (
            <Badge
              key={model}
              variant="secondary"
              className="min-w-0 max-w-[14rem] shrink justify-start rounded-sm px-1.5 py-0 font-mono text-[10px] font-normal leading-4"
            >
              <span className="min-w-0 truncate">{model}</span>
            </Badge>
          ))}
          {hidden > 0 ? (
            <Badge variant="outline" className="shrink-0 rounded-sm px-1.5 py-0 text-[10px] font-normal leading-4 text-muted-foreground">
              +{hidden}
            </Badge>
          ) : null}
        </span>
      )}
      <ChevronRight className="size-3.5 shrink-0 text-muted-foreground opacity-60" aria-hidden="true" />
    </button>
  );
}

type ModelsSelectDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  models: readonly ModelOption[];
  selected: ModelSelection;
  /** Protocols the server accepts for a single model; empty on a server that has no per-model protocols. */
  protocols: readonly string[];
  /** Called with the final choice on confirm; closing any other way discards the draft. */
  onConfirm: (next: Map<string, string[]>) => void;
  loading?: boolean;
  error?: string;
  /** Fetches the list again; shown as a refresh button and as the retry for a failed fetch. */
  onRefresh?: () => void;
};

/**
 * Picks which of a provider's models to enable. It works on a draft: ticking
 * changes nothing until Confirm, so Cancel and Escape leave the form as it was.
 */
export function ModelsSelectDialog(props: ModelsSelectDialogProps) {
  const { open, onOpenChange, models } = props;
  // A new list (first fetch, or a refetch that changed it) restarts the draft from the form's choice.
  const listKey = React.useMemo(() => models.map((model) => model.name).join("\n"), [models]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[calc(100vw-2rem)] gap-0 overflow-hidden p-0 sm:max-w-[680px]">
        <ModelsSelectBody key={listKey} {...props} />
      </DialogContent>
    </Dialog>
  );
}

/** Height of the scrolling list; fixed so the dialog does not re-centre while searching. */
const LIST_HEIGHT = "min(24rem, calc(86svh - 16rem))";
/** With no models at all (loading, failed, empty) there is nothing to scroll: a short list will do. */
const COMPACT_LIST_HEIGHT = "10rem";
/** A state cell spans the list below the 36px header (with a little slack so no scrollbar appears). */
const STATE_CELL_CLASS = "h-[calc(min(24rem,calc(86svh-16rem))-2.5rem)] py-0 text-xs text-muted-foreground";
const COMPACT_STATE_CELL_CLASS = "h-[calc(10rem-2.5rem)] py-0 text-xs text-muted-foreground";

function ModelsSelectBody({ onOpenChange, models: options, selected, protocols, onConfirm, loading = false, error = "", onRefresh }: ModelsSelectDialogProps) {
  const t = useTranslations("settings.modelsPage.selectDialog");
  const commonT = useTranslations("common");
  const models = React.useMemo(() => options.map((option) => option.name), [options]);
  const [draft, setDraft] = React.useState(() => new Set(models.filter((model) => selected.has(model))));
  // Every row keeps a protocol, ticked or not: a saved model starts on its saved one, the rest on the suggestion.
  const [protocolsByModel, setProtocolsByModel] = React.useState(
    () => new Map(options.map((option) => [option.name, [...(selected.get(option.name) ?? option.suggestedProtocols)]])),
  );
  const choiceGroups = React.useMemo(() => modelProtocolChoiceGroups(protocols), [protocols]);
  const showProtocols = choiceGroups.length > 0;
  const columns = showProtocols ? 3 : 2;
  const [query, setQuery] = React.useState("");
  const [onlySelected, setOnlySelected] = React.useState(false);

  // Chosen models first, fixed for this opening: re-sorting on every tick would move rows under the cursor.
  const [pinned] = React.useState(() => new Set(selected.keys()));
  const ordered = React.useMemo(
    () => [...models].sort((left, right) => Number(pinned.has(right)) - Number(pinned.has(left))),
    [models, pinned],
  );

  const setModelProtocols = (model: string, value: string) => {
    const choice = choiceGroups.flatMap((group) => group.choices).find((item) => item.value === value);
    if (!choice) return;
    setProtocolsByModel((current) => new Map(current).set(model, choice.protocols));
    // Picking a protocol for a model means using it.
    setDraft((current) => new Set(current).add(model));
  };

  const normalizedQuery = query.trim().toLowerCase();
  const visible = ordered.filter(
    (model) => (!onlySelected || draft.has(model)) && (!normalizedQuery || model.toLowerCase().includes(normalizedQuery)),
  );
  const visibleSelected = visible.reduce((count, model) => count + (draft.has(model) ? 1 : 0), 0);
  const headState = visible.length > 0 && visibleSelected === visible.length ? true : visibleSelected > 0 ? "indeterminate" : false;
  const showList = models.length > 0 && !loading;

  const toggle = (model: string, checked: boolean) => {
    setDraft((current) => {
      const next = new Set(current);
      if (checked) next.add(model);
      else next.delete(model);
      return next;
    });
  };

  const setVisible = (checked: boolean) => {
    setDraft((current) => {
      const next = new Set(current);
      for (const model of visible) {
        if (checked) next.add(model);
        else next.delete(model);
      }
      return next;
    });
  };

  const confirm = () => {
    onConfirm(new Map(models.filter((model) => draft.has(model)).map((model) => [model, protocolsByModel.get(model) ?? []])));
    onOpenChange(false);
  };

  // Empty, loading and error states fill the list below the header and stay inert on hover.
  const compact = models.length === 0;
  const stateRow = { rowClassName: "hover:bg-transparent", cellClassName: compact ? COMPACT_STATE_CELL_CLASS : STATE_CELL_CLASS };
  let emptyRow: React.ReactNode = null;
  if (loading) {
    emptyRow = <TableLoadingRow colSpan={columns} {...stateRow}>{t("loading")}</TableLoadingRow>;
  } else if (error && models.length === 0) {
    emptyRow = (
      <TableEmptyRow colSpan={columns} {...stateRow}>
        <div className="flex flex-col items-center gap-2.5">
          <span>{error}</span>
          {onRefresh ? (
            <Button type="button" size="sm" variant="outline" className="h-7 px-3 text-xs shadow-none" onClick={onRefresh}>
              {commonT("actions.retry")}
            </Button>
          ) : null}
        </div>
      </TableEmptyRow>
    );
  } else if (visible.length === 0) {
    emptyRow = <TableEmptyRow colSpan={columns} {...stateRow}>{onlySelected && !normalizedQuery ? t("emptySelected") : t("empty")}</TableEmptyRow>;
  }

  return (
    <DialogHeightTransition contentClassName="max-h-[min(86svh,720px)]">
      <DialogHeader className="shrink-0 px-5 pt-5 pb-3">
        <DialogTitle>{t("title")}</DialogTitle>
        <DialogDescription>{t("description")}</DialogDescription>
      </DialogHeader>

      <div className="shrink-0 px-5 pb-2">
        <div className="flex h-8 items-center gap-3">
          <div className="flex min-w-0 flex-1 items-center gap-1">
            <button
              type="button"
              aria-pressed={!onlySelected}
              className={cn(FILTER_CHIP_CLASS, !onlySelected && FILTER_CHIP_ACTIVE_CLASS)}
              onClick={() => setOnlySelected(false)}
            >
              <span>{t("all")}</span>
              <span className="font-mono tabular-nums">{models.length}</span>
            </button>
            <button
              type="button"
              aria-pressed={onlySelected}
              className={cn(FILTER_CHIP_CLASS, onlySelected && FILTER_CHIP_ACTIVE_CLASS)}
              onClick={() => setOnlySelected(true)}
            >
              <span>{t("selected")}</span>
              <span className="font-mono tabular-nums">{draft.size}</span>
            </button>
          </div>
          {onRefresh ? (
            <Button
              type="button"
              size="icon-sm"
              variant="ghost"
              className="size-7 shrink-0 text-muted-foreground shadow-none"
              disabled={loading}
              aria-label={t("refresh")}
              title={t("refresh")}
              onClick={onRefresh}
            >
              <RefreshCw className={cn("size-3.5 stroke-1", loading && "animate-spin")} />
            </Button>
          ) : null}
        </div>
      </div>

      <div className="shrink-0 px-5 pb-2">
        <div className="relative">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-3.5 -translate-y-1/2 stroke-1 text-muted-foreground" />
          <Input
            value={query}
            placeholder={t("search")}
            aria-label={t("search")}
            disabled={!showList}
            className="bg-background pl-8"
            onChange={(event) => setQuery(event.target.value)}
          />
        </div>
      </div>

      {/* A fixed height, not a max: the dialog must not resize and re-centre while the search narrows the list. */}
      <div className="min-h-0 overflow-hidden px-5 py-2">
        <Table
          className="min-w-full table-fixed"
          shellClassName="w-full"
          viewportClassName="overscroll-contain [&_thead]:sticky [&_thead]:top-0 [&_thead]:z-20"
          viewportStyle={{ height: compact ? COMPACT_LIST_HEIGHT : LIST_HEIGHT }}
        >
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              <TableHead className="w-10 px-2 py-1.5 text-center">
                <div className="flex h-6 items-center justify-center">
                  <Checkbox
                    checked={headState}
                    disabled={!showList || visible.length === 0}
                    aria-label={t("selectVisible")}
                    onCheckedChange={(value) => setVisible(value === true)}
                  />
                </div>
              </TableHead>
              <TableHead>{t("column")}</TableHead>
              {showProtocols ? <TableHead className="w-[19rem]">{t("protocolColumn")}</TableHead> : null}
            </TableRow>
          </TableHeader>
          <TableBody>
            {emptyRow ??
              visible.map((model) => {
                const checked = draft.has(model);
                return (
                  <TableRow key={model} selected={checked} className="cursor-pointer" onClick={() => toggle(model, !checked)}>
                    <TableCell className="w-10 px-2 py-1 text-center">
                      <div className="flex h-6 items-center justify-center">
                        <Checkbox
                          checked={checked}
                          aria-label={model}
                          onClick={(event) => event.stopPropagation()}
                          onCheckedChange={(value) => toggle(model, value === true)}
                        />
                      </div>
                    </TableCell>
                    <TableCell className={cn("py-1 font-mono text-xs", checked ? "text-foreground" : "text-muted-foreground")}>
                      <span className="block truncate" title={model}>
                        {model}
                      </span>
                    </TableCell>
                    {/* The select lives in a clickable row: keep its clicks from toggling the row. */}
                    {showProtocols ? (
                      <TableCell className="w-[19rem] py-1" onClick={(event) => event.stopPropagation()}>
                        <ModelProtocolSelect
                          model={model}
                          protocols={protocolsByModel.get(model) ?? []}
                          groups={choiceGroups}
                          active={checked}
                          onChange={(value) => setModelProtocols(model, value)}
                        />
                      </TableCell>
                    ) : null}
                  </TableRow>
                );
              })}
          </TableBody>
        </Table>
      </div>

      <DialogFooter className="shrink-0 px-5 py-3">
        <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
          {commonT("actions.cancel")}
        </Button>
        <Button type="button" disabled={loading || draft.size === 0} onClick={confirm}>
          {commonT("actions.confirm")}
        </Button>
      </DialogFooter>
    </DialogHeightTransition>
  );
}

const KIND_ICONS: Record<ModelProtocolKind, typeof MessageSquare> = {
  chat: MessageSquare,
  image: ImageIcon,
  video: Video,
};

/** A protocol as "icon · endpoint · vendor": the icon tells chat, image and video models apart at a glance. */
function ModelProtocolText({ choice }: { choice: ModelProtocolChoice }) {
  const Icon = KIND_ICONS[choice.kind];
  return (
    <span className="flex min-w-0 items-center gap-1.5">
      <Icon className="size-3.5 shrink-0 stroke-[1.5] text-muted-foreground" aria-hidden="true" />
      <span className="min-w-0 truncate">{choice.name}</span>
      {choice.vendor ? <span className="shrink-0 text-muted-foreground">{choice.vendor}</span> : null}
    </span>
  );
}

/**
 * Protocol of one model. The trigger is borderless so a long list reads as text, not as a column of
 * inputs; the options are grouped by vendor. A value the choices lack (from a newer server) is kept.
 */
function ModelProtocolSelect({
  model,
  protocols,
  groups,
  active,
  onChange,
}: {
  model: string;
  protocols: readonly string[];
  groups: ReturnType<typeof modelProtocolChoiceGroups>;
  /** Whether the model is ticked; the protocol of an unticked one is dimmed. */
  active: boolean;
  onChange: (value: string) => void;
}) {
  const t = useTranslations("settings.modelsPage.selectDialog");
  const current = describeModelProtocols(protocols);
  const known = groups.some((group) => group.choices.some((choice) => choice.value === current.value));
  return (
    <Select value={current.value} onValueChange={onChange}>
      <SelectTrigger
        aria-label={t("protocolFor", { model })}
        className={cn(
          "-ml-2 h-7 w-[calc(100%+0.5rem)] border-transparent bg-transparent px-2 hover:bg-muted/70 data-[state=open]:bg-muted/70 dark:bg-transparent dark:hover:bg-muted/70",
          !active && "opacity-60",
        )}
      >
        <SelectValue>
          <ModelProtocolText choice={current} />
        </SelectValue>
      </SelectTrigger>
      <SelectContent className="max-h-80">
        {known || !current.value ? null : <SelectItem value={current.value}>{current.name}</SelectItem>}
        {groups.map((group) => (
          <SelectGroup key={group.vendor}>
            {group.vendor ? <SelectLabel className="pt-2 pb-1 text-[10px]">{group.vendor}</SelectLabel> : null}
            {group.choices.map((choice) => {
              const Icon = KIND_ICONS[choice.kind];
              return (
                <SelectItem key={choice.value} value={choice.value}>
                  <Icon className="size-3.5 stroke-[1.5]" aria-hidden="true" />
                  {choice.name}
                </SelectItem>
              );
            })}
          </SelectGroup>
        ))}
      </SelectContent>
    </Select>
  );
}
