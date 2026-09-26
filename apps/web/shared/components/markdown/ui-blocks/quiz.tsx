"use client";

import { ArrowLeft, ArrowRight, Check, RotateCcw, X } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

import { HeightTransition } from "@/components/ui/height-transition";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { CollapsibleMotionContent } from "@/shared/components/collapsible-motion-content";
import { SlideSwitch } from "@/shared/components/slide-switch";
import type { UIBlockDefinition, UIBlockRenderProps, UIBlockSkeletonProps } from "./block";
import { s } from "./schema";
import { UIBlockFrame } from "./ui-block-frame";

// Options are plain strings or objects that can route to another question.
type QuizOption = string | { text: string; next?: string; explanation?: string };
type QuizQuestion = {
  id?: string;
  question: string;
  options: QuizOption[];
  // Omit for survey-style questions that only route; they do not count toward the score.
  answer?: number;
  explanation?: string;
  // Question to continue with; falls back to the next one in order. "end" stops.
  next?: string;
};

export type QuizProps = {
  title?: string;
  questions: QuizQuestion[];
};

const END = "end";

function optionText(option: QuizOption): string {
  return typeof option === "string" ? option : option.text;
}

function optionNext(option: QuizOption): string | undefined {
  return typeof option === "string" ? undefined : option.next;
}

function optionExplanation(option: QuizOption): string | undefined {
  return typeof option === "string" ? undefined : option.explanation;
}

// Branching mode kicks in as soon as anything routes; otherwise every question
// is shown at once, which reads better for a plain test.
function isBranching(questions: QuizQuestion[]): boolean {
  return questions.some((question) => question.next !== undefined || question.options.some((option) => optionNext(option) !== undefined));
}

function questionID(question: QuizQuestion, index: number): string {
  return question.id ?? `q${index + 1}`;
}

