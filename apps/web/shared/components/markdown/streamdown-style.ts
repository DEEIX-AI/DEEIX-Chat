import type { CSSProperties } from "react";

import { referencesOnlyHTMLVisualThemeVariables } from "@/shared/lib/html-visual-theme";

const SAFE_HTML_STYLE_PROPERTIES: ReadonlySet<string> = new Set([
  "alignContent",
  "alignItems",
  "alignSelf",
  "background",
  "backgroundColor",
  "border",
  "borderBlock",
  "borderBlockEnd",
  "borderBlockStart",
  "borderBottom",
  "borderColor",
  "borderInline",
  "borderInlineEnd",
  "borderInlineStart",
  "borderLeft",
  "borderBottomWidth",
  "borderRadius",
  "borderRight",
  "borderStyle",
  "borderTop",
  "borderWidth",
  "boxShadow",
  "boxSizing",
  "color",
  "columnGap",
  "display",
  "flex",
  "flexBasis",
  "flexDirection",
  "flexGrow",
  "flexShrink",
  "flexWrap",
  "fontSize",
  "fontFamily",
  "fontStyle",
  "fontWeight",
  "gap",
  "gridAutoColumns",
  "gridAutoFlow",
  "gridAutoRows",
  "gridColumn",
  "gridColumnEnd",
  "gridColumnStart",
  "gridRow",
  "gridRowEnd",
  "gridRowStart",
  "gridTemplateColumns",
  "gridTemplateRows",
  "height",
  "justifyItems",
  "justifyContent",
  "justifySelf",
  "letterSpacing",
  "lineHeight",
  "margin",
  "marginBlock",
  "marginBlockEnd",
  "marginBlockStart",
  "marginBottom",
  "marginInline",
  "marginInlineEnd",
  "marginInlineStart",
  "marginLeft",
  "marginRight",
  "marginTop",
  "maxHeight",
  "maxWidth",
  "minHeight",
  "minWidth",
  "opacity",
  "order",
  "overflow",
  "overflowX",
  "overflowY",
  "padding",
  "paddingBlock",
  "paddingBlockEnd",
  "paddingBlockStart",
  "paddingBottom",
  "paddingInline",
  "paddingInlineEnd",
  "paddingInlineStart",
  "paddingLeft",
  "paddingRight",
  "paddingTop",
  "placeContent",
  "placeItems",
  "placeSelf",
  "position",
  "rowGap",
  "textAlign",
  "top",
  "right",
  "bottom",
  "left",
  "transform",
  "verticalAlign",
  "whiteSpace",
  "width",
  "zIndex",
]);

const KATEX_SAFE_HTML_STYLE_PROPERTIES: ReadonlySet<string> = new Set([
  ...SAFE_HTML_STYLE_PROPERTIES,
  "top",
]);
const UNSAFE_STYLE_VALUE_RE = /(?:url\s*\(|expression\s*\(|javascript:|@import|[<>{}])/i;
const COLOR_PROPERTY_NAMES = new Set([
  "border",
  "background",
  "backgroundColor",
  "borderColor",
  "borderBlock",
  "borderBlockEnd",
  "borderBlockStart",
  "borderBottom",
  "borderInline",
  "borderInlineEnd",
  "borderInlineStart",
  "borderLeft",
  "borderRight",
  "borderTop",
  "boxShadow",
  "color",
]);
function isSafeHTMLStyleValue(value: string | number): boolean {
  if (typeof value === "number") {
    return Number.isFinite(value);
  }
  const normalizedValue = value.trim();
  return (
    Boolean(normalizedValue) &&
    normalizedValue.length <= 120 &&
    !UNSAFE_STYLE_VALUE_RE.test(normalizedValue) &&
    referencesOnlyHTMLVisualThemeVariables(normalizedValue)
  );
}

function sanitizeStyle(
  style: CSSProperties | undefined,
  safeProperties: ReadonlySet<string>,
): CSSProperties | undefined {
  if (!style) {
    return undefined;
  }

  const safeStyle: Record<string, string | number> = {};
  for (const [property, value] of Object.entries(style)) {
    if (!safeProperties.has(property)) {
      continue;
    }
    if (typeof value !== "string" && typeof value !== "number") {
      continue;
    }
    if (!isSafeHTMLStyleValue(value)) {
      continue;
    }
    if (COLOR_PROPERTY_NAMES.has(property)) {
      if (typeof value === "number") {
        continue;
      }
      safeStyle[property] = value;
      continue;
    }
    safeStyle[property] = value;
  }

  return Object.keys(safeStyle).length > 0 ? safeStyle : undefined;
}

export function sanitizeHTMLStyle(style: CSSProperties | undefined): CSSProperties | undefined {
  return sanitizeStyle(style, SAFE_HTML_STYLE_PROPERTIES);
}

export function sanitizeKatexHTMLStyle(style: CSSProperties | undefined): CSSProperties | undefined {
  return sanitizeStyle(style, KATEX_SAFE_HTML_STYLE_PROPERTIES);
}
