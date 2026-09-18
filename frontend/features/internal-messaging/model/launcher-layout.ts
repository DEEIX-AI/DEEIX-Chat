export type LauncherPoint = { x: number; y: number };
export type LauncherEdge = "left" | "right";
export type LauncherPosition = { point: LauncherPoint; edge: LauncherEdge };

export const LAUNCHER_SIZE = 44;
export const LAUNCHER_TAB_WIDTH = 24;
export const LAUNCHER_MARGIN = 8;
export const LAYOUT_STORAGE_PREFIX = "deeix.internal-messaging.layout.v1";

export function parseMessagingLayout(raw: string | null): Record<string, unknown> {
  try {
    const parsed: unknown = raw ? JSON.parse(raw) : undefined;
    return parsed && typeof parsed === "object" && !Array.isArray(parsed)
      ? parsed as Record<string, unknown>
      : {};
  } catch {
    return {};
  }
}

function clamp(value: number, maximum: number) {
  return Math.min(Math.max(value, LAUNCHER_MARGIN), Math.max(LAUNCHER_MARGIN, maximum));
}

export function clampLauncherPoint(
  point: LauncherPoint,
  width: number,
  height: number,
): LauncherPoint {
  return {
    x: clamp(point.x, width - LAUNCHER_SIZE - LAUNCHER_MARGIN),
    y: clamp(point.y, height - LAUNCHER_SIZE - LAUNCHER_MARGIN),
  };
}

export function dockLauncherPoint(
  point: LauncherPoint,
  width: number,
  height: number,
  edge: LauncherEdge = point.x + LAUNCHER_SIZE / 2 < width / 2 ? "left" : "right",
): LauncherPosition {
  return {
    edge,
    point: clampLauncherPoint(
      { x: edge === "left" ? LAUNCHER_MARGIN : width - LAUNCHER_SIZE - LAUNCHER_MARGIN, y: point.y },
      width,
      height,
    ),
  };
}

export function restoreLauncherPosition(
  stored: unknown,
  width: number,
  height: number,
): LauncherPosition {
  const layout = stored && typeof stored === "object"
    ? stored as { button?: Partial<LauncherPoint>; buttonEdge?: unknown }
    : undefined;
  const button = layout?.button;
  const point = button && Number.isFinite(button.x) && Number.isFinite(button.y)
    ? button as LauncherPoint
    : { x: width - LAUNCHER_SIZE - 20, y: height - LAUNCHER_SIZE - 20 };
  const edge = layout?.buttonEdge === "left" || layout?.buttonEdge === "right"
    ? layout.buttonEdge
    : undefined;
  return dockLauncherPoint(point, width, height, edge);
}
