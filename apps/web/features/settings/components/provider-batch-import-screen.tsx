"use client";

import { maskProviderApiKey, type ProviderImportLink } from "@deeix/core";
import { CheckIcon, InfoIcon, ShieldAlertIcon } from "lucide-react";
import { useRouter } from "next/navigation";
import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Spinner, SpinnerLabel } from "@/components/ui/spinner";
import { findModelProviderPreset, ModelIcon, resolveModelIconURL } from "@/entities/model";
import {
  CANCEL_BUTTON_CLASS,
  ImportFrame,
  MODELS_SETTINGS_PATH,
  PRIMARY_BUTTON_CLASS,
} from "@/features/settings/components/provider-import-frame";
import { ModelsProviderDialog } from "@/features/settings/components/sections/models/models-provider-dialog";
import {
  type ProviderBatchImportStatus,
  providerImportDraft,
  useSettingsProviderBatchImport,
} from "@/features/settings/hooks/use-settings-provider-batch-import";
import type {
  CreatePersonalProviderPayload,
  PersonalProviderAvailableModelDTO,
  PersonalProviderDTO,
  PersonalProviderProbePayload,
} from "@/shared/api/personal-providers-types";

/**
 * Confirmation screen for multi-provider links. Entries that cannot be imported
 * automatically stay listed with their status.
 */
export function ProviderBatchImportScreen({
  links,
  providers,
  maxPerUser,
  modelProtocols,
  probe,
  create,
  onLeave,
}: {
  links: readonly ProviderImportLink[];
  providers: readonly PersonalProviderDTO[];
  maxPerUser: number;
  modelProtocols: string[];
  probe: (payload: PersonalProviderProbePayload) => Promise<PersonalProviderAvailableModelDTO[]>;
  create: (payload: CreatePersonalProviderPayload) => Promise<PersonalProviderDTO>;
  onLeave: () => void;
}) {
  const t = useTranslations("settings.importPage");
  const router = useRouter();
  const initialSlots = maxPerUser - providers.length;
  const batch = useSettingsProviderBatchImport({ links, modelProtocols, slotsLeft: initialSlots, probe, create });
  const [selecting, setSelecting] = React.useState<number | null>(null);
  const [attempted, setAttempted] = React.useState(false);

  const importedCount = batch.statuses.filter((status) => status.kind === "imported").length;
  const full = initialSlots - importedCount <= 0;
  const selectingLink = selecting === null ? undefined : links[selecting];
  const selectingDraft = React.useMemo(() => (selectingLink ? providerImportDraft(selectingLink) : undefined), [selectingLink]);
  // Flags hosts that already have a provider.
  const existingByHost = React.useMemo(() => new Map(providers.map((provider) => [provider.host, provider.name])), [providers]);

  const finish = () => router.replace(MODELS_SETTINGS_PATH);

  /** Toasts newly imported entries and leaves once all are imported. */
  const settle = (importedBefore: number, after: readonly ProviderBatchImportStatus[] | null) => {
    if (!after) return;
    const imported = after.filter((status) => status.kind === "imported").length;
    if (imported > importedBefore) toast.success(t("batchDone", { count: imported - importedBefore }));
    if (imported === links.length) finish();
  };

  const importAll = async () => {
    setAttempted(true);
    settle(importedCount, await batch.runAll());
  };

  if (initialSlots <= 0) {
    return (
      <ImportFrame title={t("titleMultiple", { count: links.length })} wide>
        <div className="space-y-5">
          <p className="flex gap-1.5 text-xs leading-relaxed text-amber-700 dark:text-amber-300">
            <InfoIcon aria-hidden className="mt-0.5 size-3.5 shrink-0" />
            <span>{t("limitReached", { max: maxPerUser })}</span>
          </p>
          <div className="space-y-1.5">
            <Button type="button" className={PRIMARY_BUTTON_CLASS} onClick={finish}>
              {t("manage")}
            </Button>
            <Button type="button" variant="ghost" className={CANCEL_BUTTON_CLASS} onClick={onLeave}>
              {t("cancel")}
            </Button>
          </div>
        </div>
      </ImportFrame>
    );
  }

  return (
    <ImportFrame title={t("titleMultiple", { count: links.length })} wide>
      <div className="space-y-5">
        <ul className="max-h-[min(52svh,26rem)] divide-y divide-border/60 overflow-y-auto rounded-lg border border-border/60">
          {links.map((link, index) => (
            <BatchEntryRow
              key={`${link.baseURL}\n${maskProviderApiKey(link.apiKey)}\n${index}`}
              link={link}
              status={resolveRowStatus(batch.statuses[index], full)}
              existingName={existingByHost.get(link.host)}
              disabled={batch.running}
              onSelectModels={() => setSelecting(index)}
              onRetry={() => void batch.retry(index).then((after) => settle(importedCount, after))}
            />
          ))}
        </ul>

        <div className="space-y-1.5 text-xs leading-relaxed">
          <p className="flex gap-1.5 text-muted-foreground">
            <ShieldAlertIcon aria-hidden className="mt-0.5 size-3.5 shrink-0" />
            <span>{t("warningMultiple")}</span>
          </p>
          {initialSlots < links.length ? (
            <p className="flex gap-1.5 text-amber-700 dark:text-amber-300">
              <InfoIcon aria-hidden className="mt-0.5 size-3.5 shrink-0" />
              <span>{t("slotsLeft", { count: initialSlots })}</span>
            </p>
          ) : null}
        </div>

        <div className="space-y-1.5">
          {attempted && !batch.running ? (
            <Button type="button" className={PRIMARY_BUTTON_CLASS} onClick={finish}>
              {t("done")}
            </Button>
          ) : (
            <Button type="button" className={PRIMARY_BUTTON_CLASS} disabled={batch.running} onClick={() => void importAll()}>
              {batch.running ? <SpinnerLabel>{t("importAll")}</SpinnerLabel> : t("importAll")}
            </Button>
          )}
          <Button type="button" variant="ghost" className={CANCEL_BUTTON_CLASS} onClick={onLeave}>
            {t("cancel")}
          </Button>
        </div>
      </div>

      <ModelsProviderDialog
        open={selecting !== null}
        onOpenChange={(open) => !open && setSelecting(null)}
        modelProtocols={modelProtocols}
        initialDraft={selectingDraft}
        source="link"
        lockEndpoint
        onProbe={probe}
        onCreate={async (payload) => {
          const created = await create(payload);
          if (selecting !== null) settle(importedCount, batch.markImported(selecting, created.models.length));
          return created;
        }}
      />
    </ImportFrame>
  );
}

