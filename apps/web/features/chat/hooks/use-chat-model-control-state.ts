"use client";

import * as React from "react";

import {
  type ChatModelControlSelections,
  controlSelectionsForRequest,
  pruneControlSelections,
  readRememberedControlSelections,
  writeRememberedControlSelections,
} from "@/features/chat/model/chat-model-controls";
import type { ChatModelOption } from "@/features/chat/types/chat-runtime";
import type { ModelControlValue } from "@/entities/model";

const EMPTY_CONTROLS: ChatModelOption["controls"] = [];

/**
 * Control selections of the selected model. Choices are remembered per model on this device, so a
 * new conversation starts from the last selection; values for controls the administrator removed or
 * locked are dropped automatically.
 */
export function useChatModelControlState(selectedModel: ChatModelOption | null) {
  const platformModelName = selectedModel?.platformModelName.trim() ?? "";
  const controls = selectedModel?.controls ?? EMPTY_CONTROLS;
  const [remembered, setRemembered] = React.useState<{ model: string; selections: ChatModelControlSelections }>({
    model: "",
    selections: {},
  });

  React.useEffect(() => {
    setRemembered({ model: platformModelName, selections: readRememberedControlSelections(platformModelName) });
  }, [platformModelName]);

  const selections = React.useMemo(
    () => (remembered.model === platformModelName ? pruneControlSelections(controls, remembered.selections) : {}),
    [controls, platformModelName, remembered],
  );

  const setControlValue = React.useCallback(
    (controlID: string, value: ModelControlValue | null) => {
      if (!platformModelName) return;
      setRemembered((current) => {
        const base = current.model === platformModelName ? pruneControlSelections(controls, current.selections) : {};
        const next = { ...base };
        if (value === null) {
          delete next[controlID];
        } else {
          next[controlID] = value;
        }
        const pruned = pruneControlSelections(controls, next);
        writeRememberedControlSelections(platformModelName, pruned);
        return { model: platformModelName, selections: pruned };
      });
    },
    [controls, platformModelName],
  );

  const requestControls = React.useMemo(() => controlSelectionsForRequest(controls, selections), [controls, selections]);

  return { controls, selections, setControlValue, requestControls };
}
