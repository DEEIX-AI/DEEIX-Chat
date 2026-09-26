export {
  ChatSessionProvider,
  useChatSession,
  useConversationRunning,
} from "@/features/chat/context/chat-session-context";

// 以下导出是 share（公开分享页）消费的只读消息渲染契约。
// 内部重构时不得破坏这些符号的语义；新增跨 feature 消费必须经本出口，不允许深入内部路径。
export { ChatMessageBot } from "@/features/chat/components/message/message-bot";
export { ChatMessageUser } from "@/features/chat/components/message/message-user";
export {
  buildChildrenIndex,
  buildVisibleMessages,
  mapServerMessage,
  reconcileBranchSelections,
  toBranchKey,
} from "@/features/chat/model/chat-thread";
export type { ChatAreaMessage } from "@/features/chat/types/messages";
