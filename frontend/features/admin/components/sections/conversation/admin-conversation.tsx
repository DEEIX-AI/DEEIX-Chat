"use client";

import * as React from "react";
import { CircleHelp, Save } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { TaskModelField, type ModelOption } from "../shared/model-field";
import { SettingsFieldEditor } from "../shared/settings-runtime-panel";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import {
  SettingsFieldItem,
  SettingsFieldList,
  SettingsPage,
  SettingsSection,
  SettingsSectionSeparator,
} from "@/shared/components/settings-layout";
import { getAdminReferenceData, listAdminSettings, patchAdminSettings } from "@/features/admin/api";
import {
  applyConversationDefaults,
  buildConversationSettingsFields,
  CONVERSATION_TASK_MODEL_FOLLOW,
  fieldID,
  flattenConversationSettings,
  resolveErrorMessage,
  toEditorField,
  type ConversationSettingsField,
} from "@/features/admin/model/conversation-settings";
import { buildTaskModelOptions } from "@/features/admin/model/task-model-options";
import type { PatchSettingItem } from "@/shared/api/settings.types";
import {
  HARD_DENIED_MODEL_OPTION_PATHS,
  MODEL_OPTION_POLICY_PROTOCOL_LABELS,
  MODEL_OPTION_POLICY_PROTOCOLS,
  parseModelOptionRuleMap,
  uniqueModelOptionPaths,
  type ModelOptionRuleMap,
} from "@/shared/lib/model-option-policy";

function isModelOptionPolicyField(field: ConversationSettingsField): boolean {
  return field.key.startsWith("model_option_");
}

type ModelOptionPreviewRow = {
  path: string;
  reason: string;
  scope: string;
};

function buildRuleRows(rules: ModelOptionRuleMap, reason: string): ModelOptionPreviewRow[] {
  return MODEL_OPTION_POLICY_PROTOCOLS.flatMap((protocol) => uniqueModelOptionPaths(rules[protocol] ?? []).map((path) => ({
    path,
    reason,
    scope: MODEL_OPTION_POLICY_PROTOCOL_LABELS[protocol],
  })));
}

function ModelOptionPolicyPreview({
  mode,
  allowedPathsJSON,
  deniedPathsJSON,
  t,
}: {
  mode: string;
  allowedPathsJSON: string;
  deniedPathsJSON: string;
  t: (key: string) => string;
}) {
  const allowed = React.useMemo(() => parseModelOptionRuleMap(allowedPathsJSON), [allowedPathsJSON]);
  const denied = React.useMemo(() => parseModelOptionRuleMap(deniedPathsJSON), [deniedPathsJSON]);
  const preview = React.useMemo(
    () => {
      const deniedRows = [
        ...HARD_DENIED_MODEL_OPTION_PATHS.map((path) => ({ path, reason: t("preview.systemDenied"), scope: "Default" })),
        ...(mode === "denylist"
          ? buildRuleRows(denied.value, t("preview.hitDenylist")).filter((row) => !HARD_DENIED_MODEL_OPTION_PATHS.includes(row.path))
          : []),
      ];
      const deniedSet = new Set(deniedRows.map((row) => row.path));
      const passedRows = mode === "denylist"
        ? MODEL_OPTION_POLICY_PROTOCOLS.map((protocol) => ({
          path: t("preview.otherFields"),
          reason: t("preview.notInDenylist"),
          scope: MODEL_OPTION_POLICY_PROTOCOL_LABELS[protocol],
        }))
        : buildRuleRows(allowed.value, t("preview.hitAllowlist")).filter((row) => !deniedSet.has(row.path));
      return {
        passedRows,
        filteredRows: deniedRows,
      };
    },
    [allowed.value, denied.value, mode, t],
  );
  const error = allowed.error || denied.error;

  return (
    <div className="mt-4 space-y-3">
      <p className="text-xs font-medium text-foreground/80">{t("preview.title")}</p>
      {error ? (
        <p className="text-xs text-destructive">{error}</p>
      ) : (
        <div className="grid gap-3 md:grid-cols-2">
          <PreviewPathGroup title={t("preview.passed")} rows={preview.passedRows} emptyText={t("preview.emptyPassed")} />
          <PreviewPathGroup title={t("preview.filtered")} rows={preview.filteredRows} emptyText={t("preview.emptyFiltered")} />
        </div>
      )}
    </div>
  );
}