/** Marks pending entries as over the limit once no slots remain. */
function resolveRowStatus(status: ProviderBatchImportStatus | undefined, full: boolean): ProviderBatchImportStatus {
  const current = status ?? { kind: "pending" };
  return full && current.kind !== "imported" ? { kind: "overLimit" } : current;
}

function BatchEntryRow({
  link,
  status,
  existingName,
  disabled,
  onSelectModels,
  onRetry,
}: {
  link: ProviderImportLink;
  status: ProviderBatchImportStatus;
  /** Existing provider on the same host. */
  existingName: string | undefined;
  disabled: boolean;
  onSelectModels: () => void;
  onRetry: () => void;
}) {
  const t = useTranslations("settings.importPage");
  const preset = findModelProviderPreset(link.baseURL);

  return (
    <li className="flex min-w-0 items-center gap-3 px-3.5 py-3">
      <ModelIcon iconUrl={preset ? resolveModelIconURL(preset.icon) : null} label={link.host} />
      <div className="min-w-0 flex-1 space-y-0.5">
        <p className="truncate text-sm font-medium">{link.name || preset?.name || link.host}</p>
        {/* Show the full address; it is what a malicious link would change. */}
        <p className="truncate text-xs text-muted-foreground" title={link.baseURL}>
          {link.baseURL}
        </p>
        <p className="truncate text-[11px] tracking-wider text-muted-foreground/80">{maskProviderApiKey(link.apiKey)}</p>
        <EntryStatus status={status} existingName={existingName} />
      </div>
      {status.kind === "needsSelection" ? (
        <Button type="button" size="sm" variant="outline" className="shrink-0 shadow-none" disabled={disabled} onClick={onSelectModels}>
          {t("actions.selectModels")}
        </Button>
      ) : null}
      {status.kind === "failed" || status.kind === "noModels" ? (
        <Button type="button" size="sm" variant="outline" className="shrink-0 shadow-none" disabled={disabled} onClick={onRetry}>
          {t("actions.retry")}
        </Button>
      ) : null}
    </li>
  );
}

function EntryStatus({ status, existingName }: { status: ProviderBatchImportStatus; existingName: string | undefined }) {
  const t = useTranslations("settings.importPage");
  const warning = "text-amber-700 dark:text-amber-300";

  switch (status.kind) {
    case "pending":
      return existingName ? <StatusLine className={warning}>{t("status.existing", { name: existingName })}</StatusLine> : null;
    case "probing":
    case "saving":
      return (
        <StatusLine className="text-muted-foreground" icon={<Spinner className="size-3" />}>
          {t(`status.${status.kind}`)}
        </StatusLine>
      );
    case "throttled":
      return <StatusLine className="text-muted-foreground">{t("status.throttled", { seconds: status.seconds })}</StatusLine>;
    case "imported":
      return (
        <StatusLine className="text-emerald-700 dark:text-emerald-300" icon={<CheckIcon aria-hidden className="size-3" />}>
          {t("status.imported", { count: status.modelCount })}
        </StatusLine>
      );
    case "needsSelection":
      return <StatusLine className={warning}>{t("status.needsSelection", { count: status.modelCount })}</StatusLine>;
    case "noModels":
      return <StatusLine className={warning}>{t("status.noModels")}</StatusLine>;
    case "overLimit":
      return <StatusLine className={warning}>{t("status.overLimit")}</StatusLine>;
    case "failed":
      return <StatusLine className="text-destructive">{status.message}</StatusLine>;
  }
}

function StatusLine({ className, icon, children }: { className: string; icon?: React.ReactNode; children: React.ReactNode }) {
  return (
    <p className={`flex min-w-0 items-center gap-1 pt-0.5 text-[11px] ${className}`}>
      {icon}
      <span className="min-w-0 truncate" title={typeof children === "string" ? children : undefined}>
        {children}
      </span>
    </p>
  );
}
