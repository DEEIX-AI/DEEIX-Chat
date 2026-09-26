"use client";

import { useLocale, useTranslations } from "next-intl";
import * as React from "react";

import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Spinner } from "@/components/ui/spinner";
import { useUsageBillingLabels } from "@/features/admin/components/sections/logs/admin-usage-log-cells";
import type { LogDetail } from "@/features/admin/hooks/use-admin-logs-actions";
import {
  formatCount,
  formatDateTime,
  formatJSON,
  formatMoneyCents,
  resolveUserDisplayName,
} from "@/features/admin/model/log-display";
import {
  formatTooltipUsageCost,
  formatUsageBalance,
  usageLogRawUsageJSON,
  usageTotalTokens,
} from "@/features/admin/model/usage-log-billing";
import { cn } from "@/lib/utils";
import { CopyActionButton } from "@/shared/components/copy-action";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import type { BillingDisplayOptions } from "@/shared/lib/billing-display";
import { formatBytes } from "@/shared/lib/file-display";

function DetailRow({ label, value, mono = false }: { label: string; value: React.ReactNode; mono?: boolean }) {
  return (
    <div className="grid grid-cols-[88px_minmax(0,1fr)] gap-3 border-b border-border/50 py-2.5 last:border-b-0">
      <p className="text-xs text-muted-foreground">{label}</p>
      <div className={cn("min-w-0 break-words text-xs leading-5 text-foreground/86", mono && "font-mono")}>{value ?? "-"}</div>
    </div>
  );
}

function DetailBlock({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="space-y-2">
      <h4 className="px-1 text-xs font-medium text-foreground/88">{title}</h4>
      <div className="rounded-lg border border-border/60 bg-background px-3">{children}</div>
    </section>
  );
}

function JSONDetailSection({
  title,
  value,
  omitted,
  omittedMessage,
  copyMessages,
  copiedMessage,
}: {
  title: string;
  value: string;
  omitted: boolean;
  omittedMessage?: string;
  copyMessages: { copied: string; failed: string };
  copiedMessage: string;
}) {
  const formatted = formatJSON(value);
  return (
    <section className="space-y-2">
      <div className="flex items-center justify-between gap-3 px-1">
        <h4 className="text-xs font-medium text-foreground/88">{title}</h4>
        <CopyActionButton
          type="button"
          variant="ghost"
          size="sm"
          className="h-7 px-2 text-xs shadow-none"
          value={formatted}
          disabled={omitted}
          messages={copyMessages}
          copyOptions={{ copied: copiedMessage }}
        >
          JSON
        </CopyActionButton>
      </div>
      {omitted ? (
        <p className="rounded-lg border border-border/60 bg-muted/30 px-3 py-2 text-xs leading-5 text-muted-foreground">
          {omittedMessage}
        </p>
      ) : (
        <pre className="max-h-[320px] overflow-auto rounded-lg border border-border/60 bg-muted/35 p-3 text-xs leading-5 text-foreground/86">
          <code>{formatted}</code>
        </pre>
      )}
    </section>
  );
}

