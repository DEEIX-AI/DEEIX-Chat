import type { ConversationDTO } from "@/shared/api/conversation.types";

export type SidebarConversationChange = {
  sequence: number;
  publicID: string;
  type: "upsert" | "patch" | "remove";
  item?: ConversationDTO;
  patch?: Partial<ConversationDTO>;
};

export type SidebarRecentsControllerValue = {
  items: ConversationDTO[];
  recentItems: ConversationDTO[];
  starredItems: ConversationDTO[];
  starredTotal: number;
  loadingInitial: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  loadMoreFailed: boolean;
  transferringStarPublicID: string | null;
  lastChange: SidebarConversationChange | null;
  loadMore: () => Promise<void>;
  retryLoadMore: () => Promise<void>;
  prependNewConversation: (platformModelName?: string) => Promise<ConversationDTO | null>;
  touchByPublicID: (publicID: string, patch: Partial<ConversationDTO>) => void;
  renameByPublicID: (publicID: string, title: string) => Promise<ConversationDTO | null>;
  setStarByPublicID: (publicID: string, starred: boolean) => Promise<ConversationDTO | null>;
  loadAllStarred: () => Promise<ConversationDTO[]>;
  archiveByPublicID: (publicID: string, archived: boolean) => Promise<ConversationDTO | null>;
  deleteByPublicID: (publicID: string) => Promise<boolean>;
};
