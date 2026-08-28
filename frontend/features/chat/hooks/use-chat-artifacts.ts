"use client";

import * as React from "react";

import {
  type ChatArtifact,
  extractArtifactsFromContent,
  extractArtifactsFromMessages,
  type OpenCodeArtifactInput,
} from "@/features/chat/model/chat-artifacts";
import type { ChatAreaMessage } from "@/features/chat/types/messages";

type UseChatArtifactsParams = {
  scopeKey: string | null;
  transient?: boolean;
  messages: ChatAreaMessage[];
};

type ChatArtifactInlineLayout = "balanced" | "wide";

const ARTIFACT_INLINE_BREAKPOINT = 768;
const ARTIFACT_WIDE_BREAKPOINT = 1280;
const ARTIFACT_MIN_RATIO = 1 / 3;
const ARTIFACT_MAX_RATIO = 3 / 4;
const ARTIFACT_BALANCED_RATIO = 1 / 2;

function resolveInlineLayout(viewportWidth: number): ChatArtifactInlineLayout {
  return viewportWidth >= ARTIFACT_WIDE_BREAKPOINT ? "wide" : "balanced";
}

function resolveDefaultRatio(layout: ChatArtifactInlineLayout): number {
  return layout === "wide" ? ARTIFACT_MIN_RATIO : ARTIFACT_BALANCED_RATIO;
}

function clampArtifactRatio(value: number): number {
  return Math.min(ARTIFACT_MAX_RATIO, Math.max(ARTIFACT_MIN_RATIO, value));
}

function useArtifactViewport() {
  const [viewport, setViewport] = React.useState({
    isInline: false,
    inlineLayout: "balanced" as ChatArtifactInlineLayout,
  });

  React.useEffect(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
      return;
    }

    const mediaQuery = window.matchMedia(`(min-width: ${ARTIFACT_INLINE_BREAKPOINT}px)`);
    const syncViewport = () => {
      setViewport({
        isInline: mediaQuery.matches,
        inlineLayout: resolveInlineLayout(window.innerWidth),
      });
    };

    syncViewport();
    mediaQuery.addEventListener("change", syncViewport);
    window.addEventListener("resize", syncViewport);
    return () => {
      mediaQuery.removeEventListener("change", syncViewport);
      window.removeEventListener("resize", syncViewport);
    };
  }, []);

  return viewport;
}

function hasRelatedCode(current: string, previous: string): boolean {
  const currentCode = current.trim();
  const previousCode = previous.trim();
  return Boolean(
    currentCode &&
      previousCode &&
      (currentCode === previousCode || currentCode.startsWith(previousCode) || previousCode.startsWith(currentCode)),
  );
}

function isSameSlot(current: ChatArtifact, previous: ChatArtifact): boolean {
  return current.kind === previous.kind && current.blockIndex === previous.blockIndex;
}

function isSameLogicalArtifact(current: ChatArtifact, previous: ChatArtifact): boolean {
  if (current.id === previous.id) return true;
  if (!isSameSlot(current, previous)) return false;
  if (current.runID && previous.runID && current.runID === previous.runID) return true;
  if (current.messageID === previous.messageID || current.messageKey === previous.messageKey) return true;
  return hasRelatedCode(current.code, previous.code);
}

function findLatestArtifactInSameSlot(artifacts: ChatArtifact[], previous: ChatArtifact): ChatArtifact | null {
  for (let index = artifacts.length - 1; index >= 0; index -= 1) {
    const artifact = artifacts[index];
    if (artifact && isSameSlot(artifact, previous)) {
      return artifact;
    }
  }
  return null;
}

function findReplacementArtifact(artifacts: ChatArtifact[], previous: ChatArtifact | null): ChatArtifact | null {
  if (!previous) return null;

  return (
    artifacts.find((artifact) => artifact.id === previous.id) ??
    artifacts.find(
      (artifact) => isSameSlot(artifact, previous) && Boolean(artifact.runID && previous.runID && artifact.runID === previous.runID),
    ) ??
    artifacts.find(
      (artifact) => isSameSlot(artifact, previous) && (artifact.messageID === previous.messageID || artifact.messageKey === previous.messageKey),
    ) ??
    artifacts.find((artifact) => isSameSlot(artifact, previous) && hasRelatedCode(artifact.code, previous.code)) ??
    (previous.streaming ? findLatestArtifactInSameSlot(artifacts, previous) : null) ??
    null
  );
}