export function LogDetailSheet({
  detail: rawDetail,
  billingDisplay,
  conversationDetailLoading,
  onClose,
}: {
  detail: LogDetail | null;
  billingDisplay: BillingDisplayOptions;
  conversationDetailLoading: boolean;
  onClose: () => void;
}) {
  const locale = useLocale();
  const t = useTranslations("adminLogs.detail");
  const usageLabels = useUsageBillingLabels();
  const detail = useDialogSnapshot(rawDetail);
  const copyMessages = React.useMemo(() => ({
    copied: t("copied", { label: "" }).trim(),
    failed: t("copyFailed"),
  }), [t]);
  const resultLabel = React.useCallback(
    (value: string) => {
      switch (value) {
        case "success":
          return t("result.success");
        case "failure":
          return t("result.failure");
        case "blocked":
          return t("result.blocked");
        default:
          return value || "-";
      }
    },
    [t],
  );
  const title =
    detail?.kind === "auth"
      ? t("titles.auth")
      : detail?.kind === "usage"
        ? t("titles.usage")
        : detail?.kind === "order"
          ? t("titles.order")
          : detail?.kind === "redemption"
            ? t("titles.redemption")
          : detail?.kind === "conversation"
            ? t("titles.conversation")
        : detail?.kind === "system"
          ? t("titles.system")
          : t("titles.audit");
  const description =
    detail?.kind === "auth"
      ? `${detail.item.eventType || t("fallbacks.authEvent")} · ${formatDateTime(detail.item.occurredAt, locale)}`
      : detail?.kind === "usage"
        ? `${detail.item.platformModelName || t("fallbacks.modelCall")} · ${formatDateTime(detail.item.createdAt, locale)}`
        : detail?.kind === "order"
          ? `${detail.item.orderNo || t("fallbacks.order")} · ${formatDateTime(detail.item.createdAt, locale)}`
          : detail?.kind === "redemption"
            ? `${detail.item.codeHint || t("fallbacks.redemption")} · ${formatDateTime(detail.item.createdAt, locale)}`
          : detail?.kind === "conversation"
            ? `${detail.item.eventType || detail.item.eventScope || t("fallbacks.conversationEvent")} · ${formatDateTime(detail.item.createdAt, locale)}`
      : detail?.kind === "system"
        ? `${detail.item.event || t("fallbacks.systemEvent")} · ${formatDateTime(detail.item.createdAt, locale)}`
        : `${detail?.item.action || t("fallbacks.auditEvent")} · ${formatDateTime(detail?.item.createdAt, locale)}`;
  const requestID =
    detail && detail.kind !== "usage" && detail.kind !== "order" && detail.kind !== "redemption" && detail.kind !== "conversation"
      ? detail.item.requestID
      : "";
  const detailJSON =
    detail?.kind === "usage"
      ? detail.item.pricingSnapshotJSON
      : detail?.kind === "order" || detail?.kind === "redemption"
        ? detail.item.snapshotJSON
        : detail?.kind === "conversation"
          ? ""
          : detail?.item.detailJSON;
  const rawUsageJSON = detail?.kind === "usage" ? usageLogRawUsageJSON(detail.item) : "";
  const formattedJSON = formatJSON(detailJSON);
  const conversationJSONSections = detail?.kind === "conversation"
    ? [
        {
          key: "payload",
          label: t("jsonFields.payload"),
          value: detail.item.payloadJSON,
          size: detail.item.payloadSizeBytes,
          omitted: detail.item.payloadOmitted,
        },
        {
          key: "input",
          label: t("jsonFields.input"),
          value: detail.item.inputJSON,
          size: detail.item.inputSizeBytes,
          omitted: detail.item.inputOmitted,
        },
        {
          key: "output",
          label: t("jsonFields.output"),
          value: detail.item.outputJSON,
          size: detail.item.outputSizeBytes,
          omitted: detail.item.outputOmitted,
        },
        {
          key: "error",
          label: t("jsonFields.error"),
          value: detail.item.errorJSON,
          size: detail.item.errorSizeBytes,
          omitted: detail.item.errorOmitted,
        },
      ].filter((section) => section.omitted || section.value.trim())
    : [];

  return (
    <Sheet open={Boolean(rawDetail)} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className="sm:max-w-[480px]">
        <SheetHeader>
          <SheetTitle>{title}</SheetTitle>
          <SheetDescription>{description}</SheetDescription>
        </SheetHeader>

        <div className="min-h-0 flex-1 space-y-5 overflow-y-auto px-6 pb-6">
          {detail?.kind === "audit" ? (
            <>
              <DetailBlock title={t("blocks.event")}>
                <DetailRow label="ID" value={detail.item.id} mono />
                <DetailRow label={t("fields.action")} value={detail.item.action} />
                <DetailRow label={t("fields.resource")} value={detail.item.resource} />
                <DetailRow label={t("fields.resourceID")} value={detail.item.resourceID} mono />
                <DetailRow label={t("fields.createdAt")} value={formatDateTime(detail.item.createdAt, locale)} />
              </DetailBlock>
              <DetailBlock title={t("blocks.actor")}>
                <DetailRow label={t("fields.user")} value={resolveUserDisplayName(detail.item.actorLabel, detail.item.actorUsername, detail.item.actorUserID)} />
                <DetailRow label={t("fields.userID")} value={detail.item.actorUserID} mono />
              </DetailBlock>
              <DetailBlock title={t("blocks.request")}>
                <DetailRow label={t("fields.requestID")} value={detail.item.requestID} mono />
                <DetailRow label="IP" value={detail.item.ip} mono />
                <DetailRow label="User Agent" value={detail.item.userAgent} />
              </DetailBlock>
            </>
          ) : null}

          {detail?.kind === "auth" ? (
            <>
              <DetailBlock title={t("blocks.event")}>
                <DetailRow label="ID" value={detail.item.id} mono />
                <DetailRow label={t("fields.event")} value={detail.item.eventType} />
                <DetailRow label={t("fields.result")} value={resultLabel(detail.item.result)} />
                <DetailRow label={t("fields.reason")} value={detail.item.reason} />
                <DetailRow label={t("fields.occurredAt")} value={formatDateTime(detail.item.occurredAt, locale)} />
              </DetailBlock>
              <DetailBlock title={t("blocks.user")}>
                <DetailRow label={t("fields.user")} value={resolveUserDisplayName(detail.item.userLabel, detail.item.username, detail.item.userID)} />
                <DetailRow label={t("fields.userID")} value={detail.item.userID} mono />
              </DetailBlock>
              <DetailBlock title={t("blocks.request")}>
                <DetailRow label={t("fields.requestID")} value={detail.item.requestID} mono />
                <DetailRow label="IP" value={detail.item.clientIP} mono />
                <DetailRow label="User Agent" value={detail.item.userAgent} />
              </DetailBlock>
            </>
          ) : null}

          {detail?.kind === "system" ? (
            <>
              <DetailBlock title={t("blocks.event")}>
                <DetailRow label="ID" value={detail.item.id} mono />
                <DetailRow label={t("fields.level")} value={detail.item.level} />
                <DetailRow label={t("fields.source")} value={detail.item.source} />
                <DetailRow label={t("fields.event")} value={detail.item.event} />
                <DetailRow label={t("fields.message")} value={detail.item.message} />
                <DetailRow label={t("fields.createdAt")} value={formatDateTime(detail.item.createdAt, locale)} />
              </DetailBlock>
              <DetailBlock title={t("blocks.resource")}>
                <DetailRow label={t("fields.resource")} value={detail.item.resource} />
                <DetailRow label={t("fields.resourceID")} value={detail.item.resourceID} mono />
              </DetailBlock>
              <DetailBlock title={t("blocks.request")}>
                <DetailRow label={t("fields.requestID")} value={detail.item.requestID} mono />
                <DetailRow label="Trace ID" value={detail.item.traceID} mono />
              </DetailBlock>
            </>
          ) : null}

          {detail?.kind === "usage" ? (
            <>
              <DetailBlock title={t("blocks.call")}>
                <DetailRow label="ID" value={detail.item.id} mono />
                <DetailRow label={t("fields.caller")} value={resolveUserDisplayName(detail.item.userLabel, detail.item.username, detail.item.userID)} />
                <DetailRow label={t("fields.userID")} value={detail.item.userID} mono />
                <DetailRow label={t("fields.conversationID")} value={detail.item.conversationID} mono />
                <DetailRow label={t("fields.callTime")} value={formatDateTime(detail.item.createdAt, locale)} />
              </DetailBlock>
              <DetailBlock title={t("blocks.modelRoute")}>
                <DetailRow label={t("fields.platformModel")} value={detail.item.platformModelName} mono />
                <DetailRow label={t("fields.upstreamName")} value={detail.item.upstreamName} />
                <DetailRow label={t("fields.upstreamModel")} value={detail.item.upstreamModelName} mono />
                <DetailRow label={t("fields.bindingCode")} value={detail.item.routedBindingCode} mono />
                <DetailRow label={t("fields.protocol")} value={detail.item.providerProtocol} />
              </DetailBlock>
              <DetailBlock title={t("blocks.usageBilling")}>
                <DetailRow label={t("fields.billing")} value={`${formatTooltipUsageCost(detail.item.billedUSD, billingDisplay)} ${detail.item.isFreeModel ? `(${usageLabels.freeModelNoBilling})` : ""}`} />
                <DetailRow label={t("fields.balanceAfter")} value={formatUsageBalance(detail.item.balanceAfterUSD, billingDisplay)} />
                <DetailRow label={t("fields.totalTokens")} value={formatCount(usageTotalTokens(detail.item), locale)} mono />
                <DetailRow label={usageLabels.input} value={formatCount(detail.item.inputTokens, locale)} mono />
                <DetailRow label={usageLabels.cacheRead} value={formatCount(detail.item.cacheReadTokens, locale)} mono />
                <DetailRow label={usageLabels.billingDisplay.cacheWrite} value={formatCount(detail.item.cacheWriteTokens, locale)} mono />
                <DetailRow label={usageLabels.output} value={formatCount(detail.item.outputTokens, locale)} mono />
                <DetailRow label={t("fields.reasoning")} value={formatCount(detail.item.reasoningTokens, locale)} mono />
                <DetailRow label={t("fields.callCount")} value={formatCount(detail.item.callCount, locale)} mono />
                <DetailRow label={t("fields.latency")} value={`${formatCount(detail.item.latencyMS, locale)} ms`} mono />
              </DetailBlock>
            </>
          ) : null}

          {detail?.kind === "usage" ? (
            <section className="space-y-2">
              <div className="flex items-center justify-between gap-3 px-1">
                <h4 className="text-xs font-medium text-foreground/88">{t("rawUsageJsonTitle")}</h4>
                <CopyActionButton
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-7 px-2 text-xs shadow-none"
                  value={rawUsageJSON}
                  messages={copyMessages}
                  copyOptions={{ copied: t("copied", { label: t("rawUsageJsonTitle") }) }}
                >
                  JSON
                </CopyActionButton>
              </div>
              <pre className="max-h-[240px] overflow-auto rounded-lg border border-border/60 bg-muted/35 p-3 text-xs leading-5 text-foreground/86">
                <code>{rawUsageJSON}</code>
              </pre>
            </section>
          ) : null}

          {detail?.kind === "order" ? (
            <>
              <DetailBlock title={t("blocks.order")}>
                <DetailRow label="ID" value={detail.item.id} mono />
                <DetailRow label={t("fields.orderNo")} value={detail.item.orderNo} mono />
                <DetailRow label={t("fields.orderType")} value={detail.item.orderType} />
                <DetailRow label={t("fields.provider")} value={detail.item.provider} />
                <DetailRow label={t("fields.status")} value={detail.item.status} />
                <DetailRow label={t("fields.createdAt")} value={formatDateTime(detail.item.createdAt, locale)} />
                <DetailRow label={t("fields.paidAt")} value={formatDateTime(detail.item.paidAt, locale)} />
              </DetailBlock>
              <DetailBlock title={t("blocks.user")}>
                <DetailRow label={t("fields.user")} value={resolveUserDisplayName(detail.item.userLabel, detail.item.username, detail.item.userID)} />
                <DetailRow label={t("fields.userID")} value={detail.item.userID} mono />
              </DetailBlock>
              <DetailBlock title={t("blocks.payment")}>
                <DetailRow label={t("fields.amount")} value={`${formatMoneyCents(detail.item.payAmountCents, detail.item.payCurrency)} / ${formatMoneyCents(detail.item.baseAmountCents, detail.item.baseCurrency)}`} mono />
                <DetailRow label={t("fields.credit")} value={formatTooltipUsageCost(detail.item.creditUSD, billingDisplay)} mono />
                <DetailRow label={t("fields.interval")} value={`${detail.item.billingInterval || "-"} x ${detail.item.cycles || 0}`} />
                <DetailRow label={t("fields.externalPaymentID")} value={detail.item.externalPaymentID || "-"} mono />
                <DetailRow label={t("fields.externalCheckoutID")} value={detail.item.externalCheckoutID || "-"} mono />
              </DetailBlock>
            </>
          ) : null}

          {detail?.kind === "redemption" ? (
            <>
              <DetailBlock title={t("blocks.redemption")}>
                <DetailRow label="ID" value={detail.item.id} mono />
                <DetailRow label={t("fields.refNo")} value={detail.item.refNo || "-"} mono />
                <DetailRow label={t("fields.redeemedAt")} value={formatDateTime(detail.item.createdAt, locale)} />
              </DetailBlock>
              <DetailBlock title={t("blocks.user")}>
                <DetailRow label={t("fields.user")} value={resolveUserDisplayName(detail.item.userLabel, detail.item.username, detail.item.userID)} />
                <DetailRow label={t("fields.userID")} value={detail.item.userID} mono />
              </DetailBlock>
              <DetailBlock title={t("blocks.redemptionCode")}>
                <DetailRow label={t("fields.codeHint")} value={detail.item.codeHint || "-"} mono />
                <DetailRow label={t("fields.codeID")} value={detail.item.codeID} mono />
                <DetailRow label={t("fields.codeDescription")} value={detail.item.codeDescription || "-"} />
                <DetailRow
                  label={t("fields.codeStatus")}
                  value={
                    detail.item.codeStatus === "active" || detail.item.codeStatus === "inactive" || detail.item.codeStatus === "deleted"
                      ? t(`codeStatus.${detail.item.codeStatus}`)
                      : detail.item.codeStatus || "-"
                  }
                />
              </DetailBlock>
              <DetailBlock title={t("blocks.reward")}>
                <DetailRow
                  label={t("fields.rewardType")}
                  value={detail.item.rewardType === "subscription" ? t("rewardTypes.subscription") : t("rewardTypes.balance")}
                />
                {detail.item.rewardType === "subscription" ? (
                  <>
                    <DetailRow label={t("fields.planName")} value={detail.item.planName || `#${detail.item.planID}`} />
                    {detail.item.durationDays > 0 ? (
                      <DetailRow
                        label={t("fields.durationDays")}
                        value={t("rewardDurationDays", { count: detail.item.durationDays })}
                      />
                    ) : null}
                    <DetailRow label={t("fields.subscriptionID")} value={detail.item.subscriptionID || "-"} mono />
                  </>
                ) : (
                  <>
                    <DetailRow label={t("fields.credit")} value={formatTooltipUsageCost(detail.item.creditUSD, billingDisplay)} mono />
                    <DetailRow
                      label={t("fields.balanceBefore")}
                      value={
                        typeof detail.item.balanceBeforeNanousd === "number"
                          ? formatUsageBalance(detail.item.balanceBeforeNanousd / 1_000_000_000, billingDisplay)
                          : "-"
                      }
                      mono
                    />
                    <DetailRow
                      label={t("fields.balanceAfterRedemption")}
                      value={
                        typeof detail.item.balanceAfterNanousd === "number"
                          ? formatUsageBalance(detail.item.balanceAfterNanousd / 1_000_000_000, billingDisplay)
                          : "-"
                      }
                      mono
                    />
                  </>
                )}
              </DetailBlock>
            </>
          ) : null}

          {detail?.kind === "conversation" ? (
            <>
              <DetailBlock title={t("blocks.conversationEvent")}>
                <DetailRow label="ID" value={detail.item.id} mono />
                <DetailRow label={t("fields.runID")} value={detail.item.runID} mono />
                <DetailRow label={t("fields.eventScope")} value={detail.item.eventScope} />
                <DetailRow label={t("fields.event")} value={detail.item.eventType} />
                <DetailRow label={t("fields.status")} value={detail.item.status} />
                <DetailRow label={t("fields.stage")} value={detail.item.stage || detail.item.phase || "-"} />
                <DetailRow label={t("fields.seq")} value={detail.item.seq} mono />
                <DetailRow label={t("fields.createdAt")} value={formatDateTime(detail.item.createdAt, locale)} />
              </DetailBlock>
              <DetailBlock title={t("blocks.user")}>
                <DetailRow label={t("fields.user")} value={resolveUserDisplayName(detail.item.userLabel, detail.item.username, detail.item.userID)} />
                <DetailRow label={t("fields.userID")} value={detail.item.userID} mono />
                <DetailRow label={t("fields.conversationID")} value={detail.item.conversationID} mono />
                <DetailRow label={t("fields.messageID")} value={detail.item.messageID} mono />
              </DetailBlock>
              <DetailBlock title={t("blocks.modelRoute")}>
                <DetailRow label={t("fields.platformModel")} value={detail.item.platformModelName || "-"} mono />
                <DetailRow label={t("fields.upstreamName")} value={detail.item.upstreamName || "-"} />
                <DetailRow label={t("fields.upstreamModel")} value={detail.item.upstreamModelName || "-"} mono />
                <DetailRow label={t("fields.bindingCode")} value={detail.item.routedBindingCode || "-"} mono />
                <DetailRow label={t("fields.protocol")} value={detail.item.providerProtocol || "-"} />
              </DetailBlock>
              <DetailBlock title={t("blocks.tool")}>
                <DetailRow label={t("fields.toolName")} value={detail.item.toolName || "-"} />
                <DetailRow label={t("fields.toolCallID")} value={detail.item.toolCallID || "-"} mono />
                <DetailRow label={t("fields.latency")} value={`${formatCount(detail.item.latencyMS, locale)} ms`} mono />
                <DetailRow label={t("fields.title")} value={detail.item.title || "-"} />
                <DetailRow label={t("fields.summary")} value={detail.item.summary || "-"} />
                <DetailRow label={t("fields.payloadSize")} value={formatBytes(detail.item.payloadSizeBytes)} mono />
              </DetailBlock>
            </>
          ) : null}

          {conversationDetailLoading ? (
            <div className="flex h-28 items-center justify-center rounded-lg border border-border/60 bg-muted/20">
              <Spinner label={t("loading")} className="size-4 text-muted-foreground" />
            </div>
          ) : detail?.kind === "conversation" ? (
            conversationJSONSections.length > 0 ? (
              conversationJSONSections.map((section) => (
                <JSONDetailSection
                  key={section.key}
                  title={section.label}
                  value={section.value}
                  omitted={section.omitted}
                  omittedMessage={t("jsonOmitted", {
                    label: section.label,
                    size: formatBytes(section.size),
                  })}
                  copyMessages={copyMessages}
                  copiedMessage={t("copied", { label: section.label })}
                />
              ))
            ) : (
              <p className="rounded-lg border border-border/60 bg-muted/20 px-3 py-6 text-center text-xs text-muted-foreground">
                {t("jsonEmpty")}
              </p>
            )
          ) : (
            <section className="space-y-2">
              <div className="flex items-center justify-between gap-3 px-1">
                <h4 className="text-xs font-medium text-foreground/88">{t("jsonTitle")}</h4>
                <div className="flex items-center gap-1">
                  {requestID ? (
                    <CopyActionButton
                      type="button"
                      variant="ghost"
                      size="sm"
                      className="h-7 px-2 text-xs shadow-none"
                      value={requestID}
                      messages={copyMessages}
                      copyOptions={{ copied: t("copied", { label: t("fields.requestID") }) }}
                    >
                      {t("fields.requestID")}
                    </CopyActionButton>
                  ) : null}
                  <CopyActionButton
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="h-7 px-2 text-xs shadow-none"
                    value={formattedJSON}
                    messages={copyMessages}
                    copyOptions={{ copied: t("copied", { label: t("jsonTitle") }) }}
                  >
                    JSON
                  </CopyActionButton>
                </div>
              </div>
              <pre className="max-h-[320px] overflow-auto rounded-lg border border-border/60 bg-muted/35 p-3 text-xs leading-5 text-foreground/86">
                <code>{formattedJSON}</code>
              </pre>
            </section>
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}
