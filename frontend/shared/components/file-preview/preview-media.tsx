"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import { FastForward, FileAudio2, Maximize2, Minimize2, Minus, Pause, Play, Plus, Rewind, Volume2, VolumeX } from "lucide-react";
import Image from "next/image";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

type PreviewMediaProps = {
  kind: "image" | "audio" | "video";
  source: string;
  alt?: string;
  contentType?: string;
  toolbarContainer?: HTMLElement | null;
};

const IMAGE_MIN_ZOOM = 0.5;
const IMAGE_MAX_ZOOM = 2;
const IMAGE_ZOOM_STEP = 0.1;
const IMAGE_DEFAULT_ZOOM = 0.8;
const IMAGE_FALLBACK_SIZE = {
  width: 1200,
  height: 900,
};

function clampImageZoom(value: number): number {
  return Math.min(IMAGE_MAX_ZOOM, Math.max(IMAGE_MIN_ZOOM, value));
}

function formatTime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) {
    return "00:00";
  }

  const total = Math.floor(seconds);
  const minutes = Math.floor(total / 60);
  const remain = total % 60;
  return `${String(minutes).padStart(2, "0")}:${String(remain).padStart(2, "0")}`;
}

function resolveMediaTitle(name: string | undefined, fallback: string): string {
  if (!name) {
    return fallback;
  }

  const cleaned = name.trim();
  if (!cleaned) {
    return fallback;
  }

  return cleaned.replace(/\.[^./\\]+$/, "");
}

function resolveAudioLabel(contentType?: string, name?: string): string {
  const normalizedType = contentType?.split(";")[0]?.trim().toLowerCase();
  if (normalizedType) {
    return normalizedType;
  }

  const extension = name?.split(".").pop()?.trim();
  if (extension) {
    return extension.toUpperCase();
  }

  return "audio";
}

