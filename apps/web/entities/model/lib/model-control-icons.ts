import {
  AlignLeft,
  AtSign,
  BookOpen,
  Braces,
  Brain,
  Calculator,
  Clock,
  Code,
  CodeXml,
  Cpu,
  Database,
  Dices,
  Eye,
  Feather,
  FileImage,
  FileSearch,
  FileText,
  Gauge,
  Gem,
  Globe,
  GlobeCode,
  Hash,
  Image,
  ImagePlus,
  Images,
  Languages,
  Layers,
  Lightbulb,
  Link,
  ListChecks,
  type LucideIcon,
  MapPin,
  Maximize2,
  MessageSquare,
  Mic,
  Monitor,
  PaintBucket,
  Palette,
  Percent,
  Plug,
  Ratio,
  Repeat,
  Rocket,
  Ruler,
  Scale,
  Search,
  SearchCode,
  Settings2,
  Shapes,
  Shield,
  SlidersHorizontal,
  Sparkles,
  SquareTerminal,
  Target,
  Terminal,
  Thermometer,
  Timer,
  ToggleRight,
  Video,
  WandSparkles,
  Wrench,
  Zap,
} from "lucide-react";

import type { ModelControl } from "@/entities/model/lib/model-controls";

// Icons administrators can reference by name in `controls[].icon`.
const MODEL_CONTROL_ICONS: Record<string, LucideIcon> = {
  "align-left": AlignLeft,
  "at-sign": AtSign,
  "book-open": BookOpen,
  braces: Braces,
  brain: Brain,
  calculator: Calculator,
  clock: Clock,
  code: Code,
  cpu: Cpu,
  database: Database,
  dices: Dices,
  eye: Eye,
  feather: Feather,
  "file-image": FileImage,
  "file-search": FileSearch,
  "file-text": FileText,
  gauge: Gauge,
  gem: Gem,
  globe: Globe,
  hash: Hash,
  image: Image,
  "image-plus": ImagePlus,
  images: Images,
  languages: Languages,
  layers: Layers,
  lightbulb: Lightbulb,
  link: Link,
  "list-checks": ListChecks,
  "map-pin": MapPin,
  maximize: Maximize2,
  "message-square": MessageSquare,
  mic: Mic,
  monitor: Monitor,
  "paint-bucket": PaintBucket,
  palette: Palette,
  percent: Percent,
  plug: Plug,
  ratio: Ratio,
  repeat: Repeat,
  rocket: Rocket,
  ruler: Ruler,
  scale: Scale,
  search: Search,
  settings: Settings2,
  shapes: Shapes,
  shield: Shield,
  sliders: SlidersHorizontal,
  sparkles: Sparkles,
  target: Target,
  terminal: Terminal,
  text: AlignLeft,
  thermometer: Thermometer,
  timer: Timer,
  video: Video,
  wand: WandSparkles,
  wrench: Wrench,
  zap: Zap,
};

export const MODEL_CONTROL_ICON_NAMES = Object.keys(MODEL_CONTROL_ICONS).sort();

// Built-in icons for common request parameters, matched on the whole control id (path) first and
// then on its last segment (lowercase, separators removed), so controls compiled from
// optionControls get a sensible icon without configuration.
const PARAMETER_ICONS: Array<[readonly string[], LucideIcon]> = [
  [["effort", "reasoningeffort", "thinking", "think", "enablethinking", "thinkinglevel", "thinkingbudget", "thinkingtype", "budgettokens"], Brain],
  [["summary", "reasoningsummary", "display", "thinkingdisplay", "includethoughts", "thinkingsummaries"], Eye],
  [["temperature"], Thermometer],
  [["topp", "topk"], Dices],
  [["seed"], Hash],
  [["maxtokens", "maxoutputtokens", "maxcompletiontokens"], Ruler],
  [["verbosity"], AlignLeft],
  [["servicetier", "speed"], Zap],
  [["frequencypenalty", "presencepenalty", "repetitionpenalty"], Repeat],
  [["paralleltoolcalls", "toolchoice", "mode"], Wrench],
  [["responseformat", "responseformattype", "responsemimetype", "outputconfigformattype", "format"], Braces],
  [["logprobs", "responselogprobs", "toplogprobs"], Percent],
  [["store", "enablecache", "cachetimeout", "ttl"], Database],
  [["n", "candidatecount"], Layers],
  [["aspectratio"], Ratio],
  [["size", "imagesize", "resolution", "mediaresolution"], Maximize2],
  [["quality", "inputfidelity"], Gem],
  [["background"], PaintBucket],
  [["outputformat", "outputcompression"], FileImage],
  [["partialimages"], Images],
  [["moderation", "safetysettings"], Shield],
  [["duration"], Timer],
  [["style"], Palette],
  [["responsemodalities"], Shapes],
  [["websearch", "websearchoptions", "search"], Globe],
  [["language", "locale"], Languages],
];

function normalizeParameterKey(value: string): string {
  return value.toLowerCase().replace(/[^a-z0-9]/g, "");
}

function builtinParameterIcon(value: string): LucideIcon | null {
  const segments = value.split(".").filter(Boolean);
  for (const key of [normalizeParameterKey(value), normalizeParameterKey(segments.at(-1) ?? value)]) {
    for (const [keys, icon] of PARAMETER_ICONS) {
      if (keys.includes(key)) return icon;
    }
  }
  return null;
}

const FALLBACK_ICONS: Record<ModelControl["type"], LucideIcon> = {
  select: SlidersHorizontal,
  toggle: ToggleRight,
  number: Gauge,
};

/**
 * Icon of a model control: the administrator's `icon` when it names a known icon, otherwise a
 * built-in icon for the reasoning control and common parameters, otherwise one per control type.
 */
export function resolveModelControlIcon(control: Pick<ModelControl, "icon" | "id" | "kind" | "type">): LucideIcon {
  const configured = MODEL_CONTROL_ICONS[control.icon.trim()];
  if (configured) return configured;
  if (control.kind === "reasoning") return Brain;
  return builtinParameterIcon(control.id) ?? FALLBACK_ICONS[control.type];
}

/** Icon of a provider-native tool from its type (web search, code execution, URL fetch…). */
export function resolveNativeToolIcon(toolType: string): LucideIcon {
  const type = toolType.toLowerCase();
  if (type === "x_search") return AtSign;
  if (type.startsWith("tool_search")) return SearchCode;
  if (type.startsWith("web_search_preview")) return GlobeCode;
  if (type.includes("web_search") || type === "google_search" || type.includes("grounding")) return Globe;
  if (type.includes("code_execution") || type.includes("code_interpreter")) return CodeXml;
  if (type === "shell" || type.includes("bash") || type.includes("terminal")) return SquareTerminal;
  if (type.includes("web_fetch") || type.includes("url_context") || type.includes("fetch")) return Link;
  if (type.includes("image_generation")) return ImagePlus;
  if (type.includes("file_search")) return FileSearch;
  if (type.includes("maps")) return MapPin;
  if (type.includes("computer")) return Monitor;
  if (type.includes("advisor")) return Lightbulb;
  if (type.includes("mcp")) return Plug;
  return Wrench;
}
