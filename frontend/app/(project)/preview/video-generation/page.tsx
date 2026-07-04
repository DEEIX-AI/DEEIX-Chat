"use client";

import * as React from "react";

import { ChatArea } from "@/features/chat/components/sections/chat-area";
import { ChatEmptyState } from "@/features/chat/components/sections/chat-empty";
import { ChatInput } from "@/features/chat/components/sections/chat-input";
import type { ChatAreaMessage, MessageAttachment } from "@/features/chat/types/messages";
import type { ChatModelOption, PendingAttachment, UploadingAttachment } from "@/features/chat/types/chat-runtime";
import type { ConversationOptions } from "@/shared/api/conversation.types";
import type { FileContentResult } from "@/shared/api/file";
import type { MCPToolDTO } from "@/shared/api/mcp.types";
import type { SkillSummaryDTO } from "@/shared/api/skills.types";
import type { PreviewDialogFile } from "@/shared/components/file-preview/file-preview-dialog";
import { cn } from "@/lib/utils";

const GENERATION_MS = 10_000;
const VIDEO_MS = 5_000;
const CONTENT_WIDTH_CLASS_NAME = "max-w-[1080px]";
const DEFAULT_PROMPT =
  "Create a 5 second cinematic video of an orange concept car driving through a coastal road at sunset.";
const MODEL_NAME = "gemini-omni-flash-preview";
const MOCK_VIDEO_FILE_ID = "file_mock_gemini_omni_video";
const MOCK_VIDEO_MIME = "video/webm";
const VIDEO_OPTIONS: ConversationOptions = {
  response_format: { type: "video" },
};

type Phase = "queued" | "running" | "saving" | "complete";

const EMPTY_ATTACHMENTS: PendingAttachment[] = [];
const EMPTY_UPLOADS: UploadingAttachment[] = [];
const MODEL_OPTIONS: ChatModelOption[] = [
  {
    platformModelName: MODEL_NAME,
    icon: "gemini",
    vendor: "Google",
    kinds: ["chat", "image_gen", "image_edit", "video_gen"],
    protocols: ["gemini_interactions"],
    defaultOptions: VIDEO_OPTIONS,
    optionControls: [],
    lockedOptionPaths: [],
    nativeToolKeys: [],
    nativeTools: [],
    pricing: null,
  },
];
const EMPTY_TOOLS: MCPToolDTO[] = [];
const EMPTY_SKILLS: SkillSummaryDTO[] = [];
const DEFAULT_VIDEO_ATTACHMENT: MessageAttachment = {
  fileID: MOCK_VIDEO_FILE_ID,
  fileName: "gemini-omni-preview-5s.webm",
  mimeType: MOCK_VIDEO_MIME,
  detectedMime: MOCK_VIDEO_MIME,
  fileCategory: "video",
  sizeBytes: 1_250_000,
  kind: "file",
  processingStatus: "ready",
  processingReady: true,
};

function phaseForElapsed(elapsed: number): Phase {
  if (elapsed >= GENERATION_MS) return "complete";
  if (elapsed >= 8_000) return "saving";
  if (elapsed >= 2_000) return "running";
  return "queued";
}

function activityLabelForPhase(phase: Phase): string {
  if (phase === "queued") return "Video task queued";
  if (phase === "running") return "Generating video";
  if (phase === "saving") return "Saving video";
  return "Video ready";
}

