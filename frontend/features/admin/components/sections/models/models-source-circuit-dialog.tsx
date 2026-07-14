"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { SpinnerLabel } from "@/components/ui/spinner";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import type {
  AdminLLMModelUpstreamSourceDTO,
  AdminLLMModelCbPolicyMode,
  UpdateAdminLLMModelUpstreamSourceRequest,
} from "@/features/admin/api/llm.types";

export type ModelSourceCircuitPayload = Required<
  Pick<UpdateAdminLLMModelUpstreamSourceRequest, "cbFailureThreshold" | "cbDurationMin" | "cbWindowMin">
>;

type CircuitDraft = Record<keyof ModelSourceCircuitPayload, string>;

type ModelSourceCircuitDialogProps = {
  source: AdminLLMModelUpstreamSourceDTO | null;
  policyMode?: AdminLLMModelCbPolicyMode;
  pending: boolean;
  onClose: () => void;
  onSave: (payload: ModelSourceCircuitPayload) => Promise<void>;
};

function parseNonNegativeInteger(value: string): number {
  const parsed = Number.parseInt(value.trim() || "0", 10);
  return Number.isFinite(parsed) ? Math.max(parsed, 0) : 0;
}

export function ModelSourceCircuitDialog({
  source,
  policyMode,
  pending,
  onClose,
  onSave,
}: ModelSourceCircuitDialogProps) {
  const t = useTranslations("adminModels.sources");
  const commonT = useTranslations("common");
  const [draft, setDraft] = React.useState<CircuitDraft>({
    cbFailureThreshold: "0",
    cbDurationMin: "0",
    cbWindowMin: "0",
  });
  const stableSource = useDialogSnapshot(source);

  React.useEffect(() => {
    if (!source) return;
    setDraft({
      cbFailureThreshold: String(source.cbFailureThreshold ?? 0),
      cbDurationMin: String(source.cbDurationMin ?? 0),
      cbWindowMin: String(source.cbWindowMin ?? 0),
    });
  }, [source]);

  function setField(field: keyof CircuitDraft, value: string) {
    setDraft((current) => ({ ...current, [field]: value }));
  }

  const sourceName = stableSource ? [stableSource.upstreamName, stableSource.upstreamModelName].filter(Boolean).join(" / ") : "";

  async function handleSave() {
    await onSave({
      cbFailureThreshold: parseNonNegativeInteger(draft.cbFailureThreshold),
      cbDurationMin: parseNonNegativeInteger(draft.cbDurationMin),
      cbWindowMin: parseNonNegativeInteger(draft.cbWindowMin),
    });
  }

  return (
    <Dialog
      open={!!source}
      onOpenChange={(open) => {
        if (!open && !pending) {
          onClose();
        }
      }}
    >
      <DialogContent className="flex max-h-[min(86vh,760px)] w-[calc(100vw-2rem)] flex-col gap-0 overflow-hidden p-0 sm:max-w-[520px]">
        <DialogHeader className="shrink-0 px-4 py-4">
          <DialogTitle>{t("circuitSettings")}</DialogTitle>
          <DialogDescription>{t("circuitSettingsDescription", { name: sourceName })}</DialogDescription>
        </DialogHeader>

        <form
          className="flex min-h-0 flex-1 flex-col"
          onSubmit={(event) => {
            event.preventDefault();
            void handleSave();
          }}
        >
          <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 py-2">
            {policyMode === "enforced" ? (
              <div className="rounded-md bg-muted px-3 py-2 text-xs leading-5 text-muted-foreground">
                {t("circuitPolicyOverriddenHint")}
              </div>
            ) : null}
            <div className="space-y-1">
              <Label className="text-xs font-normal text-muted-foreground" htmlFor="source-cb-failure-threshold">
                {t("failureThreshold")}
              </Label>
              <Input
                id="source-cb-failure-threshold"
                type="number"
                min={0}
                step={1}
                value={draft.cbFailureThreshold}
                disabled={pending}
                onChange={(event) => setField("cbFailureThreshold", event.target.value)}
              />
            </div>
            <div className="space-y-1">
              <Label className="text-xs font-normal text-muted-foreground" htmlFor="source-cb-duration-min">
                {t("circuitDuration")}
              </Label>
              <Input
                id="source-cb-duration-min"
                type="number"
                min={0}
                step={1}
                value={draft.cbDurationMin}
                disabled={pending}
                onChange={(event) => setField("cbDurationMin", event.target.value)}
              />
            </div>
            <div className="space-y-1">
              <Label className="text-xs font-normal text-muted-foreground" htmlFor="source-cb-window-min">
                {t("circuitWindow")}
              </Label>
              <Input
                id="source-cb-window-min"
                type="number"
                min={0}
                step={1}
                value={draft.cbWindowMin}
                disabled={pending}
                onChange={(event) => setField("cbWindowMin", event.target.value)}
              />
            </div>
            <p className="text-xs leading-5 text-muted-foreground">{t("circuitInheritHint")}</p>
          </div>

          <DialogFooter className="shrink-0 px-4 py-3">
            <Button type="button" variant="ghost" disabled={pending} onClick={onClose}>
              {commonT("actions.cancel")}
            </Button>
            <Button type="submit" disabled={pending}>
              {pending ? <SpinnerLabel>{t("savingCircuitSettings")}</SpinnerLabel> : t("saveCircuitSettings")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
