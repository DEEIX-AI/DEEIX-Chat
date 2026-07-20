"use client";

import * as React from "react";

import { getConversationPreviewMessages } from "@/shared/api/conversation";
import type { ConversationPreviewMessageDTO } from "@/shared/api/conversation.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

const PREVIEW_DEBOUNCE_MS = 160;
const PREVIEW_CACHE_LIMIT = 24;

type ConversationPreviewState = {
  conversationID: string;
  messages: ConversationPreviewMessageDTO[];
  loading: boolean;
  loadFailed: boolean;
};

const EMPTY_PREVIEW_STATE: ConversationPreviewState = {
  conversationID: "",
  messages: [],
  loading: false,
  loadFailed: false,
};

export function useConversationSearchPreview(open: boolean) {
  const [preview, setPreview] = React.useState<ConversationPreviewState>(EMPTY_PREVIEW_STATE);
  const [revision, setRevision] = React.useState(0);
  const requestVersionRef = React.useRef(0);
  const selectedConversationIDRef = React.useRef("");
  const cacheRef = React.useRef(new Map<string, ConversationPreviewMessageDTO[]>());

  React.useEffect(() => {
    if (open) {
      return;
    }
    setPreview(EMPTY_PREVIEW_STATE);
    requestVersionRef.current += 1;
    selectedConversationIDRef.current = "";
    cacheRef.current.clear();
  }, [open]);

  React.useEffect(() => {
    const conversationID = preview.conversationID;
    if (!open || !conversationID || cacheRef.current.has(conversationID)) {
      return;
    }

    requestVersionRef.current += 1;
    const requestVersion = requestVersionRef.current;
    const abortController = new AbortController();
    const timer = window.setTimeout(() => {
      void (async () => {
        try {
          const token = await resolveAccessToken();
          if (requestVersion !== requestVersionRef.current) {
            return;
          }
          if (!token) {
            setPreview({
              conversationID,
              messages: [],
              loading: false,
              loadFailed: true,
            });
            return;
          }
          const messages = await getConversationPreviewMessages(
            token,
            conversationID,
            abortController.signal,
          );
          if (requestVersion !== requestVersionRef.current) {
            return;
          }
          writePreviewCache(cacheRef.current, conversationID, messages);
          setPreview({
            conversationID,
            messages,
            loading: false,
            loadFailed: false,
          });
        } catch {
          if (requestVersion === requestVersionRef.current) {
            setPreview({
              conversationID,
              messages: [],
              loading: false,
              loadFailed: true,
            });
          }
        }
      })();
    }, PREVIEW_DEBOUNCE_MS);

    return () => {
      window.clearTimeout(timer);
      if (requestVersionRef.current === requestVersion) {
        requestVersionRef.current += 1;
      }
      abortController.abort();
    };
  }, [open, preview.conversationID, revision]);

  const selectPreview = React.useCallback((conversationID: string) => {
    if (selectedConversationIDRef.current === conversationID) {
      return;
    }
    selectedConversationIDRef.current = conversationID;
    requestVersionRef.current += 1;
    if (!conversationID) {
      setPreview(EMPTY_PREVIEW_STATE);
      return;
    }
    const cached = readPreviewCache(cacheRef.current, conversationID);
    setPreview({
      conversationID,
      messages: cached ?? [],
      loading: !cached,
      loadFailed: false,
    });
  }, []);

  const retryPreview = React.useCallback(() => {
    const conversationID = preview.conversationID;
    if (!conversationID) {
      return;
    }
    requestVersionRef.current += 1;
    cacheRef.current.delete(conversationID);
    setPreview((current) => ({
      ...current,
      messages: [],
      loading: true,
      loadFailed: false,
    }));
    setRevision((current) => current + 1);
  }, [preview.conversationID]);

  return {
    preview,
    selectPreview,
    retryPreview,
  };
}

function readPreviewCache(
  cache: Map<string, ConversationPreviewMessageDTO[]>,
  conversationID: string,
) {
  const cached = cache.get(conversationID);
  if (!cached) {
    return undefined;
  }
  cache.delete(conversationID);
  cache.set(conversationID, cached);
  return cached;
}

function writePreviewCache(
  cache: Map<string, ConversationPreviewMessageDTO[]>,
  conversationID: string,
  messages: ConversationPreviewMessageDTO[],
) {
  cache.delete(conversationID);
  cache.set(conversationID, messages);
  while (cache.size > PREVIEW_CACHE_LIMIT) {
    const oldestConversationID = cache.keys().next().value;
    if (!oldestConversationID) {
      break;
    }
    cache.delete(oldestConversationID);
  }
}
