"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Pencil, Trash2, Plus } from "lucide-react";

import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { useSettingsChat } from "@/features/settings/hooks/use-settings-chat";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { listUserMemories, upsertUserMemory, deleteUserMemory } from "@/shared/api/memory";
import type { UserMemoryDTO } from "@/shared/api/memory.types";
import { ModelSelect, type ModelSelectOption } from "@/shared/components/model-select";
import {
  SettingsFieldList,
  SettingsFieldRow,
  SettingsPage,
  SettingsSection,
  SettingsSectionSeparator,
} from "@/shared/components/settings-layout";
import { resolveModelOptionIconUrl, resolveModelOptionLabel } from "@/shared/lib/model-option-display";
import { parseKindsJSON } from "@/shared/model/llm-schema";
import { platformModifierLabel, platformSendShortcut } from "@/shared/lib/platform-shortcuts";
import type { SendShortcut } from "@/features/settings/types/settings";

type ModelOption = ModelSelectOption;

const SYSTEM_RECOMMENDED_MODEL = "none";

// Preference memory section.

const MAX_PREFERENCES = 20;

function PreferenceCard({
  item,
  onEdit,
  onDelete,
}: {
  item: UserMemoryDTO;
  onEdit: (key: string, value: string) => void;
  onDelete: (key: string) => void;
}) {
  const t = useTranslations("settings.chatPage.memory");
  const [editingValue, setEditingValue] = React.useState<string | null>(null);
  const [confirmDelete, setConfirmDelete] = React.useState(false);
  const [saving, setSaving] = React.useState(false);

  const startEdit = () => {
    setConfirmDelete(false);
    setEditingValue(item.value);
  };

  const cancelEdit = () => setEditingValue(null);

  const commitEdit = async () => {
    const v = (editingValue ?? "").trim();
    if (!v || v === item.value) { setEditingValue(null); return; }
    setSaving(true);
    await onEdit(item.memoryKey, v);
    setSaving(false);
    setEditingValue(null);
  };

  const commitDelete = () => onDelete(item.memoryKey);

  if (editingValue !== null) {
    return (
      <div className="rounded-md border border-border/60 bg-background px-2.5 py-2">
        <div className="flex gap-2">
          <div className="min-w-0 flex-1">
            <p className="mb-1 text-[11px] font-medium text-foreground/80">{item.memoryKey}</p>
            <textarea
              autoFocus
              rows={2}
              value={editingValue}
              className="min-h-14 w-full resize-none rounded-md border border-input bg-background px-2.5 py-1.5 text-xs leading-relaxed outline-none focus:border-ring"
              onChange={(e) => setEditingValue(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); void commitEdit(); }
                if (e.key === "Escape") cancelEdit();
              }}
            />
          </div>
          <div className="flex shrink-0 flex-col gap-1 pt-5">
            <Button size="sm" className="h-6 px-2 text-[11px]" onClick={() => void commitEdit()} disabled={saving || !(editingValue ?? "").trim()}>
              {t("save")}
            </Button>
            <Button size="sm" variant="ghost" className="h-6 px-2 text-[11px]" onClick={cancelEdit}>
              {t("cancel")}
            </Button>
          </div>
        </div>
      </div>
    );
  }

  if (confirmDelete) {
    return (
      <div className="flex items-center justify-between gap-3 rounded-md bg-destructive/8 px-2.5 py-2">
        <div className="min-w-0">
          <p className="truncate text-xs font-medium">{item.memoryKey}</p>
          <p className="mt-0.5 text-[11px] text-destructive">{t("confirmDelete")}</p>
        </div>
        <div className="flex shrink-0 gap-1">
          <Button size="sm" variant="ghost" className="h-6 px-2 text-[11px]" onClick={() => setConfirmDelete(false)}>{t("cancel")}</Button>
          <Button size="sm" variant="destructive" className="h-6 px-2 text-[11px]" onClick={commitDelete}>{t("delete")}</Button>
        </div>
      </div>
    );
  }

  return (
    <div className="group flex min-h-9 items-center gap-2 rounded-md px-2 py-1.5 transition-colors hover:bg-muted/40">
      <p className="min-w-0 flex-1 truncate text-xs leading-5">
        <span className="font-medium text-foreground/80">{item.memoryKey}</span>
        <span className="text-muted-foreground">{t("separator")}{item.value}</span>
      </p>

      <div className="flex shrink-0 gap-0.5 opacity-100 transition-opacity md:opacity-0 md:group-hover:opacity-100 md:group-focus-within:opacity-100">
        <button
          type="button"
          className="inline-flex size-6 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
          onClick={startEdit}
          aria-label={t("edit")}
        >
          <Pencil className="h-3 w-3" />
        </button>
        <button
          type="button"
          className="inline-flex size-6 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-destructive"
          onClick={() => setConfirmDelete(true)}
          aria-label={t("delete")}
        >
          <Trash2 className="h-3 w-3" />
        </button>
      </div>
    </div>
  );
}

