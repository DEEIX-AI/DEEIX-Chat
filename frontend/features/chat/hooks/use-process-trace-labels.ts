"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

export type ProcessTraceLabels = {
  fileBadges: {
    directRead: string;
    budget: string;
    retrieval: string;
    fullContext: string;
    skipped: string;
    file: string;
    descriptions: {
      directRead: string;
      budget: string;
      retrieval: string;
      fullContext: string;
      skipped: string;
      file: string;
    };
  };
  rag: {
    sourceFallback: (fileID: string) => string;
    chunksShort: (count: number, scorePercent: number) => string;
    chunksTotal: (count: number) => string;
    retrievalSources: string;
    matchedContents: (count: number) => string;
    matchSummary: (count: number, sharePercent: number, scorePercent: number) => string;
    summary: (count: number) => string;
    completed: (fileCount: number, chunkCount: number) => string;
    incompleteWithFullText: string;
    incompleteNoFullText: string;
    emptyWithFullText: string;
    emptyNoFullText: string;
    lowScoreWithFullText: string;
    lowScoreNoFullText: string;
    skippedFallback: string;
  };
  fileContext: {
    includedSummary: (count: number) => string;
    includedDetail: (count: number) => string;
    skipped: (count: number) => string;
    ready: (counts: string) => string;
    separator: string;
  };
  tool: {
    status: {
      calling: string;
      completed: string;
      reused: string;
      failed: string;
    };
    names: {
      webSearch: string;
      codeInterpreter: string;
      imageGeneration: string;
      shell: string;
      generic: string;
      thinking: string;
    };
    detail: {
      request: string;
      response: string;
      error: string;
      expand: string;
      collapse: string;
      sourceFallback: (index: number) => string;
      generatedImageAlt: (index: number) => string;
      query: string;
      action: string;
      source: string;
      code: string;
      output: string;
      resultFile: string;
      prompt: string;
      command: string;
      latencySeparator: string;
    };
    nativeStatus: {
      webSearchActive: string;
      webSearchDone: string;
      webSearchFailed: string;
      codeActive: string;
      codeDone: string;
      codeFailed: string;
      imageActive: string;
      imageDone: string;
      imageFailed: string;
      shellActive: string;
      shellDone: string;
      shellFailed: string;
      genericActive: string;
      genericDone: string;
      genericFailed: string;
    };
    chain: {
      titleActive: string;
      titleDone: string;
      summaryCount: (count: number) => string;
      summaryFallback: string;
    };
  };
  think: {
    titleActive: string;
    titleDone: string;
    subtitleActive: string;
    subtitleDone: string;
  };
  promptTrace: {
    modes: {
      stateful: string;
      fullRetry: string;
      full: string;
    };
    reasons: {
      missingStoredFingerprint: string;
      missingCurrentFingerprint: string;
      fingerprintMismatch: string;
      previousRejected: string;
    };
    sentSummary: (mode: string, sent: number, full: number, tokens: number) => string;
    savedHistory: (messages: number, tokens: number) => string;
    cacheableBlocks: (count: number) => string;
    historicalEvidence: (count: number) => string;
    dynamicSources: (count: number) => string;
    listSeparator: string;
    extraSummary: (items: string) => string;
    reasonLine: (reason: string) => string;
    preparedSummary: (tokens: number) => string;
    statefulSummary: (messages: number) => string;
  };
  stages: {
    contextPlanning: string;
    contentRetrieval: string;
    fileContext: string;
    contextCompaction: string;
    skillContext: string;
    requestResult: string;
    upstreamRequestTriggered: string;
  };
  process: {
    titleActive: string;
    titleDone: string;
  };
  compaction: {
    summary: (fromTurn: number, toTurn: number) => string;
    detail: string;
    range: (fromTurn: number, toTurn: number) => string;
    tokens: (sourceTokens: number, summaryTokens: number) => string;
  };
};

