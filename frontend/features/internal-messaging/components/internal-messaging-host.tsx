"use client";

import { useVirtualizer } from "@tanstack/react-virtual";
import {
  Bell,
  BellOff,
  Check,
  ChevronDown,
  ChevronLeft,
  Clipboard,
  CornerUpLeft,
  Download,
  FileIcon,
  LoaderCircle,
  Maximize2,
  MessageCircle,
  Minus,
  Paperclip,
  Pencil,
  Pin,
  Search,
  Send,
  Smile,
  Trash2,
  Volume2,
  VolumeX,
  X,
} from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import * as React from "react";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Textarea } from "@/components/ui/textarea";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import type { CachedConversation } from "@/features/internal-messaging/model/conversation-cache";
import {
  createCachedConversation,
  MAX_CONCURRENT_PREFETCHES,
  readConversationCache,
  writeConversationCache,
} from "@/features/internal-messaging/model/conversation-cache";
import {
  applySelectedConversationLiveEvent,
  mergeMessages,
} from "@/features/internal-messaging/model/live-event-merge";
import { safeInternalMessageMarkdown } from "@/features/internal-messaging/model/message-markdown";
import { messageRowContainmentStyle } from "@/features/internal-messaging/model/message-row-visibility";
import { loadUserPages } from "@/features/internal-messaging/model/user-pages";
import { useInternalMessagingEvents } from "@/features/internal-messaging/components/use-internal-messaging-events";
import { cn } from "@/lib/utils";
import {
  deleteInternalMessagingMessage,
  downloadInternalMessagingFile,
  getInternalMessagingStatus,
  listInternalMessagingMessages,
  listInternalMessagingUsers,
  searchInternalMessagingMessages,
  updateInternalMessagingPreferences,
} from "@/shared/api/internal-messaging";
import type {
  InternalMessagingConversation,
  InternalMessagingMessage,
  InternalMessagingStatus,
  InternalMessagingUser,
} from "@/shared/api/internal-messaging.types";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import { resolveAvatarImageSrc } from "@/shared/lib/avatar";

import { useMessagingWindowLayout, RESIZE_HANDLES } from "./use-messaging-window-layout";
import { InternalMessagingLauncherButton } from "./internal-messaging-launcher-button";
import { useConversationList } from "./use-conversation-list";
import { useMessageReadState } from "./use-message-read-state";
import { useMessageComposer } from "./use-message-composer";
import { shouldSendOnEnter } from "../model/message-composer";
import {
  formatMessageHoverLabel,
  formatTime,
  notificationButtonState,
  type NotificationPermissionState,
} from "../model/message-time";

const LazyInternalMessageMarkdown = React.lazy(async () => {
  const module = await import("./internal-message-markdown");
  return { default: module.InternalMessageMarkdown };
});

type PendingMessageScroll =
  | { mode: "bottom"; behavior: ScrollBehavior }
  | { mode: "preserve"; scrollHeight: number; scrollTop: number };
type MessageFocusRequest = { id: number; sequence: number };
type MessageScrollAnchor = { id: number; offset: number };
class MessageRenderBoundary extends React.Component<
  { children: React.ReactNode; fallback: React.ReactNode },
  { failed: boolean }
> {
  state = { failed: false };

  static getDerivedStateFromError() {
    return { failed: true };
  }

  render() {
    return this.state.failed ? this.props.fallback : this.props.children;
  }
}

const NOTIFICATION_STORAGE_PREFIX = "deeix.internal-messaging.notifications.v1";
const MESSAGE_BOTTOM_THRESHOLD = 80;
const COMMON_EMOJIS = [
  "😀", "😃", "😄", "😁", "😂", "😊", "😍", "🥰",
  "😘", "😎", "🤔", "😅", "😭", "😡", "🥳", "🤩",
  "👍", "👎", "👏", "🙏", "💪", "👌", "✌️", "🤝",
  "❤️", "💔", "🔥", "🎉", "✨", "💯", "✅", "🚀",
];

function initials(value: string) {
  return value.trim().slice(0, 2).toUpperCase() || "?";
}

function displayName(user: InternalMessagingUser) {
  return user.displayName || user.username;
}

const MessagingAvatar = React.memo(function MessagingAvatar({
  user,
  className,
}: {
  user: InternalMessagingUser;
  className?: string;
}) {
  return (
    <Avatar className={className}>
      <AvatarImage src={resolveAvatarImageSrc(user.avatarURL, user) || undefined} alt={displayName(user)} className="object-cover" />
      <AvatarFallback>{initials(displayName(user))}</AvatarFallback>
    </Avatar>
  );
});

