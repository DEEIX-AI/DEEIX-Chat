"use client";

import * as React from "react";

import { listVisibleKnowledgeBases } from "@/shared/api/knowledge-bases";
import { listVisibleSkills } from "@/shared/api/skills";
import type { SkillSummaryDTO } from "@/shared/api/skills.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

export function useChatConversationDefaults({
  conversationID,
  contextKey,
  defaultsPending,
  defaultMCPToolIDs,
  defaultSkillIDs,
  defaultKnowledgeBaseIDs,
  toolsLoading,
  setSelectedToolIDs,
  setSelectedSkills,
  setSelectedKnowledgeBaseIDs,
}: {
  conversationID: string | null;
  contextKey: string;
  defaultsPending: boolean;
  defaultMCPToolIDs: number[];
  defaultSkillIDs: number[];
  defaultKnowledgeBaseIDs: string[];
  toolsLoading: boolean;
  setSelectedToolIDs: React.Dispatch<React.SetStateAction<number[]>>;
  setSelectedSkills: React.Dispatch<React.SetStateAction<SkillSummaryDTO[]>>;
  setSelectedKnowledgeBaseIDs: React.Dispatch<React.SetStateAction<string[]>>;
}) {
  const appliedMCPDefaultsKeyRef = React.useRef("");
  const appliedSkillDefaultsKeyRef = React.useRef("");
  const manuallyChangedMCPKeyRef = React.useRef("");
  const manuallyChangedSkillKeyRef = React.useRef("");
  const appliedKnowledgeBaseDefaultsKeyRef = React.useRef("");
  const manuallyChangedKnowledgeBaseKeyRef = React.useRef("");

  React.useEffect(() => {
    if (conversationID || toolsLoading || defaultsPending) {
      return;
    }
    if (
      appliedMCPDefaultsKeyRef.current === contextKey ||
      manuallyChangedMCPKeyRef.current === contextKey
    ) {
      return;
    }
    appliedMCPDefaultsKeyRef.current = contextKey;
    setSelectedToolIDs(defaultMCPToolIDs);
  }, [conversationID, contextKey, defaultMCPToolIDs, defaultsPending, setSelectedToolIDs, toolsLoading]);

  React.useEffect(() => {
    if (conversationID || defaultsPending) {
      return;
    }
    if (
      appliedSkillDefaultsKeyRef.current === contextKey ||
      manuallyChangedSkillKeyRef.current === contextKey
    ) {
      return;
    }
    if (defaultSkillIDs.length === 0) {
      appliedSkillDefaultsKeyRef.current = contextKey;
      setSelectedSkills([]);
      return;
    }

    let cancelled = false;
    void (async () => {
      try {
        const token = await resolveAccessToken();
        if (!token) {
          return;
        }
        const availableSkills = await listVisibleSkillsByIDs(token, defaultSkillIDs);
        if (cancelled || manuallyChangedSkillKeyRef.current === contextKey) {
          return;
        }
        const skillsByID = new Map(availableSkills.map((skill) => [skill.id, skill] as const));
        const defaults = defaultSkillIDs
          .map((skillID) => skillsByID.get(skillID))
          .filter((skill): skill is SkillSummaryDTO => Boolean(skill));
        appliedSkillDefaultsKeyRef.current = contextKey;
        setSelectedSkills(defaults);
      } catch {
        if (!cancelled && manuallyChangedSkillKeyRef.current !== contextKey) {
          appliedSkillDefaultsKeyRef.current = contextKey;
          setSelectedSkills([]);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [conversationID, contextKey, defaultSkillIDs, defaultsPending, setSelectedSkills]);

  React.useEffect(() => {
    if (
      conversationID ||
      defaultsPending ||
      appliedKnowledgeBaseDefaultsKeyRef.current === contextKey ||
      manuallyChangedKnowledgeBaseKeyRef.current === contextKey
    ) {
      return;
    }
    if (defaultKnowledgeBaseIDs.length === 0) {
      appliedKnowledgeBaseDefaultsKeyRef.current = contextKey;
      setSelectedKnowledgeBaseIDs([]);
      return;
    }

    let cancelled = false;
    void (async () => {
      try {
        const token = await resolveAccessToken();
        if (!token) return;
        const available = await listVisibleKnowledgeBases(token, {
          ids: defaultKnowledgeBaseIDs.slice(0, 8),
          pageSize: Math.max(1, Math.min(8, defaultKnowledgeBaseIDs.length)),
        });
        if (cancelled || manuallyChangedKnowledgeBaseKeyRef.current === contextKey) return;
        const readyIDs = new Set(
          available.results.filter((item) => item.readyFileCount > 0).map((item) => item.publicID),
        );
        appliedKnowledgeBaseDefaultsKeyRef.current = contextKey;
        setSelectedKnowledgeBaseIDs(defaultKnowledgeBaseIDs.filter((id) => readyIDs.has(id)).slice(0, 8));
      } catch {
        if (!cancelled && manuallyChangedKnowledgeBaseKeyRef.current !== contextKey) {
          appliedKnowledgeBaseDefaultsKeyRef.current = contextKey;
          setSelectedKnowledgeBaseIDs([]);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [conversationID, contextKey, defaultKnowledgeBaseIDs, defaultsPending, setSelectedKnowledgeBaseIDs]);

  const onSelectedToolsChange = React.useCallback((toolIDs: number[]) => {
    if (!conversationID) {
      manuallyChangedMCPKeyRef.current = contextKey;
    }
    setSelectedToolIDs(toolIDs);
  }, [conversationID, contextKey, setSelectedToolIDs]);

  const onSelectedSkillsChange = React.useCallback((skills: SkillSummaryDTO[]) => {
    if (!conversationID) {
      manuallyChangedSkillKeyRef.current = contextKey;
    }
    setSelectedSkills(skills);
  }, [conversationID, contextKey, setSelectedSkills]);

  const onSelectedKnowledgeBasesChange = React.useCallback((ids: string[]) => {
    if (!conversationID) {
      manuallyChangedKnowledgeBaseKeyRef.current = contextKey;
    }
    setSelectedKnowledgeBaseIDs(ids.slice(0, 8));
  }, [conversationID, contextKey, setSelectedKnowledgeBaseIDs]);

  return { onSelectedKnowledgeBasesChange, onSelectedSkillsChange, onSelectedToolsChange };
}

async function listVisibleSkillsByIDs(accessToken: string, skillIDs: number[]): Promise<SkillSummaryDTO[]> {
  const pageSize = Math.min(100, skillIDs.length);
  const firstPage = await listVisibleSkills(accessToken, { ids: skillIDs, page: 1, pageSize });
  const results = firstPage.results.slice();
  const pageCount = Math.ceil(firstPage.total / pageSize);
  for (let page = 2; page <= pageCount; page += 1) {
    const nextPage = await listVisibleSkills(accessToken, { ids: skillIDs, page, pageSize });
    results.push(...nextPage.results);
  }
  return results;
}
