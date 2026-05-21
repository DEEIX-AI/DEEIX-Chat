type FileProcessingView = {
  fileCategory?: string;
  processingStatus?: string;
  processingReady?: boolean;
  processingErrorCode?: string;
  processingErrorMessage?: string;
  extractStatus?: string;
  embedStatus?: string;
  embedError?: string;
  ragReady?: boolean;
  ocrUsed?: boolean;
};

type FileProcessingBadge = {
  label: string;
  tone: "neutral" | "info" | "success" | "warning" | "danger";
  detail?: string;
};

type FileProcessingTranslator = (key: string, values?: Record<string, string | number>) => string;

function translateFileProcessing(
  translate: FileProcessingTranslator | undefined,
  key: string,
  fallback: string,
  values?: Record<string, string | number>,
): string {
  if (!translate) {
    return fallback;
  }
  try {
    return translate(key, values);
  } catch {
    return fallback;
  }
}

export function resolveFileProcessingBadge(
  file: FileProcessingView,
  translate?: FileProcessingTranslator,
): FileProcessingBadge {
  if (file.processingStatus === "failed" || file.processingErrorCode) {
    const code = file.processingErrorCode?.trim();
    return {
      label: translateFileProcessing(translate, "processingFailed", "Processing failed"),
      tone: "danger",
      detail: file.processingErrorMessage?.trim() || (code
        ? translateFileProcessing(translate, "errorCode", `Error code: ${code}`, { code })
        : translateFileProcessing(translate, "processingFailedDetail", "File processing failed. Upload again or adjust the policy and retry.")),
    };
  }

  if (file.embedStatus === "failed") {
    return {
      label: translateFileProcessing(translate, "indexFailed", "Index failed"),
      tone: "warning",
      detail: file.embedError?.trim() || translateFileProcessing(translate, "indexFailedDetail", "The file is available for full-context chat, but smart retrieval indexing failed."),
    };
  }

  switch (file.processingStatus) {
    case "uploaded":
      return {
        label: translateFileProcessing(translate, "waiting", "Waiting"),
        tone: "neutral",
        detail: translateFileProcessing(translate, "waitingDetail", "File uploaded and waiting for the processing queue."),
      };
    case "queued":
      return {
        label: translateFileProcessing(translate, "queued", "Queued"),
        tone: "info",
        detail: translateFileProcessing(translate, "queuedDetail", "File is queued and ready for text extraction."),
      };
    case "extracting":
      return {
        label: file.ocrUsed
          ? translateFileProcessing(translate, "ocr", "OCR running")
          : translateFileProcessing(translate, "extracting", "Extracting"),
        tone: "info",
        detail: file.ocrUsed
          ? translateFileProcessing(translate, "ocrDetail", "OCR is recognizing and extracting text.")
          : translateFileProcessing(translate, "extractingDetail", "Extracting text content from the file."),
      };
    case "embedding":
      return {
        label: translateFileProcessing(translate, "embedding", "Vectorizing"),
        tone: "info",
        detail: translateFileProcessing(translate, "embeddingDetail", "Text extraction is complete. Semantic vector index is being generated."),
      };
    case "ready":
      if (file.embedStatus === "ready" || file.ragReady) {
        return {
          label: translateFileProcessing(translate, "ready", "Ready"),
          tone: "success",
          detail: translateFileProcessing(translate, "readyRagDetail", "File is ready and supports full-context injection and smart retrieval (RAG)."),
        };
      }
      return {
        label: translateFileProcessing(translate, "ready", "Ready"),
        tone: "success",
        detail: translateFileProcessing(translate, "readyFullContextDetail", "File processing is complete and can be used in full-context mode."),
      };
  }

  if (file.fileCategory === "image") {
    return {
      label: translateFileProcessing(translate, "imageReady", "Image ready"),
      tone: "success",
      detail: translateFileProcessing(translate, "imageReadyDetail", "The image can be used in this conversation."),
    };
  }

  if (file.processingReady) {
    return {
      label: translateFileProcessing(translate, "ready", "Ready"),
      tone: "success",
      detail: translateFileProcessing(translate, "genericReadyDetail", "File processing is complete and can be used in chats."),
    };
  }

  return {
    label: translateFileProcessing(translate, "preparing", "Processing"),
    tone: "info",
    detail: translateFileProcessing(translate, "preparingDetail", "Preparing the file. Please wait."),
  };
}

export function resolveEmbedStatusLabel(embedStatus: string | null | undefined, translate?: FileProcessingTranslator): string {
  switch (embedStatus?.trim()) {
    case "ready":
      return translateFileProcessing(translate, "embedReady", "Smart retrieval ready ✓");
    case "processing":
      return translateFileProcessing(translate, "embedProcessing", "Indexing…");
    case "failed":
      return translateFileProcessing(translate, "embedFailed", "Index failed");
    case "none":
    case "":
    case undefined:
    case null:
      return translateFileProcessing(translate, "embedPending", "Index pending");
    default:
      return embedStatus ?? translateFileProcessing(translate, "unknown", "Unknown");
  }
}

export function resolveExtractStatusLabel(extractStatus: string | null | undefined, translate?: FileProcessingTranslator): string {
  switch (extractStatus?.trim()) {
    case "ready":
      return translateFileProcessing(translate, "extractReady", "Extraction complete ✓");
    case "processing":
      return translateFileProcessing(translate, "extractProcessing", "Extracting…");
    case "failed":
      return translateFileProcessing(translate, "extractFailed", "Extraction failed");
    case "none":
    case "":
    case undefined:
    case null:
      return translateFileProcessing(translate, "extractPending", "Extraction pending");
    default:
      return extractStatus ?? translateFileProcessing(translate, "unknown", "Unknown");
  }
}

export function resolveFileProcessingToneClass(tone: FileProcessingBadge["tone"]): string {
  switch (tone) {
    case "success":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "info":
      return "border-sky-200 bg-sky-50 text-sky-700";
    case "warning":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "danger":
      return "border-rose-200 bg-rose-50 text-rose-700";
    default:
      return "border-border bg-muted/60 text-muted-foreground";
  }
}