function PreviewPathGroup({
  title,
  rows,
  emptyText,
}: {
  title: string;
  rows: ModelOptionPreviewRow[];
  emptyText: string;
}) {
  return (
    <div className="space-y-2 rounded-md bg-muted/30 p-2.5">
      <div className="flex items-center justify-between">
        <p className="text-xs font-medium text-foreground/80">{title}</p>
        <span className="text-[11px] text-muted-foreground">{rows.length}</span>
      </div>
      {rows.length === 0 ? (
        <p className="text-xs text-muted-foreground">{emptyText}</p>
      ) : (
        <div className="space-y-1.5">
          {rows.map((row) => (
            <div key={`${row.scope}-${row.path}-${row.reason}`} className="grid gap-0.5">
              <div className="flex min-w-0 items-center gap-2">
                <code className="min-w-0 flex-1 truncate text-xs text-foreground">{row.path}</code>
                <span className="shrink-0 text-[11px] text-muted-foreground">{row.scope}</span>
              </div>
              <span className="text-[11px] text-muted-foreground">{row.reason}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function ModelOptionPolicyGuideButton({ t }: { t: (key: string) => string }) {
  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button type="button" variant="ghost" size="sm" className="h-6 px-2 text-xs font-normal text-muted-foreground hover:text-foreground">
          <CircleHelp className="size-3.5" />
          {t("guide.button")}
        </Button>
      </DialogTrigger>
      <DialogContent className="flex max-h-[min(86vh,760px)] flex-col gap-0 overflow-hidden p-0 sm:max-w-[720px]">
        <DialogHeader className="shrink-0 px-4 py-4">
          <DialogTitle>{t("guide.title")}</DialogTitle>
          <DialogDescription>{t("guide.description")}</DialogDescription>
        </DialogHeader>

        <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 py-2 text-sm text-muted-foreground">
          <section className="space-y-2">
            <h4 className="text-sm font-medium text-foreground">{t("guide.pathTitle")}</h4>
            <div className="grid gap-3 md:grid-cols-2">
              <div className="space-y-1.5">
                <p className="text-xs font-medium text-foreground">options</p>
                <pre className="max-h-44 overflow-auto rounded-md bg-muted/50 p-3 text-xs text-foreground">
{`{
  "temperature": 0.7,
  "thinking": {
    "type": "enabled"
  },
  "generationConfig": {
    "safetySettings": {
      "threshold": "BLOCK_NONE"
    }
  }
}`}
                </pre>
              </div>
              <div className="space-y-1.5">
                <p className="text-xs font-medium text-foreground">{t("guide.pathLabel")}</p>
                <pre className="max-h-44 overflow-auto rounded-md bg-muted/50 p-3 text-xs text-foreground">
{`temperature
thinking.type
generationConfig.safetySettings.threshold`}
                </pre>
              </div>
            </div>
            <p className="text-xs">{t("guide.pathDescription")}</p>
          </section>

          <section className="space-y-2">
            <h4 className="text-sm font-medium text-foreground">{t("guide.strategyTitle")}</h4>
            <Tabs defaultValue="allowlist" className="gap-3">
              <TabsList>
                <TabsTrigger value="allowlist">{t("policy.allowlist")}</TabsTrigger>
                <TabsTrigger value="denylist">{t("policy.denylist")}</TabsTrigger>
              </TabsList>

              <TabsContent value="allowlist" className="space-y-2">
                <p className="text-xs">{t("guide.allowlistDescription")}</p>
                <pre className="max-h-48 overflow-auto rounded-md bg-muted/50 p-3 text-xs text-foreground">
{`{
  "default": [
    "temperature",
    "top_p",
    "stop"
  ],
  "openai_responses": [
    "service_tier",
    "reasoning.effort",
    "text.verbosity"
  ],
  "openai_image_generations": [
    "background",
    "moderation",
    "n",
    "output_compression",
    "output_format",
    "partial_images",
    "quality",
    "size",
    "response_format",
    "style",
    "user"
  ],
  "openai_image_edits": [
    "background",
    "input_fidelity",
    "n",
    "output_compression",
    "output_format",
    "partial_images",
    "quality",
    "response_format",
    "size",
    "user"
  ],
  "google_image_generation": [
    "generationConfig.responseModalities",
    "generationConfig.imageConfig.aspectRatio",
    "generationConfig.imageConfig.imageSize"
  ],
  "xai_image": [
    "aspect_ratio",
    "n",
    "resolution",
    "response_format"
  ],
  "xai_image_edits": [
    "aspect_ratio",
    "n",
    "resolution",
    "response_format"
  ],
  "openai_chat_completions": [
    "service_tier",
    "thinking.type"
  ],
  "anthropic_messages": [
    "speed",
    "thinking.type",
    "thinking.budget_tokens"
  ]
}`}
                </pre>
                <p className="text-xs">{t("guide.openAIServiceTierNote")}</p>
              </TabsContent>

              <TabsContent value="denylist" className="space-y-2">
                <p className="text-xs">{t("guide.denylistDescription")}</p>
                <pre className="max-h-48 overflow-auto rounded-md bg-muted/50 p-3 text-xs text-foreground">
{`{
  "default": [
    "headers",
    "api_key",
    "previous_response_id"
  ],
  "anthropic_messages": [
    "thinking.budget_tokens",
    "metadata.user_id"
  ]
}`}
                </pre>
              </TabsContent>
            </Tabs>
          </section>

          <section className="space-y-2">
            <h4 className="text-sm font-medium text-foreground">{t("guide.protocolTitle")}</h4>
            <p className="text-xs">{t("guide.protocolDescription")}</p>
            <div className="flex flex-wrap gap-1.5">
              {["default", "openai_chat_completions", "openai_responses", "openai_image_generations", "openai_image_edits", "google_image_generation", "xai_image", "xai_image_edits", "anthropic_messages", "xai_responses", "gemini_generate_content"].map((item) => (
                <code key={item} className="rounded-md bg-muted/60 px-2 py-1 text-xs text-foreground">{item}</code>
              ))}
            </div>
          </section>

          <section className="space-y-2">
            <h4 className="text-sm font-medium text-foreground">{t("guide.systemDeniedTitle")}</h4>
            <p className="text-xs">{t("guide.systemDeniedDescription")}</p>
            <div className="flex flex-wrap gap-1.5">
              {HARD_DENIED_MODEL_OPTION_PATHS.map((item) => (
                <code key={item} className="rounded-md bg-muted/60 px-2 py-1 text-xs text-foreground">{item}</code>
              ))}
            </div>
          </section>
        </div>
        <DialogFooter className="shrink-0 px-4 py-3">
          <DialogClose asChild>
            <Button type="button">{t("guide.close")}</Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
export function AdminConversationSettingsPage() {
  const t = useTranslations("adminConversation");
  const commonT = useTranslations("common");
  const conversationSettingsFields = React.useMemo(() => buildConversationSettingsFields(t), [t]);
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  const [settingsMap, setSettingsMap] = React.useState<Record<string, string>>({});
  const [savedMap, setSavedMap] = React.useState<Record<string, string>>({});
  const [modelOptions, setModelOptions] = React.useState<ModelOption[]>(() =>
    buildTaskModelOptions({
      models: [],
      followLabel: t("taskModel.follow"),
      followValue: CONVERSATION_TASK_MODEL_FOLLOW,
    }),
  );

  const loadSettings = React.useCallback(async () => {
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }
      const [grouped, referenceData] = await Promise.all([
        listAdminSettings(token),
        getAdminReferenceData(token).catch(() => null),
      ]);
      const nextModelOptions = buildTaskModelOptions({
        models: referenceData?.models ?? [],
        followLabel: t("taskModel.follow"),
        followValue: CONVERSATION_TASK_MODEL_FOLLOW,
      });
      const flattened = flattenConversationSettings(grouped);
      setModelOptions(nextModelOptions);
      setSettingsMap(flattened);
      setSavedMap(flattened);
    } catch (error) {
      toast.error(t("toast.loadFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setLoading(false);
    }
  }, [t]);

  React.useEffect(() => {
    void loadSettings();
  }, [loadSettings]);

  const dirtyFieldIDs = React.useMemo(() => {
    const result = new Set<string>();
    for (const field of conversationSettingsFields) {
      const id = fieldID(field);
      if ((settingsMap[id] ?? "") !== (savedMap[id] ?? "")) {
        result.add(id);
      }
    }
    return result;
  }, [conversationSettingsFields, savedMap, settingsMap]);

  const handleSave = React.useCallback(async () => {
    const items: PatchSettingItem[] = conversationSettingsFields
      .filter((field) => dirtyFieldIDs.has(fieldID(field)))
      .map((field) => ({
        namespace: field.namespace,
        key: field.key,
        value: settingsMap[fieldID(field)] ?? "",
      }));
    if (items.length === 0) {
      return;
    }
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }
      const grouped = await patchAdminSettings(token, { items });
      const flattened = flattenConversationSettings(grouped);
      setSettingsMap(flattened);
      setSavedMap(flattened);
      toast.success(t("toast.updated"));
    } catch (error) {
      toast.error(t("toast.saveFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }, [conversationSettingsFields, dirtyFieldIDs, settingsMap, t]);

  const conversationFields = React.useMemo(
    () => conversationSettingsFields.filter((field) => !isModelOptionPolicyField(field)),
    [conversationSettingsFields],
  );
  const modelOptionFields = React.useMemo(
    () => conversationSettingsFields.filter(isModelOptionPolicyField),
    [conversationSettingsFields],
  );
  const modelOptionMode = settingsMap["chat.model_option_policy_mode"] || "allowlist";
  const modelOptionModeField = React.useMemo(
    () => modelOptionFields.find((field) => field.key === "model_option_policy_mode") ?? null,
    [modelOptionFields],
  );
  const activeModelOptionRuleField = React.useMemo(
    () => {
      const activeKey = modelOptionMode === "denylist"
        ? "model_option_denied_paths"
        : modelOptionMode === "allowlist"
          ? "model_option_allowed_paths"
          : "";
      if (!activeKey) {
        return null;
      }
      return modelOptionFields.find((field) => field.key === activeKey) ?? null;
    },
    [modelOptionFields, modelOptionMode],
  );
  const hasDirtyField = React.useCallback(
    (fields: ConversationSettingsField[]) => fields.some((field) => dirtyFieldIDs.has(fieldID(field))),
    [dirtyFieldIDs],
  );
  const renderSaveAction = React.useCallback(
    (fields: ConversationSettingsField[]) => hasDirtyField(fields) ? (
      <Button type="button" size="sm" disabled={loading || saving} onClick={() => void handleSave()}>
        <Save className="size-3.5" />
        {commonT("actions.save")}
      </Button>
    ) : null,
    [commonT, handleSave, hasDirtyField, loading, saving],
  );
  const modelOptionActions = renderSaveAction(modelOptionFields);
  const conversationActions = renderSaveAction(conversationFields);

  function renderField(
    field: ConversationSettingsField,
    index: number,
    options?: {
      animateLayout?: boolean;
    },
  ) {
    const id = fieldID(field);
    if (id === "chat.conversation_task_model") {
      return (
        <SettingsFieldItem key={id} index={index}>
          <TaskModelField
            id={id}
            label={field.label}
            description={field.description}
            value={settingsMap[id] ?? ""}
            fallbackValue={CONVERSATION_TASK_MODEL_FOLLOW}
            dirty={(settingsMap[id] ?? "") !== (savedMap[id] ?? "")}
            disabled={loading || saving}
            modelOptions={modelOptions}
            onChange={(value) => setSettingsMap((prev) => ({ ...prev, [id]: value }))}
          />
        </SettingsFieldItem>
      );
    }
    const labelAction =
      field.key === "model_option_allowed_paths" || field.key === "model_option_denied_paths"
        ? <ModelOptionPolicyGuideButton t={t} />
        : undefined;
    const afterControl =
      field.key === "model_option_allowed_paths" || field.key === "model_option_denied_paths" ? (
        <ModelOptionPolicyPreview
          mode={modelOptionMode}
          allowedPathsJSON={settingsMap["chat.model_option_allowed_paths"] ?? ""}
          deniedPathsJSON={settingsMap["chat.model_option_denied_paths"] ?? ""}
          t={t}
        />
      ) : undefined;
    return (
      <SettingsFieldItem key={id} index={index}>
        <SettingsFieldEditor
          field={toEditorField(field)}
          value={settingsMap[id] ?? ""}
          dirty={(settingsMap[id] ?? "") !== (savedMap[id] ?? "")}
          disabled={loading || saving}
          labelAction={labelAction}
          afterControl={afterControl}
          animateLayout={options?.animateLayout ?? true}
          onChange={(value) => setSettingsMap((prev) => ({ ...prev, [id]: value }))}
        />
      </SettingsFieldItem>
    );
  }

  return (
    <SettingsPage>
      <SettingsSection title={t("sections.conversation")} actions={conversationActions}>
        <SettingsFieldList>
          {conversationFields.map((field, index) => renderField(field, index))}
        </SettingsFieldList>
      </SettingsSection>

      <SettingsSectionSeparator />

      <SettingsSection title={t("sections.optionPassthrough")} actions={modelOptionActions}>
        <SettingsFieldList>
          {modelOptionModeField ? renderField(modelOptionModeField, 0) : null}
          {activeModelOptionRuleField
            ? renderField(activeModelOptionRuleField, 1, {
              animateLayout: false,
            })
            : null}
        </SettingsFieldList>
      </SettingsSection>
    </SettingsPage>
  );
}
