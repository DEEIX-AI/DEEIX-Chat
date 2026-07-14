import type {
  ConversationDTO,
  ConversationProjectDTO,
  CreateConversationProjectRequest,
  UpdateConversationProjectRequest,
} from "@/shared/api/conversation.types";

export type SidebarConversationChange = {
  sequence: number;
  publicID: string;
  type: "upsert" | "patch" | "remove";
  item?: ConversationDTO;
  patch?: Partial<ConversationDTO>;
};

export type DeleteConversationOptions = {
  deleteFiles?: boolean;
};

export type DeleteConversationProjectOptions = {
  deleteConversations?: boolean;
  deleteFiles?: boolean;
};

export type SidebarConversationsControllerValue = {
  items: ConversationDTO[];
  recentItems: ConversationDTO[];
  starredItems: ConversationDTO[];
  projects: ConversationProjectDTO[];
  starredTotal: number;
  loadingInitial: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  loadMoreFailed: boolean;
  transferringStarPublicID: string | null;
  lastChange: SidebarConversationChange | null;
  loadMore: () => Promise<void>;
  retryLoadMore: () => Promise<void>;
  prependNewConversation: (platformModelName?: string, projectID?: string) => Promise<ConversationDTO | null>;
  touchByPublicID: (publicID: string, patch: Partial<ConversationDTO>) => void;
  renameByPublicID: (publicID: string, title: string) => Promise<ConversationDTO | null>;
  regenerateTitleByPublicID: (publicID: string) => Promise<ConversationDTO | null>;
  createProject: (payload: CreateConversationProjectRequest) => Promise<ConversationProjectDTO | null>;
  updateProject: (projectID: string, payload: UpdateConversationProjectRequest) => Promise<ConversationProjectDTO | null>;
  deleteProject: (projectID: string, options?: DeleteConversationProjectOptions) => Promise<boolean>;
  reorderProjects: (projectIDs: string[]) => Promise<void>;
  setProjectByPublicID: (publicID: string, projectID?: string) => Promise<ConversationDTO | null>;
  batchSetProjectByPublicIDs: (publicIDs: string[], projectID?: string) => Promise<number>;
  setStarByPublicID: (publicID: string, starred: boolean) => Promise<ConversationDTO | null>;
  loadAllStarred: () => Promise<ConversationDTO[]>;
  archiveByPublicID: (publicID: string, archived: boolean) => Promise<ConversationDTO | null>;
  deleteByPublicID: (publicID: string, options?: DeleteConversationOptions) => Promise<boolean>;
};