function buildMessages(prompt: string, phase: Phase, elapsed: number, createdAt: string): ChatAreaMessage[] {
  const isComplete = phase === "complete";
  const updatedAt = new Date(new Date(createdAt).getTime() + Math.min(elapsed, GENERATION_MS)).toISOString();
  const userMessage: ChatAreaMessage = {
    key: "preview-user",
    publicID: "msg_preview_user",
    parentPublicID: null,
    sourcePublicID: null,
    role: "user",
    content: prompt,
    contentType: "text",
    branchReason: "default",
    status: "success",
    platformModelName: MODEL_NAME,
    createdAt,
    updatedAt: createdAt,
  };
  const assistantMessage: ChatAreaMessage = {
    key: "preview-assistant",
    publicID: "msg_preview_assistant",
    parentPublicID: userMessage.publicID,
    sourcePublicID: null,
    role: "assistant",
    contentType: "video",
    content: isComplete ? "Generated 5-second video is ready." : "",
    branchReason: "default",
    status: isComplete ? "success" : "pending",
    runID: "run_preview_gemini_omni_video",
    platformModelName: MODEL_NAME,
    createdAt,
    updatedAt,
    isPending: !isComplete,
    isStreaming: !isComplete,
    isFileProc: !isComplete,
    activityLabel: activityLabelForPhase(phase),
    attachments: isComplete ? [DEFAULT_VIDEO_ATTACHMENT] : [],
    inputTokens: isComplete ? 32 : undefined,
    outputTokens: isComplete ? 0 : undefined,
    latencyMS: isComplete ? GENERATION_MS : undefined,
  };

  return [userMessage, assistantMessage];
}

function drawMockVideoFrame(ctx: CanvasRenderingContext2D, progress: number) {
  const { width, height } = ctx.canvas;
  const carX = -width * 0.22 + progress * width * 1.35;
  const sunX = width * (0.18 + progress * 0.18);

  const sky = ctx.createLinearGradient(0, 0, width, height);
  sky.addColorStop(0, "#f59e0b");
  sky.addColorStop(0.45, "#38bdf8");
  sky.addColorStop(1, "#1e293b");
  ctx.fillStyle = sky;
  ctx.fillRect(0, 0, width, height);

  ctx.fillStyle = "rgba(254, 240, 138, 0.9)";
  ctx.beginPath();
  ctx.arc(sunX, height * 0.24, 34, 0, Math.PI * 2);
  ctx.fill();

  ctx.fillStyle = "#315a66";
  ctx.beginPath();
  ctx.moveTo(0, height * 0.58);
  ctx.bezierCurveTo(width * 0.2, height * 0.42, width * 0.32, height * 0.54, width * 0.48, height * 0.4);
  ctx.bezierCurveTo(width * 0.62, height * 0.26, width * 0.78, height * 0.5, width, height * 0.34);
  ctx.lineTo(width, height);
  ctx.lineTo(0, height);
  ctx.closePath();
  ctx.fill();

  ctx.fillStyle = "#1f2937";
  ctx.fillRect(0, height * 0.72, width, height * 0.28);
  ctx.strokeStyle = "rgba(248, 250, 252, 0.72)";
  ctx.lineWidth = 5;
  ctx.setLineDash([70, 48]);
  ctx.beginPath();
  ctx.moveTo(0, height * 0.84);
  ctx.lineTo(width, height * 0.84);
  ctx.stroke();
  ctx.setLineDash([]);

  ctx.save();
  ctx.translate(carX, height * 0.68);
  ctx.fillStyle = "#f97316";
  roundRect(ctx, 86, 30, 170, 54, 16);
  ctx.fill();
  ctx.fillStyle = "#fb923c";
  ctx.beginPath();
  ctx.moveTo(118, 30);
  ctx.lineTo(155, -8);
  ctx.lineTo(210, -8);
  ctx.lineTo(246, 30);
  ctx.closePath();
  ctx.fill();
  ctx.fillStyle = "#dbeafe";
  roundRect(ctx, 158, 4, 44, 23, 5);
  ctx.fill();
  ctx.fillStyle = "#111827";
  ctx.beginPath();
  ctx.arc(126, 88, 19, 0, Math.PI * 2);
  ctx.arc(220, 88, 19, 0, Math.PI * 2);
  ctx.fill();
  ctx.fillStyle = "#cbd5e1";
  ctx.beginPath();
  ctx.arc(126, 88, 7, 0, Math.PI * 2);
  ctx.arc(220, 88, 7, 0, Math.PI * 2);
  ctx.fill();
  ctx.restore();
}

function roundRect(ctx: CanvasRenderingContext2D, x: number, y: number, width: number, height: number, radius: number) {
  ctx.beginPath();
  ctx.moveTo(x + radius, y);
  ctx.arcTo(x + width, y, x + width, y + height, radius);
  ctx.arcTo(x + width, y + height, x, y + height, radius);
  ctx.arcTo(x, y + height, x, y, radius);
  ctx.arcTo(x, y, x + width, y, radius);
  ctx.closePath();
}

