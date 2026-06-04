"use client";

import * as React from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { ChatMessageBot } from "@/features/chat/components/message/message-bot";
import { ChatMessageUser } from "@/features/chat/components/message/message-user";
import { StreamdownRender } from "@/features/chat/components/markdown/streamdown-render";
import {
  buildChildrenIndex,
  buildVisibleMessages,
  mapServerMessage,
  reconcileBranchSelections,
  toBranchKey,
} from "@/features/chat/model/chat-thread";
import type { ChatAreaMessage } from "@/features/chat/types/messages";
import { cloneSharedConversation, getSharedConversation } from "@/shared/api/conversation";
import type {
  MessageDTO,
  PublicSharedConversationDTO,
  PublicSharedMessageDTO,
} from "@/shared/api/conversation.types";
import { fetchSharedFileContent, type FileContentResult } from "@/shared/api/file";
import type { PreviewDialogFile } from "@/features/files/components/preview/file-preview-dialog";
import { CenteredEmptyState } from "@/components/ui/empty-state";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { AppLogo } from "@/shared/components/app-logo";
import { useOptionalAuthSession } from "@/shared/auth/auth-session-context";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { useAppLocale } from "@/i18n/app-i18n-provider";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";

function formatSharedAt(value: string, locale: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function PublicShareSkeleton() {
  return (
    <div className="mx-auto flex h-full w-full max-w-[820px] flex-col px-4 py-6">
      <div className="flex items-start justify-between gap-4 pb-5">
        <div className="min-w-0 flex-1 space-y-2">
          <Skeleton className="h-5 w-56 max-w-full rounded-md bg-muted/55" />
          <Skeleton className="h-3 w-36 max-w-full rounded-md bg-muted/40" />
        </div>
        <Skeleton className="h-8 w-20 shrink-0 rounded-md bg-muted/55" />
      </div>
      <div className="space-y-7">
        <div className="flex justify-end">
          <Skeleton className="h-16 w-[min(25rem,76%)] rounded-xl bg-muted/55" />
        </div>
        <Skeleton className="h-32 w-full rounded-lg bg-muted/45" />
        <div className="flex justify-end">
          <Skeleton className="h-12 w-[min(18rem,68%)] rounded-xl bg-muted/50" />
        </div>
      </div>
    </div>
  );
}

function toReadOnlyMessageDTO(item: PublicSharedMessageDTO): MessageDTO {
  return {
    id: 0,
    conversationID: 0,
    userID: 0,
    publicID: item.publicID,
    parentMessageID: null,
    parentPublicID: item.parentPublicID,
    runID: item.runID,
    role: item.role,
    contentType: item.contentType,
    content: item.content,
    branchReason: item.branchReason === "retry" || item.branchReason === "edit" ? item.branchReason : "default",
    sourceMessageID: null,
    sourcePublicID: item.sourcePublicID,
    tokenUsage: item.tokenUsage ?? 0,
    inputTokens: item.inputTokens ?? 0,
    outputTokens: item.outputTokens ?? 0,
    cacheReadTokens: item.cacheReadTokens ?? 0,
    cacheWriteTokens: item.cacheWriteTokens ?? 0,
    reasoningTokens: item.reasoningTokens ?? 0,
    latencyMS: item.latencyMS ?? 0,
    status: item.status || "success",
    errorCode: item.errorCode || "",
    errorMessage: item.errorMessage || "",
    attachments: item.attachments || "[]",
    processTrace: item.processTrace,
    myFeedback: "",
    thumbsUpCount: 0,
    thumbsDownCount: 0,
    editedAt: item.editedAt ?? null,
    createdAt: item.createdAt,
    updatedAt: item.updatedAt,
  };
}

function mapPublicSharedMessage(item: PublicSharedMessageDTO, fallbackModel: string): ChatAreaMessage {
  const message = mapServerMessage(toReadOnlyMessageDTO(item));
  const platformModelName = item.platformModelName?.trim() || fallbackModel.trim();
  return {
    ...message,
    platformModelName,
    billingCost: undefined,
    branchNavigator: undefined,
  };
}

const noop = () => undefined;
const noopAsync = async () => undefined;

function branchSelectionsFromDefaultPath(
  messages: ChatAreaMessage[],
  defaultMessagePublicIDs: string[],
): Record<string, string> {
  const byPublicID = new Map(messages.map((message) => [message.publicID, message]));
  const selections: Record<string, string> = {};
  for (const publicID of defaultMessagePublicIDs) {
    const message = byPublicID.get(publicID.trim());
    if (!message) {
      continue;
    }
    selections[toBranchKey(message.parentPublicID)] = message.publicID;
  }
  return reconcileBranchSelections(messages, selections);
}

function PublicSharedMessage({
  item,
  loadContent,
  onCycleBranch,
}: {
  item: ChatAreaMessage;
  loadContent: (file: PreviewDialogFile) => Promise<FileContentResult>;
  onCycleBranch: (parentPublicID: string | null, direction: "previous" | "next") => void;
}) {
  if (item.role === "user") {
    return (
      <ChatMessageUser
        item={item}
        busy={false}
        onRetryUserMessage={noopAsync}
        onEditUserMessage={async () => false}
        onCycleMessageBranch={onCycleBranch}
        onCopy={noop}
        readOnly
        attachmentContentLoader={loadContent}
        showBranchNavigator
      />
    );
  }

  if (item.role === "assistant") {
    return (
      <ChatMessageBot
        item={item}
        busy={false}
        reaction={null}
        onRetryAssistantMessage={noopAsync}
        onEditAssistantMessage={async () => false}
        onCycleMessageBranch={onCycleBranch}
        onReactAssistantMessage={noop}
        onCopy={noop}
        showModelInfo
        showLatency
        showTokenUsage
        showBillingCost={false}
        readOnly
        attachmentContentLoader={loadContent}
        showBranchNavigator
      />
    );
  }

  return (
    <div className="min-w-0 max-w-none overflow-hidden text-sm leading-8 text-muted-foreground [overflow-wrap:anywhere]">
      <StreamdownRender content={item.content} streaming={false} />
    </div>
  );
}

export function PublicSharePage() {
  const t = useTranslations("share");
  const { locale } = useAppLocale();
  const resolveErrorMessage = useLocalizedErrorMessage();
  const router = useRouter();
  const searchParams = useSearchParams();
  const authSession = useOptionalAuthSession();
  const shareID = searchParams.get("conversation_id")?.trim() || "";
  const [data, setData] = React.useState<PublicSharedConversationDTO | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [errorMsg, setErrorMsg] = React.useState("");
  const [branchSelections, setBranchSelections] = React.useState<Record<string, string>>({});
  const [resolvedAccessToken, setResolvedAccessToken] = React.useState("");
  const [cloning, setCloning] = React.useState(false);

  React.useEffect(() => {
    let cancelled = false;
    async function loadShare() {
      setLoading(true);
      setErrorMsg("");
      try {
        const result = await getSharedConversation(shareID);
        if (!cancelled) {
          setData(result);
        }
      } catch (error) {
        if (!cancelled) {
          setData(null);
          setErrorMsg(resolveErrorMessage(error, t("notFoundDescription")));
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    }
    if (shareID) {
      void loadShare();
    } else {
      setLoading(false);
      setErrorMsg(t("notFoundDescription"));
    }
    return () => {
      cancelled = true;
    };
  }, [resolveErrorMessage, shareID, t]);

  React.useEffect(() => {
    if (authSession?.accessToken) {
      setResolvedAccessToken(authSession.accessToken);
      return;
    }
    let cancelled = false;
    async function checkSession() {
      try {
        const token = await resolveAccessToken();
        if (!cancelled) {
          setResolvedAccessToken(token);
        }
      } catch {
        if (!cancelled) {
          setResolvedAccessToken("");
        }
      }
    }
    void checkSession();
    return () => {
      cancelled = true;
    };
  }, [authSession?.accessToken]);

  const messages = React.useMemo(
    () => data?.messages.map((message) => mapPublicSharedMessage(message, data.model)) ?? [],
    [data],
  );
  const defaultSelectionKey = React.useMemo(
    () => `${data?.shareID ?? ""}:${data?.defaultMessagePublicIDs?.join(",") ?? ""}`,
    [data],
  );
  React.useEffect(() => {
    if (!data) {
      setBranchSelections({});
      return;
    }
    setBranchSelections(branchSelectionsFromDefaultPath(messages, data.defaultMessagePublicIDs ?? []));
  }, [data, defaultSelectionKey, messages]);

  const visibleMessages = React.useMemo(
    () => buildVisibleMessages(messages, branchSelections),
    [branchSelections, messages],
  );
  const onCycleBranch = React.useCallback(
    (parentPublicID: string | null, direction: "previous" | "next") => {
      setBranchSelections((previous) => {
        const children = buildChildrenIndex(messages);
        const parentKey = toBranchKey(parentPublicID);
        const siblings = children.get(parentKey) ?? [];
        if (siblings.length <= 1) {
          return previous;
        }
        const currentPublicID = previous[parentKey] || siblings[siblings.length - 1]?.publicID;
        const currentIndex = Math.max(0, siblings.findIndex((candidate) => candidate.publicID === currentPublicID));
        const nextIndex =
          direction === "previous"
            ? Math.max(0, currentIndex - 1)
            : Math.min(siblings.length - 1, currentIndex + 1);
        const next = siblings[nextIndex];
        if (!next || next.publicID === currentPublicID) {
          return previous;
        }
        return {
          ...previous,
          [parentKey]: next.publicID,
        };
      });
    },
    [messages],
  );

  const loadSharedContent = React.useCallback(
    (file: PreviewDialogFile) => fetchSharedFileContent(shareID, file.fileID),
    [shareID],
  );
  const accessToken = authSession?.accessToken || resolvedAccessToken;
  const loginNextPath = React.useMemo(() => {
    const params = new URLSearchParams();
    if (shareID) {
      params.set("conversation_id", shareID);
    }
    const nextPath = params.toString() ? `/share?${params.toString()}` : "/share";
    return `/login?next=${encodeURIComponent(nextPath)}`;
  }, [shareID]);

  const handleContinueConversation = React.useCallback(async () => {
    if (!shareID) {
      return;
    }
    if (!accessToken) {
      router.push(loginNextPath);
      return;
    }
    setCloning(true);
    try {
      const conversation = await cloneSharedConversation(accessToken, shareID);
      router.push(`/chat?conversation_id=${encodeURIComponent(conversation.publicID)}`);
    } catch (error) {
      toast.error(t("cloneFailed"), { description: resolveErrorMessage(error, t("cloneFailed")) });
    } finally {
      setCloning(false);
    }
  }, [accessToken, loginNextPath, resolveErrorMessage, router, shareID, t]);

  if (loading) {
    return <PublicShareSkeleton />;
  }

  if (!data) {
    return (
      <CenteredEmptyState
        className="h-svh"
        title={t("notFoundTitle")}
        description={errorMsg || t("notFoundDescription")}
      />
    );
  }

  const createdAt = formatSharedAt(data.createdAt, locale);

  return (
    <main className="h-full min-h-0 w-full overflow-y-auto bg-background text-foreground">
      <div className="mx-auto min-h-full w-full max-w-[820px] px-4 pb-24 pt-5 md:pt-6">
        <header className="flex items-start justify-between gap-4 pb-5">
          <div className="min-w-0 flex-1">
            <h1 className="truncate text-base font-semibold leading-6">{data.title || t("title")}</h1>
            <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
              {createdAt ? <span>{createdAt}</span> : null}
              <span>{t("snapshotMessages", { count: data.messages.length })}</span>
            </div>
          </div>
          <Link href="/" aria-label="DEEIX Chat" className="mt-0.5 inline-flex h-8 shrink-0 items-center">
            <AppLogo width={78} height={24} priority className="h-6 w-auto" />
          </Link>
        </header>

        <div className="space-y-7">
          {visibleMessages.map((message) => (
            <div key={message.publicID} className="min-w-0">
              <PublicSharedMessage
                item={message}
                loadContent={loadSharedContent}
                onCycleBranch={onCycleBranch}
              />
            </div>
          ))}
        </div>

        <div className="pointer-events-none fixed inset-x-0 bottom-5 z-30 flex justify-center">
          <Button
            type="button"
            className="pointer-events-auto h-9 rounded-full bg-primary/90 px-5 shadow-lg shadow-primary/20 hover:bg-primary"
            onClick={handleContinueConversation}
            disabled={cloning}
          >
            {accessToken ? (cloning ? t("continuing") : t("continueConversation")) : t("signInToContinue")}
          </Button>
        </div>
      </div>
    </main>
  );
}
