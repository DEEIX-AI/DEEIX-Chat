"use client";

import * as React from "react";
import { GripVertical, Plus, Save, Trash2, Workflow } from "lucide-react";
import { useTranslations } from "next-intl";

import { cn } from "@/lib/utils";
import { useAdminDiscussionsTemplates } from "@/features/admin/hooks/use-admin-discussions-templates";
import { DiscussionsIconPicker } from "@/features/admin/components/sections/discussions/discussions-icon-picker";
import { DiscussionsWorkflowPreview } from "@/features/admin/components/sections/discussions/discussions-workflow-preview";
import type {
  DiscussionPlaceholder,
  DiscussionRound,
  DiscussionTemplateConfig,
  DiscussionTemplateDTO,
} from "@/shared/api/discussion-types";
import {
  defaultDiscussionRoundPrompt,
  discussionPlaceholderLabels,
  discussionRoundPrompt,
  orderDiscussionPlaceholders,
} from "@/shared/lib/discussion";

type Draft = {
  publicID?: string;
  name: string;
  description: string;
  icon: string;
  status: "active" | "inactive";
  maxRounds: number;
  sortOrder: number;
  config: DiscussionTemplateConfig;
};

function newStepID(): string {
  return `s_${Math.random().toString(36).slice(2, 10)}`;
}

function nextPlaceholderKey(placeholders: readonly DiscussionPlaceholder[]): string {
  const used = new Set(placeholders.map((item) => item.key));
  let index = 1;
  while (used.has(`p${index}`)) {
    index += 1;
  }
  return `p${index}`;
}

function createDraft(): Draft {
  return {
    name: "新讨论模板",
    description: "",
    icon: "",
    status: "active",
    maxRounds: 2,
    sortOrder: 0,
    config: {
      // The summary placeholder leads the list: it is the last node of the workflow.
      placeholders: [
        { key: "judge", label: "总结模型", defaultModel: "", required: true },
        { key: "p1", label: "模型 1", defaultModel: "", required: true },
        { key: "p2", label: "模型 2", defaultModel: "", required: true },
      ],
      rounds: [
        {
          index: 1,
          parallel: true,
          parallelPrompt: defaultDiscussionRoundPrompt(0, true),
          serialPrompt: defaultDiscussionRoundPrompt(0, false),
          steps: [
            { id: newStepID(), participant: "p1", prompt: defaultDiscussionRoundPrompt(0, true) },
            { id: newStepID(), participant: "p2", prompt: defaultDiscussionRoundPrompt(0, true) },
          ],
        },
        {
          index: 2,
          parallel: true,
          parallelPrompt: defaultDiscussionRoundPrompt(1, true),
          serialPrompt: defaultDiscussionRoundPrompt(1, false),
          steps: [
            { id: newStepID(), participant: "p1", prompt: defaultDiscussionRoundPrompt(1, true) },
            { id: newStepID(), participant: "p2", prompt: defaultDiscussionRoundPrompt(1, true) },
          ],
        },
      ],
      summary: { participant: "judge", prompt: "请综合成功发言，给出面向用户的最终答案；失败发言不是证据。" },
    },
  };
}

function seedRoundPrompt(round: DiscussionRound): DiscussionRound {
  // Legacy templates only store steps[].prompt. Treat it as the prompt of the mode they were
  // saved in, so switching away and back cannot overwrite the existing text with a default.
  const legacyPrompt = round.steps[0]?.prompt ?? "";
  return round.parallel
    ? { ...round, parallelPrompt: round.parallelPrompt ?? legacyPrompt }
    : { ...round, serialPrompt: round.serialPrompt ?? legacyPrompt };
}

function toDraft(template: DiscussionTemplateDTO): Draft {
  const config = structuredClone(template.config);
  // Legacy templates name their models A/B/C; load them as 模型 1, 模型 2 so saving persists it.
  const labels = discussionPlaceholderLabels(config.placeholders, config.summary.participant);
  config.placeholders = config.placeholders.map((item) => ({
    ...item,
    label: labels.get(item.key) ?? item.label,
  }));
  config.rounds = config.rounds.map(seedRoundPrompt);
  return {
    publicID: template.publicID,
    name: template.name,
    description: template.description,
    icon: template.icon,
    status: template.status,
    maxRounds: template.maxRounds,
    sortOrder: template.sortOrder,
    config,
  };
}

