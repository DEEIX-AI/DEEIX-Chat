"use client";

import { formatProviderImportLink, PROVIDER_IMPORT_PROTOCOLS } from "@deeix/core";
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

/** Select value for "leave protocol out of the link"; Radix Select has no empty value. */
const PROTOCOL_AUTO = "auto";

type LinkValues = { url: string; key: string; protocol: string; name: string };

const EMPTY_VALUES: LinkValues = { url: "", key: "", protocol: PROTOCOL_AUTO, name: "" };

/** Import-link template with a fill-in table that previews the finished link. */
export function ModelsImportLinkDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const t = useTranslations("settings.modelsPage.importLink");
  const commonT = useTranslations("common");

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {/* Focusing the first field would select the link on open. */}
      <DialogContent className="sm:max-w-[520px]" onOpenAutoFocus={(event) => event.preventDefault()}>
        <DialogHeader>
          <DialogTitle>{t("title")}</DialogTitle>
          <DialogDescription>{t("description")}</DialogDescription>
        </DialogHeader>
        {/* Mounted only while open, so a typed key is dropped when the dialog closes. */}
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
  const pageT = useTranslations("settings.modelsPage");
  const commonT = useTranslations("common");
  const origin = useSettingsProviderImportOrigin();
  const linkId = React.useId();
  const [values, setValues] = React.useState<LinkValues>(EMPTY_VALUES);
  // Empty url/key keep their placeholders, so the untouched form shows the chat-link template.
  const link = origin
    ? formatProviderImportLink(origin, {
        baseURL: values.url,
        apiKey: values.key,
        protocol: PROVIDER_IMPORT_PROTOCOLS.find((protocol) => protocol === values.protocol) ?? null,
        name: values.name,
      })
    : "";
  const set = (field: keyof LinkValues) => (value: string) => setValues((current) => ({ ...current, [field]: value }));

  const textRow = (field: "url" | "key" | "name", required: boolean) => (
    <ParameterRow name={field} required={required} requiredLabel={t("required")}>
      <Input
        value={values[field]}
        onChange={(event) => set(field)(event.target.value)}
        placeholder={t(`parameters.${field}`)}
        aria-label={field}
        autoComplete="off"
        spellCheck={false}
        className="h-7 text-xs"
      />
    </ParameterRow>
  );

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
        <p className="text-[11px] text-muted-foreground">{t("linkHint", { address: "{address}", key: "{key}" })}</p>
      </div>

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
            <Select value={values.protocol} onValueChange={set("protocol")}>
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