function Quiz({ id, props }: UIBlockRenderProps<QuizProps>) {
  const t = useTranslations("chat.markdown.uiBlock");
  const questions = props.questions;
  const branching = React.useMemo(() => isBranching(questions), [questions]);
  const indexByID = React.useMemo(() => new Map(questions.map((question, index) => [questionID(question, index), index])), [questions]);
  // Visited questions in order (branching) — the last entry is the current one.
  const [path, setPath] = React.useState<number[]>([0]);
  const [picked, setPicked] = React.useState<Record<number, number>>({});
  const [direction, setDirection] = React.useState<1 | -1>(1);
  const [ended, setEnded] = React.useState(false);

  const scored = questions.filter((question) => question.answer !== undefined);
  const visibleScored = branching ? scored.filter((question) => path.includes(questions.indexOf(question))) : scored;
  const correct = visibleScored.reduce((count, question) => count + (picked[questions.indexOf(question)] === question.answer ? 1 : 0), 0);
  const answered = Object.keys(picked).length;
  const finished = branching ? ended : answered === questions.length;

  const reset = () => {
    setPicked({});
    setPath([0]);
    setEnded(false);
    setDirection(-1);
  };

  const resolveNext = (question: QuizQuestion, qIndex: number, option: QuizOption): number | typeof END => {
    const target = optionNext(option) ?? question.next;
    if (target === END) {
      return END;
    }
    if (target !== undefined) {
      const index = indexByID.get(target);
      if (index !== undefined) {
        return index;
      }
    }
    return qIndex + 1 < questions.length ? qIndex + 1 : END;
  };

  const choose = (qIndex: number, oIndex: number) => {
    setPicked((current) => ({ ...current, [qIndex]: oIndex }));
  };
  const advance = () => {
    const qIndex = path[path.length - 1] as number;
    const question = questions[qIndex] as QuizQuestion;
    const option = question.options[picked[qIndex] ?? 0] as QuizOption;
    const next = resolveNext(question, qIndex, option);
    setDirection(1);
    if (next === END) {
      setEnded(true);
    } else {
      // Stepping into a question again (loops) clears its previous pick.
      setPicked((current) => {
        const { [next]: _dropped, ...rest } = current;
        return rest;
      });
      setPath((current) => [...current, next]);
    }
  };
  const back = () => {
    setDirection(-1);
    if (ended) {
      setEnded(false);
      return;
    }
    setPath((current) => {
      if (current.length <= 1) {
        return current;
      }
      const dropped = current[current.length - 1] as number;
      setPicked((picks) => {
        const { [dropped]: _removed, ...rest } = picks;
        return rest;
      });
      return current.slice(0, -1);
    });
  };

  const renderQuestion = (question: QuizQuestion, qIndex: number, ordinal: number) => {
    const chosen = picked[qIndex];
    const revealed = chosen !== undefined;
    const graded = question.answer !== undefined;
    const answerIndex = graded ? Math.min(Math.max(0, Math.trunc(question.answer as number)), question.options.length - 1) : -1;
    const chosenExplanation = revealed ? optionExplanation(question.options[chosen] as QuizOption) : undefined;
    const explanation = chosenExplanation ?? question.explanation;
    return (
      <div>
        <p className="text-sm font-medium leading-5">
          <span className="mr-1.5 text-muted-foreground tabular-nums">{ordinal}.</span>
          {question.question}
        </p>
        <div className="mt-2 grid gap-1.5">
          {question.options.map((option, oIndex) => {
            const isAnswer = graded && oIndex === answerIndex;
            const isChosen = oIndex === chosen;
            return (
              <button
                key={`${id}-q-${qIndex}-o-${oIndex}`}
                type="button"
                disabled={revealed && !branching}
                aria-pressed={isChosen}
                onClick={() => choose(qIndex, oIndex)}
                className={cn(
                  "flex items-center gap-2 rounded-md border-[0.5px] px-3 py-2 text-left text-sm transition-colors disabled:cursor-default",
                  !revealed && "border-border bg-background hover:border-primary/40 hover:bg-accent",
                  revealed && graded && isAnswer && "border-green-500/50 bg-green-500/10",
                  revealed && graded && isChosen && !isAnswer && "border-red-500/50 bg-red-500/10",
                  revealed && !graded && isChosen && "border-primary/50 bg-primary/5",
                  revealed && !isAnswer && !isChosen && "border-border text-muted-foreground opacity-70",
                )}
              >
                <span className="flex size-4 shrink-0 items-center justify-center">
                  {revealed && graded && isAnswer ? <Check className="size-4 text-green-600 dark:text-green-400" strokeWidth={2.2} /> : null}
                  {revealed && graded && isChosen && !isAnswer ? <X className="size-4 text-red-600 dark:text-red-400" strokeWidth={2.2} /> : null}
                  {revealed && !graded && isChosen ? <span className="size-3 rounded-full border-4 border-primary" /> : null}
                  {!revealed || (!isAnswer && !isChosen) ? <span className="size-3 rounded-full border border-border" /> : null}
                </span>
                <span>{optionText(option)}</span>
              </button>
            );
          })}
        </div>
        {explanation ? (
          <CollapsibleMotionContent open={revealed}>
            <p className="mt-2 whitespace-pre-wrap rounded-md bg-muted/50 px-3 py-2 text-xs leading-5 text-muted-foreground">{explanation}</p>
          </CollapsibleMotionContent>
        ) : null}
      </div>
    );
  };

  const scoreLabel = visibleScored.length > 0 ? t("quizScore", { correct, total: branching ? visibleScored.length : scored.length }) : null;

  return (
    <div className="px-4 pb-4 pt-5">
      <div className="flex items-baseline justify-between gap-3">
        {props.title ? <h3 className="text-lg font-semibold leading-7">{props.title}</h3> : <span />}
        <div className="flex shrink-0 items-center gap-3 text-xs tabular-nums text-muted-foreground">
          {branching ? <span>{t("quizStep", { step: path.length })}</span> : null}
          {scoreLabel ? <span className={cn(finished && "font-medium text-foreground")}>{scoreLabel}</span> : null}
          {answered > 0 || path.length > 1 ? (
            <button type="button" onClick={reset} className="inline-flex items-center gap-1 hover:text-foreground">
              <RotateCcw className="size-3" strokeWidth={1.8} />
              {t("reset")}
            </button>
          ) : null}
        </div>
      </div>

      {branching ? (
        <HeightTransition className="mt-3">
          <SlideSwitch itemKey={ended ? END : `${path.length}:${path[path.length - 1]}`} direction={direction}>
            {ended ? (
              <div className="rounded-lg border-[0.5px] border-primary/40 bg-primary/5 px-4 py-3">
                <p className="text-sm font-medium">{t("quizDone")}</p>
                {scoreLabel ? <p className="mt-1 text-xs text-muted-foreground">{scoreLabel}</p> : null}
              </div>
            ) : (
              renderQuestion(questions[path[path.length - 1] as number] as QuizQuestion, path[path.length - 1] as number, path.length)
            )}
            <div className="mt-3 flex items-center justify-between text-xs">
              <button type="button" onClick={back} disabled={path.length <= 1 && !ended} className="inline-flex items-center gap-1 text-muted-foreground hover:text-foreground disabled:invisible">
                <ArrowLeft className="size-3" strokeWidth={1.8} />
                {t("back")}
              </button>
              {!ended ? (
                <button
                  type="button"
                  onClick={advance}
                  disabled={picked[path[path.length - 1] as number] === undefined}
                  className="inline-flex items-center gap-1 rounded-md bg-primary px-3 py-1.5 font-medium text-primary-foreground transition-opacity disabled:opacity-40"
                >
                  {t("quizNext")}
                  <ArrowRight className="size-3" strokeWidth={2} />
                </button>
              ) : null}
            </div>
          </SlideSwitch>
        </HeightTransition>
      ) : (
        <ol className="mt-3 space-y-4">
          {questions.map((question, qIndex) => (
            <li key={`${id}-q-${qIndex}`}>{renderQuestion(question, qIndex, qIndex + 1)}</li>
          ))}
        </ol>
      )}
    </div>
  );
}

