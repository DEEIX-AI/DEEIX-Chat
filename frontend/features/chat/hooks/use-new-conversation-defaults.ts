"use client";

import * as React from "react";

import { listVisibleSkills } from "@/shared/api/skills";
import type { SkillSummaryDTO } from "@/shared/api/skills.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

export function useNewConversationDefaults({
  conversationID,
  contextKey,
  defaultsPending,
  defaultMCPToolIDs,
  defaultSkillIDs,
  toolsLoading,
  setSelectedToolIDs,
  setSelectedSkills,
}: {
  conversationID: string | null;
  contextKey: string;
  defaultsPending: boolean;
  defaultMCPToolIDs: number[];
  defaultSkillIDs: number[];
  toolsLoading: boolean;
  setSelectedToolIDs: React.Dispatch<React.SetStateAction<number[]>>;
  setSelectedSkills: React.Dispatch<React.SetStateAction<SkillSummaryDTO[]>>;
}) {
  const appliedMCPDefaultsKeyRef = React.useRef("");
  const appliedSkillDefaultsKeyRef = React.useRef("");
  const manuallyChangedMCPKeyRef = React.useRef("");
  const manuallyChangedSkillKeyRef = React.useRef("");

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

  return { onSelectedSkillsChange, onSelectedToolsChange };
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
