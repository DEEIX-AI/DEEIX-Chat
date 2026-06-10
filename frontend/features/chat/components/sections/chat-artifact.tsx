"use client";

import * as React from "react";
import { AnimatePresence, motion } from "motion/react";
import { Download, X } from "lucide-react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import {
  buildArtifactPreviewDocument,
  downloadArtifactHTML,
  resolveArtifactDownloadName,
  type ChatArtifact,
} from "@/features/chat/model/chat-artifacts";
import { CopyActionButton } from "@/shared/components/copy-action";
import { cn } from "@/lib/utils";

type ChatArtifactWorkspaceProps = {
  artifact: ChatArtifact | null;
  artifacts: ChatArtifact[];
  isInlineViewport: boolean;
  onArtifactChange: (artifactID: string) => void;
  onClose: () => void;
  onResizeReset: () => void;
  onResizeStart: (event: React.PointerEvent<HTMLButtonElement>) => void;
};

type ChatArtifactPanelProps = {
  artifact: ChatArtifact;
  artifacts: ChatArtifact[];
  className?: string;
  onArtifactChange: (artifactID: string) => void;
  onClose: () => void;
};

type ArtifactPreviewFrameProps = {
  documentHTML: string;
  title: string;
};

const workspaceTransition = {
  duration: 0.5,
  ease: [0.16, 1, 0.3, 1] as const,
};
const DESKTOP_SHELL_GUTTER_PX = 16;
const ARTIFACT_IFRAME_PERMISSIONS = [
  "accelerometer 'none'",
  "autoplay 'none'",
  "camera 'none'",
  "clipboard-read 'none'",
  "clipboard-write 'none'",
  "encrypted-media 'none'",
  "fullscreen 'none'",
  "geolocation 'none'",
  "gyroscope 'none'",
  "microphone 'none'",
  "midi 'none'",
  "payment 'none'",
  "serial 'none'",
  "usb 'none'",
  "bluetooth 'none'",
].join("; ");

function artifactLanguageLabel(artifact: ChatArtifact): string {
  if (artifact.kind === "javascript") return "JS";
  return (artifact.language || artifact.kind).toUpperCase();
}

function artifactLabel(artifact: ChatArtifact, index: number): string {
  return `${artifactLanguageLabel(artifact)} #${index + 1}`;
}

function ArtifactActionButton({
  label,
  children,
  onClick,
  disabled,
}: {
  label: string;
  children: React.ReactNode;
  onClick: () => void;
  disabled?: boolean;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="size-7 rounded-md text-muted-foreground hover:bg-foreground/[0.04] hover:text-foreground"
          aria-label={label}
          disabled={disabled}
          onClick={onClick}
        >
          {children}
        </Button>
      </TooltipTrigger>
      <TooltipContent side="bottom">{label}</TooltipContent>
    </Tooltip>
  );
}

function ArtifactPreviewFrame({ documentHTML, title }: ArtifactPreviewFrameProps) {
  const frameRef = React.useRef<HTMLIFrameElement | null>(null);

  React.useEffect(() => {
    const frame = frameRef.current;
    if (!frame || frame.srcdoc === documentHTML) {
      return;
    }
    frame.srcdoc = documentHTML;
  }, [documentHTML]);

  return (
    <iframe
      ref={frameRef}
      title={title}
      allow={ARTIFACT_IFRAME_PERMISSIONS}
      sandbox="allow-scripts"
      referrerPolicy="no-referrer"
      srcDoc={documentHTML}
      className="h-full min-h-[320px] w-full bg-white"
    />
  );
}