function AddPreferenceDialog({
  open,
  addKey,
  addValue,
  adding,
  atLimit,
  onOpenChange,
  onKeyChange,
  onValueChange,
  onAdd,
}: {
  open: boolean;
  addKey: string;
  addValue: string;
  adding: boolean;
  atLimit: boolean;
  onOpenChange: (open: boolean) => void;
  onKeyChange: (value: string) => void;
  onValueChange: (value: string) => void;
  onAdd: () => void;
}) {
  const t = useTranslations("settings.chatPage.memory");
  return (
    <Dialog open={open} onOpenChange={(nextOpen) => {
      if (!adding) {
        onOpenChange(nextOpen);
      }
    }}>
      <DialogContent className="sm:max-w-[460px]">
        <form
          className="space-y-4"
          onSubmit={(event) => {
            event.preventDefault();
            onAdd();
          }}
        >
          <DialogHeader>
            <DialogTitle>{t("addTitle")}</DialogTitle>
            <DialogDescription>
              {t("addDescription")}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-3">
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">{t("name")}</p>
              <Input
                id="preference-key"
                autoFocus
                placeholder={t("namePlaceholder")}
                value={addKey}
                disabled={adding || atLimit}
                onChange={(event) => onKeyChange(event.target.value)}
              />
            </div>

            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">{t("content")}</p>
              <Textarea
                id="preference-value"
                placeholder={t("contentPlaceholder")}
                value={addValue}
                disabled={adding || atLimit}
                className="h-24 resize-none overflow-y-auto [field-sizing:fixed]"
                onChange={(event) => onValueChange(event.target.value)}
              />
            </div>
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              disabled={adding}
              onClick={() => onOpenChange(false)}
            >
              {t("cancel")}
            </Button>
            <Button
              type="submit"
              disabled={adding || atLimit || !addKey.trim() || !addValue.trim()}
            >
              {adding ? t("adding") : t("add")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function PreferenceMemorySection() {
  const t = useTranslations("settings.chatPage.memory");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [items, setItems] = React.useState<UserMemoryDTO[]>([]);
  const [loadingMems, setLoadingMems] = React.useState(true);
  const [addKey, setAddKey] = React.useState("");
  const [addValue, setAddValue] = React.useState("");
  const [adding, setAdding] = React.useState(false);
  const [addDialogOpen, setAddDialogOpen] = React.useState(false);

  React.useEffect(() => {
    void (async () => {
      setLoadingMems(true);
      try {
        const token = await resolveAccessToken();
        if (!token) return;
        const all = await listUserMemories(token);
        setItems(all.filter((m) => m.scope === "preference"));
      } catch {
        // ignore
      } finally {
        setLoadingMems(false);
      }
    })();
  }, []);

  const handleAdd = React.useCallback(async () => {
    const key = addKey.trim();
    const value = addValue.trim();
    if (!key || !value) return;
    setAdding(true);
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      await upsertUserMemory(token, key, value, "preference");
      setItems((prev) => {
        const exists = prev.find((m) => m.memoryKey === key);
        if (exists) {
          return prev.map((m) => m.memoryKey === key ? { ...m, value } : m);
        }
        return [{
          id: Date.now(),
          userID: 0,
          memoryKey: key,
          value,
          scope: "preference",
          updatedBy: "user",
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        }, ...prev];
      });
      setAddKey("");
      setAddValue("");
      setAddDialogOpen(false);
      toast.success(t("added"));
    } catch (error) {
      toast.error(t("addFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setAdding(false);
    }
  }, [addKey, addValue, resolveErrorMessage, t]);

  const handleEdit = React.useCallback(async (memoryKey: string, value: string) => {
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      await upsertUserMemory(token, memoryKey, value, "preference");
      setItems((prev) => prev.map((m) => m.memoryKey === memoryKey ? { ...m, value } : m));
      toast.success(t("updated"));
    } catch (error) {
      toast.error(t("updateFailed"), { description: resolveErrorMessage(error) });
    }
  }, [resolveErrorMessage, t]);

  const handleDelete = React.useCallback(async (memoryKey: string) => {
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      await deleteUserMemory(token, memoryKey);
      setItems((prev) => prev.filter((m) => m.memoryKey !== memoryKey));
      toast.success(t("deleted"));
    } catch (error) {
      toast.error(t("deleteFailed"), { description: resolveErrorMessage(error) });
    }
  }, [resolveErrorMessage, t]);

  const preferenceCount = items.length;
  const atLimit = preferenceCount >= MAX_PREFERENCES;

  return (
    <SettingsSection title={t("sectionTitle")}>
      <div className="space-y-2">
        <div className="flex h-8 items-center justify-between gap-3">
          <span className="text-[11px] tabular-nums text-muted-foreground">
            {loadingMems ? `-- / ${MAX_PREFERENCES}` : `${preferenceCount} / ${MAX_PREFERENCES}`}
          </span>
          <Button
            size="sm"
            variant="ghost"
            className="h-7 gap-1.5 px-2 text-xs"
            disabled={loadingMems || atLimit}
            onClick={() => setAddDialogOpen(true)}
          >
            <Plus className="h-3.5 w-3.5" />
            {atLimit ? t("full") : t("addPreference")}
          </Button>
        </div>

        <AddPreferenceDialog
          open={addDialogOpen}
          addKey={addKey}
          addValue={addValue}
          adding={adding}
          atLimit={atLimit}
          onOpenChange={(open) => {
            setAddDialogOpen(open);
            if (!open) {
              setAddKey("");
              setAddValue("");
            }
          }}
          onKeyChange={setAddKey}
          onValueChange={setAddValue}
          onAdd={() => void handleAdd()}
        />

        {loadingMems ? (
          <div className="space-y-1">
            <Skeleton className="h-9 w-full rounded-md" />
            <Skeleton className="h-9 w-4/5 rounded-md" />
          </div>
        ) : items.length === 0 ? (
          <div className="flex h-9 items-center rounded-md bg-muted/30 px-2.5">
            <p className="text-xs text-muted-foreground">{t("empty")}</p>
          </div>
        ) : items.length > 0 ? (
          <div className="space-y-1">
            {items.map((item) => (
              <PreferenceCard
                key={item.memoryKey}
                item={item}
                onEdit={handleEdit}
                onDelete={handleDelete}
              />
            ))}
          </div>
        ) : null}
      </div>
    </SettingsSection>
  );
}

// Main component.

export function SettingsChat() {
  const t = useTranslations("settings.chatPage");
  const { settings, loading, billingMode, vendorGroups, handleBool, handleEnum, handleDefaultModel } = useSettingsChat();
  const billingEnabled = billingMode !== "self";
  const [modifierLabel, setModifierLabel] = React.useState<"Command" | "Ctrl">("Ctrl");
  const [modifierShortcut, setModifierShortcut] = React.useState<Exclude<SendShortcut, "enter">>("ctrl_enter");
  const modelOptions = React.useMemo<ModelOption[]>(
    () => [
      { label: t("defaultModel.systemRecommended"), value: SYSTEM_RECOMMENDED_MODEL, iconUrl: null },
      ...vendorGroups.flatMap(([, items]) =>
        items
          .filter((model) => model.platformModelName.trim() && parseKindsJSON(model.kindsJSON).includes("chat"))
          .map((model) => ({
            label: resolveModelOptionLabel(model.platformModelName),
            value: model.platformModelName,
            iconUrl: resolveModelOptionIconUrl({
              platformModelName: model.platformModelName,
              vendor: model.vendor ?? "",
              icon: model.icon ?? "",
            }),
          })),
      ),
    ],
    [t, vendorGroups],
  );

  React.useEffect(() => {
    setModifierLabel(platformModifierLabel());
    setModifierShortcut(platformSendShortcut());
  }, []);

  const sendShortcutLabel = settings.sendShortcut === "enter" ? "Enter" : `${modifierLabel}+Enter`;

  return (
    <SettingsPage>
      <SettingsSection title={t("defaultModel.sectionTitle")}>
        <SettingsFieldList>
          <SettingsFieldRow
            title={t("defaultModel.title")}
            description={t("defaultModel.description")}
          >
            {loading ? (
              <Skeleton className="h-8 w-full rounded-md" />
            ) : (
              <ModelSelect
                value={settings.defaultModel}
                fallbackValue={SYSTEM_RECOMMENDED_MODEL}
                options={modelOptions}
                contentClassName="min-w-[min(320px,calc(100vw-2rem))]"
                onChange={handleDefaultModel}
                disabled={loading}
              />
            )}
          </SettingsFieldRow>
          <div className="pt-4">
            <SettingsFieldRow
              title={t("defaultModel.autoTitle")}
              description={t("defaultModel.autoTitleDescription")}
            >
              <Switch
                checked={settings.autoGenerateTitle}
                onCheckedChange={handleBool("chat.auto_generate_title", "autoGenerateTitle")}
                disabled={loading}
                aria-label={t("defaultModel.autoTitle")}
              />
            </SettingsFieldRow>
          </div>
        </SettingsFieldList>
      </SettingsSection>

      <SettingsSectionSeparator />

      <SettingsSection title={t("input.sectionTitle")}>
        <SettingsFieldList>
          <SettingsFieldRow
            title={t("input.shortcutTitle")}
            description={t("input.shortcutDescription", { shortcut: sendShortcutLabel })}
          >
            <Select
              value={settings.sendShortcut === "enter" ? "enter" : modifierShortcut}
              onValueChange={handleEnum("chat.send_on_enter", "sendShortcut")}
              disabled={loading}
            >
              <SelectTrigger size="sm" className="text-left md:text-right *:data-[slot=select-value]:flex-1 *:data-[slot=select-value]:justify-start md:*:data-[slot=select-value]:justify-end">
                <SelectValue />
              </SelectTrigger>
              <SelectContent align="start">
                <SelectItem value="enter">Enter</SelectItem>
                <SelectItem value={modifierShortcut}>{modifierLabel}+Enter</SelectItem>
              </SelectContent>
            </Select>
          </SettingsFieldRow>
          <div className="pt-4">
            <SettingsFieldRow
              title={t("input.heightTitle")}
              description={t("input.heightDescription")}
            >
              <Select
                value={settings.inputHeight}
                onValueChange={handleEnum("chat.input_height", "inputHeight")}
                disabled={loading}
              >
                <SelectTrigger size="sm" className="text-left md:text-right *:data-[slot=select-value]:flex-1 *:data-[slot=select-value]:justify-start md:*:data-[slot=select-value]:justify-end">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent align="start">
                  <SelectItem value="compact">{t("input.height.compact")}</SelectItem>
                  <SelectItem value="standard">{t("input.height.standard")}</SelectItem>
                  <SelectItem value="loose">{t("input.height.loose")}</SelectItem>
                </SelectContent>
              </Select>
            </SettingsFieldRow>
          </div>
          <div className="pt-4">
            <SettingsFieldRow
              title={t("input.restoreDraftTitle")}
              description={t("input.restoreDraftDescription")}
            >
              <Switch
                checked={settings.restoreDraftOnFailure}
                onCheckedChange={handleBool("chat.restore_draft_on_failure", "restoreDraftOnFailure")}
                disabled={loading}
                aria-label={t("input.restoreDraftTitle")}
              />
            </SettingsFieldRow>
          </div>
          <div className="pt-4">
            <SettingsFieldRow
              title={t("input.preserveDraftTitle")}
              description={t("input.preserveDraftDescription")}
            >
              <Switch
                checked={settings.preserveConversationDrafts}
                onCheckedChange={handleBool("chat.preserve_conversation_drafts", "preserveConversationDrafts")}
                disabled={loading}
                aria-label={t("input.preserveDraftTitle")}
              />
            </SettingsFieldRow>
          </div>
          <div className="pt-4">
            <SettingsFieldRow
              title={t("input.deleteFilesDefaultTitle")}
              description={t("input.deleteFilesDefaultDescription")}
            >
              <Switch
                checked={settings.deleteFilesByDefault}
                onCheckedChange={handleBool("chat.delete_conversation_files_by_default", "deleteFilesByDefault")}
                disabled={loading}
                aria-label={t("input.deleteFilesDefaultTitle")}
              />
            </SettingsFieldRow>
          </div>
        </SettingsFieldList>
      </SettingsSection>

      <SettingsSectionSeparator />

      <SettingsSection title={t("display.sectionTitle")}>
        <SettingsFieldList>
          <div>
            <SettingsFieldRow
              title={t("display.markdownTitle")}
              description={t("display.markdownDescription")}
            >
              <Switch
                checked={settings.markdownRender}
                onCheckedChange={handleBool("chat.markdown_render", "markdownRender")}
                disabled={loading}
                aria-label={t("display.markdownTitle")}
              />
            </SettingsFieldRow>
          </div>

          <div className="pt-4">
            <SettingsFieldRow
              title={t("display.modelTitle")}
              description={t("display.modelDescription")}
            >
              <Switch
                checked={settings.showModelInfo}
                onCheckedChange={handleBool("chat.show_model_info", "showModelInfo")}
                disabled={loading}
                aria-label={t("display.modelTitle")}
              />
            </SettingsFieldRow>
          </div>

          <div className="pt-4">
            <SettingsFieldRow
              title={t("display.tokenTitle")}
              description={t("display.tokenDescription")}
            >
              <Switch
                checked={settings.showTokenUsage}
                onCheckedChange={handleBool("chat.show_token_usage", "showTokenUsage")}
                disabled={loading}
                aria-label={t("display.tokenTitle")}
              />
            </SettingsFieldRow>
          </div>

          <div className="pt-4">
            <SettingsFieldRow
              title={t("display.latencyTitle")}
              description={t("display.latencyDescription")}
            >
              <Switch
                checked={settings.showLatency}
                onCheckedChange={handleBool("chat.show_latency", "showLatency")}
                disabled={loading}
                aria-label={t("display.latencyTitle")}
              />
            </SettingsFieldRow>
          </div>

          <div className="pt-4">
            <SettingsFieldRow
              title={t("display.costTitle")}
              description={billingEnabled ? t("display.costDescription") : t("display.costDescriptionSelfMode")}
            >
              <Switch
                checked={billingEnabled && settings.showBillingCost}
                onCheckedChange={handleBool("chat.show_billing_cost", "showBillingCost")}
                disabled={loading || !billingEnabled}
                aria-label={t("display.costTitle")}
              />
            </SettingsFieldRow>
          </div>
        </SettingsFieldList>
      </SettingsSection>

      <SettingsSectionSeparator />

      <SettingsSection title={t("context.sectionTitle")}>
        <SettingsFieldList>
          <SettingsFieldRow
            title={t("context.autoCompactTitle")}
            description={t("context.autoCompactDescription")}
          >
            <Switch
              checked={settings.contextCompactAuto}
              onCheckedChange={handleBool("chat.context_compact_auto", "contextCompactAuto")}
              disabled={loading}
              aria-label={t("context.autoCompactTitle")}
            />
          </SettingsFieldRow>
        </SettingsFieldList>
      </SettingsSection>

      <SettingsSectionSeparator />

      <SettingsSection title={t("file.sectionTitle")}>
        <SettingsFieldList>
          <SettingsFieldRow
            title={t("file.modeTitle")}
            description={
              settings.fileMode === "auto"
                ? t("file.modeDescription.auto")
                : settings.fileMode === "full_context"
                  ? t("file.modeDescription.fullContext")
                  : t("file.modeDescription.rag")
            }
          >
            <Select
              value={settings.fileMode}
              onValueChange={handleEnum("chat.file_mode", "fileMode")}
              disabled={loading}
            >
              <SelectTrigger size="sm" className="text-left md:text-right *:data-[slot=select-value]:flex-1 *:data-[slot=select-value]:justify-start md:*:data-[slot=select-value]:justify-end">
                <SelectValue />
              </SelectTrigger>
              <SelectContent align="start">
                <SelectItem value="auto">{t("file.mode.auto")}</SelectItem>
                <SelectItem value="full_context">{t("file.mode.fullContext")}</SelectItem>
                <SelectItem value="rag">{t("file.mode.rag")}</SelectItem>
              </SelectContent>
            </Select>
          </SettingsFieldRow>
        </SettingsFieldList>
      </SettingsSection>

      <SettingsSectionSeparator />

      <PreferenceMemorySection />
    </SettingsPage>
  );
}