export function PreviewMedia({ kind, source, alt, contentType, toolbarContainer }: PreviewMediaProps) {
  const t = useTranslations("files.previewErrors");
  const mediaRef = React.useRef<HTMLAudioElement | HTMLVideoElement | null>(null);
  const imagePreviewRef = React.useRef<HTMLDivElement | null>(null);
  const imageScrollRegionRef = React.useRef<HTMLDivElement | null>(null);
  const videoPreviewRef = React.useRef<HTMLDivElement | null>(null);
  const previousVolumeRef = React.useRef(1);
  const [playing, setPlaying] = React.useState(false);
  const [volume, setVolume] = React.useState(1);
  const [duration, setDuration] = React.useState(0);
  const [currentTime, setCurrentTime] = React.useState(0);
  const [imageZoom, setImageZoom] = React.useState(IMAGE_DEFAULT_ZOOM);
  const [imageIsFullscreen, setImageIsFullscreen] = React.useState(false);
  const [videoIsFullscreen, setVideoIsFullscreen] = React.useState(false);
  const [imageSize, setImageSize] = React.useState<{ width: number; height: number }>(IMAGE_FALLBACK_SIZE);
  const [imageViewport, setImageViewport] = React.useState<{ width: number; height: number }>({ width: 0, height: 0 });
  const isSVG = kind === "image" && (contentType?.split(";")[0]?.trim().toLowerCase() === "image/svg+xml");
  const progress = duration > 0 ? Math.min(currentTime / duration, 1) : 0;
  const audioTitle = React.useMemo(() => resolveMediaTitle(alt, t("untitledAudio")), [alt, t]);
  const audioLabel = React.useMemo(() => resolveAudioLabel(contentType, alt), [alt, contentType]);
  const videoTitle = React.useMemo(() => resolveMediaTitle(alt, t("untitledAudio")), [alt, t]);
  const videoLabel = React.useMemo(() => resolveAudioLabel(contentType, alt), [alt, contentType]);

  React.useEffect(() => {
    if (kind !== "image") {
      return undefined;
    }

    const handleFullscreenChange = () => {
      setImageIsFullscreen(document.fullscreenElement === imagePreviewRef.current);
    };

    document.addEventListener("fullscreenchange", handleFullscreenChange);
    return () => {
      document.removeEventListener("fullscreenchange", handleFullscreenChange);
    };
  }, [kind]);

  React.useEffect(() => {
    if (kind !== "video") {
      return undefined;
    }

    const handleFullscreenChange = () => {
      setVideoIsFullscreen(document.fullscreenElement === videoPreviewRef.current);
    };

    document.addEventListener("fullscreenchange", handleFullscreenChange);
    return () => {
      document.removeEventListener("fullscreenchange", handleFullscreenChange);
    };
  }, [kind]);

  React.useEffect(() => {
    if (kind !== "image") {
      return undefined;
    }

    let cancelled = false;
    const probe = new window.Image();

    probe.onload = () => {
      if (cancelled) {
        return;
      }

      setImageSize({
        width: probe.naturalWidth || IMAGE_FALLBACK_SIZE.width,
        height: probe.naturalHeight || IMAGE_FALLBACK_SIZE.height,
      });
    };

    probe.onerror = () => {
      if (cancelled) {
        return;
      }

      setImageSize(IMAGE_FALLBACK_SIZE);
    };

    probe.src = source;
    setImageZoom(IMAGE_DEFAULT_ZOOM);

    return () => {
      cancelled = true;
      probe.onload = null;
      probe.onerror = null;
    };
  }, [kind, source]);

  React.useEffect(() => {
    if (kind !== "image") {
      setImageViewport({ width: 0, height: 0 });
      return undefined;
    }

    const node = imageScrollRegionRef.current;
    if (!node) {
      return undefined;
    }

    const updateViewport = () => {
      setImageViewport({
        width: Math.max(node.clientWidth, 0),
        height: Math.max(node.clientHeight, 0),
      });
    };

    updateViewport();

    const observer = new ResizeObserver(updateViewport);
    observer.observe(node);
    return () => observer.disconnect();
  }, [kind]);

  React.useEffect(() => {
    if (kind === "image") {
      return;
    }

    setPlaying(false);
    setDuration(0);
    setCurrentTime(0);
  }, [kind, source]);

  const syncMediaMetrics = React.useCallback((media: HTMLAudioElement | HTMLVideoElement) => {
    setDuration(media.duration || 0);
    setCurrentTime(media.currentTime || 0);
    const nextVolume = media.muted ? 0 : media.volume || 0;
    setVolume(nextVolume);

    if (nextVolume > 0.001) {
      previousVolumeRef.current = nextVolume;
    }
  }, []);

  const handleMediaLoadedMetadata = React.useCallback((event: React.SyntheticEvent<HTMLAudioElement | HTMLVideoElement>) => {
    syncMediaMetrics(event.currentTarget);
  }, [syncMediaMetrics]);

  const handleMediaTimeUpdate = React.useCallback((event: React.SyntheticEvent<HTMLAudioElement | HTMLVideoElement>) => {
    setCurrentTime(event.currentTarget.currentTime || 0);
  }, []);

  const handleMediaPlay = React.useCallback(() => {
    setPlaying(true);
  }, []);

  const handleMediaPause = React.useCallback(() => {
    setPlaying(false);
  }, []);

  const handleMediaEnded = React.useCallback((event: React.SyntheticEvent<HTMLAudioElement | HTMLVideoElement>) => {
    setPlaying(false);
    setCurrentTime(event.currentTarget.duration || 0);
  }, []);

  const handleMediaVolumeChange = React.useCallback((event: React.SyntheticEvent<HTMLAudioElement | HTMLVideoElement>) => {
    const media = event.currentTarget;
    const nextVolume = media.muted ? 0 : media.volume || 0;
    setVolume(nextVolume);

    if (nextVolume > 0.001) {
      previousVolumeRef.current = nextVolume;
    }
  }, []);

  const togglePlay = React.useCallback(async () => {
    const media = mediaRef.current;
    if (!media) {
      return;
    }

    if (media.paused) {
      await media.play();
    } else {
      media.pause();
    }
  }, []);

  const handleSeek = React.useCallback((value: string) => {
    const media = mediaRef.current;
    if (!media) {
      return;
    }

    const nextTime = Number.parseFloat(value);
    if (!Number.isFinite(nextTime)) {
      return;
    }
    media.currentTime = nextTime;
    setCurrentTime(nextTime);
  }, []);

  const seekBy = React.useCallback((deltaSeconds: number) => {
    const media = mediaRef.current;
    if (!media) {
      return;
    }

    const nextTime = Math.min(Math.max(media.currentTime + deltaSeconds, 0), media.duration || 0);
    media.currentTime = nextTime;
    setCurrentTime(nextTime);
  }, []);

  const handleVolumeInput = React.useCallback((value: string) => {
    const media = mediaRef.current;
    if (!media) {
      return;
    }

    const nextVolume = Number.parseFloat(value);
    if (!Number.isFinite(nextVolume)) {
      return;
    }

    media.volume = nextVolume;
    media.muted = nextVolume <= 0.001;
    setVolume(nextVolume);

    if (nextVolume > 0.001) {
      previousVolumeRef.current = nextVolume;
    }
  }, []);

  const toggleMute = React.useCallback(() => {
    const media = mediaRef.current;
    if (!media) {
      return;
    }

    if (media.muted || media.volume <= 0.001) {
      const nextVolume = previousVolumeRef.current > 0.001 ? previousVolumeRef.current : 1;
      media.muted = false;
      media.volume = nextVolume;
      setVolume(nextVolume);
      return;
    }

    previousVolumeRef.current = media.volume;
    media.muted = true;
    media.volume = 0;
    setVolume(0);
  }, []);

  const imageFitScale = React.useMemo(() => {
    if (kind !== "image") {
      return 1;
    }

    if (!imageSize.width || !imageSize.height || !imageViewport.width || !imageViewport.height) {
      return 1;
    }

    const availableWidth = Math.max(imageViewport.width - 32, 0);
    const availableHeight = Math.max(imageViewport.height - 48, 0);

    return Math.min(1, availableWidth / imageSize.width, availableHeight / imageSize.height);
  }, [imageSize.height, imageSize.width, imageViewport.height, imageViewport.width, kind]);

  const imageEffectiveScale = imageFitScale * imageZoom;
  const scaledImageWidth = imageSize.width * imageEffectiveScale;
  const scaledImageHeight = imageSize.height * imageEffectiveScale;

  React.useLayoutEffect(() => {
    if (kind !== "image") {
      return;
    }

    const viewport = imageScrollRegionRef.current;
    if (!viewport) {
      return;
    }

    const nextScrollLeft = Math.max((scaledImageWidth - viewport.clientWidth) / 2, 0);
    const nextScrollTop = Math.max((scaledImageHeight - viewport.clientHeight) / 2, 0);

    viewport.scrollTo({
      left: nextScrollLeft,
      top: nextScrollTop,
      behavior: "auto",
    });
  }, [imageViewport.height, imageViewport.width, kind, scaledImageHeight, scaledImageWidth, source]);

  const toggleImageFullscreen = React.useCallback(async () => {
    const element = imagePreviewRef.current;
    if (!element) {
      return;
    }

    if (document.fullscreenElement === element) {
      await document.exitFullscreen();
      return;
    }

    await element.requestFullscreen();
  }, []);

  const toggleVideoFullscreen = React.useCallback(async () => {
    const element = videoPreviewRef.current;
    if (!element) {
      return;
    }

    if (document.fullscreenElement === element) {
      await document.exitFullscreen();
      return;
    }

    await element.requestFullscreen();
  }, []);

  const imageToolbar = (
    <div className="flex items-center gap-1.5">
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="size-7 rounded-full"
        onClick={() => setImageZoom((value) => clampImageZoom(value - IMAGE_ZOOM_STEP))}
        disabled={imageZoom <= IMAGE_MIN_ZOOM}
      >
        <Minus className="size-3.5" />
      </Button>
      <span className="min-w-11 text-center text-[11px] text-muted-foreground">{Math.round(imageZoom * 100)}%</span>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="size-7 rounded-full"
        onClick={() => setImageZoom((value) => clampImageZoom(value + IMAGE_ZOOM_STEP))}
        disabled={imageZoom >= IMAGE_MAX_ZOOM}
      >
        <Plus className="size-3.5" />
      </Button>
      <Button type="button" variant="ghost" size="icon" className="size-7 rounded-full" onClick={() => void toggleImageFullscreen()}>
        {imageIsFullscreen ? <Minimize2 className="size-3.5" /> : <Maximize2 className="size-3.5" />}
      </Button>
    </div>
  );

  return (
    <div className="flex h-full min-h-0 w-full flex-col">
      {kind === "image" ? (
        <div ref={imagePreviewRef} className="flex min-h-0 flex-1 flex-col bg-background">
          {toolbarContainer ? createPortal(imageToolbar, toolbarContainer) : (
            <div className="flex shrink-0 items-center justify-end gap-1.5 px-1 py-2">{imageToolbar}</div>
          )}

          <div ref={imageScrollRegionRef} className="min-h-0 flex-1 overflow-auto">
              <div
                className="flex min-h-full min-w-full w-max items-center justify-center box-border px-4 py-6"
                style={{ minHeight: imageViewport.height > 0 ? `${imageViewport.height}px` : undefined }}
              >
                <div
                  className="relative shrink-0"
                  style={{
                    width: `${scaledImageWidth}px`,
                    height: `${scaledImageHeight}px`,
                  }}
                >
                  <div
                    style={{
                      transform: `scale(${imageEffectiveScale})`,
                      transformOrigin: "top left",
                      width: `${imageSize.width}px`,
                      height: `${imageSize.height}px`,
                    }}
                  >
                    {isSVG ? (
                      <object
                        data={source}
                        type="image/svg+xml"
                        aria-label={alt || "SVG preview"}
                        className="block rounded-lg"
                        style={{ width: `${imageSize.width}px`, height: `${imageSize.height}px` }}
                      >
                        <Image
                          src={source}
                          alt={alt || "SVG preview"}
                          className="block rounded-lg object-contain"
                          width={imageSize.width}
                          height={imageSize.height}
                          sizes="100vw"
                          unoptimized
                          style={{ width: `${imageSize.width}px`, height: `${imageSize.height}px` }}
                        />
                      </object>
                    ) : (
                      <Image
                        src={source}
                        alt={alt || "Image preview"}
                        className="block rounded-lg object-contain"
                        width={imageSize.width}
                        height={imageSize.height}
                        sizes="100vw"
                        unoptimized
                        style={{ width: `${imageSize.width}px`, height: `${imageSize.height}px` }}
                      />
                    )}
                  </div>
                </div>
              </div>
          </div>
        </div>
      ) : kind === "video" ? (
        <div className="flex min-h-full flex-1 items-center justify-center px-4 py-6">
          <div className="w-full max-w-[min(100%,980px)]">
            <div
              ref={videoPreviewRef}
              className={cn(
                "relative mx-auto max-w-full",
                videoIsFullscreen ? "flex h-full w-full items-center justify-center bg-[oklch(0.9791_0.0041_91.45)] p-6" : "w-full",
              )}
            >
              <div
                className={cn(
                  "relative max-w-full",
                  videoIsFullscreen ? "w-fit" : "mx-auto w-fit max-w-[80%]",
                )}
              >
                <video
                  ref={mediaRef as React.RefObject<HTMLVideoElement>}
                  src={source}
                  preload="metadata"
                  playsInline
                  className={cn(
                    "block h-auto w-auto max-w-full rounded-[24px] bg-transparent object-contain shadow-[0_18px_44px_-34px_color-mix(in_oklch,var(--foreground)_30%,transparent)]",
                    videoIsFullscreen ? "max-h-[calc(100vh-48px)]" : "max-h-[min(62vh,720px)]",
                  )}
                  onClick={() => void togglePlay()}
                  onDoubleClick={() => void toggleVideoFullscreen()}
                  onLoadedMetadata={handleMediaLoadedMetadata}
                  onTimeUpdate={handleMediaTimeUpdate}
                  onPlay={handleMediaPlay}
                  onPause={handleMediaPause}
                  onEnded={handleMediaEnded}
                  onVolumeChange={handleMediaVolumeChange}
                />

                {!playing ? (
                  <button
                    type="button"
                    className="absolute left-1/2 top-1/2 z-20 flex size-14 -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full bg-background/88 text-foreground shadow-[0_14px_32px_-20px_color-mix(in_oklch,var(--foreground)_34%,transparent)] backdrop-blur-sm transition hover:bg-background"
                    onClick={() => void togglePlay()}
                  >
                    <Play className="ml-0.5 size-6" strokeWidth={1.9} />
                  </button>
                ) : null}

                <div className="absolute inset-x-3 bottom-3 z-20">
                  <div className="mx-auto max-w-[680px] rounded-2xl bg-background px-3 py-3">
                    <div className="flex items-center gap-2 text-xs">
                      <span className="truncate font-medium text-foreground">{videoTitle}</span>
                      <span className="shrink-0 text-border">|</span>
                      <span className="truncate text-muted-foreground">{videoLabel}</span>
                    </div>

                    <div className="mt-2 flex items-center gap-3 text-[10px] text-muted-foreground">
                      <span className="shrink-0 tabular-nums">{formatTime(currentTime)}</span>
                      <div className="relative flex-1 h-1.5 rounded-full bg-muted">
                        <div
                          className="absolute inset-y-0 left-0 rounded-full bg-foreground/70 transition-[width] duration-200 ease-out"
                          style={{ width: `${progress * 100}%` }}
                        />
                        <input
                          type="range"
                          min={0}
                          max={Math.max(duration, 0)}
                          step={0.1}
                          value={Math.min(currentTime, duration || 0)}
                          className="absolute inset-0 h-full w-full cursor-pointer appearance-none opacity-0"
                          onChange={(event) => handleSeek(event.target.value)}
                        />
                      </div>
                      <span className="shrink-0 tabular-nums">{formatTime(duration)}</span>
                    </div>

                    <div className="mt-2 flex items-center justify-between gap-2">
                      <div className="flex min-w-0 items-center gap-1.5">
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          className="size-8 rounded-full text-muted-foreground hover:bg-accent hover:text-foreground"
                          onClick={() => toggleMute()}
                        >
                          {volume <= 0.001 ? <VolumeX className="size-3" strokeWidth={1.5} /> : <Volume2 className="size-3" strokeWidth={1.5} />}
                        </Button>

                        <div className="relative hidden h-5 w-16 shrink-0 md:block">
                          <div className="absolute inset-x-0 top-1/2 h-1.5 -translate-y-1/2 rounded-full bg-muted" />
                          <div
                            className="absolute left-0 top-1/2 h-1.5 -translate-y-1/2 rounded-full bg-primary"
                            style={{ width: `${Math.max(volume, 0) * 100}%` }}
                          />
                          <input
                            type="range"
                            min={0}
                            max={1}
                            step={0.01}
                            value={volume}
                            className="absolute inset-0 h-full w-full cursor-pointer appearance-none opacity-0"
                            onChange={(event) => handleVolumeInput(event.target.value)}
                          />
                        </div>
                      </div>

                      <div className="flex shrink-0 items-center gap-1">
                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          className="h-6 px-3 py-1 text-[11px] text-muted-foreground hover:bg-accent hover:text-foreground"
                          onClick={() => seekBy(-5)}
                        >
                          <Rewind className="size-3.5" strokeWidth={1.6} />
                          <span>5s</span>
                        </Button>

                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          className="size-8 rounded-full bg-muted/60 hover:bg-accent"
                          onClick={() => void togglePlay()}
                        >
                          {playing ? <Pause className="size-4" strokeWidth={1.9} /> : <Play className="ml-0.5 size-4" strokeWidth={1.9} />}
                        </Button>

                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          className="h-6 px-3 py-1 text-[11px] text-muted-foreground hover:bg-accent hover:text-foreground"
                          onClick={() => seekBy(5)}
                        >
                          <FastForward className="size-3.5" strokeWidth={1.6} />
                          <span>5s</span>
                        </Button>

                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          className="size-6 px-3 py-1 text-muted-foreground hover:bg-accent hover:text-foreground"
                          onClick={() => void toggleVideoFullscreen()}
                        >
                          {videoIsFullscreen ? <Minimize2 className="size-3" strokeWidth={1.6} /> : <Maximize2 className="size-3" strokeWidth={1.6} />}
                        </Button>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      ) : (
        <div className="flex min-h-full flex-1 items-center justify-center px-4 py-5">
          <div className="relative w-full max-w-[620px]">
            <audio
              ref={mediaRef as React.RefObject<HTMLAudioElement>}
              src={source}
              preload="metadata"
              onLoadedMetadata={handleMediaLoadedMetadata}
              onTimeUpdate={handleMediaTimeUpdate}
              onPlay={handleMediaPlay}
              onPause={handleMediaPause}
              onEnded={handleMediaEnded}
              onVolumeChange={handleMediaVolumeChange}
            />
            <div className="relative mx-auto flex w-full max-w-[520px] items-center gap-4 rounded-xl bg-muted/60 px-4 py-4">
              <div className="flex shrink-0 flex-col items-center">
                <div className="group relative flex size-20 items-center justify-center overflow-hidden rounded-xl bg-foreground/5">
                  <div className="relative z-10 flex items-center justify-center rounded-full">
                    <FileAudio2 className="size-8"/>
                  </div>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="absolute inset-0 z-20 size-full rounded-xl border-0 bg-background/28 text-foreground opacity-0 shadow-none backdrop-blur-md transition-opacity duration-200 hover:bg-background/36 group-hover:opacity-100 focus-visible:opacity-100"
                    onClick={() => void togglePlay()}
                  >
                    {playing ? <Pause className="size-8" strokeWidth={1.8} /> : <Play className="ml-1 size-8" strokeWidth={1.8} />}
                  </Button>
                </div>
              </div>

              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <h3 className="truncate text-2xl font-medium text-foreground">{audioTitle}</h3>
                  <p className="truncate text-xs text-muted-foreground">｜{audioLabel}</p>
                </div>

                <div className="mt-2">
                  <div className="relative h-1.5 rounded-full bg-foreground/10">
                    <div
                      className="absolute inset-y-0 left-0 rounded-full bg-foreground/70 transition-[width] duration-200 ease-out"
                      style={{ width: `${progress * 100}%` }}
                    />
                    <input
                      type="range"
                      min={0}
                      max={Math.max(duration, 0)}
                      step={0.1}
                      value={Math.min(currentTime, duration || 0)}
                      className="absolute inset-0 h-full w-full cursor-pointer appearance-none opacity-0"
                      onChange={(event) => handleSeek(event.target.value)}
                    />
                  </div>

                  <div className="mt-1.5 flex items-center justify-between gap-3 text-[10px] text-muted-foreground">
                    <span>{formatTime(currentTime)}</span>
                    <span>{formatTime(duration)}</span>
                  </div>
                </div>

              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