export function useProcessTraceLabels(): ProcessTraceLabels {
  const t = useTranslations("chat.processTrace");

  return React.useMemo(
    () => ({
      fileBadges: {
        directRead: t("fileBadges.directRead"),
        budget: t("fileBadges.budget"),
        retrieval: t("fileBadges.retrieval"),
        fullContext: t("fileBadges.fullContext"),
        skipped: t("fileBadges.skipped"),
        file: t("fileBadges.file"),
        descriptions: {
          directRead: t("fileBadges.descriptions.directRead"),
          budget: t("fileBadges.descriptions.budget"),
          retrieval: t("fileBadges.descriptions.retrieval"),
          fullContext: t("fileBadges.descriptions.fullContext"),
          skipped: t("fileBadges.descriptions.skipped"),
          file: t("fileBadges.descriptions.file"),
        },
      },
      rag: {
        sourceFallback: (fileID: string) => t("rag.sourceFallback", { fileID }),
        chunksShort: (count: number, scorePercent: number) => t("rag.chunksShort", { count, scorePercent }),
        chunksTotal: (count: number) => t("rag.chunksTotal", { count }),
        retrievalSources: t("rag.retrievalSources"),
        matchedContents: (count: number) => t("rag.matchedContents", { count }),
        matchSummary: (count: number, sharePercent: number, scorePercent: number) =>
          t("rag.matchSummary", { count, sharePercent, scorePercent }),
        summary: (count: number) => t("rag.summary", { count }),
        completed: (fileCount: number, chunkCount: number) => t("rag.completed", { fileCount, chunkCount }),
        incompleteWithFullText: t("rag.incompleteWithFullText"),
        incompleteNoFullText: t("rag.incompleteNoFullText"),
        emptyWithFullText: t("rag.emptyWithFullText"),
        emptyNoFullText: t("rag.emptyNoFullText"),
        lowScoreWithFullText: t("rag.lowScoreWithFullText"),
        lowScoreNoFullText: t("rag.lowScoreNoFullText"),
        skippedFallback: t("rag.skippedFallback"),
      },
      fileContext: {
        includedSummary: (count: number) => t("fileContext.includedSummary", { count }),
        includedDetail: (count: number) => t("fileContext.includedDetail", { count }),
        skipped: (count: number) => t("fileContext.skipped", { count }),
        ready: (counts: string) => t("fileContext.ready", { counts }),
        separator: t("fileContext.separator"),
      },
      tool: {
        status: {
          calling: t("tool.status.calling"),
          completed: t("tool.status.completed"),
          reused: t("tool.status.reused"),
          failed: t("tool.status.failed"),
        },
        names: {
          webSearch: t("tool.names.webSearch"),
          codeInterpreter: t("tool.names.codeInterpreter"),
          imageGeneration: t("tool.names.imageGeneration"),
          shell: t("tool.names.shell"),
          generic: t("tool.names.generic"),
          thinking: t("tool.names.thinking"),
        },
        detail: {
          request: t("tool.detail.request"),
          response: t("tool.detail.response"),
          error: t("tool.detail.error"),
          expand: t("tool.detail.expand"),
          collapse: t("tool.detail.collapse"),
          sourceFallback: (index: number) => t("tool.detail.sourceFallback", { index }),
          generatedImageAlt: (index: number) => t("tool.detail.generatedImageAlt", { index }),
          query: t("tool.detail.query"),
          action: t("tool.detail.action"),
          source: t("tool.detail.source"),
          code: t("tool.detail.code"),
          output: t("tool.detail.output"),
          resultFile: t("tool.detail.resultFile"),
          prompt: t("tool.detail.prompt"),
          command: t("tool.detail.command"),
          latencySeparator: t("tool.detail.latencySeparator"),
        },
        nativeStatus: {
          webSearchActive: t("tool.nativeStatus.webSearchActive"),
          webSearchDone: t("tool.nativeStatus.webSearchDone"),
          webSearchFailed: t("tool.nativeStatus.webSearchFailed"),
          codeActive: t("tool.nativeStatus.codeActive"),
          codeDone: t("tool.nativeStatus.codeDone"),
          codeFailed: t("tool.nativeStatus.codeFailed"),
          imageActive: t("tool.nativeStatus.imageActive"),
          imageDone: t("tool.nativeStatus.imageDone"),
          imageFailed: t("tool.nativeStatus.imageFailed"),
          shellActive: t("tool.nativeStatus.shellActive"),
          shellDone: t("tool.nativeStatus.shellDone"),
          shellFailed: t("tool.nativeStatus.shellFailed"),
          genericActive: t("tool.nativeStatus.genericActive"),
          genericDone: t("tool.nativeStatus.genericDone"),
          genericFailed: t("tool.nativeStatus.genericFailed"),
        },
        chain: {
          titleActive: t("tool.chain.titleActive"),
          titleDone: t("tool.chain.titleDone"),
          summaryCount: (count: number) => t("tool.chain.summaryCount", { count }),
          summaryFallback: t("tool.chain.summaryFallback"),
        },
      },
      think: {
        titleActive: t("think.titleActive"),
        titleDone: t("think.titleDone"),
        subtitleActive: t("think.subtitleActive"),
        subtitleDone: t("think.subtitleDone"),
      },
      promptTrace: {
        modes: {
          stateful: t("promptTrace.modes.stateful"),
          fullRetry: t("promptTrace.modes.fullRetry"),
          full: t("promptTrace.modes.full"),
        },
        reasons: {
          missingStoredFingerprint: t("promptTrace.reasons.missingStoredFingerprint"),
          missingCurrentFingerprint: t("promptTrace.reasons.missingCurrentFingerprint"),
          fingerprintMismatch: t("promptTrace.reasons.fingerprintMismatch"),
          previousRejected: t("promptTrace.reasons.previousRejected"),
        },
        sentSummary: (mode: string, sent: number, full: number, tokens: number) =>
          t("promptTrace.sentSummary", { mode, sent, full, tokens }),
        savedHistory: (messages: number, tokens: number) => t("promptTrace.savedHistory", { messages, tokens }),
        cacheableBlocks: (count: number) => t("promptTrace.cacheableBlocks", { count }),
        historicalEvidence: (count: number) => t("promptTrace.historicalEvidence", { count }),
        dynamicSources: (count: number) => t("promptTrace.dynamicSources", { count }),
        listSeparator: t("promptTrace.listSeparator"),
        extraSummary: (items: string) => t("promptTrace.extraSummary", { items }),
        reasonLine: (reason: string) => t("promptTrace.reasonLine", { reason }),
        preparedSummary: (tokens: number) => t("promptTrace.preparedSummary", { tokens }),
        statefulSummary: (messages: number) => t("promptTrace.statefulSummary", { messages }),
      },
      stages: {
        contextPlanning: t("stages.contextPlanning"),
        contentRetrieval: t("stages.contentRetrieval"),
        fileContext: t("stages.fileContext"),
        contextCompaction: t("stages.contextCompaction"),
        skillContext: t("stages.skillContext"),
        requestResult: t("stages.requestResult"),
        upstreamRequestTriggered: t("stages.upstreamRequestTriggered"),
      },
      process: {
        titleActive: t("process.titleActive"),
        titleDone: t("process.titleDone"),
      },
      compaction: {
        summary: (fromTurn: number, toTurn: number) => t("compaction.summary", { fromTurn, toTurn }),
        detail: t("compaction.detail"),
        range: (fromTurn: number, toTurn: number) => t("compaction.range", { fromTurn, toTurn }),
        tokens: (sourceTokens: number, summaryTokens: number) => t("compaction.tokens", { sourceTokens, summaryTokens }),
      },
    }),
    [t],
  );
}
