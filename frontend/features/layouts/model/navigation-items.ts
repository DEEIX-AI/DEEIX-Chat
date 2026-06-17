import { Layers } from "@/components/animate-ui/icons/layers"
import { MessageCircleMore } from "@/components/animate-ui/icons/message-circle-more"
import { PlusIcon } from "@/components/ui/plus"
import { Search } from "@/components/animate-ui/icons/search"
import { Blend } from "@/components/animate-ui/icons/blend"
import type { NavigationItem } from "@/features/layouts/types/navigation"

export const NAVIGATION_ITEMS = [
  {
    id: "newChat",
    title: "New chat",
    url: "#",
    icon: PlusIcon,
    variant: "primary",
    group: "primary",
    shortcut: ["command", "shift", "O"],
  },
  {
    id: "search",
    title: "Search",
    url: "#",
    icon: Search,
    group: "primary",
    shortcut: ["command", "K"],
  },
  {
    id: "recent",
    title: "Recent",
    url: "/recent",
    icon: MessageCircleMore,
    group: "secondary",
  },
  {
    id: "files",
    title: "Files",
    url: "/files",
    icon: Layers,
    group: "secondary",
  },
  {
    id: "skillsPrompt",
    title: "Skills & Prompts",
    url: "/skills-prompt",
    icon: Blend,
    group: "secondary",
  },
] as const satisfies readonly NavigationItem[]
