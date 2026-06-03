"use client";

import * as React from "react";
import { BugPlay, Pause, Play, RefreshCw } from "lucide-react";
import { motion } from "motion/react";
import { useTranslations } from "next-intl";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { JsonCodeEditor } from "@/shared/components/json-code-editor";

export type SettingsFieldType = "int" | "bool" | "string" | "password" | "textarea" | "json" | "select" | "tabs" | "button";

export type SettingsFieldOption = {
  label: string;
  value: string;
};

export type SettingsFieldAction = {
  key: string;
  label: string;
  icon?: React.ComponentType<{ className?: string }>;
  disabled?: boolean;
  spinning?: boolean;
  onClick: () => void;
};

export type SettingsFieldDefinition = {
  id: string;
  label: string;
  description: string;
  type: SettingsFieldType;
  placeholder?: string;
  valueUnit?: "mb";
  options?: SettingsFieldOption[];
  actions?: SettingsFieldAction[];
  statusBadge?: ServiceRuntimeStatusBadge;
  loading?: boolean;
  serviceRuntime?: SettingsFieldServiceRuntime;
};

export type ServiceRuntimeAction = SettingsFieldAction;

export type ServiceRuntimeIconKey = "bugplay" | "play" | "pause" | "refresh";
export type ServiceRuntimeActionName = "start" | "stop" | "restart" | "test";
export type ServiceRuntimeActionKey = "toggle" | "restart" | "test";
export type ServiceRuntimeStatusKey = "running" | "stopped";
export type ServiceRuntimeStatusBadgeTone = "neutral" | "success" | "warning" | "error";

export type ServiceRuntimeStatusBadge = {
  label: string;
  tone?: ServiceRuntimeStatusBadgeTone;
  detail?: React.ReactNode;
};

export type ServiceRuntimeActionSchema = {
  key: ServiceRuntimeActionKey;
  label: string;
  icon: ServiceRuntimeIconKey;
  action?: ServiceRuntimeActionName;
  spinWhen?: ServiceRuntimeActionName;
  variants?: Partial<
    Record<
      ServiceRuntimeStatusKey,
      {
        label: string;
        icon: ServiceRuntimeIconKey;
        action: Extract<ServiceRuntimeActionName, "start" | "stop">;
      }
    >
  >;
};

export type ServiceRuntimeDetailItem = {
  label: string;
  value?: React.ReactNode;
};

export type ServiceRuntimeState = {
  status?: string;
  reachable?: boolean;
  message?: string;
  details?: ServiceRuntimeDetailItem[];
};

export type SettingsFieldServiceRuntime = {
  runtime?: ServiceRuntimeState | null;
  loading?: boolean;
  actionDisabled?: boolean;
  pendingAction?: ServiceRuntimeActionName | "";
  actions?: ServiceRuntimeActionSchema[];
  onAction?: (action: ServiceRuntimeActionName) => void;
};

export type ServiceRuntimePanelProps = {
  label: string;
  description: string;
  runtime?: ServiceRuntimeState | null;
  dirty?: boolean;
  dirtyMessage?: string;
  loading?: boolean;
  loadingMessage?: string;
  actionDisabled?: boolean;
  pendingAction?: ServiceRuntimeActionName | "";
  actions: ServiceRuntimeActionSchema[];
  onAction?: (action: ServiceRuntimeActionName) => void;
};

function resolveServiceRuntimeBadgeClassName(tone: ServiceRuntimeStatusBadgeTone = "neutral"): string {
  switch (tone) {
    case "success":
      return "border-emerald-200 bg-emerald-50 text-emerald-700 hover:bg-emerald-50";
    case "warning":
      return "border-amber-200 bg-amber-50 text-amber-700 hover:bg-amber-50";
    case "error":
      return "border-red-200 bg-red-50 text-red-700 hover:bg-red-50";
    default:
      return "border-border/70 bg-muted/40 text-muted-foreground hover:bg-accent/40";
  }
}

const RUNTIME_ICON_MAP = {
  bugplay: BugPlay,
  play: Play,
  pause: Pause,
  refresh: RefreshCw,
} as const;

function resolveRuntimeStatusKey(status?: string): ServiceRuntimeStatusKey {
  return status === "running" ? "running" : "stopped";
}

