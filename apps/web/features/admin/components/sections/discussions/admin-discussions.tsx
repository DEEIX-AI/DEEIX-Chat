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
  DiscussionStep,
  DiscussionTemplateConfig,
  DiscussionTemplateDTO,
} from "@/shared/api/discussion-types";
import { orderDiscussionPlaceholders } from "@/shared/lib/discussion";

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
        { key: "p1", label: "讨论模型 1", defaultModel: "", required: true },
        { key: "p2", label: "讨论模型 2", defaultModel: "", required: true },
      ],
      rounds: [
        {
          index: 1,
          parallel: true,
          steps: [
            { id: newStepID(), participant: "p1", prompt: "请独立回答用户问题，给出清晰、具体、可执行的答案。" },
            { id: newStepID(), participant: "p2", prompt: "请独立回答用户问题，给出清晰、具体、可执行的答案。" },
          ],
        },
        {
          index: 2,
          parallel: true,
          steps: [
            { id: newStepID(), participant: "p1", prompt: "阅读上一轮讨论，补充遗漏、纠正错误、挑战薄弱推理。" },
            { id: newStepID(), participant: "p2", prompt: "阅读上一轮讨论，补充遗漏、纠正错误、挑战薄弱推理。" },
          ],
        },
      ],
      summary: { participant: "judge", prompt: "请综合成功发言，给出面向用户的最终答案；失败发言不是证据。" },
    },
  };
}

function toDraft(template: DiscussionTemplateDTO): Draft {
  return {
    publicID: template.publicID,
    name: template.name,
    description: template.description,
    icon: template.icon,
    status: template.status,
    maxRounds: template.maxRounds,
    sortOrder: template.sortOrder,
    config: structuredClone(template.config),
  };
}

