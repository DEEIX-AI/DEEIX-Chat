"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import {
  adoptDiscussionSnapshot,
  getDiscussionSnapshot,
  refreshDiscussionSnapshot,
  rollbackDiscussionSnapshot,
} from "@/shared/api/discussion";
import type {
  DiscussionConversationSnapshot,
  DiscussionRuntimePlan,
  DiscussionSnapshotContent,
  DiscussionTemplateDTO,
} from "@/shared/api/discussion-types";
import type { StreamMessageEvent } from "@/shared/api/conversation-types";
import { ApiError } from "@/shared/api/http-client";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import {
  canDisableDiscussionModel,
  discussionRuntimeBindings,
} from "@/shared/lib/discussion";
import { isRecord } from "@/shared/lib/type-guards";

function discussionBindingsStorageKey(conversationPublicID: string | null, templatePublicID: string): string {
  return `deeix:discussion-bindings:${conversationPublicID || "new"}:${templatePublicID}`;
}

function discussionManualStorageKey(conversationPublicID: string | null, templatePublicID: string): string {
  return `deeix:discussion-manual-models:${conversationPublicID || "new"}:${templatePublicID}`;
}

function discussionDisabledStorageKey(conversationPublicID: string | null, templatePublicID: string): string {
  return `deeix:discussion-disabled:${conversationPublicID || "new"}:${templatePublicID}`;
}

function readStoredBindings(key: string): Record<string, string> {
  if (typeof window === "undefined") return {};
  try {
    const parsed: unknown = JSON.parse(window.localStorage.getItem(key) ?? "{}");
    if (!isRecord(parsed)) return {};
    return Object.fromEntries(
      Object.entries(parsed).filter((entry): entry is [string, string] => typeof entry[1] === "string" && Boolean(entry[1].trim())),
    );
  } catch {
    return {};
  }
}

function readStoredStringList(key: string): string[] | null {
  if (typeof window === "undefined") return null;
  const raw = window.localStorage.getItem(key);
  if (raw === null) return null;
  try {
    const parsed: unknown = JSON.parse(raw);
    return Array.isArray(parsed)
      ? Array.from(new Set(parsed.filter((item): item is string => typeof item === "string" && Boolean(item.trim()))))
      : null;
  } catch {
    return null;
  }
}

function readStoredDisabled(key: string): string[] {
  return readStoredStringList(key) ?? [];
}

function writeStored(key: string, value: unknown): void {
  try {
    window.localStorage.setItem(key, JSON.stringify(value));
  } catch {
    // The server snapshot is authoritative; local preferences are best-effort.
  }
}

function snapshotBindings(content: DiscussionSnapshotContent | null | undefined): Record<string, string> {
  return content ? discussionRuntimeBindings(content.plan) : {};
}

function inferManualBindings(stored: Record<string, string>, base: Record<string, string>): Set<string> {
  const manual = new Set<string>();
  for (const [participant, model] of Object.entries(stored)) {
    if (base[participant]?.trim() && base[participant] !== model) {
      manual.add(participant);
    }
  }
  return manual;
}

function bindingsForTemplate(
  template: DiscussionTemplateDTO,
  sources: readonly Record<string, string>[],
  manualModels: ReadonlySet<string>,
): Record<string, string> {
  const [snapshot, seed, stored] = sources;
  const next: Record<string, string> = {};
  for (const placeholder of template.config.placeholders) {
    const key = placeholder.key;
    next[key] = manualModels.has(key)
      ? stored[key] || snapshot[key] || seed[key] || placeholder.defaultModel || ""
      : snapshot[key] || seed[key] || placeholder.defaultModel || "";
  }
  return next;
}

