"use client";

import { formatProviderImportLink, MAX_PROVIDER_IMPORT_ENTRIES, PROVIDER_IMPORT_PROTOCOLS } from "@deeix/core";
import { Plus, Trash2 } from "lucide-react";
import * as React from "react";
import { useTranslations } from "next-intl";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { useSettingsProviderImportOrigin } from "@/features/settings/hooks/use-settings-provider-import-origin";
import { modelProviderProtocolLabel } from "@/features/settings/model/model-provider-protocols";
import { CopyActionButton } from "@/shared/components/copy-action";

/** Radix Select has no empty value, so this stands for "no protocol". */
const PROTOCOL_AUTO = "auto";

type LinkValues = { url: string; key: string; protocol: string; name: string };

/** The id keeps each table's state stable when an entry is removed. */
type LinkEntry = LinkValues & { id: number };

const EMPTY_VALUES: LinkValues = { url: "", key: "", protocol: PROTOCOL_AUTO, name: "" };

/** Builds an import link from one table per service. */
export function ModelsImportLinkDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const t = useTranslations("settings.modelsPage.importLink");
  const commonT = useTranslations("common");

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {/* Keep focus off the link field on open. */}
      <DialogContent className="sm:max-w-[520px]" onOpenAutoFocus={(event) => event.preventDefault()}>
        <DialogHeader>
          <DialogTitle>{t("title")}</DialogTitle>
          <DialogDescription>{t("description")}</DialogDescription>
        </DialogHeader>
        {/* Unmounted on close so typed keys are discarded. */}
        <ImportLinkBody />
        <DialogFooter>
          <Button type="button" onClick={() => onOpenChange(false)}>
            {commonT("actions.close")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function ImportLinkBody() {
  const t = useTranslations("settings.modelsPage.importLink");
  const commonT = useTranslations("common");
  const origin = useSettingsProviderImportOrigin();
  const linkId = React.useId();
  const nextEntryID = React.useRef(1);
  const [entries, setEntries] = React.useState<LinkEntry[]>(() => [{ id: 0, ...EMPTY_VALUES }]);
  // Empty fields are written as placeholders.
  const link = origin
    ? formatProviderImportLink(
        origin,
        entries.map((entry) => ({
          baseURL: entry.url,
          apiKey: entry.key,
          protocol: PROVIDER_IMPORT_PROTOCOLS.find((protocol) => protocol === entry.protocol) ?? null,
          name: entry.name,
        })),
      )
    : "";
  const multiple = entries.length > 1;

  const update = (id: number, field: keyof LinkValues, value: string) =>
    setEntries((current) => current.map((entry) => (entry.id === id ? { ...entry, [field]: value } : entry)));
  const addEntry = () => {
    const id = nextEntryID.current;
    nextEntryID.current += 1;
    setEntries((current) => [...current, { id, ...EMPTY_VALUES }]);
  };
  const removeEntry = (id: number) => setEntries((current) => current.filter((entry) => entry.id !== id));

  return (
    <div className="min-w-0 space-y-4">
      <div className="min-w-0 space-y-1">
        <label htmlFor={linkId} className="text-xs text-muted-foreground">
          {t("link")}
        </label>
        <div className="flex min-w-0 gap-2">
          <Input id={linkId} value={link} readOnly className="min-w-0 text-xs" onFocus={(event) => event.currentTarget.select()} />
          <CopyActionButton
            type="button"
            variant="ghost"
            size="icon"
            className="size-8 shrink-0 text-muted-foreground shadow-none"
            value={link}
            messages={{ copied: t("copied"), failed: commonT("errors.copyFailed") }}
            aria-label={t("copyLink")}
            title={t("copyLink")}
          />
        </div>
        <p className="text-[11px] text-muted-foreground">
          {multiple ? t("linkHintMultiple") : t("linkHint", { address: "{address}", key: "{key}" })}
        </p>
      </div>

      <div className="max-h-[min(52svh,26rem)] min-w-0 space-y-3 overflow-y-auto">
        {entries.map((entry, index) => (
          <LinkEntryTable
            key={entry.id}
            values={entry}
            title={multiple ? t("entryTitle", { index: index + 1 }) : null}
            onChange={(field, value) => update(entry.id, field, value)}
            onRemove={multiple ? () => removeEntry(entry.id) : null}
          />
        ))}
      </div>

      <Button
        type="button"
        size="sm"
        variant="ghost"
        className="text-muted-foreground"
        disabled={entries.length >= MAX_PROVIDER_IMPORT_ENTRIES}
        onClick={addEntry}
      >
        <Plus className="size-3.5 stroke-1" />
        {t("addEntry")}
      </Button>
    </div>
  );
}

function LinkEntryTable({
  values,
  title,
  onChange,
  onRemove,
}: {
  values: LinkValues;
  /** Hint for multi-service links. */
  title: string | null;
  onChange: (field: keyof LinkValues, value: string) => void;
  onRemove: (() => void) | null;
}) {
  const t = useTranslations("settings.modelsPage.importLink");
  const pageT = useTranslations("settings.modelsPage");

  const textRow = (field: "url" | "key" | "name", required: boolean) => (
    <ParameterRow name={field} required={required} requiredLabel={t("required")}>
      <Input
        value={values[field]}
        onChange={(event) => onChange(field, event.target.value)}
        placeholder={t(`parameters.${field}`)}
        aria-label={field}
        autoComplete="off"
        spellCheck={false}
        className="h-7 text-xs"
      />
    </ParameterRow>
  );

  return (
    <div className="min-w-0 space-y-1">
      {title ? (
        <div className="flex h-7 items-center justify-between gap-2">
          <span className="text-xs font-medium">{title}</span>
          {onRemove ? (
            <Button
              type="button"
              variant="ghost"
              size="icon-xs"
              className="text-muted-foreground shadow-none"
              aria-label={t("removeEntry", { title })}
              title={t("removeEntry", { title })}
              onClick={onRemove}
            >
              <Trash2 className="size-3.5 stroke-1" />
            </Button>
          ) : null}
        </div>
      ) : null}
      <Table className="min-w-full table-fixed" shellClassName="w-full">
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            <TableHead className="w-32">{t("columns.name")}</TableHead>
            <TableHead>{t("columns.value")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {textRow("url", true)}
          {textRow("key", true)}
          <ParameterRow name="protocol" required={false} requiredLabel={t("required")}>
            <Select value={values.protocol} onValueChange={(value) => onChange("protocol", value)}>
              <SelectTrigger size="sm" aria-label="protocol" className="h-7 w-full text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={PROTOCOL_AUTO}>{t("protocolAuto")}</SelectItem>
                {PROVIDER_IMPORT_PROTOCOLS.map((protocol) => (
                  <SelectItem key={protocol} value={protocol}>
                    {modelProviderProtocolLabel(protocol, (key) => pageT(key))}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </ParameterRow>
          {textRow("name", false)}
        </TableBody>
      </Table>
    </div>
  );
}

function ParameterRow({
  name,
  required,
  requiredLabel,
  children,
}: {
  name: string;
  required: boolean;
  requiredLabel: string;
  children: React.ReactNode;
}) {
  return (
    <TableRow className="hover:bg-transparent">
      <TableCell className="py-1.5">
        <div className="flex items-center gap-1.5">
          <span className="text-xs text-foreground">{name}</span>
          {required ? (
            <Badge variant="secondary" className="rounded-sm px-1 py-0 text-[10px] font-normal leading-4">
              {requiredLabel}
            </Badge>
          ) : null}
        </div>
      </TableCell>
      <TableCell className="py-1.5">{children}</TableCell>
    </TableRow>
  );
}
