import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import { useShellProjectCatalog } from "@/features/shell/hooks/use-shell-project-catalog";
import type { ProjectDraft } from "@/features/shell/types/project";
import { listVisibleKnowledgeBases } from "@/shared/api/knowledge-bases";
import type { KnowledgeBaseDTO } from "@/shared/api/knowledge-bases-types";
import { listAvailableMCPTools } from "@/shared/api/mcp";
import type { MCPToolDTO } from "@/shared/api/mcp-types";
import { listPublicModels } from "@/shared/api/model";
import type { PublicModelDTO } from "@/shared/api/model-types";
import { getMCPPolicy } from "@/shared/api/settings";
import { listVisibleSkills } from "@/shared/api/skills";
import type { SkillSummaryDTO } from "@/shared/api/skills-types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { normalizeImageAttachmentProcessorSelection } from "@/entities/mcp";

function getSkillID(skill: SkillSummaryDTO): number {
  return skill.id;
}

function getKnowledgeBaseID(knowledgeBase: KnowledgeBaseDTO): string {
  return knowledgeBase.publicID;
}

/**
 * Loads the catalogs behind the project dialog's default pickers (MCP tools and
 * policy, models, skills, knowledge bases) while the dialog is open, and prunes
 * draft selections that are no longer available.
 */
export function useShellProjectDialogDefaults({
  open,
  draft,
  setDraft,
  knowledgeBaseEnabled,
}: {
  open: boolean;
  draft: ProjectDraft | null;
  setDraft: React.Dispatch<React.SetStateAction<ProjectDraft | null>>;
  knowledgeBaseEnabled: boolean;
}) {
  const t = useTranslations("recent.projects");
  const [catalogLoading, setCatalogLoading] = React.useState(false);
  const [modelCatalogLoading, setModelCatalogLoading] = React.useState(false);
  const [mcpTools, setMCPTools] = React.useState<MCPToolDTO[]>([]);
  const [models, setModels] = React.useState<PublicModelDTO[]>([]);
  const [selectionLimit, setSelectionLimit] = React.useState(1);

  React.useEffect(() => {
    if (!open) {
      setCatalogLoading(false);
      setMCPTools([]);
      return;
    }

    let cancelled = false;
    setCatalogLoading(true);
    void (async () => {
      try {
        const token = await resolveAccessToken();
        if (!token) {
          throw new Error("missing access token");
        }
        const [tools, policy] = await Promise.all([
          listAvailableMCPTools(token),
          getMCPPolicy(token),
        ]);
        if (!cancelled) {
          setMCPTools(tools);
          setSelectionLimit(Math.max(1, policy.maxSelectedToolsPerMessage));
          const availableMCPToolIDs = new Set(tools.map((tool) => tool.id));
          setDraft((current) => {
            if (!current) {
              return current;
            }
            const defaultMCPToolIDs = normalizeImageAttachmentProcessorSelection(
              current.defaultMCPToolIDs.filter((id) => availableMCPToolIDs.has(id)).slice(0, Math.max(1, policy.maxSelectedToolsPerMessage)),
              tools,
            );
            const unchangedMCPTools = defaultMCPToolIDs.length === current.defaultMCPToolIDs.length;
            return unchangedMCPTools
              ? current
              : { ...current, defaultMCPToolIDs };
          });
        }
      } catch {
        if (!cancelled) {
          setMCPTools([]);
          toast.error(t("defaultsLoadFailed"));
        }
      } finally {
        if (!cancelled) {
          setCatalogLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open, setDraft, t]);

  React.useEffect(() => {
    if (!open) {
      setModelCatalogLoading(false);
      setModels([]);
      return;
    }

    const controller = new AbortController();
    setModelCatalogLoading(true);
    void (async () => {
      try {
        const token = await resolveAccessToken();
        if (!token) {
          throw new Error("missing access token");
        }
        const items = await listPublicModels(token, controller.signal);
        if (!controller.signal.aborted) {
          setModels(items);
        }
      } catch {
        if (!controller.signal.aborted) {
          setModels([]);
          toast.error(t("defaultModelsLoadFailed"));
        }
      } finally {
        if (!controller.signal.aborted) {
          setModelCatalogLoading(false);
        }
      }
    })();
    return () => {
      controller.abort();
    };
  }, [open, t]);

  const handleCatalogLoadError = React.useCallback(() => {
    toast.error(t("defaultsLoadFailed"));
  }, [t]);
  const handleSkillIDsResolved = React.useCallback((
    requestedIDs: number[],
    availableIDs: ReadonlySet<number>,
  ) => {
    const requestedIDSet = new Set(requestedIDs);
    setDraft((current) => {
      if (!current) return current;
      const defaultSkillIDs = current.defaultSkillIDs.filter(
        (id) => !requestedIDSet.has(id) || availableIDs.has(id),
      );
      return defaultSkillIDs.length === current.defaultSkillIDs.length
        ? current
        : { ...current, defaultSkillIDs };
    });
  }, [setDraft]);
  const handleKnowledgeBaseIDsResolved = React.useCallback((
    requestedIDs: string[],
    availableIDs: ReadonlySet<string>,
  ) => {
    const requestedIDSet = new Set(requestedIDs);
    setDraft((current) => {
      if (!current) return current;
      const defaultKnowledgeBaseIDs = current.defaultKnowledgeBaseIDs
        .filter((id) => !requestedIDSet.has(id) || availableIDs.has(id))
        .slice(0, 8);
      return defaultKnowledgeBaseIDs.length === current.defaultKnowledgeBaseIDs.length &&
        defaultKnowledgeBaseIDs.every((id, index) => id === current.defaultKnowledgeBaseIDs[index])
        ? current
        : { ...current, defaultKnowledgeBaseIDs };
    });
  }, [setDraft]);
  const skillCatalog = useShellProjectCatalog({
    open,
    selectedIDs: draft?.defaultSkillIDs ?? [],
    loadPage: listVisibleSkills,
    getID: getSkillID,
    onSelectedIDsResolved: handleSkillIDsResolved,
    onError: handleCatalogLoadError,
  });
  const knowledgeBaseCatalog = useShellProjectCatalog({
    open: open && knowledgeBaseEnabled,
    selectedIDs: draft?.defaultKnowledgeBaseIDs.slice(0, 8) ?? [],
    loadPage: listVisibleKnowledgeBases,
    getID: getKnowledgeBaseID,
    onSelectedIDsResolved: handleKnowledgeBaseIDsResolved,
    onError: handleCatalogLoadError,
  });

  return {
    catalogLoading,
    modelCatalogLoading,
    mcpTools,
    models,
    selectionLimit,
    skillCatalog,
    knowledgeBaseCatalog,
  };
}