function syncRoundPrompt(round: DiscussionRound, prompt: string): DiscussionRound {
  return {
    ...round,
    steps: round.steps.map((step) => ({ ...step, prompt })),
  };
}

function normalizeRounds(rounds: DiscussionRound[]): DiscussionRound[] {
  return rounds.map((round, index) => {
    const normalized = seedRoundPrompt({
      ...round,
      index: index + 1,
      parallel: round.parallel ?? true,
    });
    // steps[].prompt mirrors the selected mode so older readers and the runtime keep one prompt.
    return syncRoundPrompt(normalized, discussionRoundPrompt(normalized));
  });
}

export function AdminDiscussionsPage() {
  const t = useTranslations("adminDiscussions");
  const {
    templates,
    models,
    loading,
    error: requestError,
    saveTemplate,
    deleteTemplate,
  } = useAdminDiscussionsTemplates();
  const [draft, setDraft] = React.useState<Draft>(createDraft());
  const [saving, setSaving] = React.useState(false);
  const [formError, setFormError] = React.useState("");
  const [workflowOpen, setWorkflowOpen] = React.useState(false);
  const [dragRound, setDragRound] = React.useState<number | null>(null);

  React.useEffect(() => {
    if (!draft.publicID && templates[0]) {
      setDraft(toDraft(templates[0]));
    }
  }, [draft.publicID, templates]);

  const summaryKey = draft.config.summary.participant;
  const orderedPlaceholders = React.useMemo(
    () => orderDiscussionPlaceholders(draft.config.placeholders, summaryKey),
    [draft.config.placeholders, summaryKey],
  );
  const discussionPlaceholders = orderedPlaceholders.filter((item) => item.key !== summaryKey);
  const labels = React.useMemo(
    () => discussionPlaceholderLabels(draft.config.placeholders, summaryKey),
    [draft.config.placeholders, summaryKey],
  );

  const updateConfig = (patch: Partial<DiscussionTemplateConfig>) => {
    setDraft((current) => ({ ...current, config: { ...current.config, ...patch } }));
  };

  const updatePlaceholder = (key: string, patch: Partial<DiscussionPlaceholder>) => {
    setDraft((current) => ({
      ...current,
      config: {
        ...current.config,
        placeholders: current.config.placeholders.map((item) =>
          item.key === key ? { ...item, ...patch } : item,
        ),
      },
    }));
  };

  const addDiscussionModel = () => {
    setDraft((current) => {
      const key = nextPlaceholderKey(current.config.placeholders);
      const index = current.config.placeholders.filter(
        (item) => item.key !== current.config.summary.participant,
      ).length + 1;
      return {
        ...current,
        config: {
          ...current.config,
          placeholders: [
            ...current.config.placeholders,
            { key, label: t("defaultModelLabel", { index }), defaultModel: "", required: true },
          ],
          // A new discussion model joins every round with that round's prompt.
          rounds: current.config.rounds.map((round) => ({
            ...round,
            steps: [
              ...round.steps,
              { id: newStepID(), participant: key, prompt: discussionRoundPrompt(round) },
            ],
          })),
        },
      };
    });
  };

  const removeDiscussionModel = (key: string) => {
    setDraft((current) => {
      if (key === current.config.summary.participant) {
        return current;
      }
      return {
        ...current,
        config: {
          ...current.config,
          placeholders: current.config.placeholders.filter((item) => item.key !== key),
          rounds: current.config.rounds.map((round) => ({
            ...round,
            steps: round.steps.filter((step) => step.participant !== key),
          })),
        },
      };
    });
  };

  const updateRoundPrompt = (roundIndex: number, prompt: string) => {
    setDraft((current) => ({
      ...current,
      config: {
        ...current.config,
        rounds: current.config.rounds.map((round, index) => {
          if (index !== roundIndex) return round;
          // Editing only touches the selected mode; the other mode keeps its own prompt.
          return round.parallel
            ? syncRoundPrompt({ ...round, parallelPrompt: prompt }, prompt)
            : syncRoundPrompt({ ...round, serialPrompt: prompt }, prompt);
        }),
      },
    }));
  };

  const updateRoundParallel = (roundIndex: number, parallel: boolean) => {
    setDraft((current) => ({
      ...current,
      config: {
        ...current.config,
        rounds: current.config.rounds.map((round, index) => {
          if (index !== roundIndex || round.parallel === parallel) return round;
          // Switching modes restores that mode's own prompt. A mode that was never edited is
          // filled with its default instead of inheriting the other mode's text.
          const stored = parallel ? round.parallelPrompt : round.serialPrompt;
          const prompt = stored?.trim() ? stored : defaultDiscussionRoundPrompt(index, parallel);
          return syncRoundPrompt(
            parallel
              ? { ...round, parallel, parallelPrompt: prompt }
              : { ...round, parallel, serialPrompt: prompt },
            prompt,
          );
        }),
      },
    }));
  };

  const resetRoundPrompt = (roundIndex: number) => {
    setDraft((current) => ({
      ...current,
      config: {
        ...current.config,
        rounds: current.config.rounds.map((round, index) => {
          if (index !== roundIndex) return round;
          const prompt = defaultDiscussionRoundPrompt(index, round.parallel);
          return round.parallel
            ? syncRoundPrompt({ ...round, parallelPrompt: prompt }, prompt)
            : syncRoundPrompt({ ...round, serialPrompt: prompt }, prompt);
        }),
      },
    }));
  };

  const toggleRoundParticipant = (roundIndex: number, participant: string) => {
    setDraft((current) => ({
      ...current,
      config: {
        ...current.config,
        rounds: current.config.rounds.map((round, index) => {
          if (index !== roundIndex) return round;
          const active = round.steps.some((step) => step.participant === participant);
          if (active) {
            // Keep at least one model in a round: the round prompt lives on its steps.
            if (round.steps.length <= 1) {
              return round;
            }
            return { ...round, steps: round.steps.filter((step) => step.participant !== participant) };
          }
          return {
            ...round,
            steps: [
              ...round.steps,
              { id: newStepID(), participant, prompt: discussionRoundPrompt(round) },
            ],
          };
        }),
      },
    }));
  };

  // Selecting every model of a round is the common case, so it gets its own switch. The
  // per-model selection it replaces is remembered and restored when the switch is turned off.
  const roundSelectionsRef = React.useRef(new Map<number, string[]>());

  const setRoundParticipants = (
    current: Draft,
    roundIndex: number,
    participants: readonly string[],
  ): Draft => ({
    ...current,
    config: {
      ...current.config,
      rounds: current.config.rounds.map((round, index) => {
        if (index !== roundIndex) return round;
        const prompt = discussionRoundPrompt(round);
        const kept = round.steps.filter((step) => participants.includes(step.participant));
        const added = participants
          .filter((participant) => !kept.some((step) => step.participant === participant))
          .map((participant) => ({ id: newStepID(), participant, prompt }));
        const ordered = participants
          .map((participant) => [...kept, ...added].find((step) => step.participant === participant))
          .filter((step): step is NonNullable<typeof step> => Boolean(step));
        return { ...round, steps: ordered };
      }),
    },
  });

  const toggleRoundAll = (roundIndex: number) => {
    setDraft((current) => {
      const round = current.config.rounds[roundIndex];
      if (!round) return current;
      const keys = current.config.placeholders
        .filter((item) => item.key !== current.config.summary.participant)
        .map((item) => item.key);
      if (keys.length === 0) return current;
      const allActive = keys.every((key) => round.steps.some((step) => step.participant === key));
      if (allActive) {
        const remembered = (roundSelectionsRef.current.get(roundIndex) ?? []).filter((key) =>
          keys.includes(key),
        );
        return setRoundParticipants(current, roundIndex, remembered.length > 0 ? remembered : keys.slice(0, 1));
      }
      roundSelectionsRef.current.set(
        roundIndex,
        round.steps.map((step) => step.participant),
      );
      return setRoundParticipants(current, roundIndex, keys);
    });
  };

  const addRound = () => {
    setDraft((current) => {
      const roundIndex = current.config.rounds.length;
      const parallelPrompt = defaultDiscussionRoundPrompt(roundIndex, true);
      return {
        ...current,
        config: {
          ...current.config,
          rounds: [
            ...current.config.rounds,
            {
              index: roundIndex + 1,
              parallel: true,
              parallelPrompt,
              serialPrompt: defaultDiscussionRoundPrompt(roundIndex, false),
              steps: current.config.placeholders
                .filter((item) => item.key !== current.config.summary.participant)
                .map((item) => ({
                  id: newStepID(),
                  participant: item.key,
                  prompt: parallelPrompt,
                })),
            },
          ],
        },
        maxRounds: Math.max(current.maxRounds, roundIndex + 1),
      };
    });
  };

  const removeRound = (index: number) => {
    setDraft((current) => ({
      ...current,
      config: {
        ...current.config,
        rounds: current.config.rounds.filter((_, itemIndex) => itemIndex !== index),
      },
    }));
  };

  const reorderRound = (from: number, to: number) => {
    if (from === to) return;
    roundSelectionsRef.current.clear();
    setDraft((current) => {
      const rounds = [...current.config.rounds];
      const [item] = rounds.splice(from, 1);
      rounds.splice(to, 0, item);
      return { ...current, config: { ...current.config, rounds: normalizeRounds(rounds) } };
    });
  };

  const save = async () => {
    setFormError("");
    if (!draft.name.trim()) {
      setFormError(t("nameRequired"));
      return;
    }
    if (draft.config.placeholders.length === 0) {
      setFormError(t("placeholderRequired"));
      return;
    }
    if (draft.config.rounds.length === 0 || draft.config.rounds.some((round) => round.steps.length === 0)) {
      setFormError(t("roundRequired"));
      return;
    }
    setSaving(true);
    const labels = discussionPlaceholderLabels(draft.config.placeholders, draft.config.summary.participant);
    const payload = {
      name: draft.name.trim(),
      description: draft.description.trim(),
      icon: draft.icon.trim(),
      status: draft.status,
      maxRounds: Math.max(draft.maxRounds, draft.config.rounds.length),
      sortOrder: draft.sortOrder,
      config: {
        ...draft.config,
        placeholders: draft.config.placeholders.map((item) => ({
          ...item,
          label: labels.get(item.key) ?? item.label,
        })),
        rounds: normalizeRounds(draft.config.rounds),
      },
    };
    const saved = await saveTemplate(draft.publicID, payload);
    if (saved) {
      setDraft(toDraft(saved));
    }
    setSaving(false);
  };

  const remove = async (publicID: string) => {
    if (await deleteTemplate(publicID)) {
      if (draft.publicID === publicID) {
        setDraft(createDraft());
      }
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h2 className="flex items-center gap-2 text-xl font-semibold">
            <Workflow className="size-5 text-primary" />
            {t("title")}
          </h2>
          <p className="mt-1 text-sm text-muted-foreground">{t("description")}</p>
        </div>
        <div className="flex items-center gap-2">
          <button
            type="button"
            aria-pressed={workflowOpen}
            className={cn(
              "inline-flex h-9 items-center gap-1.5 rounded-lg border px-3 text-sm transition-colors",
              workflowOpen
                ? "border-primary/50 bg-primary/10 text-primary"
                : "border-border hover:bg-muted",
            )}
            onClick={() => setWorkflowOpen((open) => !open)}
          >
            <Workflow className="size-4" />
            {t("workflow")}
          </button>
          <button
            type="button"
            className="inline-flex h-9 items-center gap-1.5 rounded-lg border border-border px-3 text-sm hover:bg-muted"
            onClick={() => setDraft(createDraft())}
          >
            <Plus className="size-4" />
            {t("newTemplate")}
          </button>
        </div>
      </div>

      <div className="grid gap-4 xl:grid-cols-[200px_minmax(0,1fr)]">
        <aside className="space-y-1.5 self-start rounded-xl border border-border/60 p-2">
          {loading ? <p className="p-2 text-sm text-muted-foreground">{t("loading")}</p> : null}
          {templates.map((template) => (
            <div
              key={template.publicID}
              className={cn(
                "group flex items-center gap-2 rounded-lg px-2 py-2 text-sm",
                draft.publicID === template.publicID ? "bg-primary/10 text-primary" : "hover:bg-muted",
              )}
            >
              <button
                type="button"
                className="min-w-0 flex-1 truncate text-left"
                onClick={() => setDraft(toDraft(template))}
              >
                {template.name}
                <span className="ml-1 text-[11px] text-muted-foreground">
                  {template.status === "active" ? "" : t("inactive")}
                </span>
              </button>
              <button
                type="button"
                className="opacity-0 transition-opacity group-hover:opacity-100"
                aria-label={t("delete")}
                onClick={() => void remove(template.publicID)}
              >
                <Trash2 className="size-3.5 text-muted-foreground hover:text-rose-500" />
              </button>
            </div>
          ))}
          {!loading && templates.length === 0 ? (
            <p className="p-2 text-sm text-muted-foreground">{t("empty")}</p>
          ) : null}
        </aside>

        <section className="min-w-0 space-y-4 rounded-xl border border-border/60 p-4">
          <div className="grid gap-3 md:grid-cols-2">
            <label className="space-y-1 text-sm">
              <span className="text-muted-foreground">{t("name")}</span>
              <input
                className="h-9 w-full rounded-lg border border-border/70 bg-background px-3 outline-none focus:border-primary"
                value={draft.name}
                onChange={(event) => setDraft((current) => ({ ...current, name: event.target.value }))}
              />
            </label>
            <label className="space-y-1 text-sm">
              <span className="text-muted-foreground">{t("status")}</span>
              <select
                className="h-9 w-full rounded-lg border border-border/70 bg-background px-3 outline-none focus:border-primary"
                value={draft.status}
                onChange={(event) =>
                  setDraft((current) => ({ ...current, status: event.target.value === "inactive" ? "inactive" : "active" }))
                }
              >
                <option value="active">{t("active")}</option>
                <option value="inactive">{t("inactive")}</option>
              </select>
            </label>
            <label className="space-y-1 text-sm md:col-span-2">
              <span className="text-muted-foreground">{t("descriptionLabel")}</span>
              <input
                className="h-9 w-full rounded-lg border border-border/70 bg-background px-3 outline-none focus:border-primary"
                value={draft.description}
                onChange={(event) => setDraft((current) => ({ ...current, description: event.target.value }))}
              />
            </label>
            <div className="space-y-1 text-sm">
              <span className="text-muted-foreground">{t("icon")}</span>
              <DiscussionsIconPicker
                value={draft.icon}
                label={draft.name}
                onChange={(icon) => setDraft((current) => ({ ...current, icon }))}
              />
            </div>
            <label className="space-y-1 text-sm">
              <span className="text-muted-foreground">{t("maxRounds")}</span>
              <input
                type="number"
                min={1}
                max={10}
                className="h-9 w-full rounded-lg border border-border/70 bg-background px-3 outline-none focus:border-primary"
                value={draft.maxRounds}
                onChange={(event) =>
                  setDraft((current) => ({ ...current, maxRounds: Number(event.target.value) || 1 }))
                }
              />
            </label>
          </div>

          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold">{t("placeholders")}</h3>
              <button
                type="button"
                className="inline-flex items-center gap-1 text-xs text-primary"
                onClick={addDiscussionModel}
              >
                <Plus className="size-3.5" />
                {t("addDiscussionModel")}
              </button>
            </div>
            {/* One placeholder per row: the summary model first, then the discussion models in order. */}
            <div className="space-y-1.5">
              {orderedPlaceholders.map((placeholder) => {
                const isSummary = placeholder.key === summaryKey;
                const discussionIndex =
                  discussionPlaceholders.findIndex((item) => item.key === placeholder.key) + 1;
                return (
                  <div
                    key={placeholder.key}
                    className="grid items-center gap-2 rounded-lg border border-border/50 p-2 md:grid-cols-[7rem_minmax(0,1fr)_auto]"
                  >
                    <span
                      className={cn(
                        "text-xs",
                        isSummary ? "font-medium text-primary" : "text-muted-foreground",
                      )}
                    >
                      {isSummary ? t("summaryModel") : t("discussionModel", { index: discussionIndex })}
                    </span>
                    <select
                      className="h-8 min-w-0 rounded-md border border-border/60 bg-background px-2 text-xs"
                      value={placeholder.defaultModel ?? ""}
                      onChange={(event) => updatePlaceholder(placeholder.key, { defaultModel: event.target.value })}
                    >
                      <option value="">{t("defaultModel")}</option>
                      {models.map((model) => (
                        <option key={model.platformModelName} value={model.platformModelName}>
                          {model.platformModelName}
                        </option>
                      ))}
                    </select>
                    {isSummary ? (
                      <span className="w-4" aria-hidden="true" />
                    ) : (
                      <button
                        type="button"
                        aria-label={t("delete")}
                        onClick={() => removeDiscussionModel(placeholder.key)}
                      >
                        <Trash2 className="size-3.5 text-muted-foreground hover:text-rose-500" />
                      </button>
                    )}
                  </div>
                );
              })}
            </div>
          </div>

          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold">{t("rounds")}</h3>
              <button
                type="button"
                className="inline-flex items-center gap-1 text-xs text-primary"
                onClick={addRound}
              >
                <Plus className="size-3.5" />
                {t("addRound")}
              </button>
            </div>

            {draft.config.rounds.map((round, roundIndex) => (
              <div
                key={`round-${roundIndex}`}
                className={cn(
                  "rounded-xl border border-border/60 bg-background/60 p-3",
                  dragRound === roundIndex && "border-primary/60",
                )}
                onDragOver={(event) => event.preventDefault()}
                onDrop={() => {
                  if (dragRound !== null) reorderRound(dragRound, roundIndex);
                  setDragRound(null);
                }}
              >
                <div className="mb-2 flex items-center gap-2 text-sm font-medium">
                  <span
                    className="inline-flex cursor-grab items-center"
                    draggable
                    onDragStart={() => setDragRound(roundIndex)}
                    onDragEnd={() => setDragRound(null)}
                  >
                    <GripVertical className="size-4 text-muted-foreground" />
                  </span>
                  {t("round", { round: roundIndex + 1 })}
                  <span className="inline-flex rounded-full bg-muted p-0.5 text-[10px] font-normal">
                    <button
                      type="button"
                      aria-pressed={round.parallel}
                      className={cn(
                        "rounded-full px-1.5 py-0.5",
                        round.parallel ? "bg-background text-primary" : "text-muted-foreground",
                      )}
                      onClick={() => updateRoundParallel(roundIndex, true)}
                    >
                      {t("parallel")}
                    </button>
                    <button
                      type="button"
                      aria-pressed={!round.parallel}
                      className={cn(
                        "rounded-full px-1.5 py-0.5",
                        !round.parallel ? "bg-background text-primary" : "text-muted-foreground",
                      )}
                      onClick={() => updateRoundParallel(roundIndex, false)}
                    >
                      {t("serial")}
                    </button>
                  </span>
                  <button
                    type="button"
                    className="ml-auto text-muted-foreground hover:text-rose-500"
                    aria-label={t("delete")}
                    onClick={() => removeRound(roundIndex)}
                  >
                    <Trash2 className="size-3.5" />
                  </button>
                </div>
                <p className="mb-2 text-[11px] leading-4 text-muted-foreground">
                  {round.parallel
                    ? roundIndex === 0
                      ? t("roundIndependent")
                      : t("roundSeesPrevious")
                    : t("roundSerial")}
                </p>

                <div className="mb-1 flex items-center justify-between gap-2">
                  <span className="text-[11px] text-muted-foreground">
                    {round.parallel ? t("parallelPrompt") : t("serialPrompt")}
                  </span>
                  <button
                    type="button"
                    className="text-[11px] text-primary hover:underline"
                    onClick={() => resetRoundPrompt(roundIndex)}
                  >
                    {t("resetPrompt")}
                  </button>
                </div>
                <textarea
                  className="min-h-20 w-full rounded-md border border-border/60 bg-background px-2 py-1 text-xs"
                  value={discussionRoundPrompt(round)}
                  placeholder={t("promptPlaceholder")}
                  onChange={(event) => updateRoundPrompt(roundIndex, event.target.value)}
                />

                <p className="mt-2 mb-1 text-[11px] text-muted-foreground">{t("roundModels")}</p>
                <div className="flex flex-wrap gap-1.5">
                  {(() => {
                    const allActive =
                      discussionPlaceholders.length > 0 &&
                      discussionPlaceholders.every((placeholder) =>
                        round.steps.some((step) => step.participant === placeholder.key),
                      );
                    return (
                      <>
                        <button
                          type="button"
                          aria-pressed={allActive}
                          className={cn(
                            "inline-flex items-center gap-1 rounded-full border px-2 py-1 text-[11px] transition-colors",
                            allActive
                              ? "border-primary/50 bg-primary/10 text-primary"
                              : "border-border/60 text-muted-foreground hover:text-foreground",
                          )}
                          onClick={() => toggleRoundAll(roundIndex)}
                        >
                          {t("allModels")}
                        </button>
                        {discussionPlaceholders.map((placeholder) => {
                          const active = round.steps.some((step) => step.participant === placeholder.key);
                          return (
                            <button
                              key={placeholder.key}
                              type="button"
                              aria-pressed={active}
                              disabled={allActive}
                              className={cn(
                                "inline-flex items-center gap-1 rounded-full border px-2 py-1 text-[11px] transition-colors",
                                allActive
                                  ? "border-primary/20 bg-primary/5 text-primary/50"
                                  : active
                                    ? "border-primary/50 bg-primary/10 text-primary"
                                    : "border-border/60 text-muted-foreground hover:text-foreground",
                              )}
                              onClick={() => toggleRoundParticipant(roundIndex, placeholder.key)}
                            >
                              {labels.get(placeholder.key) ?? placeholder.key}
                              <span className="font-mono text-[10px] opacity-70">
                                {placeholder.defaultModel || t("chooseAtChat")}
                              </span>
                            </button>
                          );
                        })}
                      </>
                    );
                  })()}
                </div>
              </div>
            ))}
          </div>

          <div className="rounded-xl border border-primary/30 bg-primary/5 p-3">
            <h3 className="mb-1 text-sm font-semibold">{t("summary")}</h3>
            <p className="mb-2 text-[11px] leading-4 text-muted-foreground">{t("summaryHint")}</p>
            <p className="mb-2 text-[11px] text-muted-foreground">
              {t("summaryUses", {
                model: draft.config.placeholders.find((item) => item.key === summaryKey)?.defaultModel ||
                  t("chooseAtChat"),
              })}
            </p>
            <textarea
              className="min-h-16 w-full rounded-md border border-border/60 bg-background px-2 py-1 text-xs"
              value={draft.config.summary.prompt}
              onChange={(event) =>
                updateConfig({ summary: { ...draft.config.summary, prompt: event.target.value } })
              }
            />
          </div>

          {formError || requestError ? <p className="text-sm text-rose-500">{formError || requestError}</p> : null}
          <div className="flex items-center justify-end gap-2">
            <button
              type="button"
              className="inline-flex h-9 items-center gap-1.5 rounded-lg bg-primary px-4 text-sm font-medium text-primary-foreground disabled:opacity-60"
              disabled={saving}
              onClick={() => void save()}
            >
              <Save className="size-4" />
              {saving ? t("saving") : t("save")}
            </button>
          </div>
        </section>

      </div>

      {/* Narrow screens: the workflow opens at the bottom of the page, after the editor. */}
      {workflowOpen ? (
        <div className="xl:hidden">
          <DiscussionsWorkflowPreview config={draft.config} onClose={() => setWorkflowOpen(false)} />
        </div>
      ) : null}
      {/* Wide screens: the workflow is docked to the right edge, full height, without squeezing the editor. */}
      {workflowOpen ? (
        <div className="fixed inset-y-0 right-0 z-30 hidden w-[23rem] overflow-y-auto border-l border-border/60 bg-background px-4 py-6 shadow-[-16px_0_32px_-28px_rgba(0,0,0,0.35)] xl:block 2xl:w-[27rem] 2xl:px-6">
          <DiscussionsWorkflowPreview
            config={draft.config}
            onClose={() => setWorkflowOpen(false)}
            className="rounded-none border-0 bg-transparent p-0"
          />
        </div>
      ) : null}
    </div>
  );
}