function createMockVideoBlob(): Promise<Blob> {
  if (typeof MediaRecorder === "undefined") {
    return Promise.reject(new Error("MediaRecorder is not available in this browser."));
  }

  const canvas = document.createElement("canvas");
  canvas.width = 960;
  canvas.height = 540;
  const ctx = canvas.getContext("2d");
  if (!ctx) {
    return Promise.reject(new Error("Canvas is not available in this browser."));
  }

  const stream = canvas.captureStream(30);
  const mimeType = ["video/webm;codecs=vp9", "video/webm;codecs=vp8", MOCK_VIDEO_MIME].find((item) =>
    MediaRecorder.isTypeSupported(item),
  );
  const recorder = new MediaRecorder(stream, mimeType ? { mimeType } : undefined);
  const chunks: BlobPart[] = [];

  return new Promise((resolve, reject) => {
    let frameID = 0;
    const stopTracks = () => stream.getTracks().forEach((track) => track.stop());

    recorder.ondataavailable = (event) => {
      if (event.data.size > 0) {
        chunks.push(event.data);
      }
    };
    recorder.onerror = () => {
      cancelAnimationFrame(frameID);
      stopTracks();
      reject(new Error("Failed to render mock video."));
    };
    recorder.onstop = () => {
      stopTracks();
      resolve(new Blob(chunks, { type: recorder.mimeType || mimeType || MOCK_VIDEO_MIME }));
    };

    recorder.start();
    const startedAt = performance.now();
    const render = (now: number) => {
      const progress = Math.min(1, (now - startedAt) / VIDEO_MS);
      drawMockVideoFrame(ctx, progress);
      if (progress < 1) {
        frameID = requestAnimationFrame(render);
        return;
      }
      recorder.stop();
    };
    frameID = requestAnimationFrame(render);
  });
}