export function InternalMessagingWindowHost({
  initiallyOpen = false,
  initialStatus,
}: {
  initiallyOpen?: boolean;
  initialStatus?: InternalMessagingStatus;
}) {
  const t = useTranslations("internalMessaging");
  const locale = useLocale();
  const { accessToken, user } = useAuthSession();
  const [enabled, setEnabled] = React.useState(initialStatus?.enabled || false);
  const [maxFileBytes, setMaxFileBytes] = React.useState(
    initialStatus?.maxFileBytes || 20 * 1024 * 1024,
  );
  const [browserNotificationsAllowed, setBrowserNotificationsAllowed] = React.useState(
    initialStatus?.browserNotifications ?? true,
  );
  const [open, setOpen] = React.useState(initiallyOpen);
  const { mobileLayout, expanded, setExpanded, windowBounds, startWindowDrag, moveWindow, stopWindowDrag, startResize, resizeWindow, stopResize } = useMessagingWindowLayout(user?.publicID || "");
  const [users, setUsers] = React.useState<InternalMessagingUser[]>([]);
  const [conversations, setConversations] = React.useState<InternalMessagingConversation[]>([]);
  const [directoryView, setDirectoryView] = React.useState<"recent" | "users">("recent");
  const [query, setQuery] = React.useState("");
  const [hasMoreUsers, setHasMoreUsers] = React.useState(false);
  const [selected, setSelected] = React.useState<InternalMessagingUser | null>(null);
  const selectedPublicID = selected?.publicID || "";
  const [messages, setMessages] = React.useState<InternalMessagingMessage[]>([]);
  const [loadingUsers, setLoadingUsers] = React.useState(false);
  const [loadingMessages, setLoadingMessages] = React.useState(false);
  const [loadingOlderMessages, setLoadingOlderMessages] = React.useState(false);
  const [hasMoreMessages, setHasMoreMessages] = React.useState(false);
  const [nextBefore, setNextBefore] = React.useState(0);
  const [unreadByUser, setUnreadByUser] = React.useState<Record<string, number>>({});
  const [totalUnread, setTotalUnread] = React.useState(initialStatus?.unreadCount || 0);
  const [messageSearchOpen, setMessageSearchOpen] = React.useState(false);
  const [messageSearchQuery, setMessageSearchQuery] = React.useState("");
  const [messageSearchResults, setMessageSearchResults] = React.useState<InternalMessagingMessage[]>([]);
  const [searchingMessages, setSearchingMessages] = React.useState(false);
  const [notificationsEnabled, setNotificationsEnabled] = React.useState(false);
  const [notificationPermission, setNotificationPermission] =
    React.useState<NotificationPermissionState>("default");
  const [revealedTimestampID, setRevealedTimestampID] = React.useState<number | null>(null);
  const [directoryError, setDirectoryError] = React.useState("");
  const [messageError, setMessageError] = React.useState("");
  const [actionError, setActionError] = React.useState("");
  const [newMessagesBelow, setNewMessagesBelow] = React.useState(false);
  const [messageFocusRequest, setMessageFocusRequest] =
    React.useState<MessageFocusRequest | null>(null);
  const [draggingFile, setDraggingFile] = React.useState(false);
  const [previewMessage, setPreviewMessage] =
    React.useState<InternalMessagingMessage | null>(null);
  const [emojiPickerOpen, setEmojiPickerOpen] = React.useState(false);
  const [connected, setConnected] = React.useState(false);
  const [presenceReady, setPresenceReady] = React.useState(false);
  const [onlineByUser, setOnlineByUser] = React.useState<Record<string, boolean>>({});
  const windowRef = React.useRef<HTMLElement | null>(null);
  const emojiPopoverRef = React.useRef<HTMLDivElement | null>(null);
  const windowOpenRef = React.useRef(open);
  const resumingRef = React.useRef(false);
  const minimizedScrollRef = React.useRef<{ scrollTop: number; nearBottom: boolean } | null>(null);
  const restoringAnchorRef = React.useRef<MessageScrollAnchor | null>(null);
  const selectedRef = React.useRef<InternalMessagingUser | null>(null);
  const directoryPagesRef = React.useRef(1);
  const directoryRequestRef = React.useRef<AbortController | null>(null);
  const fileInputRef = React.useRef<HTMLInputElement | null>(null);
  const composerRef = React.useRef<HTMLTextAreaElement | null>(null);
  const messageViewportRef = React.useRef<HTMLDivElement | null>(null);
  const messagesRef = React.useRef<InternalMessagingMessage[]>([]);
  const reconnectHistoryRef = React.useRef(false);
  const historyRequiredRef = React.useRef(true);
  const hasConnectedRef = React.useRef(false);
  const nearMessageBottomRef = React.useRef(true);
  const pendingMessageScrollRef = React.useRef<PendingMessageScroll | null>(null);
  const messageRequestRef = React.useRef<{
    sequence: number;
    controller: AbortController | null;
  }>({ sequence: 0, controller: null });
  const conversationRefreshTimerRef = React.useRef<number | null>(null);
  const messageRefreshTimerRef = React.useRef<number | null>(null);
  const messageCacheRef = React.useRef<Map<string, CachedConversation>>(new Map());
  const messagePrefetchRef = React.useRef<Map<string, Promise<void>>>(new Map());
  selectedRef.current = selected;
  messagesRef.current = messages;
  windowOpenRef.current = open;

  const scrollToMessageBottom = React.useCallback((behavior: ScrollBehavior = "smooth") => {
    const viewport = messageViewportRef.current;
    if (!viewport || !windowOpenRef.current) return;
    restoringAnchorRef.current = null;
    // Smooth targets drift while virtual rows are being measured.
    viewport.scrollTo({ top: viewport.scrollHeight, behavior: messagesRef.current.length > 120 ? "auto" : behavior });
    nearMessageBottomRef.current = true;
    setNewMessagesBelow(false);
  }, []);

  const maintainMessageBottom = React.useCallback(() => {
    if (nearMessageBottomRef.current && !reconnectHistoryRef.current) scrollToMessageBottom("auto");
  }, [scrollToMessageBottom]);

  const messageByID = React.useMemo(
    () => new Map(messages.map((message) => [message.id, message])),
    [messages],
  );

  React.useEffect(() => {
    if (!selectedPublicID) return;
    writeConversationCache(
      messageCacheRef.current,
      selectedPublicID,
      createCachedConversation(messages, hasMoreMessages, nextBefore),
    );
  }, [hasMoreMessages, messages, nextBefore, selectedPublicID]);

  React.useLayoutEffect(() => {
    const viewport = messageViewportRef.current;
    if (!open || !viewport) return;
    const minimized = minimizedScrollRef.current;
    if (minimized) {
      minimizedScrollRef.current = null;
      pendingMessageScrollRef.current = null;
      nearMessageBottomRef.current = minimized.nearBottom;
      if (!restoringAnchorRef.current) viewport.scrollTop = minimized.scrollTop;
      if (minimized.nearBottom && !reconnectHistoryRef.current) scrollToMessageBottom("auto");
      return;
    }
    const pending = pendingMessageScrollRef.current;
    if (!pending) return;
    pendingMessageScrollRef.current = null;
    if (pending.mode === "preserve") {
      viewport.scrollTop =
        pending.scrollTop + (viewport.scrollHeight - pending.scrollHeight);
      return;
    }
    scrollToMessageBottom(pending.behavior);
  }, [messages, open, scrollToMessageBottom]);

  const handleMessageScroll = (event: React.UIEvent<HTMLDivElement>) => {
    if (!windowOpenRef.current) return;
    const viewport = event.currentTarget;
    const distance = viewport.scrollHeight - viewport.clientHeight - viewport.scrollTop;
    const nearBottom = distance <= MESSAGE_BOTTOM_THRESHOLD;
    nearMessageBottomRef.current = nearBottom;
    if (nearBottom && !reconnectHistoryRef.current) {
      setNewMessagesBelow(false);
      acknowledgeVisibleMessages();
    }
  };

  React.useEffect(() => {
    let disposed = false;
    const refresh = () => {
      void getInternalMessagingStatus(accessToken)
        .then((status) => {
          if (disposed) return;
          setEnabled(status.enabled);
          setTotalUnread(status.unreadCount || 0);
          setMaxFileBytes(status.maxFileBytes || 20 * 1024 * 1024);
          setBrowserNotificationsAllowed(status.browserNotifications);
        })
        .catch(() => {
          // Keep the configured feature and drafts visible during transient outages.
        });
    };
    refresh();
    const timer = window.setInterval(refresh, 30_000);
    window.addEventListener("focus", refresh);
    return () => {
      disposed = true;
      window.clearInterval(timer);
      window.removeEventListener("focus", refresh);
    };
  }, [accessToken]);

  React.useEffect(() => {
    if (enabled) return;
    resumingRef.current = false;
    minimizedScrollRef.current = null;
    restoringAnchorRef.current = null;
    historyRequiredRef.current = true;
    setOpen(false);
    setSelected(null);
    setMessages([]);
    setPreviewMessage(null);
  }, [enabled]);

  React.useEffect(() => {
    try {
      setNotificationsEnabled(window.localStorage.getItem(`${NOTIFICATION_STORAGE_PREFIX}:${user?.publicID}`) === "true");
    } catch { setNotificationsEnabled(false); }
  }, [user?.publicID]);

  React.useEffect(() => {
    if (!("Notification" in window)) {
      setNotificationPermission("unsupported");
      return;
    }
    setNotificationPermission(Notification.permission);
    if (!navigator.permissions?.query) return;
    let disposed = false;
    let status: PermissionStatus | undefined;
    const sync = () => {
      if (!disposed && "Notification" in window) {
        setNotificationPermission(Notification.permission);
      }
    };
    void navigator.permissions
      .query({ name: "notifications" })
      .then((result) => {
        if (disposed) return;
        status = result;
        sync();
        result.addEventListener("change", sync);
      })
      .catch(() => {
        // Safari and some embedded browsers reject the notifications permission query.
      });
    return () => {
      disposed = true;
      status?.removeEventListener("change", sync);
    };
  }, []);

  React.useEffect(() => {
    if (!browserNotificationsAllowed) setNotificationsEnabled(false);
  }, [browserNotificationsAllowed]);

  const conversationList = useConversationList(accessToken, enabled, (result) => {
    setConversations(result.results);
    const profiles = new Map(result.results.map((item) => [item.user.publicID, item.user]));
    setUsers((current) => current.map((item) => profiles.get(item.publicID) || item));
    setTotalUnread(result.totalUnread);
    setUnreadByUser(Object.fromEntries(result.results.map((item) => [item.user.publicID, item.unreadCount])));
  });
  const loadConversations = conversationList.refresh;

  const composer = useMessageComposer(user?.publicID || "", selected?.publicID || "", accessToken,
    (peerID, message) => {
      messageCacheRef.current.delete(peerID);
      if (selectedRef.current?.publicID === peerID) {
        if (windowOpenRef.current) {
          pendingMessageScrollRef.current = { mode: "bottom", behavior: "smooth" };
        } else {
          setNewMessagesBelow(true);
        }
        setMessages((items) => mergeMessages(items, [message]));
      }
      void loadConversations();
    });
  const { draft, setDraft, replyTo, setReplyTo, editing, setEditing, send } = composer;
  const sending = composer.outgoing.some((item) => item.status === "sending" && !item.file);
  const uploading = composer.outgoing.some((item) => item.status === "sending" && item.file);

  const scheduleConversationRefresh = React.useCallback(() => {
    if (conversationRefreshTimerRef.current) return;
    conversationRefreshTimerRef.current = window.setTimeout(() => {
      conversationRefreshTimerRef.current = null;
      void loadConversations();
    }, 150);
  }, [loadConversations]);

  const acknowledgeVisibleMessages = useMessageReadState({
    accessToken, open, enabled: enabled && !reconnectHistoryRef.current && !historyRequiredRef.current,
    peerID: selected?.publicID || "",
    throughMID: messages.reduce((maximum, item) => Math.max(maximum, item.id), 0),
    unreadCount: unreadByUser[selected?.publicID || ""] || 0,
    nearBottom: nearMessageBottomRef, onRead: scheduleConversationRefresh,
  });

  const loadUsers = React.useCallback(
    async (reset = false, more = false) => {
      if (!enabled) return;
      if (!reset && directoryRequestRef.current) return;
      const controller = new AbortController();
      directoryRequestRef.current?.abort();
      directoryRequestRef.current = controller;
      if (reset) directoryPagesRef.current = 1;
      setLoadingUsers(true);
      setDirectoryError("");
      try {
        const { snapshot, loaded } = await loadUserPages(
          (page, signal) => listInternalMessagingUsers(accessToken, query, page, signal),
          directoryPagesRef.current + (more ? 1 : 0), controller.signal,
        );
        if (!snapshot || directoryRequestRef.current !== controller) return;
        directoryPagesRef.current = loaded;
        setUsers(snapshot.results);
        setHasMoreUsers(snapshot.hasMore);
      } catch {
        if (!controller.signal.aborted) setDirectoryError(t("errors.directory"));
      } finally {
        if (directoryRequestRef.current === controller) {
          directoryRequestRef.current = null;
          setLoadingUsers(false);
        }
      }
    },
    [accessToken, enabled, query, t],
  );

  const loadMessages = React.useCallback(
    async (recipientPublicID: string, before = 0) => {
      if (!before) historyRequiredRef.current = true;
      messageRequestRef.current.controller?.abort();
      const controller = new AbortController();
      const sequence = messageRequestRef.current.sequence + 1;
      messageRequestRef.current = { sequence, controller };
      setLoadingMessages(!before);
      setLoadingOlderMessages(Boolean(before));
      if (before) setActionError("");
      else setMessageError("");
      try {
        const page = await listInternalMessagingMessages(
          accessToken,
          recipientPublicID,
          before || undefined,
          controller.signal,
        );
        if (
          sequence !== messageRequestRef.current.sequence ||
          selectedRef.current?.publicID !== recipientPublicID
        ) {
          return;
        }
        const next = mergeMessages(page.results);
        const previous = messagesRef.current;
        const viewport = messageViewportRef.current;
        if (before && viewport) {
          pendingMessageScrollRef.current = {
            mode: "preserve",
            scrollHeight: viewport.scrollHeight,
            scrollTop: viewport.scrollTop,
          };
        } else if (!before) {
          if (previous.length === 0 || nearMessageBottomRef.current) {
            pendingMessageScrollRef.current = { mode: "bottom", behavior: "auto" };
          } else {
            const previousMID = previous.reduce(
              (maximum, item) => Math.max(maximum, item.id),
              0,
            );
            const nextMID = next.reduce((maximum, item) => Math.max(maximum, item.id), 0);
            if (nextMID > previousMID) setNewMessagesBelow(true);
          }
        }
        const merged = before ? mergeMessages(next, previous) : next;
        if (!before) {
          reconnectHistoryRef.current = false;
          historyRequiredRef.current = false;
        }
        setMessages(merged);
        setHasMoreMessages(page.hasMore);
        setNextBefore(page.nextBefore);
        writeConversationCache(
          messageCacheRef.current,
          recipientPublicID,
          createCachedConversation(merged, page.hasMore, page.nextBefore),
        );
      } catch {
        if (controller.signal.aborted || sequence !== messageRequestRef.current.sequence) return;
        if (before) setActionError(t("errors.olderHistory"));
        else setMessageError(t("errors.history"));
      } finally {
        if (sequence === messageRequestRef.current.sequence) {
          messageRequestRef.current.controller = null;
          setLoadingMessages(false);
          setLoadingOlderMessages(false);
        }
      }
    },
    [accessToken, t],
  );

  const prefetchMessages = React.useCallback(
    (recipient: InternalMessagingUser) => {
      const cached = readConversationCache(messageCacheRef.current, recipient.publicID);
      if (cached) return;
      if (messagePrefetchRef.current.has(recipient.publicID)) return;
      if (messagePrefetchRef.current.size >= MAX_CONCURRENT_PREFETCHES) return;
      const request = listInternalMessagingMessages(accessToken, recipient.publicID)
        .then((page) => {
          writeConversationCache(
            messageCacheRef.current,
            recipient.publicID,
            createCachedConversation(
              mergeMessages(page.results),
              page.hasMore,
              page.nextBefore,
            ),
          );
        })
        .catch(() => undefined)
        .finally(() => messagePrefetchRef.current.delete(recipient.publicID));
      messagePrefetchRef.current.set(recipient.publicID, request);
    },
    [accessToken],
  );

  React.useEffect(() => {
    if (!enabled) return;
    const refresh = () => { if (document.visibilityState === "visible") void loadConversations(); };
    refresh();
    const timer = window.setInterval(refresh, 30_000);
    window.addEventListener("focus", refresh);
    return () => {
      window.clearInterval(timer);
      window.removeEventListener("focus", refresh);
    };
  }, [enabled, loadConversations]);
  React.useEffect(() => {
    if (!open || directoryView !== "users") return;
    void loadUsers(true);
    const refresh = () => { if (document.visibilityState === "visible") void loadUsers(); };
    const timer = window.setInterval(refresh, 30_000);
    window.addEventListener("focus", refresh);
    return () => {
      window.clearInterval(timer);
      window.removeEventListener("focus", refresh);
      directoryRequestRef.current?.abort();
      directoryRequestRef.current = null;
    };
  }, [directoryView, loadUsers, open]);
  React.useEffect(() => {
    if (!enabled || !open || !selectedPublicID) return;
    let request: AbortController | null = null;
    let disposed = false;
    const refresh = async () => {
      if (document.visibilityState !== "visible" || request) return;
      const controller = new AbortController();
      request = controller;
      try {
        // Search by stable ID: the peer may be outside loaded pages or no longer match the directory query.
        const result = await listInternalMessagingUsers(accessToken, selectedPublicID, 1, controller.signal);
        if (disposed || controller.signal.aborted) return;
        const profile = result.results.find((item) => item.publicID === selectedPublicID);
        if (profile) {
          setSelected((current) => current?.publicID === profile.publicID ? profile : current);
        }
      } catch {
        // Retain the last known profile during transient failures.
      } finally {
        if (request === controller) request = null;
      }
    };
    void refresh();
    const onFocus = () => { void refresh(); };
    const timer = window.setInterval(onFocus, 30_000);
    window.addEventListener("focus", onFocus);
    return () => {
      disposed = true;
      window.clearInterval(timer);
      window.removeEventListener("focus", onFocus);
      request?.abort();
    };
  }, [accessToken, enabled, open, selectedPublicID]);
  React.useEffect(() => {
    // Profile refreshes must not replace loaded history or move the scroll position.
    if (!open || !selectedPublicID) return;
    if (resumingRef.current) {
      resumingRef.current = false;
      // Live events keep this mounted conversation current while minimized.
      // If the stream had a gap, keep the reading position and offer a history refresh.
      // A canceled initial load still needs history, even if a live message arrived meanwhile.
      if (!historyRequiredRef.current) return;
    }
    void loadMessages(selectedPublicID);
  }, [loadMessages, open, selectedPublicID]);
  React.useEffect(
    () => () => {
      messageRequestRef.current.controller?.abort();
      if (conversationRefreshTimerRef.current) {
        window.clearTimeout(conversationRefreshTimerRef.current);
      }
      if (messageRefreshTimerRef.current) window.clearTimeout(messageRefreshTimerRef.current);
    },
    [],
  );

  useInternalMessagingEvents(enabled, accessToken, (event) => {
    if (
      event.type !== "chat" &&
      event.type !== "users_state" &&
      event.type !== "users_state_changed"
    ) {
      return;
    }
    if (event.type === "users_state" || event.type === "users_state_changed") {
      const states = event.users || [];
      setOnlineByUser((current) => {
        if (event.type === "users_state") {
          return Object.fromEntries(states.map((item) => [item.publicID, item.online]));
        }
        const updated = { ...current };
        for (const item of states) updated[item.publicID] = item.online;
        return updated;
      });
      setPresenceReady(true);
      return;
    }
    const senderPublicID = event.fromUserPublicID;
    if (!senderPublicID) return;
    const recipient = selectedRef.current;
    const peerPublicID = event.conversationPublicID;
    const canonical = event.message;
    const reaction = event.detail?.type === "reaction";
    const incoming = senderPublicID !== user?.publicID;
    const selectedConversation = Boolean(
      recipient && peerPublicID && recipient.publicID === peerPublicID,
    );

    if (canonical && peerPublicID) {
      if (selectedConversation) {
        const decision = applySelectedConversationLiveEvent({
          loaded: messagesRef.current,
          incoming: canonical,
          reaction,
        });
        if (decision.mergedAsTail) {
          if (open && nearMessageBottomRef.current && !reconnectHistoryRef.current) {
            pendingMessageScrollRef.current = { mode: "bottom", behavior: "auto" };
          } else if (incoming) {
            setNewMessagesBelow(true);
          }
        }
        if (decision.changed) {
          setMessages((current) =>
            applySelectedConversationLiveEvent({
              loaded: current,
              incoming: canonical,
              reaction,
            }).messages,
          );
        }
      }
      if ((!open || !selectedConversation) && incoming && !reaction) {
        setUnreadByUser((current) => ({
          ...current,
          [peerPublicID]: Math.min(99, (current[peerPublicID] || 0) + 1),
        }));
        setTotalUnread((current) => current + 1);
      }
      scheduleConversationRefresh();
    } else {
      // Older servers or a transient indexing failure may omit canonical data.
      // Merge an event storm into one bounded history/conversation refresh.
      // Retain the gap even if minimization cancels the delayed fallback refresh.
      if (recipient) {
        reconnectHistoryRef.current = true;
        setNewMessagesBelow(true);
      }
      if (open && recipient && !messageRefreshTimerRef.current) {
        messageRefreshTimerRef.current = window.setTimeout(() => {
          messageRefreshTimerRef.current = null;
          const currentRecipient = selectedRef.current;
          if (windowOpenRef.current && currentRecipient) void loadMessages(currentRecipient.publicID);
        }, 150);
      }
      scheduleConversationRefresh();
      return;
    }

    const readingConversation = open && selectedConversation && document.visibilityState === "visible" && document.hasFocus() && nearMessageBottomRef.current;
    if (!incoming || reaction || readingConversation) return;

    const previousConversation = conversations.find(
      (item) => item.user.publicID === peerPublicID,
    );
    // A missing durable preference is treated as muted. This avoids leaking a
    // notification for the first event of an unknown/muted conversation while
    // the already-debounced conversation refresh catches up.
    if (!previousConversation || previousConversation.muted) return;
    const sender =
      previousConversation.user || users.find((item) => item.publicID === peerPublicID);
    if (
      notificationsEnabled &&
      document.visibilityState !== "visible" &&
      "Notification" in window &&
      Notification.permission === "granted"
    ) {
      new Notification(sender ? displayName(sender) : t("title"), {
        body:
          event.detail?.content_type === "vocechat/file"
            ? t("notifications.file")
            : event.detail?.content || t("notifications.message"),
      });
    }
  }, (connected) => {
    setConnected(connected);
    if (!connected) {
      setPresenceReady(false);
      return;
    }
    if (hasConnectedRef.current) {
      scheduleConversationRefresh();
      const recipient = selectedRef.current;
      if (recipient) {
        reconnectHistoryRef.current = true;
        if (open && nearMessageBottomRef.current && document.visibilityState === "visible") {
          void loadMessages(recipient.publicID);
        } else {
          setNewMessagesBelow(true);
        }
      }
    }
    hasConnectedRef.current = true;
  });

  const selectUser = (next: InternalMessagingUser) => {
    resumingRef.current = false;
    minimizedScrollRef.current = null;
    restoringAnchorRef.current = null;
    reconnectHistoryRef.current = false;
    historyRequiredRef.current = true;
    messageRequestRef.current.controller?.abort();
    messageRequestRef.current = {
      sequence: messageRequestRef.current.sequence + 1,
      controller: null,
    };
    const cached = readConversationCache(messageCacheRef.current, next.publicID);
    setSelected(next);
    setMessages(cached?.messages || []);
    setHasMoreMessages(cached?.hasMore || false);
    setNextBefore(cached?.nextBefore || 0);
    setLoadingMessages(false);
    setLoadingOlderMessages(false);
    setMessageError("");
    setActionError("");
    setNewMessagesBelow(false);
    nearMessageBottomRef.current = true;
    pendingMessageScrollRef.current = cached
      ? { mode: "bottom", behavior: "auto" }
      : null;
    setMessageSearchOpen(false);
    setMessageSearchQuery("");
    setMessageSearchResults([]);
    setMessageFocusRequest(null);
    setRevealedTimestampID(null);
  };

  const notificationState = notificationButtonState({
    adminAllowed: browserNotificationsAllowed,
    permission: notificationPermission,
    enabled: notificationsEnabled,
  });

  const toggleNotifications = async () => {
    if (!user?.publicID || !browserNotificationsAllowed || !("Notification" in window)) return;
    if (Notification.permission === "denied") return;
    const currentlyActive =
      notificationsEnabled && Notification.permission === "granted";
    let next = !currentlyActive;
    if (next && Notification.permission !== "granted") {
      const permission = await Notification.requestPermission();
      setNotificationPermission(permission);
      next = permission === "granted";
    }
    setNotificationsEnabled(next);
    window.localStorage.setItem(
      `${NOTIFICATION_STORAGE_PREFIX}:${user.publicID}`,
      String(next),
    );
  };

  const updateConversationPreference = async (
    conversation: InternalMessagingConversation,
    preferences: { pinned?: boolean; muted?: boolean },
  ) => {
    setActionError("");
    try {
      await updateInternalMessagingPreferences(
        accessToken,
        conversation.user.publicID,
        preferences,
      );
      await loadConversations();
    } catch {
      setActionError(t("errors.preferences"));
    }
  };

  const searchMessages = async () => {
    if (!selected || !messageSearchQuery.trim()) return;
    const recipientPublicID = selected.publicID;
    const searchQuery = messageSearchQuery.trim();
    setSearchingMessages(true);
    setActionError("");
    try {
      const result = await searchInternalMessagingMessages(
        accessToken,
        searchQuery,
        recipientPublicID,
      );
      if (selectedRef.current?.publicID !== recipientPublicID) return;
      setMessageSearchResults(mergeMessages(result.results));
    } catch {
      if (selectedRef.current?.publicID === recipientPublicID) {
        setActionError(t("errors.search"));
      }
    } finally {
      setSearchingMessages(false);
    }
  };

  const minimize = React.useCallback(() => {
    if (!windowOpenRef.current) return;
    const viewport = messageViewportRef.current;
    minimizedScrollRef.current = viewport ? {
      scrollTop: viewport.scrollTop,
      nearBottom: nearMessageBottomRef.current,
    } : null;
    restoringAnchorRef.current = null;
    if (viewport && !nearMessageBottomRef.current) {
      const top = viewport.getBoundingClientRect().top;
      const row = Array.from(viewport.querySelectorAll<HTMLElement>("[data-message-id]"))
        .find((element) => element.getBoundingClientRect().bottom > top);
      if (row) {
        restoringAnchorRef.current = {
          id: Number(row.dataset.messageId),
          offset: row.getBoundingClientRect().top - top,
        };
      }
    }
    resumingRef.current = true;
    windowOpenRef.current = false;
    messageRequestRef.current.controller?.abort();
    messageRequestRef.current = {
      sequence: messageRequestRef.current.sequence + 1,
      controller: null,
    };
    if (messageRefreshTimerRef.current) {
      window.clearTimeout(messageRefreshTimerRef.current);
      messageRefreshTimerRef.current = null;
    }
    setOpen(false);
    setLoadingMessages(false);
    setLoadingOlderMessages(false);
    setEmojiPickerOpen(false);
    setPreviewMessage(null);
    setDraggingFile(false);
    pendingMessageScrollRef.current = null;
  }, []);

  React.useEffect(() => {
    if (!open || mobileLayout || previewMessage) return;
    let startedOutside = false;
    const onPointerDown = (event: PointerEvent) => {
      startedOutside = false;
      if (event.button !== 0 || !event.isPrimary) return;
      const target = event.target;
      if (!(target instanceof Node)) return;
      if (windowRef.current?.contains(target) || emojiPopoverRef.current?.contains(target)) return;
      startedOutside = true;
    };
    const onClick = () => {
      // Wait for a complete outside click so the newly mounted launcher cannot intercept pointerup.
      // An inside-origin drag/resize never qualifies; do not cancel the underlying page action.
      if (startedOutside) minimize();
      startedOutside = false;
    };
    const onPointerCancel = () => { startedOutside = false; };
    document.addEventListener("pointerdown", onPointerDown, true);
    document.addEventListener("click", onClick, true);
    document.addEventListener("pointercancel", onPointerCancel, true);
    return () => {
      document.removeEventListener("pointerdown", onPointerDown, true);
      document.removeEventListener("click", onClick, true);
      document.removeEventListener("pointercancel", onPointerCancel, true);
    };
  }, [minimize, mobileLayout, open, previewMessage]);

  const chooseEdit = (message: InternalMessagingMessage) => {
    setEditing(message);
    setReplyTo(null);
    setDraft(message.content);
  };

  const removeMessage = async (message: InternalMessagingMessage) => {
    if (!window.confirm(t("messages.confirmDelete"))) return;
    const recipientPublicID = selected?.publicID;
    setActionError("");
    try {
      await deleteInternalMessagingMessage(accessToken, message.id);
      if (selectedRef.current?.publicID !== recipientPublicID) return;
      setMessages((items) =>
        mergeMessages(
          items.map((item) =>
            item.id === message.id
              ? { ...item, content: "", file: undefined, deleted: true }
              : item,
          ),
        ),
      );
      if (editing?.id === message.id) {
        setEditing(null);
        setDraft("");
      }
      void loadConversations();
    } catch {
      if (selectedRef.current?.publicID === recipientPublicID) {
        setActionError(t("errors.delete"));
      }
    }
  };

  const uploadFile = async (file: File) => {
    if (!selected || uploading) return;
    if (file.size > maxFileBytes) {
      setActionError(
        t("file.tooLarge", {
          size: formatFileSize(maxFileBytes, locale, t("file.unknownSize")),
        }),
      );
      return;
    }
    await composer.upload(file);
    if (fileInputRef.current) fileInputRef.current.value = "";
  };

  const pasteFile = (event: React.ClipboardEvent<HTMLTextAreaElement>) => {
    if (editing || uploading) return;
    const image = [...event.clipboardData.files].find((file) => file.type.startsWith("image/"));
    if (!image) return;
    event.preventDefault();
    void uploadFile(image);
  };

  const dropFile = (event: React.DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    setDraggingFile(false);
    if (editing || uploading) return;
    const file = event.dataTransfer.files[0];
    if (file) void uploadFile(file);
  };

  const insertEmoji = (emoji: string) => {
    const composer = composerRef.current;
    const start = composer?.selectionStart ?? draft.length;
    const end = composer?.selectionEnd ?? draft.length;
    const next = `${draft.slice(0, start)}${emoji}${draft.slice(end)}`;
    setDraft(next);
    setEmojiPickerOpen(false);
    window.setTimeout(() => {
      composer?.focus();
      composer?.setSelectionRange(start + emoji.length, start + emoji.length);
    });
  };

  const focusSearchResult = async (result: InternalMessagingMessage) => {
    nearMessageBottomRef.current = false;
    if (!messages.some((item) => item.id === result.id) && selected) {
      const recipientPublicID = selected.publicID;
      setActionError("");
      try {
        const page = await listInternalMessagingMessages(
          accessToken,
          recipientPublicID,
          result.id + 1,
        );
        if (selectedRef.current?.publicID !== recipientPublicID) return;
        setMessages((current) => mergeMessages(page.results, current));
      } catch {
        if (selectedRef.current?.publicID === recipientPublicID) {
          setActionError(t("errors.locate"));
        }
        return;
      }
    }
    setMessageSearchOpen(false);
    setMessageFocusRequest((current) => ({ id: result.id, sequence: (current?.sequence || 0) + 1 }));
  };

  if (!enabled || !user) return null;

  return (
    <>
        <aside
          ref={windowRef}
          inert={!open}
          aria-hidden={!open}
          data-messaging-window
          className={cn(
            "fixed z-[70] flex min-h-0 flex-col overflow-hidden bg-background shadow-2xl overscroll-contain",
            mobileLayout ? "inset-0 rounded-none border-0" : "rounded-2xl border",
            !open && "invisible pointer-events-none",
          )}
          style={
            mobileLayout
              ? { width: "100%", height: "100dvh" }
              : expanded ? { left: "5vw", top: "5dvh", width: "90vw", height: "90dvh" } : {
                  left: windowBounds.x,
                  top: windowBounds.y,
                  width: windowBounds.width,
                  height: windowBounds.height,
                }
          }
        >
          <header
            className={cn(
              "flex min-h-14 shrink-0 select-none items-center gap-2 border-b px-3",
              mobileLayout
                ? "cursor-default touch-auto pt-[env(safe-area-inset-top)]"
                : "h-14 cursor-move touch-none",
            )}
            onPointerDown={expanded ? undefined : startWindowDrag}
            onPointerMove={moveWindow}
            onPointerUp={stopWindowDrag}
            onPointerCancel={stopWindowDrag}
          >
            {selected ? (
              <HeaderIconButton label={t("aria.back")} onClick={() => setSelected(null)}>
                <ChevronLeft />
              </HeaderIconButton>
            ) : (
              <MessageCircle className="size-4 text-primary" />
            )}
            {selected ? <MessagingAvatar user={selected} /> : null}
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-semibold">
                {selected ? displayName(selected) : t("title")}
              </p>
              <p className="truncate text-[11px] text-muted-foreground">
                {selected
                  ? presenceReady
                    ? `${selected.username} · ${t(onlineByUser[selected.publicID] ? "presence.online" : "presence.offline")}`
                    : selected.username
                  : t(mobileLayout ? "mobileHint" : "dragHint")}
              </p>
            </div>
            {!mobileLayout ? (
              <HeaderIconButton
                label={t(expanded ? "aria.restore" : "aria.expand")}
                onClick={() => setExpanded((value) => !value)}
              >
                <Maximize2 />
              </HeaderIconButton>
            ) : null}
            {selected ? (
              <HeaderIconButton
                label={t("aria.search")}
                onClick={() => setMessageSearchOpen((current) => !current)}
              >
                <Search />
              </HeaderIconButton>
            ) : null}
            <HeaderIconButton
              label={t(`aria.${notificationState.tooltipKey}`)}
              disabled={notificationState.disabled}
              onClick={() => void toggleNotifications()}
            >
              {notificationState.active ? <Bell /> : <BellOff />}
            </HeaderIconButton>
            <HeaderIconButton label={t("aria.minimize")} onClick={minimize}>
              <Minus />
            </HeaderIconButton>
          </header>

          {!connected ? <p role="status" className="border-b bg-muted px-3 py-2 text-xs">{t("connection.reconnecting")}</p> : null}
          {actionError ? (
            <div
              role="alert"
              className="flex shrink-0 items-center gap-2 border-b border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive"
            >
              <span className="min-w-0 flex-1">{actionError}</span>
              <button
                type="button"
                aria-label={t("aria.dismissError")}
                className="rounded p-0.5 hover:bg-destructive/10"
                onClick={() => setActionError("")}
              >
                <X className="size-3" />
              </button>
            </div>
          ) : null}

          {selected ? (
            <div
              className="relative flex min-h-0 flex-1 flex-col"
              onDragEnter={(event) => {
                if (!event.dataTransfer.types.includes("Files")) return;
                event.preventDefault();
                setDraggingFile(true);
              }}
              onDragOver={(event) => {
                if (!event.dataTransfer.types.includes("Files")) return;
                event.preventDefault();
                event.dataTransfer.dropEffect = "copy";
              }}
              onDragLeave={(event) => {
                if (event.currentTarget.contains(event.relatedTarget as Node)) return;
                setDraggingFile(false);
              }}
              onDrop={dropFile}
            >
              {messageSearchOpen ? (
                <div className="border-b p-2">
                  <div className="flex gap-2">
                    <Input
                      value={messageSearchQuery}
                      onChange={(event) => setMessageSearchQuery(event.target.value)}
                      onKeyDown={(event) => {
                        if (event.key === "Enter") void searchMessages();
                      }}
                      placeholder={t("search.placeholder")}
                    />
                    <Button
                      size="sm"
                      disabled={!messageSearchQuery.trim() || searchingMessages}
                      onClick={() => void searchMessages()}
                    >
                      {searchingMessages ? (
                        <LoaderCircle className="animate-spin" />
                      ) : (
                        t("search.action")
                      )}
                    </Button>
                  </div>
                  {messageSearchResults.length > 0 ? (
                    <div className="mt-2 max-h-32 space-y-1 overflow-y-auto">
                      {messageSearchResults.map((item) => (
                        <button
                          type="button"
                          key={item.id}
                          className="block w-full truncate rounded px-2 py-1 text-left text-xs hover:bg-accent"
                          onClick={() => void focusSearchResult(item)}
                        >
                          {item.content}
                        </button>
                      ))}
                    </div>
                  ) : null}
                </div>
              ) : null}
              <div
                ref={messageViewportRef}
                className="min-h-0 flex-1 space-y-3 overflow-y-auto p-3 overscroll-contain"
                onScroll={handleMessageScroll}
                onPointerDownCapture={() => { restoringAnchorRef.current = null; }}
                onWheelCapture={() => { restoringAnchorRef.current = null; }}
                onKeyDownCapture={() => { restoringAnchorRef.current = null; }}
              >
                {messageError ? (
                  <div
                    role="alert"
                    className="flex items-center justify-between gap-2 rounded-md bg-destructive/10 px-2 py-1.5 text-xs text-destructive"
                  >
                    <span>{messageError}</span>
                    <Button
                      type="button"
                      size="sm"
                      variant="ghost"
                      disabled={loadingMessages}
                      onClick={() => void loadMessages(selected.publicID)}
                    >
                      {t("actions.retryLoad")}
                    </Button>
                  </div>
                ) : null}
                {hasMoreMessages ? (
                  <Button
                    variant="ghost"
                    size="sm"
                    className="w-full"
                    disabled={loadingOlderMessages}
                    onClick={() => void loadMessages(selected.publicID, nextBefore)}
                  >
                    {loadingOlderMessages ? t("messages.loading") : t("messages.loadOlder")}
                  </Button>
                ) : null}
                {loadingMessages && messages.length === 0 ? (
                  <Loading />
                ) : messages.length === 0 ? (
                  composer.outgoing.length === 0 && !messageError ? <Empty label={t("messages.empty")} /> : null
                ) : (
                  <MessageRows
                    active={open}
                    followBottom={open && !reconnectHistoryRef.current}
                    messages={messages}
                    viewportRef={messageViewportRef}
                    followBottomRef={nearMessageBottomRef}
                    restoringAnchorRef={restoringAnchorRef}
                    focusRequest={messageFocusRequest}
                  >
                    {(message) => {
                    const mine = message.fromUserPublicID === user.publicID;
                    const hoverTime = formatMessageHoverLabel(
                      message.createdAt,
                      locale,
                      {
                        yesterday: t("time.yesterday"),
                        edited: t("messages.edited"),
                      },
                      Boolean(message.editedAt && !message.deleted),
                    );
                    return (
                      <div
                        id={`internal-message-${message.id}`}
                        data-message-id={message.id}
                        className={cn("flex w-full items-start gap-2", mine ? "justify-end" : "justify-start")}
                        style={messageRowContainmentStyle({
                          loadingOlderMessages,
                          preservingOlderScroll:
                            pendingMessageScrollRef.current?.mode === "preserve",
                        })}
                      >
                        {!mine ? <MessagingAvatar user={selected} className="size-7 text-xs" /> : null}
                        <div
                          className="group/message relative max-w-[min(85%,calc(100%-9rem))]"
                          onPointerUp={(event) => {
                            if (event.pointerType === "mouse") return;
                            if ((event.target as HTMLElement).closest("button, a")) return;
                            setRevealedTimestampID((current) =>
                              current === message.id ? null : message.id,
                            );
                          }}
                        >
                          <div
                            className={cn(
                              "rounded-2xl px-3 py-2 text-sm",
                              mine ? "bg-primary text-primary-foreground" : "bg-muted",
                            )}
                          >
                          {message.replyToID > 0 ? (
                            <div
                              className={cn(
                                "mb-1 border-l-2 pl-2 text-xs opacity-75",
                                mine ? "border-primary-foreground/50" : "border-foreground/30",
                              )}
                            >
                              {messageByID.get(message.replyToID)?.content ||
                                t("messages.olderReply")}
                            </div>
                          ) : null}
                          <MessageRenderBoundary
                            key={`${message.id}:${message.editedAt}:${message.deleted}`}
                            fallback={
                              <p className="italic opacity-70">{t("messages.renderError")}</p>
                            }
                          >
                            {message.deleted ? (
                              <p className="italic opacity-70">{t("messages.deleted")}</p>
                            ) : message.file ? (
                              <MessageFile
                                accessToken={accessToken}
                                message={message}
                                onError={() => setActionError(t("errors.fileDownload"))}
                                onContentResize={maintainMessageBottom}
                                onPreview={() => setPreviewMessage(message)}
                              />
                            ) : (
                              <React.Suspense
                                fallback={
                                  <p className="whitespace-pre-wrap break-words text-sm">
                                    {message.content}
                                  </p>
                                }
                              >
                                <LazyInternalMessageMarkdown
                                  content={safeInternalMessageMarkdown(message.content)}
                                className={cn(
                                  "break-words text-sm [&_a]:underline [&_a]:underline-offset-2 [&_p]:my-0",
                                  mine &&
                                    "text-primary-foreground [&_a]:text-primary-foreground",
                                )}
                                />
                              </React.Suspense>
                            )}
                          </MessageRenderBoundary>
                          </div>
                          {hoverTime ? (
                            <time
                              dateTime={message.createdAt}
                              className={cn(
                                "pointer-events-none absolute bottom-1 max-w-[6.75rem] text-[10px] leading-tight text-muted-foreground transition-opacity",
                                revealedTimestampID === message.id
                                  ? "opacity-100"
                                  : "opacity-0 group-hover/message:opacity-100 group-focus-within/message:opacity-100",
                                mine
                                  ? "right-full mr-1.5 text-right"
                                  : "left-full ml-1.5 text-left",
                              )}
                            >
                              {hoverTime}
                            </time>
                          ) : null}
                          {!message.deleted ? (
                            <span
                            className={cn(
                                "absolute -top-3 hidden items-center rounded-full border bg-background text-foreground shadow-sm group-hover/message:flex group-focus-within/message:flex max-sm:flex",
                                mine ? "right-1" : "left-1",
                            )}
                          >
                              {message.content ? (
                                <MessageAction label={t("messages.copy")} onClick={() => void navigator.clipboard.writeText(message.content)}>
                                  <Clipboard className="size-3" />
                                </MessageAction>
                              ) : null}
                              <MessageAction label={t("messages.reply")} onClick={() => { setReplyTo(message); setEditing(null); }}>
                                <CornerUpLeft className="size-3" />
                              </MessageAction>
                              {mine && !message.file ? (
                                <MessageAction label={t("messages.edit")} onClick={() => chooseEdit(message)}>
                                  <Pencil className="size-3" />
                                </MessageAction>
                              ) : null}
                              {mine ? (
                                <MessageAction label={t("messages.delete")} onClick={() => void removeMessage(message)}>
                                  <Trash2 className="size-3" />
                                </MessageAction>
                              ) : null}
                            </span>
                          ) : null}
                        </div>
                        {mine ? <MessagingAvatar user={user} className="size-7 text-xs" /> : null}
                      </div>
                    );
                    }}
                  </MessageRows>
                )}
                {composer.outgoing.map((item) => (
                  <div key={item.id} className="flex items-start justify-end gap-2">
                    <div className="max-w-[min(85%,calc(100%-9rem))] rounded-2xl bg-primary/10 px-3 py-2 text-sm">
                      <p className="whitespace-pre-wrap break-words">{item.content}</p>
                      <div className="mt-1 flex items-center gap-2 text-xs" role="status">
                        <span>{t(item.status === "sending" ? "messages.sending" : "messages.failed")}</span>
                        {item.status === "failed" ? <>
                          <Button size="sm" variant="ghost" onClick={() => void composer.retry(item.id)}>{t("messages.retry")}</Button>
                          <Button size="sm" variant="ghost" onClick={() => composer.discard(item.id)}>{t("messages.discard")}</Button>
                        </> : null}
                      </div>
                    </div>
                    <MessagingAvatar user={user} className="size-7 text-xs" />
                  </div>
                ))}
              </div>
              {newMessagesBelow ? (
                <Button
                  type="button"
                  size="sm"
                  variant="secondary"
                  className="absolute bottom-20 left-1/2 z-10 -translate-x-1/2 rounded-full shadow-lg"
                  onClick={() => {
                    if ((reconnectHistoryRef.current || historyRequiredRef.current) && selected) {
                      nearMessageBottomRef.current = true;
                      void loadMessages(selected.publicID);
                    } else {
                      scrollToMessageBottom("smooth");
                      acknowledgeVisibleMessages();
                    }
                  }}
                >
                  <ChevronDown className="size-3" />
                  {t("messages.newBelow")}
                </Button>
              ) : null}
              <div className="border-t p-3 pb-[max(0.75rem,env(safe-area-inset-bottom))]">
                {[...draft.trim()].length > 4000 ? <p role="alert" className="text-xs text-destructive">{t("composer.tooLong")}</p> : null}
                {replyTo || editing ? (
                  <div className="mb-2 flex items-center gap-2 rounded-md bg-muted px-2 py-1 text-xs">
                    {editing ? <Pencil className="size-3" /> : <CornerUpLeft className="size-3" />}
                    <span className="min-w-0 flex-1 truncate">
                      {editing
                        ? t("composer.editing")
                        : t("composer.replying", {
                            content:
                              replyTo?.content || replyTo?.file?.name || t("composer.file"),
                          })}
                    </span>
                    <button
                      type="button"
                      aria-label={t("aria.cancelAction")}
                      onClick={() => {
                        setReplyTo(null);
                        setEditing(null);
                        if (editing) setDraft("");
                      }}
                    >
                      <X className="size-3" />
                    </button>
                  </div>
                ) : null}
                <div className="flex items-end gap-2">
                  <input
                    ref={fileInputRef}
                    type="file"
                    className="hidden"
                    onChange={(event) => {
                      const file = event.target.files?.[0];
                      if (file) void uploadFile(file);
                    }}
                  />
                  <Button
                    aria-label={t("aria.attach")}
                    type="button"
                    variant="ghost"
                    size="icon"
                    disabled={uploading || sending || Boolean(editing)}
                    onClick={() => fileInputRef.current?.click()}
                  >
                    {uploading ? <LoaderCircle className="animate-spin" /> : <Paperclip />}
                  </Button>
                  <Popover open={open && emojiPickerOpen} onOpenChange={setEmojiPickerOpen}>
                    <PopoverTrigger asChild>
                      <Button
                        aria-label={t("aria.emoji")}
                        type="button"
                        variant="ghost"
                        size="icon"
                      >
                        <Smile />
                      </Button>
                    </PopoverTrigger>
                    <PopoverContent
                      ref={emojiPopoverRef}
                      onCloseAutoFocus={(event) => {
                        if (!windowOpenRef.current) event.preventDefault();
                      }}
                      side="top"
                      align="start"
                      className="z-[80] grid w-64 grid-cols-8 gap-1 p-2"
                    >
                      {COMMON_EMOJIS.map((emoji) => (
                        <button
                          type="button"
                          key={emoji}
                          className="flex size-7 items-center justify-center rounded text-lg hover:bg-accent"
                          onClick={() => insertEmoji(emoji)}
                        >
                          {emoji}
                        </button>
                      ))}
                    </PopoverContent>
                  </Popover>
                  <Textarea
                    ref={composerRef}
                    value={draft}
                    onChange={(event) => setDraft(event.target.value)}
                    onPaste={pasteFile}
                    onKeyDown={(event) => {
                      if (shouldSendOnEnter({ key: event.key, shiftKey: event.shiftKey, isComposing: event.nativeEvent.isComposing, keyCode: event.nativeEvent.keyCode })) {
                        event.preventDefault();
                        void send();
                      }
                    }}
                    aria-label={t("composer.placeholder")}
                    placeholder={t("composer.placeholder")}
                    className="max-h-28 min-h-10 resize-none text-base sm:text-sm"
                  />
                  <Button
                    aria-label={t("aria.send")}
                    size="icon"
                    disabled={!draft.trim() || [...draft.trim()].length > 4000}
                    onClick={() => void send()}
                  >
                    {sending ? <LoaderCircle className="animate-spin" /> : editing ? <Check /> : <Send />}
                  </Button>
                </div>
              </div>
              {draggingFile ? (
                <div className="pointer-events-none absolute inset-2 z-20 flex items-center justify-center rounded-xl border-2 border-dashed border-primary bg-background/90 text-sm font-medium text-primary backdrop-blur-sm">
                  {t("composer.dropFile")}
                </div>
              ) : null}
            </div>
          ) : (
            <div className="flex min-h-0 flex-1 flex-col">
              <div className="space-y-2 border-b p-3">
                <div className="grid grid-cols-2 rounded-lg bg-muted p-1">
                  <button
                    type="button"
                    className={cn(
                      "rounded-md px-2 py-1 text-xs",
                      directoryView === "recent" && "bg-background font-medium shadow-sm",
                    )}
                    onClick={() => setDirectoryView("recent")}
                  >
                    {t("directory.recent")}
                  </button>
                  <button
                    type="button"
                    className={cn(
                      "rounded-md px-2 py-1 text-xs",
                      directoryView === "users" && "bg-background font-medium shadow-sm",
                    )}
                    onClick={() => setDirectoryView("users")}
                  >
                    {t("directory.allUsers")}
                  </button>
                </div>
                {directoryView === "users" ? (
                  <div className="relative">
                    <Search className="pointer-events-none absolute left-2.5 top-2 size-3.5 text-muted-foreground" />
                    <Input
                      value={query}
                      onChange={(event) => setQuery(event.target.value)}
                      placeholder={t("directory.searchPlaceholder")}
                      className="pl-8"
                    />
                  </div>
                ) : null}
              </div>
              <div className="min-h-0 flex-1 overflow-y-auto p-2">
                {directoryView === "recent" && conversationList.failed ? <p role="alert" className="text-xs text-destructive">{t("errors.directory")}<Button variant="ghost" onClick={() => void loadConversations()}>{t("messages.retry")}</Button></p> : null}
                {directoryView === "recent" ? (
                  conversationList.loading && conversations.length === 0 ? <Loading /> : conversations.length === 0 ? (
                    <Empty label={t("directory.noRecent")} />
                  ) : (
                    conversations.map((conversation) => {
                      const item = conversation.user;
                      const unread = unreadByUser[item.publicID] ?? conversation.unreadCount;
                      return (
                        <div
                          key={item.publicID}
                          className="group/conversation flex items-center rounded-lg hover:bg-accent"
                        >
                          <button
                            type="button"
                            className="flex min-w-0 flex-1 items-center gap-3 p-2 text-left"
                            onClick={() => selectUser(item)}
                            onPointerEnter={() => prefetchMessages(item)}
                            onFocus={() => prefetchMessages(item)}
                            onTouchStart={() => prefetchMessages(item)}
                          >
                            <span className="relative shrink-0">
                              <MessagingAvatar user={item} />
                              {presenceReady ? (
                                <PresenceIndicator
                                  online={Boolean(onlineByUser[item.publicID])}
                                  label={t(
                                    onlineByUser[item.publicID]
                                      ? "presence.online"
                                      : "presence.offline",
                                  )}
                                />
                              ) : null}
                            </span>
                            <span className="min-w-0 flex-1">
                              <span className="flex items-center gap-1">
                                <span className="truncate text-sm font-medium">
                                  {displayName(item)}
                                </span>
                                {conversation.pinned ? (
                                  <IconTooltip label={t("aria.pinned")}>
                                    <span
                                      className="inline-flex"
                                      role="img"
                                      aria-label={t("aria.pinned")}
                                    >
                                      <Pin className="size-3 text-primary" />
                                    </span>
                                  </IconTooltip>
                                ) : null}
                                {conversation.muted ? (
                                  <IconTooltip label={t("aria.muted")}>
                                    <span
                                      className="inline-flex"
                                      role="img"
                                      aria-label={t("aria.muted")}
                                    >
                                      <VolumeX className="size-3 text-muted-foreground" />
                                    </span>
                                  </IconTooltip>
                                ) : null}
                              </span>
                              <span className="block truncate text-xs text-muted-foreground">
                                {conversation.lastMessagePreview || t("directory.start")}
                              </span>
                            </span>
                            <span className="flex shrink-0 flex-col items-end gap-1">
                              <span className="text-[10px] text-muted-foreground">
                                {formatTime(conversation.lastMessageAt, locale)}
                              </span>
                              {unread > 0 ? (
                                <span className="flex min-w-5 items-center justify-center rounded-full bg-destructive px-1.5 py-0.5 text-[10px] font-medium text-destructive-foreground">
                                  {unread > 99 ? "99+" : unread}
                                </span>
                              ) : null}
                            </span>
                          </button>
                          <span className="mr-1 hidden shrink-0 group-hover/conversation:flex group-focus-within/conversation:flex max-sm:flex">
                            <IconTooltip
                              label={conversation.pinned ? t("aria.unpin") : t("aria.pin")}
                            >
                              <button
                                type="button"
                                aria-label={
                                  conversation.pinned ? t("aria.unpin") : t("aria.pin")
                                }
                                className="rounded p-1 hover:bg-background"
                                onClick={() =>
                                  void updateConversationPreference(conversation, {
                                    pinned: !conversation.pinned,
                                  })
                                }
                              >
                                <Pin className="size-3" />
                              </button>
                            </IconTooltip>
                            <IconTooltip
                              label={conversation.muted ? t("aria.unmute") : t("aria.mute")}
                            >
                              <button
                                type="button"
                                aria-label={
                                  conversation.muted ? t("aria.unmute") : t("aria.mute")
                                }
                                className="rounded p-1 hover:bg-background"
                                onClick={() =>
                                  void updateConversationPreference(conversation, {
                                    muted: !conversation.muted,
                                  })
                                }
                              >
                                {conversation.muted ? (
                                  <Volume2 className="size-3" />
                                ) : (
                                  <VolumeX className="size-3" />
                                )}
                              </button>
                            </IconTooltip>
                          </span>
                        </div>
                      );
                    })
                  )
                ) : loadingUsers && users.length === 0 ? (
                  <Loading />
                ) : users.length === 0 ? (
                  <Empty label={directoryError || t("directory.noUsers")} />
                ) : (
                  <>
                    {directoryError ? (
                      <p
                        role="alert"
                        className="mb-2 rounded-md bg-destructive/10 px-2 py-1.5 text-xs text-destructive"
                      >
                        {directoryError}
                      </p>
                    ) : null}
                    {users.map((item) => {
                      const unread = unreadByUser[item.publicID] || 0;
                      return (
                        <button
                          type="button"
                          key={item.publicID}
                          className="flex w-full items-center gap-3 rounded-lg p-2 text-left hover:bg-accent"
                          onClick={() => selectUser(item)}
                          onPointerEnter={() => prefetchMessages(item)}
                          onFocus={() => prefetchMessages(item)}
                          onTouchStart={() => prefetchMessages(item)}
                        >
                          <span className="relative shrink-0">
                            <MessagingAvatar user={item} />
                            {presenceReady ? (
                              <PresenceIndicator
                                online={Boolean(onlineByUser[item.publicID])}
                                label={t(
                                  onlineByUser[item.publicID]
                                    ? "presence.online"
                                    : "presence.offline",
                                )}
                              />
                            ) : null}
                          </span>
                          <span className="min-w-0 flex-1">
                            <span className="block truncate text-sm font-medium">
                              {displayName(item)}
                            </span>
                            <span className="block truncate text-xs text-muted-foreground">
                              {item.username}
                            </span>
                          </span>
                          {unread > 0 ? (
                            <span className="flex min-w-5 shrink-0 items-center justify-center rounded-full bg-destructive px-1.5 py-0.5 text-[10px] font-medium text-destructive-foreground">
                              {unread > 99 ? "99+" : unread}
                            </span>
                          ) : null}
                        </button>
                      );
                    })}
                    {hasMoreUsers ? (
                      <Button
                        variant="ghost"
                        className="mt-1 w-full"
                        disabled={loadingUsers}
                        onClick={() => void loadUsers(false, true)}
                      >
                        {loadingUsers ? t("messages.loading") : t("directory.loadMore")}
                      </Button>
                    ) : null}
                  </>
                )}
                {directoryView === "recent" && conversationList.hasMore ? <Button variant="ghost" className="w-full" disabled={conversationList.loading} onClick={() => void loadConversations(true)}>{t(conversationList.loading ? "messages.loading" : "directory.loadMoreConversations")}</Button> : null}
              </div>
            </div>
          )}

          {!mobileLayout && !expanded
            ? RESIZE_HANDLES.map(({ direction, className }) => (
                <button
                  aria-label={t("aria.resize", { direction })}
                  type="button"
                  tabIndex={-1}
                  key={direction}
                  data-direction={direction}
                  className={cn(
                    "absolute z-10 touch-none border-0 bg-transparent p-0",
                    className,
                  )}
                  onPointerDown={startResize}
                  onPointerMove={resizeWindow}
                  onPointerUp={stopResize}
                  onPointerCancel={stopResize}
                />
              ))
            : null}
        </aside>

      {open && previewMessage ? (
        <ImageLightbox
          accessToken={accessToken}
          message={previewMessage}
          onClose={() => setPreviewMessage(null)}
          onError={() => setActionError(t("errors.fileDownload"))}
        />
      ) : null}

      {!open ? (
        <InternalMessagingLauncherButton
          key={user.publicID}
          accountID={user.publicID}
          unreadCount={totalUnread}
          label={t("aria.open")}
          onOpen={() => setOpen(true)}
        />
      ) : null}
    </>
  );
}

