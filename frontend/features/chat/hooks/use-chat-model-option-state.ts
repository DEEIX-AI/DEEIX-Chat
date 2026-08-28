"use client";

import * as React from "react";
import {
  readCachedModelOptions,
  removeCachedModelOptions,
  writeCachedModelOptions,
} from "@/features/chat/model/chat-model-options-cache";
import {
  cloneConversationOptions,
  isConversationOptionsObject,
  sanitizeConversationOptions,
} from "@/features/chat/model/conversation-options";
import type { ChatModelOption } from "@/features/chat/types/chat-runtime";
import type { ConversationOptions } from "@/shared/api/conversation.types";

/**
 * 所选模型的参数状态：切换模型时按「本地缓存优先、否则模型默认值」初始化；
 * 用户修改即写入 localStorage；模型默认值变更且用户未改动时跟随更新；支持重置与恢复后端默认值。
 */
export function useChatModelOptionState({
  selectedModel,
  selectedPlatformModelName,
  chatPreferencesLoaded,
  reuseModelOptions,
  refreshModelOption,
}: {
  selectedModel: ChatModelOption | null;
  selectedPlatformModelName: string;
  chatPreferencesLoaded: boolean;
  reuseModelOptions: boolean;
  refreshModelOption: (platformModelName: string) => Promise<ChatModelOption | null>;
}) {
  const [options, setOptions] = React.useState<ConversationOptions>({});
  const initializedOptionsModelRef = React.useRef("");
  const selectedModelDefaultOptionsRef = React.useRef<ConversationOptions>({});

  React.useEffect(() => {
    const platformModelName = selectedModel?.platformModelName.trim() || "";
    if (!selectedModel || !platformModelName) {
      initializedOptionsModelRef.current = "";
      selectedModelDefaultOptionsRef.current = {};
      setOptions({});
      return;
    }
    if (!chatPreferencesLoaded) {
      return;
    }
    const nextDefaultOptions = cloneConversationOptions(selectedModel.defaultOptions);
    const previousDefaultOptions = selectedModelDefaultOptionsRef.current;
    if (initializedOptionsModelRef.current !== platformModelName) {
      initializedOptionsModelRef.current = platformModelName;
      selectedModelDefaultOptionsRef.current = nextDefaultOptions;
      const cachedOptions = reuseModelOptions ? readCachedModelOptions(platformModelName) : null;
      setOptions(cloneConversationOptions(cachedOptions ?? nextDefaultOptions));
      return;
    }
    selectedModelDefaultOptionsRef.current = nextDefaultOptions;
    const previousDefaultOptionsJSON = JSON.stringify(previousDefaultOptions);
    if (previousDefaultOptionsJSON === JSON.stringify(nextDefaultOptions)) {
      return;
    }
    setOptions((currentOptions) => {
      if (JSON.stringify(currentOptions) !== previousDefaultOptionsJSON) {
        return currentOptions;
      }
      removeCachedModelOptions(platformModelName);
      return cloneConversationOptions(nextDefaultOptions);
    });
  }, [chatPreferencesLoaded, reuseModelOptions, selectedModel]);

  const setModelOptions = React.useCallback(
    (action: React.SetStateAction<ConversationOptions>) => {
      setOptions((previous) => {
        const next = typeof action === "function" ? action(previous) : action;
        const normalized = isConversationOptionsObject(next) ? sanitizeConversationOptions(next) : {};
        const platformModelName = selectedModel?.platformModelName.trim() || "";
        if (platformModelName) {
          writeCachedModelOptions(platformModelName, normalized);
        }
        return normalized;
      });
    },
    [selectedModel?.platformModelName],
  );

  const resetModelOptions = React.useCallback(
    (defaults?: ConversationOptions) => {
      const platformModelName = selectedModel?.platformModelName.trim() || "";
      const nextDefaults = cloneConversationOptions(defaults ?? selectedModel?.defaultOptions ?? {});
      if (platformModelName) {
        removeCachedModelOptions(platformModelName);
      }
      setOptions(nextDefaults);
    },
    [selectedModel],
  );

  const restoreBackendDefaultModelOptions = React.useCallback(async () => {
    const platformModelName = selectedModel?.platformModelName.trim() || selectedPlatformModelName.trim();
    if (!platformModelName) {
      return null;
    }
    const refreshedModel = await refreshModelOption(platformModelName);
    return refreshedModel ? cloneConversationOptions(refreshedModel.defaultOptions) : null;
  }, [refreshModelOption, selectedModel?.platformModelName, selectedPlatformModelName]);

  return {
    options,
    setModelOptions,
    resetModelOptions,
    restoreBackendDefaultModelOptions,
  };
}