// One question block (title + four 36px options) per item streamed so far.
function QuizSkeleton({ items = 0 }: UIBlockSkeletonProps) {
  return (
    <UIBlockFrame className="px-4 pb-4 pt-5">
      <Skeleton className="h-7 w-1/3 rounded-md" />
      <div className="mt-3 space-y-4">
        {Array.from({ length: Math.max(1, items) }).map((_, question) => (
          <div key={`quiz-q-${question}`}>
            <Skeleton className="h-5 w-2/3 rounded-md" />
            <div className="mt-2 space-y-1.5">
              {Array.from({ length: 4 }).map((_, option) => (
                <Skeleton key={`quiz-${question}-${option}`} className="h-9 rounded-md" />
              ))}
            </div>
          </div>
        ))}
      </div>
    </UIBlockFrame>
  );
}

const OPTION_SCHEMA = s.object({ text: s.string(), next: s.string(), explanation: s.string() }, ["text"]);

export const quizDefinition: UIBlockDefinition<QuizProps> = {
  name: "quiz",
  version: 1,
  schema: s.object(
    {
      title: s.string(),
      questions: s.array(
        s.object(
          {
            id: s.string(),
            question: s.string(),
            options: s.array(s.oneOf([s.string(), OPTION_SCHEMA]), { minItems: 2 }),
            answer: s.number(),
            explanation: s.string(),
            next: s.string(),
          },
          ["question", "options"],
        ),
        { minItems: 1 },
      ),
    },
    ["questions"],
  ),
  Component: Quiz,
  Skeleton: QuizSkeleton,
};