function MessageRows({
  active,
  followBottom,
  messages,
  viewportRef,
  followBottomRef,
  restoringAnchorRef,
  focusRequest,
  children,
}: {
  active: boolean;
  followBottom: boolean;
  messages: InternalMessagingMessage[];
  viewportRef: React.RefObject<HTMLDivElement | null>;
  followBottomRef: React.RefObject<boolean>;
  restoringAnchorRef: React.RefObject<MessageScrollAnchor | null>;
  focusRequest: MessageFocusRequest | null;
  children: (message: InternalMessagingMessage) => React.ReactNode;
}) {
  const enabled = messages.length > 120;
  const virtualizer = useVirtualizer({
    count: enabled ? messages.length : 0,
    enabled,
    estimateSize: () => 76,
    getScrollElement: () => viewportRef.current,
    getItemKey: (index) => messages[index]?.id || index,
    overscan: 12,
  });

  const totalSize = virtualizer.getTotalSize();
  const virtualRows = virtualizer.getVirtualItems();
  React.useLayoutEffect(() => {
    // Estimated heights change again after rows mount. Follow those measurements
    // while at the tail, including the transition into virtual rendering.
    if (followBottom && enabled && followBottomRef.current && messages.length) {
      virtualizer.scrollToIndex(messages.length - 1, { align: "end", behavior: "auto" });
    }
  }, [enabled, followBottom, followBottomRef, focusRequest, messages.length, totalSize, virtualizer]);

  React.useLayoutEffect(() => {
    const anchor = restoringAnchorRef.current;
    const viewport = viewportRef.current;
    if (!active || !anchor || !viewport) return;
    const row = viewport.querySelector<HTMLElement>(`[data-message-id="${anchor.id}"]`);
    if (row) {
      const delta = row.getBoundingClientRect().top - viewport.getBoundingClientRect().top - anchor.offset;
      if (Math.abs(delta) > 1) {
        if (enabled) virtualizer.scrollToOffset(viewport.scrollTop + delta);
        else viewport.scrollTop += delta;
      }
    } else if (enabled) {
      const index = messages.findIndex((message) => message.id === anchor.id);
      if (index < 0) return;
      const offset = virtualizer.getOffsetForIndex(index, "start");
      if (offset) virtualizer.scrollToOffset(offset[0] - anchor.offset);
    }
    // Keep correcting while virtual rows are measured, until the reader interacts again.
  }, [active, enabled, messages, restoringAnchorRef, totalSize, viewportRef, virtualizer, virtualRows]);

  const handledFocus = React.useRef<number | undefined>(undefined);
  React.useLayoutEffect(() => {
    if (!active || !focusRequest || handledFocus.current === focusRequest.sequence) return;
    const index = messages.findIndex((message) => message.id === focusRequest.id);
    if (index < 0) return;
    restoringAnchorRef.current = null;
    handledFocus.current = focusRequest.sequence;
    if (enabled) {
      virtualizer.scrollToIndex(index, { align: "center", behavior: "smooth" });
      return;
    }
    document.getElementById(`internal-message-${focusRequest.id}`)?.scrollIntoView({
      behavior: "smooth",
      block: "center",
    });
  }, [active, enabled, focusRequest, messages, restoringAnchorRef, virtualizer]);

  if (!enabled) {
    return <div className="space-y-3">{messages.map((message) => <React.Fragment key={message.id}>{children(message)}</React.Fragment>)}</div>;
  }

  return (
    <div className="relative w-full" style={{ height: totalSize }}>
      {virtualRows.map((virtualRow) => {
        const message = messages[virtualRow.index];
        if (!message) return null;
        return (
          <div
            key={message.id}
            ref={virtualizer.measureElement}
            data-index={virtualRow.index}
            className="absolute left-0 top-0 w-full pb-3"
            style={{ transform: `translateY(${virtualRow.start}px)` }}
          >
            {children(message)}
          </div>
        );
      })}
    </div>
  );
}