function ChatArtifactPanel({
  artifact,
  artifacts,
  className,
  onArtifactChange,
  onClose,
}: ChatArtifactPanelProps) {
  const t = useTranslations("chat.artifacts");
  const previewHTML = React.useMemo(
    () => buildArtifactPreviewDocument(artifact.kind, artifact.code),
    [artifact.code, artifact.kind],
  );
  const canPreview = artifact.code.trim().length > 0;
  const artifactOptions = React.useMemo(
    () => artifacts.map((item, index) => ({ item, label: artifactLabel(item, index) })),
    [artifacts],
  );

  const handleDownload = React.useCallback(() => {
    if (!canPreview) return;
    downloadArtifactHTML(resolveArtifactDownloadName(artifact.kind), previewHTML);
  }, [artifact.kind, canPreview, previewHTML]);

  return (
    <aside
      className={cn(
        "flex h-full min-h-0 w-full flex-col border-l border-border/55 bg-background",
        className,
      )}
      aria-label={t("title")}
    >
      <Tabs defaultValue="preview" className="flex min-h-0 w-full flex-1 flex-col gap-0">
        <div className="relative flex h-12 shrink-0 items-center justify-between gap-2 border-b border-border/40 px-3">
          <div className="flex min-w-0 max-w-[calc(50%-72px)] items-center gap-2">
            <h2 className="shrink-0 text-sm font-semibold tracking-tight">{t("title")}</h2>
            <Select value={artifact.id} onValueChange={onArtifactChange} disabled={artifacts.length <= 1}>
              <SelectTrigger className="h-6 w-[92px] min-w-0 rounded-md px-2 text-[11px]">
                <SelectValue aria-label={t("selectArtifact")} />
              </SelectTrigger>
              <SelectContent align="start">
                {artifactOptions.map(({ item, label }) => (
                  <SelectItem key={item.id} value={item.id}>
                    {label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <TabsList className="absolute left-1/2 top-1/2 h-8 -translate-x-1/2 -translate-y-1/2">
            <TabsTrigger value="preview" className="px-2">
              {t("preview")}
            </TabsTrigger>
            <TabsTrigger value="source" className="px-2">
              {t("source")}
            </TabsTrigger>
          </TabsList>

          <div className="flex min-w-0 items-center justify-end gap-0.5">
            <Tooltip>
              <TooltipTrigger asChild>
                <CopyActionButton
                  key={`${artifact.id}:${artifact.code}`}
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="size-7 rounded-md text-muted-foreground hover:bg-foreground/[0.04] hover:text-foreground"
                  value={artifact.code}
                  messages={{ copied: t("sourceCopied"), failed: t("copyFailed") }}
                  iconClassName="size-3"
                  aria-label={t("copySource")}
                />
              </TooltipTrigger>
              <TooltipContent side="bottom">{t("copySource")}</TooltipContent>
            </Tooltip>
            <ArtifactActionButton label={t("downloadHtml")} disabled={!canPreview} onClick={handleDownload}>
              <Download className="size-3" />
            </ArtifactActionButton>
            <ArtifactActionButton label={t("close")} onClick={onClose}>
              <X className="size-3" />
            </ArtifactActionButton>
          </div>
        </div>

        <TabsContent value="preview" className="mt-0 min-h-0 flex-1 overflow-hidden">
          {canPreview ? (
            <ArtifactPreviewFrame
              key={artifact.id}
              documentHTML={previewHTML}
              title={t("previewTitle")}
            />
          ) : (
            <div className="flex h-full min-h-[320px] items-center justify-center bg-muted/15 px-6 text-center text-sm text-muted-foreground">
              {t("empty")}
            </div>
          )}
        </TabsContent>

        <TabsContent value="source" className="mt-0 min-h-0 flex-1 overflow-hidden">
          <pre className="h-full min-h-[320px] overflow-auto bg-muted/20 p-4 text-xs leading-5 text-foreground">
            <code className="font-mono">{artifact.code}</code>
          </pre>
        </TabsContent>
      </Tabs>
    </aside>
  );
}

export function ChatArtifactWorkspace({
  artifact,
  artifacts,
  isInlineViewport,
  onArtifactChange,
  onClose,
  onResizeReset,
  onResizeStart,
}: ChatArtifactWorkspaceProps) {
  const t = useTranslations("chat.artifacts");
  const isDesktopOpen = Boolean(artifact && isInlineViewport);
  const desktopBleedWidth = `calc(100% + ${DESKTOP_SHELL_GUTTER_PX}px)`;

  return (
    <>
      <motion.div
        className="hidden min-h-0 overflow-hidden md:block"
        initial={false}
        animate={{
          marginBottom: isDesktopOpen ? -DESKTOP_SHELL_GUTTER_PX : 0,
          marginRight: isDesktopOpen ? -DESKTOP_SHELL_GUTTER_PX : 0,
        }}
        transition={workspaceTransition}
        style={{
          height: isDesktopOpen ? `calc(100% + ${DESKTOP_SHELL_GUTTER_PX}px)` : "100%",
          width: isDesktopOpen ? desktopBleedWidth : "100%",
          willChange: "margin",
        }}
      >
        <AnimatePresence initial={false}>
          {artifact ? (
            <motion.div
              key="artifact-panel-desktop"
              className="relative h-full min-h-0 overflow-hidden"
              initial={{ opacity: 0, x: 32 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: 32 }}
              transition={workspaceTransition}
              style={{ width: "100%", willChange: "transform, opacity" }}
            >
              <button
                type="button"
                className="group absolute -left-2 inset-y-0 z-20 hidden w-4 cursor-col-resize touch-none items-center justify-center bg-transparent outline-none md:flex"
                aria-label={t("resize")}
                title={t("resize")}
                onDoubleClick={onResizeReset}
                onPointerDown={onResizeStart}
              >
                <span className="h-9 w-1 translate-x-2 rounded-full bg-foreground/20 opacity-0 transition-[background-color,opacity] group-hover:bg-foreground/32 group-hover:opacity-100 group-focus-visible:bg-foreground/32 group-focus-visible:opacity-100" />
              </button>
              <ChatArtifactPanel
                artifact={artifact}
                artifacts={artifacts}
                className="h-full"
                onArtifactChange={onArtifactChange}
                onClose={onClose}
              />
            </motion.div>
          ) : null}
        </AnimatePresence>
      </motion.div>

      <AnimatePresence initial={false}>
        {artifact ? (
          <motion.div
            key="artifact-panel-mobile"
            className="absolute inset-0 z-30 min-h-0 overflow-hidden md:hidden"
            initial={{ opacity: 0, x: 32 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: 32 }}
            transition={workspaceTransition}
            style={{ willChange: "transform, opacity" }}
          >
            <ChatArtifactPanel
              artifact={artifact}
              artifacts={artifacts}
              className="h-full"
              onArtifactChange={onArtifactChange}
              onClose={onClose}
            />
          </motion.div>
        ) : null}
      </AnimatePresence>
    </>
  );
}