export function useChatArtifacts({ scopeKey, transient = false, messages }: UseChatArtifactsParams) {
  const { isInline, inlineLayout } = useArtifactViewport();
  const artifacts = React.useMemo(() => extractArtifactsFromMessages(messages), [messages]);
  const latestArtifact = artifacts.at(-1) ?? null;
  const [activeArtifactID, setActiveArtifactID] = React.useState<string | null>(null);
  const [dismissedArtifactID, setDismissedArtifactID] = React.useState<string | null>(null);
  const [lastActiveArtifact, setLastActiveArtifact] = React.useState<ChatArtifact | null>(null);
  const [customArtifactRatio, setCustomArtifactRatio] = React.useState<number | null>(null);
  const dismissedArtifactRef = React.useRef<ChatArtifact | null>(null);
  const previousScopeRef = React.useRef({ key: scopeKey, transient });
  const artifactRatio = customArtifactRatio ?? resolveDefaultRatio(inlineLayout);
  const activeArtifact = React.useMemo(
    () =>
      activeArtifactID
        ? artifacts.find((artifact) => artifact.id === activeArtifactID) ??
          findReplacementArtifact(artifacts, lastActiveArtifact) ??
          lastActiveArtifact
        : null,
    [activeArtifactID, artifacts, lastActiveArtifact],
  );

  React.useEffect(() => {
    const previousScope = previousScopeRef.current;
    if (previousScope.key === scopeKey && previousScope.transient === transient) {
      return;
    }
    if (
      !previousScope.transient &&
      !transient &&
      (activeArtifact?.streaming || latestArtifact?.streaming || lastActiveArtifact?.streaming)
    ) {
      return;
    }
    previousScopeRef.current = { key: scopeKey, transient };
    setLastActiveArtifact(null);
    dismissedArtifactRef.current = null;
    setActiveArtifactID(null);
    setDismissedArtifactID(null);
  }, [activeArtifact?.streaming, lastActiveArtifact?.streaming, latestArtifact?.streaming, scopeKey, transient]);

  React.useEffect(() => {
    if (activeArtifact) {
      setLastActiveArtifact(activeArtifact);
    }
  }, [activeArtifact]);

  React.useEffect(() => {
    if (artifacts.length === 0 && !activeArtifactID) {
      setLastActiveArtifact(null);
      dismissedArtifactRef.current = null;
      setDismissedArtifactID(null);
      return;
    }

    if (!activeArtifactID || activeArtifact?.id === activeArtifactID) {
      return;
    }

    setActiveArtifactID(activeArtifact?.id ?? null);
  }, [activeArtifact, activeArtifactID, artifacts]);

  React.useEffect(() => {
    if (!isInline || !latestArtifact?.streaming || dismissedArtifactID === latestArtifact.id) {
      return;
    }
    if (dismissedArtifactRef.current && isSameLogicalArtifact(latestArtifact, dismissedArtifactRef.current)) {
      return;
    }
    if (activeArtifactID !== latestArtifact.id) {
      setActiveArtifactID(latestArtifact.id);
    }
  }, [activeArtifactID, dismissedArtifactID, isInline, latestArtifact]);

  const openArtifact = React.useCallback((message: ChatAreaMessage, input: OpenCodeArtifactInput) => {
    const messageArtifacts = extractArtifactsFromContent(message);
    const selected =
      messageArtifacts.find((artifact) => artifact.kind === input.kind && artifact.code === input.code) ??
      messageArtifacts.find((artifact) => artifact.kind === input.kind) ??
      messageArtifacts.at(-1);

    if (!selected) return;

    dismissedArtifactRef.current = null;
    setDismissedArtifactID(null);
    setActiveArtifactID(selected.id);
  }, []);

  const closeArtifact = React.useCallback(() => {
    const dismissedArtifact = activeArtifact ?? latestArtifact;
    dismissedArtifactRef.current = dismissedArtifact ?? null;
    setDismissedArtifactID(dismissedArtifact?.id ?? null);
    setActiveArtifactID(null);
  }, [activeArtifact, latestArtifact]);

  const selectArtifact = React.useCallback((artifactID: string) => {
    setActiveArtifactID(artifactID);
  }, []);

  const setArtifactRatio = React.useCallback((ratio: number) => {
    setCustomArtifactRatio(clampArtifactRatio(ratio));
  }, []);

  const resetArtifactRatio = React.useCallback(() => {
    setCustomArtifactRatio(null);
  }, []);

  return {
    activeArtifact,
    artifactRatio,
    artifacts,
    closeArtifact,
    isInlineViewport: isInline,
    openArtifact,
    resetArtifactRatio,
    selectArtifact,
    setArtifactRatio,
  };
}
