export const HTML_VISUAL_THEME_VARIABLES = [
  "--background",
  "--foreground",
  "--pure",
  "--pure-foreground",
  "--card",
  "--card-foreground",
  "--popover",
  "--popover-foreground",
  "--primary",
  "--primary-foreground",
  "--secondary",
  "--secondary-foreground",
  "--muted",
  "--muted-foreground",
  "--accent",
  "--accent-foreground",
  "--destructive",
  "--destructive-foreground",
  "--border",
  "--input",
  "--ring",
  "--chart-1",
  "--chart-2",
  "--chart-3",
  "--chart-4",
  "--chart-5",
  "--font-sans",
  "--font-serif",
  "--font-mono",
  "--font-economist",
  "--font-songti",
  "--font-heiti",
  "--font-chat",
  "--font-chat-weight",
  "--font-chat-strong-weight",
  "--ui-font-scale",
  "--chat-font-scale",
  "--tracking-normal",
  "--radius",
  "--spacing",
  "--shadow-x",
  "--shadow-y",
  "--shadow-blur",
  "--shadow-spread",
  "--shadow-opacity",
  "--shadow-color",
  "--shadow-2xs",
  "--shadow-xs",
  "--shadow-sm",
  "--shadow",
  "--shadow-md",
  "--shadow-lg",
  "--shadow-xl",
  "--shadow-2xl",
] as const;

export type HTMLVisualThemeSnapshot = {
  colorScheme: "light" | "dark";
  variables: Array<readonly [string, string]>;
};

const HTML_VISUAL_THEME_VARIABLE_SET = new Set<string>(HTML_VISUAL_THEME_VARIABLES);
const CSS_VARIABLE_REFERENCE_RE = /var\s*\(\s*(--[a-z0-9_-]+)/giu;
const CSS_VARIABLE_FUNCTION_RE = /var\s*\(/giu;
const UNSAFE_THEME_VALUE_RE = /[<>{};]/u;

export function referencesOnlyHTMLVisualThemeVariables(value: string): boolean {
  const functionCount = value.match(CSS_VARIABLE_FUNCTION_RE)?.length ?? 0;
  if (functionCount === 0) {
    return true;
  }

  CSS_VARIABLE_REFERENCE_RE.lastIndex = 0;
  let referenceCount = 0;
  while (true) {
    const match = CSS_VARIABLE_REFERENCE_RE.exec(value);
    if (match === null) {
      break;
    }
    referenceCount += 1;
    if (!HTML_VISUAL_THEME_VARIABLE_SET.has(match[1])) {
      return false;
    }
  }
  return referenceCount === functionCount;
}

export function captureHTMLVisualThemeSnapshot(
  colorScheme: "light" | "dark",
): HTMLVisualThemeSnapshot {
  if (typeof window === "undefined") {
    return { colorScheme, variables: [] };
  }

  const computedStyle = window.getComputedStyle(document.documentElement);
  const variables: Array<readonly [string, string]> = [];
  for (const name of HTML_VISUAL_THEME_VARIABLES) {
    const value = computedStyle.getPropertyValue(name).trim();
    if (!value || value.length > 512 || UNSAFE_THEME_VALUE_RE.test(value)) {
      continue;
    }
    variables.push([name, value]);
  }
  return { colorScheme, variables };
}