type RuntimeStatusLabels = {
  running: string;
  restarting: string;
  created: string;
  paused: string;
  unhealthy: string;
  failed: string;
  unavailable: string;
  unconfigured: string;
  stopped: string;
  unknown: string;
};

function resolveRuntimeStatusLabel(status: string | undefined, labels: RuntimeStatusLabels): string {
  switch (status) {
    case "running":
      return labels.running;
    case "restarting":
      return labels.restarting;
    case "created":
      return labels.created;
    case "paused":
      return labels.paused;
    case "unhealthy":
      return labels.unhealthy;
    case "failed":
      return labels.failed;
    case "unavailable":
      return labels.unavailable;
    case "unconfigured":
      return labels.unconfigured;
    case "stopped":
    case "exited":
      return labels.stopped;
    default:
      return status?.trim() ? status : labels.unknown;
  }
}

function resolveRuntimeStatusBadgeTone(runtime?: ServiceRuntimeState | null): ServiceRuntimeStatusBadgeTone {
  if (!runtime) {
    return "neutral";
  }
  switch (runtime.status) {
    case "running":
      return runtime.reachable ? "success" : "warning";
    case "restarting":
    case "created":
      return "warning";
    case "unhealthy":
    case "failed":
    case "unavailable":
      return "error";
    default:
      return "neutral";
  }
}

function resolveRuntimeMessageToneClassName({
  runtime,
  dirty,
  loading,
  pendingAction,
}: Pick<ServiceRuntimePanelProps, "runtime" | "dirty" | "loading" | "pendingAction">): string {
  if (dirty) {
    return "text-amber-700";
  }
  if (pendingAction) {
    return "text-muted-foreground";
  }
  if (loading) {
    return "text-muted-foreground";
  }
  switch (runtime?.status) {
    case "running":
      return "text-emerald-700";
    case "unhealthy":
      return "text-amber-700";
    case "failed":
    case "unavailable":
      return "text-red-700";
    default:
      return "text-muted-foreground";
  }
}

function buildRuntimeTooltipDetail(
  runtime: ServiceRuntimeState | null | undefined,
  labels: { status: string; connection: string; reachable: string; unreachable: string; detail: string },
): React.ReactNode {
  const lines = [
    runtime?.status ? { label: labels.status, value: runtime.status } : undefined,
    typeof runtime?.reachable === "boolean" ? { label: labels.connection, value: runtime.reachable ? labels.reachable : labels.unreachable } : undefined,
    ...(runtime?.details ?? []),
    runtime?.message?.trim() ? { label: labels.detail, value: runtime.message.trim() } : undefined,
  ].filter(Boolean) as ServiceRuntimeDetailItem[];

  if (lines.length === 0) {
    return undefined;
  }

  return (
    <div className="space-y-1">
      {lines.map((line, index) => (
        <p key={`${line.label}-${index}`}>
          {line.label}：{line.value}
        </p>
      ))}
    </div>
  );
}

function resolveRuntimeMessage({
  runtime,
  dirty,
  dirtyMessage,
  loading,
  loadingMessage,
}: Pick<ServiceRuntimePanelProps, "runtime" | "dirty" | "dirtyMessage" | "loading" | "loadingMessage">): string | undefined {
  if (dirty && dirtyMessage?.trim()) {
    return dirtyMessage.trim();
  }
  if (runtime?.status === "running" && runtime.reachable) {
    return undefined;
  }
  if (loading && loadingMessage?.trim()) {
    return loadingMessage.trim();
  }
  return undefined;
}

function resolveRuntimeIcon(icon: ServiceRuntimeIconKey) {
  return RUNTIME_ICON_MAP[icon];
}

const layoutTransition = { duration: 0.2, ease: [0.22, 1, 0.36, 1] } as const;