function Loading() {
  const t = useTranslations("internalMessaging");
  return (
    <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
      <LoaderCircle className="mr-2 size-4 animate-spin" />
      {t("messages.loading")}
    </div>
  );
}

function Empty({ label }: { label: string }) {
  return (
    <div className="flex h-full items-center justify-center p-8 text-center text-xs text-muted-foreground">
      {label}
    </div>
  );
}

function PresenceIndicator({ online, label }: { online: boolean; label: string }) {
  return (
    <span
      role="img"
      aria-label={label}
      title={label}
      className={cn(
        "absolute bottom-0 right-0 size-3 rounded-full border-2 border-background",
        online ? "bg-emerald-500" : "bg-muted-foreground/50",
      )}
    />
  );
}

function HeaderIconButton({
  label,
  disabled,
  onClick,
  children,
}: {
  label: string;
  disabled?: boolean;
  onClick?: () => void;
  children: React.ReactNode;
}) {
  return (
    <Tooltip delayDuration={200}>
      <TooltipTrigger asChild>
        <span className="inline-flex">
          <Button
            aria-label={label}
            variant="ghost"
            size="icon-sm"
            disabled={disabled}
            onClick={onClick}
          >
            {children}
          </Button>
        </span>
      </TooltipTrigger>
      <TooltipContent className="pointer-events-none z-[80]" side="bottom" sideOffset={6}>
        {label}
      </TooltipContent>
    </Tooltip>
  );
}