export default function Page() {
  const [draft, setDraft] = React.useState(DEFAULT_PROMPT);
  const [submittedPrompt, setSubmittedPrompt] = React.useState("");
  const [submittedAt, setSubmittedAt] = React.useState("");
  const [startedAt, setStartedAt] = React.useState<number | null>(null);
  const [elapsed, setElapsed] = React.useState(0);
  const [selectedPlatformModelName, setSelectedPlatformModelName] = React.useState(MODEL_NAME);
  const [options, setOptions] = React.useState<ConversationOptions>(VIDEO_OPTIONS);
  const messageContentRef = React.useRef<HTMLDivElement | null>(null);
  const mockVideoBlobRef = React.useRef<Promise<Blob> | null>(null);

  React.useEffect(() => {
    if (startedAt === null) {
      return undefined;
    }
    const timer = window.setInterval(() => {
      const nextElapsed = Math.min(GENERATION_MS, performance.now() - startedAt);
      setElapsed(nextElapsed);
      if (nextElapsed >= GENERATION_MS) {
        window.clearInterval(timer);
      }
    }, 120);
    return () => window.clearInterval(timer);
  }, [startedAt]);

  const phase = startedAt === null ? null : phaseForElapsed(elapsed);
  const messages = React.useMemo(
    () => (phase && submittedAt ? buildMessages(submittedPrompt, phase, elapsed, submittedAt) : []),
    [elapsed, phase, submittedAt, submittedPrompt],
  );
  const busy = Boolean(phase && phase !== "complete");

  const onSendMessage = React.useCallback(() => {
    const nextPrompt = draft.trim();
    if (!nextPrompt) {
      return;
    }
    mockVideoBlobRef.current = createMockVideoBlob();
    setSubmittedPrompt(nextPrompt);
    setSubmittedAt(new Date().toISOString());
    setDraft("");
    setElapsed(0);
    setStartedAt(performance.now());
  }, [draft]);

  const loadMockVideoContent = React.useCallback(async (file: PreviewDialogFile): Promise<FileContentResult> => {
    if (file.fileID !== MOCK_VIDEO_FILE_ID) {
      throw new Error("Unknown preview file.");
    }
    if (!mockVideoBlobRef.current) {
      mockVideoBlobRef.current = createMockVideoBlob();
    }
    const blob = await mockVideoBlobRef.current;
    return {
      blob,
      contentType: blob.type || MOCK_VIDEO_MIME,
      disposition: `attachment; filename="${file.fileName}"`,
      contentLength: blob.size,
    };
  }, []);

  const chatInput = (
    <ChatInput
      draft={draft}
      loading={false}
      sending={busy}
      uploading={false}
      isConversationMode={messages.length > 0}
      maxFilesPerMessage={1}
      fileMode="auto"
      inputHeight="standard"
      attachments={EMPTY_ATTACHMENTS}
      uploadingAttachments={EMPTY_UPLOADS}
      modelOptions={MODEL_OPTIONS}
      billingDisplayCurrency="USD"
      billingDisplayUsdToCnyRate={null}
      selectedPlatformModelName={selectedPlatformModelName}
      availableTools={EMPTY_TOOLS}
      selectedToolIDs={[]}
      selectedSkills={EMPTY_SKILLS}
      defaultToolIDs={[]}
      queuedMessages={[]}
      htmlVisualPromptEnabled={false}
      maxSelectedTools={0}
      maxSelectedSkills={0}
      toolsLoading={false}
      options={options}
      defaultOptions={VIDEO_OPTIONS}
      modelOptionPolicy={null}
      modelLoading={false}
      modelDisabled={false}
      onDraftChange={setDraft}
      onModelChange={setSelectedPlatformModelName}
      onSelectedToolsChange={() => undefined}
      onSelectedSkillsChange={() => undefined}
      onDefaultToolsChange={() => undefined}
      onHTMLVisualPromptChange={() => undefined}
      onOptionsChange={setOptions}
      onOptionsReset={(defaults) => setOptions(defaults ?? VIDEO_OPTIONS)}
      onOptionsDefaultRestore={() => Promise.resolve(VIDEO_OPTIONS)}
      onAttachExistingFile={() => undefined}
      onUploadFiles={() => undefined}
      onCaptureScreenshot={() => undefined}
      onRemoveAttachment={() => undefined}
      onSendMessage={onSendMessage}
      onStopMessage={() => setElapsed(GENERATION_MS)}
      onDeleteQueuedMessage={() => undefined}
      onEditQueuedMessage={() => undefined}
      onGuideQueuedMessage={() => undefined}
    />
  );

  return (
    <main className="flex h-full min-h-0 w-full bg-background text-foreground">
      <div className="relative flex h-full min-h-0 w-full flex-1 flex-col overflow-hidden md:overflow-visible">
        {messages.length === 0 ? (
          <ChatEmptyState
            greetingTitle="Gemini video generation preview"
            badgeLabel="Preview"
            contentWidthClassName={CONTENT_WIDTH_CLASS_NAME}
          >
            {chatInput}
          </ChatEmptyState>
        ) : (
          <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
            <ChatArea
              title="Gemini video generation preview"
              starred={false}
              canOperateConversation={false}
              messages={messages}
              busy={busy}
              messageContentRef={messageContentRef}
              onScroll={() => undefined}
              onRetryUserMessage={() => undefined}
              onRetryAssistantMessage={() => undefined}
              onEditAssistantMessage={() => false}
              onEditUserMessage={() => false}
              modelOptions={MODEL_OPTIONS}
              selectedPlatformModelName={selectedPlatformModelName}
              onModelChange={setSelectedPlatformModelName}
              attachmentContentLoader={loadMockVideoContent}
              onCycleMessageBranch={() => undefined}
              markdownRender
              showModelInfo
              showLatency
              showTokenUsage={false}
              contentWidthClassName={CONTENT_WIDTH_CLASS_NAME}
            />
            <div className="relative z-10 shrink-0 px-3 pb-3 md:px-6">
              <div className={cn("mx-auto w-full", CONTENT_WIDTH_CLASS_NAME)}>{chatInput}</div>
            </div>
          </div>
        )}
      </div>
    </main>
  );
}