export function SettingsFieldEditor({
  field,
  value,
  configured,
  dirty,
  disabled,
  labelAction,
  afterControl,
  onChange,
}: {
  field: SettingsFieldDefinition;
  value: string;
  configured?: boolean;
  dirty: boolean;
  disabled: boolean;
  labelAction?: React.ReactNode;
  afterControl?: React.ReactNode;
  onChange?: (value: string) => void;
}) {
  const t = useTranslations("common");
  const runtimeStatusLabels = React.useMemo<RuntimeStatusLabels>(
    () => ({
      running: t("runtime.status.running"),
      restarting: t("runtime.status.restarting"),
      created: t("runtime.status.created"),
      paused: t("runtime.status.paused"),
      unhealthy: t("runtime.status.unhealthy"),
      failed: t("runtime.status.failed"),
      unavailable: t("runtime.status.unavailable"),
      unconfigured: t("runtime.status.unconfigured"),
      stopped: t("runtime.status.stopped"),
      unknown: t("states.unknown"),
    }),
    [t],
  );
  const runtimeDetailLabels = React.useMemo(
    () => ({
      status: t("runtime.detail.status"),
      connection: t("runtime.detail.connection"),
      reachable: t("runtime.detail.reachable"),
      unreachable: t("runtime.detail.unreachable"),
      detail: t("runtime.detail.detail"),
    }),
    [t],
  );
  const dirtyBadge = dirty ? <Badge variant="ghost" className="relative -mt-1.5 text-[8px] font-medium text-amber-800">{t("states.unsaved")}</Badge> : null;
  const inlineRuntimeStatusBadge =
    field.statusBadge ??
    (field.serviceRuntime
      ? {
          label: resolveRuntimeStatusLabel(field.serviceRuntime.runtime?.status, runtimeStatusLabels),
          tone: resolveRuntimeStatusBadgeTone(field.serviceRuntime.runtime),
          detail: buildRuntimeTooltipDetail(field.serviceRuntime.runtime, runtimeDetailLabels),
        }
      : undefined);
  const inlineRuntimeActions =
    field.actions ??
    field.serviceRuntime?.actions?.map((action) => {
      const statusKey = resolveRuntimeStatusKey(field.serviceRuntime?.runtime?.status);
      const resolvedVariant = action.variants?.[statusKey];
      const resolvedAction = resolvedVariant?.action ?? action.action;
      const Icon = resolveRuntimeIcon(resolvedVariant?.icon ?? action.icon);
      return {
        key: action.key,
        label: resolvedVariant?.label ?? action.label,
        icon: Icon,
        disabled: field.serviceRuntime?.actionDisabled,
        spinning: action.spinWhen ? field.serviceRuntime?.pendingAction === action.spinWhen : false,
        onClick: () => {
          if (resolvedAction) {
            field.serviceRuntime?.onAction?.(resolvedAction);
          }
        },
      } satisfies SettingsFieldAction;
    });
  const inlineRuntimeLoading = field.loading ?? field.serviceRuntime?.loading;
  const statusBadgeNode = inlineRuntimeStatusBadge ? (
    <Badge
      variant="outline"
      className={cn(
        "relative -mt-1.5 border-border/50 bg-muted/40 text-muted-foreground hover:bg-accent/40",
        resolveServiceRuntimeBadgeClassName(inlineRuntimeStatusBadge.tone),
      )}
    >
      {inlineRuntimeStatusBadge.label}
    </Badge>
  ) : null;
  const labelMetaNode = (
    <>
      {statusBadgeNode && inlineRuntimeStatusBadge?.detail ? (
        <Tooltip>
          <TooltipTrigger asChild>{statusBadgeNode}</TooltipTrigger>
          <TooltipContent sideOffset={6} className="max-w-xs whitespace-pre-line break-words text-xs text-left">
            {inlineRuntimeStatusBadge.detail}
          </TooltipContent>
        </Tooltip>
      ) : (
        statusBadgeNode
      )}
      {inlineRuntimeLoading ? <Spinner className="relative -top-px size-3.5 text-muted-foreground" /> : null}
    </>
  );

  if (field.type === "textarea" || field.type === "json") {
    return (
      <motion.div layout transition={layoutTransition} className="min-w-0">
        <Field>
          <div className="min-w-0 space-y-2">
            <div>
              <div className="flex items-center justify-between gap-3">
                <div className="min-w-0 flex items-center gap-2">
                  <FieldLabel htmlFor={field.id}>{field.label}</FieldLabel>
                  {dirtyBadge}
                </div>
                {labelAction ? <div className="shrink-0">{labelAction}</div> : null}
              </div>
              {field.description ? <FieldDescription className="text-[11px]">{field.description}</FieldDescription> : null}
            </div>

            {field.type === "json" ? (
              <JsonCodeEditor
                id={field.id}
                value={value}
                onChange={(nextValue) => onChange?.(nextValue)}
                placeholder={field.placeholder}
                height={260}
                disabled={disabled}
              />
            ) : (
              <Textarea
                id={field.id}
                value={value}
                onChange={(event) => onChange?.(event.target.value)}
                placeholder={field.placeholder}
                className="h-28 resize-none overflow-y-auto [field-sizing:fixed]"
                disabled={disabled}
              />
            )}
            {afterControl}
          </div>
        </Field>
      </motion.div>
    );
  }

  return (
    <motion.div layout transition={layoutTransition} className="min-w-0">
      <Field>
        <div className="flex min-w-0 flex-col gap-2 md:flex-row md:items-start md:gap-4 xl:gap-6">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <FieldLabel htmlFor={field.id}>{field.label}</FieldLabel>
              {labelMetaNode}
              {dirtyBadge}
            </div>
            {field.description ? <FieldDescription className="text-[11px]">{field.description}</FieldDescription> : null}
          </div>

          <div className="w-full min-w-0 md:w-44 md:shrink-0 xl:w-52">
            {field.type === "bool" ? (
              <div className="flex items-end justify-start md:justify-end">
                <Switch id={field.id} checked={value === "true"} onCheckedChange={(checked) => onChange?.(checked ? "true" : "false")} disabled={disabled} />
              </div>
            ) : field.type === "tabs" ? (
              <Tabs value={value} onValueChange={(next) => onChange?.(next)} className="w-full">
                <TabsList className="grid h-8 w-full grid-cols-2">
                  {(field.options ?? []).map((option) => (
                    <TabsTrigger key={option.value} value={option.value} disabled={disabled}>
                      {option.label}
                    </TabsTrigger>
                  ))}
                </TabsList>
              </Tabs>
            ) : field.type === "button" ? (
              <div className="flex items-center justify-start gap-2 md:justify-end">
                {(inlineRuntimeActions ?? []).map((action) => {
                  const Icon = action.icon;
                  return (
                    <Button
                      key={action.key}
                      type="button"
                      variant="outline"
                      size="sm"
                      className="h-7 rounded-md border-border/50 px-2.5 text-[12px] font-normal shadow-none hover:bg-accent/40"
                      disabled={disabled || action.disabled}
                      onClick={action.onClick}
                    >
                      {Icon ? <Icon className={cn("size-3.5 stroke-1", action.spinning ? "animate-spin" : "")} /> : null}
                      {action.label}
                    </Button>
                  );
                })}
              </div>
            ) : field.type === "select" ? (
              <Select value={value} onValueChange={(next) => onChange?.(next)} disabled={disabled}>
                <SelectTrigger
                  id={field.id}
                  size="sm"
                  className="text-left md:text-right *:data-[slot=select-value]:flex-1 *:data-[slot=select-value]:justify-start md:*:data-[slot=select-value]:justify-end"
                >
                  <SelectValue placeholder={field.placeholder ?? t("select.placeholder")} />
                </SelectTrigger>
                <SelectContent align="start">
                  {(field.options ?? []).map((option) => (
                    <SelectItem key={option.value} value={option.value} className="text-left md:text-right">
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            ) : (
              <div className="grid w-full min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-1">
                <Input
                  id={field.id}
                  type={field.type === "password" ? "password" : "text"}
                  value={value}
                  onChange={(event) => onChange?.(event.target.value)}
                  placeholder={field.type === "password" && configured && !value ? t("input.configuredPasswordPlaceholder") : field.placeholder}
                  inputMode={field.valueUnit === "mb" ? "decimal" : field.type === "int" ? "numeric" : "text"}
                  disabled={disabled}
                  className="min-w-0 text-left md:text-right"
                />
                {(inlineRuntimeActions ?? []).map((action) => {
                  const Icon = action.icon;
                  return (
                    <Button
                      key={action.key}
                      type="button"
                      variant="secondary"
                      size="icon"
                      className="size-8 shrink-0 rounded-md shadow-none active:scale-90 transition-transform"
                      disabled={disabled || action.disabled}
                      onClick={action.onClick}
                    >
                      {Icon ? <Icon className={cn("size-3.5 stroke-1", action.spinning ? "animate-spin" : "")} /> : null}
                    </Button>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      </Field>
    </motion.div>
  );
}

export function ServiceRuntimePanel({
  label,
  runtime,
  dirty,
  dirtyMessage,
  loading,
  loadingMessage,
  actionDisabled,
  pendingAction,
  actions,
  onAction,
}: ServiceRuntimePanelProps) {
  const t = useTranslations("common");
  const runtimeStatusLabels = React.useMemo<RuntimeStatusLabels>(
    () => ({
      running: t("runtime.status.running"),
      restarting: t("runtime.status.restarting"),
      created: t("runtime.status.created"),
      paused: t("runtime.status.paused"),
      unhealthy: t("runtime.status.unhealthy"),
      failed: t("runtime.status.failed"),
      unavailable: t("runtime.status.unavailable"),
      unconfigured: t("runtime.status.unconfigured"),
      stopped: t("runtime.status.stopped"),
      unknown: t("states.unknown"),
    }),
    [t],
  );
  const runtimeDetailLabels = React.useMemo(
    () => ({
      status: t("runtime.detail.status"),
      connection: t("runtime.detail.connection"),
      reachable: t("runtime.detail.reachable"),
      unreachable: t("runtime.detail.unreachable"),
      detail: t("runtime.detail.detail"),
    }),
    [t],
  );
  const statusBadge: ServiceRuntimeStatusBadge = {
    label: resolveRuntimeStatusLabel(runtime?.status, runtimeStatusLabels),
    tone: resolveRuntimeStatusBadgeTone(runtime),
    detail: buildRuntimeTooltipDetail(runtime, runtimeDetailLabels),
  };
  const statusKey = resolveRuntimeStatusKey(runtime?.status);
  const message = resolveRuntimeMessage({ runtime, dirty, dirtyMessage, loading, loadingMessage });
  const messageToneClassName = resolveRuntimeMessageToneClassName({ runtime, dirty, loading, pendingAction });
  const badgeNode = statusBadge ? (
    <Badge
      variant="ghost"
      className={cn(
        "relative rounded-full text-[8px] font-medium shadow-none",
        resolveServiceRuntimeBadgeClassName(statusBadge.tone),
      )}
    >
      {statusBadge.label}
    </Badge>
  ) : null;

  return (
    <motion.div layout transition={layoutTransition} className="min-w-0">
      <Field>
        <div className="flex min-w-0 flex-col gap-2 md:flex-row md:items-start md:justify-between md:gap-4 xl:gap-6">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <FieldLabel className="items-center leading-none">{label}</FieldLabel>
              {badgeNode && statusBadge?.detail ? (
                <Tooltip>
                  <TooltipTrigger asChild>{badgeNode}</TooltipTrigger>
                  <TooltipContent sideOffset={6} className="max-w-sm whitespace-pre-line break-words text-left">
                    {statusBadge.detail}
                  </TooltipContent>
                </Tooltip>
              ) : (
                badgeNode
              )}
              {loading ? <Spinner className="relative -top-px size-3.5 text-muted-foreground" /> : null}
            </div>
            {message ? <p className={cn("break-words text-[11px]", messageToneClassName)}>{message}</p> : null}
          </div>

          <div className="w-full min-w-0 md:w-44 md:shrink-0 xl:w-52">
            <SettingsFieldEditor
              field={{
                id: `${label}-actions`,
                label: "",
                description: "",
                type: "button",
                actions: actions.map((action) => {
                  const resolvedVariant = action.variants?.[statusKey];
                  const resolvedAction = resolvedVariant?.action ?? action.action;
                  const Icon = resolveRuntimeIcon(resolvedVariant?.icon ?? action.icon);
                  return {
                    key: action.key,
                    label: resolvedVariant?.label ?? action.label,
                    icon: Icon,
                    disabled: actionDisabled,
                    spinning: action.spinWhen ? pendingAction === action.spinWhen : false,
                    onClick: () => {
                      if (resolvedAction) {
                        onAction?.(resolvedAction);
                      }
                    },
                  } satisfies ServiceRuntimeAction;
                }),
              }}
              value=""
              dirty={false}
              disabled={false}
            />
          </div>
        </div>
      </Field>
    </motion.div>
  );
}

export function joinServiceRuntimeMeta(parts: Array<string | undefined | null | false>): string | undefined {
  const items = parts.map((part) => (typeof part === "string" ? part.trim() : "")).filter(Boolean);
  if (items.length === 0) {
    return undefined;
  }
  return items.join(" · ");
}
