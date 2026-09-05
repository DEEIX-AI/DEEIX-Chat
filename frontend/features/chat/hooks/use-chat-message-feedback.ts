"use client";

import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import type { AssistantReaction } from "@/features/chat/components/message/message-meta";
import type { ChatAreaMessage } from "@/features/chat/types/messages";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { setMessageFeedback } from "@/shared/api/conversation";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

export function useChatMessageFeedback(
  messages: ChatAreaMessage[],
  { persist = true }: { persist?: boolean } = {},
) {
  const t = useTranslations("chat.feedback");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [overrides, setOverrides] = React.useState<Record<string, AssistantReaction>>({});
  const activeIDs = React.useMemo(() => new Set(messages.map((item) => item.publicID)), [messages]);
  const activeIDsRef = React.useRef(activeIDs);
  const activeIDsKey = React.useMemo(() => Array.from(activeIDs).join("|"), [activeIDs]);

  React.useEffect(() => {
    activeIDsRef.current = activeIDs;
  }, [activeIDs]);

  React.useEffect(() => {
    setOverrides((prev) => {
      const nextEntries = Object.entries(prev).filter(([publicID]) => activeIDsRef.current.has(publicID));
      if (nextEntries.length === Object.keys(prev).length) {
        return prev;
      }
      return Object.fromEntries(nextEntries);
    });
  }, [activeIDsKey]);

  const getReaction = React.useCallback(
    (item: ChatAreaMessage): AssistantReaction => {
      if (Object.hasOwn(overrides, item.publicID)) {
        return overrides[item.publicID] ?? null;
      }
      return item.myFeedback ?? null;
    },
    [overrides],
  );

  const onReactAssistantMessage = React.useCallback(
    async (publicID: string, reaction: AssistantReaction) => {
      const target = messages.find((item) => item.publicID === publicID && item.role === "assistant");
      if (!target) {
        return;
      }

      const previousReaction = getReaction(target);
      setOverrides((prev) => ({
        ...prev,
        [publicID]: reaction,
      }));

      if (!persist) {
        if (reaction === "up") {
          toast.success(t("liked"));
        } else if (reaction === "down") {
          toast.success(t("disliked"));
        } else {
          toast.success(t("cleared"));
        }
        return;
      }

      try {
        const token = await resolveAccessToken();
        if (!token) {
          throw new Error(t("signInRequired"));
        }

        const result = await setMessageFeedback(token, publicID, reaction ? { feedback: reaction } : {});
        setOverrides((prev) => ({
          ...prev,
          [publicID]: result.myFeedback || null,
        }));

        if (result.myFeedback === "up") {
          toast.success(t("liked"));
          return;
        }
        if (result.myFeedback === "down") {
          toast.success(t("disliked"));
          return;
        }
        toast.success(t("cleared"));
      } catch (error) {
        setOverrides((prev) => ({
          ...prev,
          [publicID]: previousReaction,
        }));
        const description = resolveErrorMessage(error, t("retryLater"));
        toast.error(t("failed"), { description });
      }
    },
    [getReaction, messages, persist, resolveErrorMessage, t],
  );

  return {
    getReaction,
    onReactAssistantMessage,
  };
}