export function useChatDiscussion({
  conversationPublicID,
  selectedPlatformModelName,
  discussionTemplates,
  discussionSeedBindings,
  refreshDiscussionTemplates,
}: {
  conversationPublicID: string | null;
  selectedPlatformModelName: string;
  discussionTemplates: DiscussionTemplateDTO[];
  discussionSeedBindings?: Record<string, string> | null;
  refreshDiscussionTemplates: () => Promise<DiscussionTemplateDTO[]>;
}) {
  const t = useTranslations("chat.discussion");
  const templatePublicID = selectedPlatformModelName.startsWith("discussion:")
    ? selectedPlatformModelName.slice("discussion:".length).trim()
    : "";
  const liveTemplate = discussionTemplates.find((item) => item.publicID === templatePublicID) ?? null;
  const [snapshot, setSnapshot] = React.useState<DiscussionConversationSnapshot | null>(null);
  const [snapshotLoading, setSnapshotLoading] = React.useState(false);
  const [snapshotBusy, setSnapshotBusy] = React.useState(false);
  const [refreshConfirmationPending, setRefreshConfirmationPending] = React.useState(false);
  const [snapshotError, setSnapshotError] = React.useState("");
  const [snapshotNotice, setSnapshotNotice] = React.useState("");
  const [canAdoptPending, setCanAdoptPending] = React.useState(false);
  const [discussionBindings, setDiscussionBindings] = React.useState<Record<string, string>>({});
  const [disabledModels, setDisabledModels] = React.useState<string[]>([]);
  const snapshotContent = snapshot?.pending ?? snapshot?.active ?? null;
  const selectedDiscussionTemplate = snapshotContent?.template ?? liveTemplate;
  const bindingsRef = React.useRef<Record<string, string>>({});
  const manualModelsRef = React.useRef<Set<string>>(new Set());
  const snapshotRef = React.useRef<DiscussionConversationSnapshot | null>(null);
  const bindingScopeRef = React.useRef("");
  const bindingRevisionRef = React.useRef("");
  const disabledScopeRef = React.useRef("");

  React.useEffect(() => { bindingsRef.current = discussionBindings; }, [discussionBindings]);
  React.useEffect(() => { snapshotRef.current = snapshot; }, [snapshot]);

  React.useEffect(() => {
    let cancelled = false;
    setSnapshot(null);
    setSnapshotError("");
    setSnapshotNotice("");
    setCanAdoptPending(false);
    setRefreshConfirmationPending(false);
    if (!conversationPublicID || !templatePublicID) {
      setSnapshotLoading(false);
      return;
    }
    setSnapshotLoading(true);
    void (async () => {
      try {
        const token = await resolveAccessToken();
        if (!token) throw new Error("missing access token");
        const result = await getDiscussionSnapshot(token, conversationPublicID, templatePublicID);
        if (!cancelled) setSnapshot(result);
      } catch (error) {
        if (!cancelled) setSnapshotError(error instanceof ApiError && error.status === 409 ? t("snapshotConflict") : t("snapshotLoadFailed"));
      } finally {
        if (!cancelled) setSnapshotLoading(false);
      }
    })();
    return () => { cancelled = true; };
  }, [conversationPublicID, t, templatePublicID]);

  React.useEffect(() => {
    const template = selectedDiscussionTemplate;
    const scope = `${conversationPublicID || "new"}|${templatePublicID}`;
    const revision = snapshot ? `${snapshot.activeVersion}:${snapshot.pendingVersion}:${snapshot.updatedAt}` : "";
    const scopeChanged = bindingScopeRef.current !== scope;
    const previousScope = bindingScopeRef.current;
    bindingScopeRef.current = scope;
    bindingRevisionRef.current = revision;
    if (!template) {
      manualModelsRef.current = new Set();
      setDiscussionBindings((current) => Object.keys(current).length === 0 ? current : {});
      return;
    }

    let stored = readStoredBindings(discussionBindingsStorageKey(conversationPublicID, templatePublicID));
    const [previousConversationID, previousTemplateID] = previousScope.split("|");
    const carried = scopeChanged && previousConversationID === "new" && previousTemplateID === templatePublicID && !snapshotContent
      ? bindingsRef.current
      : null;
    if (carried && Object.keys(carried).length > 0) {
      stored = carried;
      writeStored(discussionBindingsStorageKey(conversationPublicID, templatePublicID), carried);
      writeStored(discussionManualStorageKey(conversationPublicID, templatePublicID), [...manualModelsRef.current]);
    } else {
      const savedManual = readStoredStringList(discussionManualStorageKey(conversationPublicID, templatePublicID));
      if (savedManual) {
        manualModelsRef.current = new Set(savedManual);
      } else {
        const base = snapshotBindings(snapshotContent);
        manualModelsRef.current = inferManualBindings(stored, Object.keys(base).length > 0 ? base : discussionSeedBindings ?? {});
      }
    }
    const next = bindingsForTemplate(
      template,
      [snapshotBindings(snapshotContent), discussionSeedBindings ?? {}, stored],
      manualModelsRef.current,
    );
    writeStored(discussionBindingsStorageKey(conversationPublicID, templatePublicID), next);
    setDiscussionBindings(next);
  }, [conversationPublicID, discussionSeedBindings, selectedDiscussionTemplate, snapshot, snapshotContent, templatePublicID]);

  React.useEffect(() => {
    const scope = `${conversationPublicID || "new"}|${templatePublicID}`;
    if (disabledScopeRef.current === scope) return;
    const previousScope = disabledScopeRef.current;
    disabledScopeRef.current = scope;
    if (!templatePublicID) {
      setDisabledModels([]);
      return;
    }
    const [previousConversationID, previousTemplateID] = previousScope.split("|");
    if (previousConversationID === "new" && previousTemplateID === templatePublicID && conversationPublicID) {
      writeStored(discussionDisabledStorageKey(conversationPublicID, templatePublicID), disabledModels);
      return;
    }
    setDisabledModels(readStoredDisabled(discussionDisabledStorageKey(conversationPublicID, templatePublicID)));
  }, [conversationPublicID, disabledModels, templatePublicID]);

  const setDiscussionBinding = React.useCallback((participant: string, model: string) => {
    if (snapshotRef.current?.active && !snapshotRef.current.pending) return;
    const placeholder = selectedDiscussionTemplate?.config.placeholders.find((item) => item.key === participant);
    if (model.trim() && model.trim() === placeholder?.defaultModel?.trim()) {
      manualModelsRef.current.delete(participant);
    } else {
      manualModelsRef.current.add(participant);
    }
    writeStored(discussionManualStorageKey(conversationPublicID, templatePublicID), [...manualModelsRef.current]);
    setDiscussionBindings((current) => {
      const next = { ...current, [participant]: model };
      writeStored(discussionBindingsStorageKey(conversationPublicID, templatePublicID), next);
      return next;
    });
  }, [conversationPublicID, selectedDiscussionTemplate, templatePublicID]);

  const setDiscussionModelDisabled = React.useCallback((participant: string, shouldDisable: boolean) => {
    const template = selectedDiscussionTemplate;
    if (!template) return;
    setDisabledModels((current) => {
      if (shouldDisable && !canDisableDiscussionModel(template.config, participant, current)) return current;
      const next = shouldDisable
        ? Array.from(new Set([...current, participant]))
        : current.filter((item) => item !== participant);
      writeStored(discussionDisabledStorageKey(conversationPublicID, templatePublicID), next);
      return next;
    });
  }, [conversationPublicID, selectedDiscussionTemplate, templatePublicID]);

  const useDefaultDiscussionModels = React.useCallback(() => {
    if (!selectedDiscussionTemplate || (snapshotRef.current?.active && !snapshotRef.current.pending)) return;
    manualModelsRef.current = new Set();
    writeStored(discussionManualStorageKey(conversationPublicID, templatePublicID), []);
    const next = bindingsForTemplate(selectedDiscussionTemplate, [{}, {}, {}], manualModelsRef.current);
    writeStored(discussionBindingsStorageKey(conversationPublicID, templatePublicID), next);
    setDiscussionBindings(next);
    setSnapshotNotice(t("defaultsRestored"));
  }, [conversationPublicID, selectedDiscussionTemplate, t, templatePublicID]);

  const runSnapshotAction = React.useCallback(async (
    action: "refresh" | "rollback" | "adopt",
  ) => {
    if (!templatePublicID || snapshotBusy) return;
    if (action === "refresh" && snapshotRef.current?.pending && !refreshConfirmationPending) {
      setRefreshConfirmationPending(true);
      setSnapshotNotice(t("refreshAgainConfirm"));
      return;
    }
    setRefreshConfirmationPending(false);
    setSnapshotBusy(true);
    setSnapshotError("");
    setSnapshotNotice("");
    try {
      if (!conversationPublicID) {
        if (action !== "refresh") return;
        const templates = await refreshDiscussionTemplates();
        const latest = templates.find((item) => item.publicID === templatePublicID);
        if (!latest) throw new Error("discussion template is no longer available");
        const next = bindingsForTemplate(latest, [{}, {}, bindingsRef.current], manualModelsRef.current);
        writeStored(discussionBindingsStorageKey(conversationPublicID, templatePublicID), next);
        setDiscussionBindings(next);
        setSnapshotNotice(t("refreshDraftDone"));
        return;
      }
      const token = await resolveAccessToken();
      if (!token) throw new Error("missing access token");
      const expectedPendingVersion = snapshotRef.current?.pendingVersion ?? 0;
      const result = action === "refresh"
        ? await refreshDiscussionSnapshot(token, conversationPublicID, templatePublicID, {
            models: bindingsRef.current,
            manualModels: [...manualModelsRef.current],
            expectedPendingVersion,
          })
        : action === "rollback"
          ? await rollbackDiscussionSnapshot(token, conversationPublicID, templatePublicID, expectedPendingVersion)
          : await adoptDiscussionSnapshot(token, conversationPublicID, templatePublicID, expectedPendingVersion);
      snapshotRef.current = result;
      setSnapshot(result);
      setCanAdoptPending(false);
      setSnapshotNotice(t(action === "refresh" ? "refreshReady" : action === "rollback" ? "rollbackDone" : "adoptDone"));
    } catch (error) {
      setSnapshotError(error instanceof ApiError && error.status === 409 ? t("snapshotConflict") : t("snapshotActionFailed"));
    } finally {
      setSnapshotBusy(false);
    }
  }, [conversationPublicID, refreshConfirmationPending, refreshDiscussionTemplates, snapshotBusy, t, templatePublicID]);

  const onDiscussionStreamEvent = React.useCallback((event: StreamMessageEvent) => {
    if (event.type !== "discussion_done" || event.state.templateId !== templatePublicID) return;
    if (event.state.status === "completed_with_errors" && snapshotRef.current?.pending) {
      setCanAdoptPending(true);
      setSnapshotNotice(t("pendingNotCommitted"));
    }
    if (!event.snapshotCommitted) return;
    const current = snapshotRef.current;
    const template = event.activeSnapshot?.template ?? current?.pending?.template ?? selectedDiscussionTemplate;
    if (!template) return;
    const active: DiscussionSnapshotContent = event.activeSnapshot ?? {
      template,
      config: template.config,
      plan: { rounds: event.state.rounds, summary: event.state.summary } satisfies DiscussionRuntimePlan,
    };
    const pendingVersion = event.pendingVersion ?? 0;
    const next: DiscussionConversationSnapshot = {
      templatePublicID,
      active,
      activeVersion: event.activeVersion ?? Math.max(current?.activeVersion ?? 0, 1),
      pending: pendingVersion > 0 ? current?.pending ?? null : null,
      pendingVersion,
      updatedAt: new Date().toISOString(),
    };
    snapshotRef.current = next;
    setSnapshot(next);
    setCanAdoptPending(false);
    setSnapshotNotice(t("pendingCommitted"));
  }, [selectedDiscussionTemplate, t, templatePublicID]);

  return {
    selectedDiscussionTemplate,
    discussionBindings,
    setDiscussionBinding,
    discussionDisabledModels: disabledModels,
    setDiscussionModelDisabled,
    discussionSnapshot: snapshot,
    discussionSnapshotLoading: snapshotLoading,
    discussionSnapshotBusy: snapshotBusy,
    discussionSnapshotError: snapshotError,
    discussionSnapshotNotice: snapshotNotice,
    discussionBindingsLocked: Boolean(snapshot?.active && !snapshot.pending),
    canAdoptDiscussionPending: canAdoptPending,
    discussionRefreshConfirmationPending: refreshConfirmationPending,
    refreshDiscussionConfiguration: () => runSnapshotAction("refresh"),
    cancelDiscussionRefresh: () => {
      setRefreshConfirmationPending(false);
      setSnapshotNotice("");
    },
    rollbackDiscussionConfiguration: () => runSnapshotAction("rollback"),
    adoptDiscussionConfiguration: () => runSnapshotAction("adopt"),
    useDefaultDiscussionModels,
    onDiscussionStreamEvent,
  };
}
