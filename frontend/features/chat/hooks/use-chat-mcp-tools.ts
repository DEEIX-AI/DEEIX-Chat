"use client";

import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";
import {
  DEFAULT_MCP_TOOLS_SETTING_KEY,
  filterAvailableMCPToolIDs,
  normalizeAvailableMCPTools,
  parseDefaultMCPToolIDs,
} from "@/features/chat/model/chat-mcp-tool-defaults";
import { listAvailableMCPTools } from "@/shared/api/mcp";
import type { MCPToolDTO } from "@/shared/api/mcp.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import {
  hasMultipleImageAttachmentProcessors,
  normalizeImageAttachmentProcessorSelection,
} from "@/shared/lib/mcp-tool-selection";
import { updateUserSettings, useUserSettings } from "@/shared/model/user-settings-store";

/**
 * MCP 工具的可用清单与默认偏好：加载并规整可用工具，对所选工具做数量上限裁剪
 * 与图片处理器唯一性归一；默认工具偏好读写用户设置，保存失败时回滚本地状态。
 */
export function useChatMCPTools({
  mcpMaxSelectedTools,
  selectedToolIDs,
  setSelectedToolIDs,
}: {
  mcpMaxSelectedTools: number;
  selectedToolIDs: number[];
  setSelectedToolIDs: React.Dispatch<React.SetStateAction<number[]>>;
}) {
  const t = useTranslations("chat");
  const { settings: userSettings, loaded: userSettingsLoaded } = useUserSettings();
  const [availableTools, setAvailableTools] = React.useState<MCPToolDTO[]>([]);
  const [toolsLoading, setToolsLoading] = React.useState(true);
  const [defaultToolIDs, setDefaultToolIDs] = React.useState<number[]>([]);

  React.useEffect(() => {
    if (toolsLoading) {
      return;
    }
    const normalized = normalizeImageAttachmentProcessorSelection(
      filterAvailableMCPToolIDs(selectedToolIDs, availableTools, mcpMaxSelectedTools),
      availableTools,
    );
    if (normalized.length === selectedToolIDs.length && normalized.every((id, index) => id === selectedToolIDs[index])) {
      return;
    }
    setSelectedToolIDs(normalized);
  }, [availableTools, mcpMaxSelectedTools, selectedToolIDs, setSelectedToolIDs, toolsLoading]);

  React.useEffect(() => {
    setSelectedToolIDs((current) => {
      if (current.length <= mcpMaxSelectedTools) {
        return current;
      }
      return current.slice(0, mcpMaxSelectedTools);
    });
  }, [mcpMaxSelectedTools, setSelectedToolIDs]);

  React.useEffect(() => {
    let cancelled = false;

    async function loadTools() {
      setToolsLoading(true);
      try {
        const token = await resolveAccessToken();
        if (!token) {
          if (!cancelled) {
            setAvailableTools([]);
            setSelectedToolIDs([]);
          }
          return;
        }
        const toolsResult = await listAvailableMCPTools(token);
        if (cancelled) {
          return;
        }
        const tools = normalizeAvailableMCPTools(toolsResult);
        setAvailableTools(tools);
        setSelectedToolIDs((previous) => normalizeImageAttachmentProcessorSelection(
          filterAvailableMCPToolIDs(previous, tools, mcpMaxSelectedTools),
          tools,
        ));
      } catch {
        if (!cancelled) {
          setAvailableTools([]);
          setSelectedToolIDs([]);
        }
      } finally {
        if (!cancelled) {
          setToolsLoading(false);
        }
      }
    }

    void loadTools();
    return () => {
      cancelled = true;
    };
  }, [mcpMaxSelectedTools, setSelectedToolIDs]);

  React.useEffect(() => {
    if (!userSettingsLoaded) {
      return;
    }
    setDefaultToolIDs(normalizeImageAttachmentProcessorSelection(
      filterAvailableMCPToolIDs(
        parseDefaultMCPToolIDs(userSettings[DEFAULT_MCP_TOOLS_SETTING_KEY]),
        availableTools,
        mcpMaxSelectedTools,
      ),
      availableTools,
    ));
  }, [availableTools, mcpMaxSelectedTools, userSettings, userSettingsLoaded]);

  const onDefaultToolIDsChange = React.useCallback(async (nextToolIDs: number[]) => {
    const nextDefaults = filterAvailableMCPToolIDs(nextToolIDs, availableTools, mcpMaxSelectedTools);
    if (hasMultipleImageAttachmentProcessors(nextDefaults, availableTools)) {
      toast.error(t("composer.mcpImageProcessorLimitTitle"), {
        description: t("composer.mcpImageProcessorLimitDescription"),
      });
      return;
    }
    const previousDefaults = defaultToolIDs;
    setDefaultToolIDs(nextDefaults);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        throw new Error(t("composer.sessionExpired"));
      }
      await updateUserSettings(token, {
        [DEFAULT_MCP_TOOLS_SETTING_KEY]: JSON.stringify(nextDefaults),
      });
      toast.success(t("composer.defaultMCPToolsSaved"));
    } catch (error) {
      setDefaultToolIDs(previousDefaults);
      toast.error(t("composer.defaultMCPToolsSaveFailed"), {
        description: error instanceof Error ? error.message : t("composer.retryLater"),
      });
    }
  }, [availableTools, defaultToolIDs, mcpMaxSelectedTools, t]);

  return {
    availableTools,
    toolsLoading,
    defaultToolIDs,
    onDefaultToolIDsChange,
  };
}