function IconTooltip({
  label,
  children,
}: {
  label: string;
  children: React.ReactElement;
}) {
  return (
    <Tooltip delayDuration={200}>
      <TooltipTrigger asChild>{children}</TooltipTrigger>
      <TooltipContent className="pointer-events-none z-[80]" side="top" sideOffset={6}>
        {label}
      </TooltipContent>
    </Tooltip>
  );
}

function MessageAction({
  label,
  onClick,
  children,
}: {
  label: string;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      className="rounded-full p-1.5 hover:bg-accent"
      onClick={onClick}
    >
      {children}
    </button>
  );
}

function MessageFile({
  accessToken,
  message,
  onError,
  onContentResize,
  onPreview,
}: {
  accessToken: string;
  message: InternalMessagingMessage;
  onError: () => void;
  onContentResize: () => void;
  onPreview: () => void;
}) {
  const t = useTranslations("internalMessaging");
  const locale = useLocale();
  const [imageURL, setImageURL] = React.useState("");
  const [shouldLoadImage, setShouldLoadImage] = React.useState(false);
  const containerRef = React.useRef<HTMLDivElement | null>(null);
  const file = message.file;
  const onErrorRef = React.useRef(onError);
  onErrorRef.current = onError;

  React.useEffect(() => {
    if (!file?.image || shouldLoadImage) return;
    const element = containerRef.current;
    if (!element || !("IntersectionObserver" in window)) {
      setShouldLoadImage(true);
      return;
    }
    const observer = new IntersectionObserver(
      (entries) => {
        if (!entries.some((entry) => entry.isIntersecting)) return;
        setShouldLoadImage(true);
        observer.disconnect();
      },
      { rootMargin: "320px" },
    );
    observer.observe(element);
    return () => observer.disconnect();
  }, [file?.image, shouldLoadImage]);

  React.useEffect(() => {
    if (!file?.image || !shouldLoadImage) return;
    let disposed = false;
    let objectURL = "";
    void downloadInternalMessagingFile(accessToken, message.id, { thumbnail: true })
      .then((blob) => {
        if (disposed) return;
        objectURL = URL.createObjectURL(blob);
        setImageURL(objectURL);
      })
      .catch(() => {
        if (!disposed) onErrorRef.current();
      });
    return () => {
      disposed = true;
      if (objectURL) URL.revokeObjectURL(objectURL);
    };
  }, [accessToken, file?.image, message.id, shouldLoadImage]);

  if (!file) return null;

  const download = async () => {
    try {
      const blob = await downloadInternalMessagingFile(accessToken, message.id, { download: true });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = file.name;
      anchor.click();
      window.setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch {
      onError();
    }
  };

  return (
    <div ref={containerRef} className="block max-w-full text-left">
      {file.image && imageURL ? (
        <button type="button" className="group/image relative block" onClick={onPreview}>
          <img
            src={imageURL}
            loading="lazy"
            className="mb-1 max-h-56 max-w-full rounded-lg object-contain"
            alt={file.name}
            onLoad={onContentResize}
          />
          <span className="absolute inset-0 flex items-center justify-center rounded-lg bg-black/0 opacity-0 transition group-hover/image:bg-black/25 group-hover/image:opacity-100">
            <Maximize2 className="size-5 text-white drop-shadow" />
          </span>
        </button>
      ) : (
        <button type="button" className="flex items-center gap-2" onClick={() => void download()}>
          <FileIcon className="size-8 shrink-0" />
          <span className="min-w-0">
            <span className="block truncate font-medium">{file.name}</span>
            <span className="block text-xs opacity-70">
              {formatFileSize(file.size, locale, t("file.unknownSize"))}
            </span>
          </span>
          <Download className="size-4 shrink-0" />
        </button>
      )}
      {file.image && imageURL ? (
        <button
          type="button"
          className="flex w-full items-center gap-1 text-xs opacity-75"
          onClick={() => void download()}
        >
          <span className="min-w-0 flex-1 truncate">{file.name}</span>
          <Download className="size-3" />
        </button>
      ) : null}
    </div>
  );
}

function ImageLightbox({
  accessToken,
  message,
  onClose,
  onError,
}: {
  accessToken: string;
  message: InternalMessagingMessage;
  onClose: () => void;
  onError: () => void;
}) {
  const t = useTranslations("internalMessaging");
  const [imageURL, setImageURL] = React.useState("");
  const onErrorRef = React.useRef(onError);
  onErrorRef.current = onError;

  React.useEffect(() => {
    let disposed = false;
    let objectURL = "";
    void downloadInternalMessagingFile(accessToken, message.id)
      .then((blob) => {
        if (disposed) return;
        objectURL = URL.createObjectURL(blob);
        setImageURL(objectURL);
      })
      .catch(() => {
        if (!disposed) onErrorRef.current();
      });
    return () => {
      disposed = true;
      if (objectURL) URL.revokeObjectURL(objectURL);
    };
  }, [accessToken, message.id]);

  React.useEffect(() => {
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("keydown", closeOnEscape);
    return () => window.removeEventListener("keydown", closeOnEscape);
  }, [onClose]);

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={t("aria.imagePreview")}
      className="fixed inset-0 z-[100] flex items-center justify-center bg-black/85 p-4"
    >
      <button
        type="button"
        aria-label={t("aria.closePreview")}
        className="absolute inset-0 cursor-default"
        onClick={onClose}
      />
      <button
        type="button"
        aria-label={t("aria.closePreview")}
        className="absolute right-4 top-[max(1rem,env(safe-area-inset-top))] z-10 rounded-full bg-black/50 p-2 text-white hover:bg-black/70"
        onClick={onClose}
      >
        <X className="size-5" />
      </button>
      {imageURL ? (
        <img
          src={imageURL}
          alt={message.file?.name || ""}
          className="relative max-h-full max-w-full object-contain"
        />
      ) : (
        <LoaderCircle className="size-8 animate-spin text-white" />
      )}
    </div>
  );
}

function formatFileSize(bytes: number, locale: string, unknownSize: string) {
  if (!Number.isFinite(bytes) || bytes <= 0) return unknownSize;
  const format = (value: number) =>
    new Intl.NumberFormat(locale, { maximumFractionDigits: 1 }).format(value);
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${format(bytes / 1024)} KB`;
  return `${format(bytes / 1024 / 1024)} MB`;
}