function normalizeRounds(rounds: DiscussionRound[]): DiscussionRound[] {
  return rounds.map((round, index) => ({
    ...round,
    index: index + 1,
    parallel: true,
  }));
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
  const [dragStep, setDragStep] = React.useState<{ round: number; step: number } | null>(null);

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
            { key, label: t("discussionModel", { index }), defaultModel: "", required: true },
          ],
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

  const updateStep = (roundIndex: number, stepIndex: number, patch: Partial<DiscussionStep>) => {
    setDraft((current) => ({
      ...current,
      config: {
        ...current.config,
        rounds: current.config.rounds.map((round, index) =>
          index === roundIndex
            ? {
                ...round,
                steps: round.steps.map((step, itemIndex) =>
                  itemIndex === stepIndex ? { ...step, ...patch } : step,
                ),
              }
            : round,
        ),
      },
    }));
  };

  const addRound = () => {
    setDraft((current) => ({
      ...current,
      config: {
        ...current.config,
        rounds: [
          ...current.config.rounds,
          {
            index: current.config.rounds.length + 1,
            parallel: true,
            steps: current.config.placeholders
              .filter((item) => item.key !== current.config.summary.participant)
              .map((item) => ({
                id: newStepID(),
                participant: item.key,
                prompt: "阅读已有讨论，补充遗漏、纠正错误、挑战薄弱推理。",
              })),
          },
        ],
      },
      maxRounds: Math.max(current.maxRounds, current.config.rounds.length + 1),
    }));
  };

  const addStep = (roundIndex: number) => {
    const participant =
      draft.config.placeholders.find((item) => item.key !== summaryKey)?.key ?? summaryKey;
    setDraft((current) => ({
      ...current,
      config: {
        ...current.config,
        rounds: current.config.rounds.map((round, index) =>
          index === roundIndex
            ? {
                ...round,
                steps: [...round.steps, { id: newStepID(), participant, prompt: "" }],
              }
            : round,
        ),
      },
    }));
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

  const removeStep = (roundIndex: number, stepIndex: number) => {
    setDraft((current) => ({
      ...current,
      config: {
        ...current.config,
        rounds: current.config.rounds.map((round, index) =>
          index === roundIndex
            ? { ...round, steps: round.steps.filter((_, itemIndex) => itemIndex !== stepIndex) }
            : round,
        ),
      },
    }));
  };

  const reorderRound = (from: number, to: number) => {
    if (from === to) return;
    setDraft((current) => {
      const rounds = [...current.config.rounds];
      const [item] = rounds.splice(from, 1);
      rounds.splice(to, 0, item);
      return { ...current, config: { ...current.config, rounds: normalizeRounds(rounds) } };
    });
  };

  const reorderStep = (roundIndex: number, from: number, to: number) => {
    if (from === to) return;
    setDraft((current) => ({
      ...current,
      config: {
        ...current.config,
        rounds: current.config.rounds.map((round, index) => {
          if (index !== roundIndex) return round;
          const steps = [...round.steps];
          const [item] = steps.splice(from, 1);
          steps.splice(to, 0, item);
          return { ...round, steps };
        }),
      },
    }));
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
    const payload = {
      name: draft.name.trim(),
      description: draft.description.trim(),
      icon: draft.icon.trim(),
      status: draft.status,
      maxRounds: Math.max(draft.maxRounds, draft.config.rounds.length),
      sortOrder: draft.sortOrder,
      config: { ...draft.config, rounds: normalizeRounds(draft.config.rounds) },
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
                    className="grid items-center gap-2 rounded-lg border border-border/50 p-2 md:grid-cols-[7rem_minmax(0,1fr)_minmax(0,12rem)_auto]"
                  >
                    <span
                      className={cn(
                        "text-xs",
                        isSummary ? "font-medium text-primary" : "text-muted-foreground",
                      )}
                    >
                      {isSummary ? t("summaryModel") : t("discussionModel", { index: discussionIndex })}
                    </span>
                    <input
                      className="h-8 min-w-0 rounded-md border border-border/60 bg-background px-2 text-xs"
                      value={placeholder.label}
                      placeholder={t("placeholderLabel")}
                      onChange={(event) => updatePlaceholder(placeholder.key, { label: event.target.value })}
                      aria-label={t("placeholderLabel")}
                    />
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
                  <span className="rounded-full bg-muted px-1.5 text-[10px] font-normal text-muted-foreground">
                    {t("parallel")}
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
                  {roundIndex === 0 ? t("roundIndependent") : t("roundSeesPrevious")}
                </p>

                <div className="space-y-2">
                  {round.steps.map((step, stepIndex) => (
                    <div
                      key={step.id}
                      className={cn(
                        "grid gap-2 rounded-lg border border-border/40 p-2 md:grid-cols-[auto_minmax(120px,200px)_minmax(0,1fr)_auto]",
                        dragStep?.round === roundIndex && dragStep.step === stepIndex && "border-primary/60",
                      )}
                      onDragOver={(event) => event.preventDefault()}
                      onDrop={() => {
                        if (dragStep?.round === roundIndex) {
                          reorderStep(roundIndex, dragStep.step, stepIndex);
                        }
                        setDragStep(null);
                      }}
                    >
                      <span
                        className="mt-1.5 inline-flex cursor-grab self-start"
                        draggable
                        onDragStart={() => setDragStep({ round: roundIndex, step: stepIndex })}
                        onDragEnd={() => setDragStep(null)}
                      >
                        <GripVertical className="size-4 text-muted-foreground" />
                      </span>
                      <select
                        className="h-8 min-w-0 rounded-md border border-border/60 bg-background px-2 text-xs"
                        value={step.participant}
                        onChange={(event) => updateStep(roundIndex, stepIndex, { participant: event.target.value })}
                      >
                        {orderedPlaceholders.map((placeholder) => (
                          <option key={placeholder.key} value={placeholder.key}>
                            {placeholder.label || placeholder.key}
                            {" · "}
                            {placeholder.defaultModel || t("chooseAtChat")}
                          </option>
                        ))}
                      </select>
                      <textarea
                        className="min-h-16 rounded-md border border-border/60 bg-background px-2 py-1 text-xs"
                        value={step.prompt}
                        placeholder={t("promptPlaceholder")}
                        onChange={(event) => updateStep(roundIndex, stepIndex, { prompt: event.target.value })}
                      />
                      <button
                        type="button"
                        className="self-start text-muted-foreground hover:text-rose-500"
                        aria-label={t("delete")}
                        onClick={() => removeStep(roundIndex, stepIndex)}
                      >
                        <Trash2 className="size-3.5" />
                      </button>
                    </div>
                  ))}
                </div>

                <button
                  type="button"
                  className="mt-2 inline-flex items-center gap-1 text-xs text-primary"
                  onClick={() => addStep(roundIndex)}
                >
                  <Plus className="size-3.5" />
                  {t("addStep")}
                </button>
              </div>
            ))}
          </div>

          <div className="rounded-xl border border-primary/30 bg-primary/5 p-3">
            <h3 className="mb-1 text-sm font-semibold">{t("summary")}</h3>
            <p className="mb-2 text-[11px] leading-4 text-muted-foreground">{t("summaryHint")}</p>
            <div className="grid gap-2 md:grid-cols-[minmax(120px,180px)_minmax(0,1fr)]">
              <select
                className="h-8 min-w-0 rounded-md border border-border/60 bg-background px-2 text-xs"
                value={draft.config.summary.participant}
                onChange={(event) =>
                  updateConfig({ summary: { ...draft.config.summary, participant: event.target.value } })
                }
              >
                {orderedPlaceholders.map((placeholder) => (
                  <option key={placeholder.key} value={placeholder.key}>
                    {placeholder.label || placeholder.key}
                    {" · "}
                    {placeholder.defaultModel || t("chooseAtChat")}
                  </option>
                ))}
              </select>
              <textarea
                className="min-h-16 rounded-md border border-border/60 bg-background px-2 py-1 text-xs"
                value={draft.config.summary.prompt}
                onChange={(event) =>
                  updateConfig({ summary: { ...draft.config.summary, prompt: event.target.value } })
                }
              />
            </div>
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
      {/* Wide screens: a floating panel that never shrinks the editor. */}
      {workflowOpen ? (
        <div className="fixed top-24 right-4 z-40 hidden w-72 max-h-[calc(100vh-7rem)] overflow-y-auto rounded-xl shadow-xl 2xl:w-80 xl:block">
          <DiscussionsWorkflowPreview config={draft.config} onClose={() => setWorkflowOpen(false)} />
        </div>
      ) : null}
    </div>
  );
}
