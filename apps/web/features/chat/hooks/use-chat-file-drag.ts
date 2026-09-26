"use client";

import * as React from "react";

function dragEventContainsFiles(event: React.DragEvent<HTMLElement>): boolean {
  return Array.from(event.dataTransfer.types ?? []).includes("Files");
}

function droppedFiles(event: React.DragEvent<HTMLElement>): File[] {
  return Array.from(event.dataTransfer.files ?? []).filter(
    (file) => file.name.trim() || file.size > 0,
  );
}

/**
 * 文件拖拽上传：按拖入/拖出深度计数维护高亮状态（避免子元素间移动误关），
 * drop 时过滤出有效文件并交给上传回调；禁用期间重置状态且不接受拖放。
 */
export function useChatFileDrag({
  disabled,
  onUploadFiles,
}: {
  disabled: boolean;
  onUploadFiles: (files: File[]) => unknown;
}) {
  const fileDragDepthRef = React.useRef(0);
  const [fileDragActive, setFileDragActive] = React.useState(false);

  const resetFileDragState = React.useCallback(() => {
    fileDragDepthRef.current = 0;
    setFileDragActive(false);
  }, []);

  const onFileDragEnter = React.useCallback(
    (event: React.DragEvent<HTMLDivElement>) => {
      if (!dragEventContainsFiles(event)) {
        return;
      }
      event.preventDefault();
      event.stopPropagation();
      if (disabled) {
        return;
      }
      fileDragDepthRef.current += 1;
      setFileDragActive(true);
    },
    [disabled],
  );

  const onFileDragOver = React.useCallback(
    (event: React.DragEvent<HTMLDivElement>) => {
      if (!dragEventContainsFiles(event)) {
        return;
      }
      event.preventDefault();
      event.stopPropagation();
      event.dataTransfer.dropEffect = disabled ? "none" : "copy";
    },
    [disabled],
  );

  const onFileDragLeave = React.useCallback((event: React.DragEvent<HTMLDivElement>) => {
    if (!dragEventContainsFiles(event)) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    fileDragDepthRef.current = Math.max(0, fileDragDepthRef.current - 1);
    if (fileDragDepthRef.current === 0) {
      setFileDragActive(false);
    }
  }, []);

  const onFileDrop = React.useCallback(
    (event: React.DragEvent<HTMLDivElement>) => {
      if (!dragEventContainsFiles(event)) {
        return;
      }
      event.preventDefault();
      event.stopPropagation();
      const files = droppedFiles(event);
      resetFileDragState();
      if (disabled || files.length === 0) {
        return;
      }
      void onUploadFiles(files);
    },
    [disabled, onUploadFiles, resetFileDragState],
  );

  React.useEffect(() => {
    if (disabled) {
      resetFileDragState();
    }
  }, [disabled, resetFileDragState]);

  return {
    fileDragActive,
    onFileDragEnter,
    onFileDragOver,
    onFileDragLeave,
    onFileDrop,
  };
}
