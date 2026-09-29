// Public entry of the conversation entity; code outside entities/conversation/ imports only this file.
// Unused re-exports are tree-shaken (package.json "sideEffects"), so UI and logic share one entry.
export { ConversationLabelsDialog } from "@/entities/conversation/components/conversation-labels-dialog";
export {
  ConversationLabelsManagerDialog,
  type ConversationLabelsTarget,
} from "@/entities/conversation/components/conversation-labels-manager-dialog";
export { ConversationLabelsMenuItem } from "@/entities/conversation/components/conversation-labels-menu-item";
export {
  ConversationProjectMenuItems,
  ConversationProjectSubmenu,
} from "@/entities/conversation/components/conversation-project-submenu";
export { ConversationShareDialog } from "@/entities/conversation/components/conversation-share-dialog";
export {
  ConversationShareExportIconDropdown,
  ConversationShareExportSubmenu,
} from "@/entities/conversation/components/conversation-share-export-menu";
export {
  SidebarConversationsProvider,
  useSidebarConversationField,
} from "@/entities/conversation/context/sidebar-conversations-context";
export { useConversationExport } from "@/entities/conversation/hooks/use-conversation-export";
export { sharePatchFromDTO } from "@/entities/conversation/hooks/use-conversation-share-dialog";
export { resolveConversationDefaultModel } from "@/entities/conversation/lib/conversation-default-model";
export { downloadConversationExport } from "@/entities/conversation/lib/conversation-export";
export { parseConversationLabelsJSON } from "@/entities/conversation/lib/conversation-labels";
export {
  conversationMatchesSearch,
  normalizeConversationSearchText,
} from "@/entities/conversation/lib/conversation-search";
export {
  isArchivedConversation,
  mergeUniqueByPublicID,
  removeByPublicID,
  sortByStarredAtDesc,
  sortByUpdatedAtDesc,
  upsertByPublicID,
} from "@/entities/conversation/model/conversation-list";
export type { SidebarConversationChange } from "@/entities/conversation/types/sidebar-conversations";
