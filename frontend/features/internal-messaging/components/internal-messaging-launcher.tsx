"use client";

import { useTranslations } from "next-intl";
import * as React from "react";

import {
  useInternalMessagingEvents,
} from "@/features/internal-messaging/components/use-internal-messaging-events";
import {
  getInternalMessagingStatus,
  listInternalMessagingConversations,
} from "@/shared/api/internal-messaging";
import type {
  InternalMessagingConversation,
  InternalMessagingStatus,
} from "@/shared/api/internal-messaging.types";
import { useAuthSession } from "@/shared/auth/auth-session-context";

import { InternalMessagingLauncherButton } from "./internal-messaging-launcher-button";

function createLazyInternalMessagingWindow() {
  return React.lazy(async () => {
    const module = await import("./internal-messaging-host");
    return { default: module.InternalMessagingWindowHost };
  });
}

class MessagingWindowBoundary extends React.Component<
  {
    children: React.ReactNode;
    fallback: React.ReactNode;
    onError?: () => void;
  },
  { failed: boolean }
> {
  state = { failed: false };

  static getDerivedStateFromError() {
    return { failed: true };
  }

  componentDidCatch() {
    this.props.onError?.();
  }

  render() {
    return this.state.failed ? this.props.fallback : this.props.children;
  }
}

const NOTIFICATION_STORAGE_PREFIX = "deeix.internal-messaging.notifications.v1";

export function InternalMessagingHost() {
  const t = useTranslations("internalMessaging");
  const { accessToken, user } = useAuthSession();
  const [status, setStatus] = React.useState<InternalMessagingStatus>();
  const [activated, setActivated] = React.useState(false);
  const [windowLoadFailed, setWindowLoadFailed] = React.useState(false);
  const [loadAttempt, setLoadAttempt] = React.useState(0);
  const LazyInternalMessagingWindow = React.useMemo(
    createLazyInternalMessagingWindow,
    [loadAttempt],
  );
  const conversationsRef = React.useRef<InternalMessagingConversation[]>([]);
  const refreshTimerRef = React.useRef<number | null>(null);

  const refreshStatus = React.useCallback(async () => {
    if (!accessToken) return;
    try {
      setStatus(await getInternalMessagingStatus(accessToken));
    } catch {
      // Keep the last configuration while the connection indicator retries.
    }
  }, [accessToken]);

  const refreshConversations = React.useCallback(async () => {
    if (!accessToken) return;
    try {
      conversationsRef.current = (
        await listInternalMessagingConversations(accessToken)
      ).results;
    } catch {
      // Keep the last preference snapshot while the optional service reconnects.
    }
  }, [accessToken]);

  React.useEffect(() => {
    if (activated || !accessToken) return;
    void refreshStatus();
    const refresh = () => void refreshStatus();
    const timer = window.setInterval(refresh, 30_000);
    window.addEventListener("focus", refresh);
    return () => {
      window.clearInterval(timer);
      window.removeEventListener("focus", refresh);
    };
  }, [accessToken, activated, refreshStatus]);

  React.useEffect(() => {
    if (activated || !status?.enabled) return;
    void refreshConversations();
  }, [activated, refreshConversations, status?.enabled]);

  useInternalMessagingEvents(
    Boolean((!activated || windowLoadFailed) && status?.enabled),
    accessToken,
    (event) => {
      if (event.type !== "chat") return;
      const incoming = Boolean(
        event.fromUserPublicID && event.fromUserPublicID !== user?.publicID,
      );
      const reaction = event.detail?.type === "reaction";
      if (incoming && !reaction) {
        setStatus((current) =>
          current ? { ...current, unreadCount: current.unreadCount + 1 } : current,
        );
        const conversation = conversationsRef.current.find(
          (item) => item.user.publicID === event.conversationPublicID,
        );
        const notificationsEnabled = Boolean(
          user?.publicID &&
            window.localStorage.getItem(
              `${NOTIFICATION_STORAGE_PREFIX}:${user.publicID}`,
            ) === "true",
        );
        if (
          status?.browserNotifications &&
          notificationsEnabled &&
          conversation &&
          !conversation.muted &&
          document.visibilityState !== "visible" &&
          "Notification" in window &&
          Notification.permission === "granted"
        ) {
          new Notification(
            conversation.user.displayName || conversation.user.username || t("title"),
            {
              body:
                event.detail?.content_type === "vocechat/file"
                  ? t("notifications.file")
                  : event.detail?.content || t("notifications.message"),
            },
          );
        }
      }
      if (refreshTimerRef.current) return;
      refreshTimerRef.current = window.setTimeout(() => {
        refreshTimerRef.current = null;
        void refreshStatus();
        void refreshConversations();
      }, 150);
    },
  );

  React.useEffect(
    () => () => {
      if (refreshTimerRef.current) window.clearTimeout(refreshTimerRef.current);
    },
    [],
  );

  const open = () => setActivated(true);

  const retryWindowLoad = () => {
    setWindowLoadFailed(false);
    setLoadAttempt((current) => current + 1);
  };

  if (activated && status) {
    return (
      <MessagingWindowBoundary
        key={`${user?.publicID || "signed-out"}:${loadAttempt}`}
        onError={() => setWindowLoadFailed(true)}
        fallback={
          <LauncherLoadFailure
            key={user?.publicID}
            accountID={user?.publicID || ""}
            unreadCount={status.unreadCount}
            label={t("aria.open")}
            errorMessage={t("errors.windowLoad")}
            retryLabel={t("actions.retryLoad")}
            onOpen={retryWindowLoad}
          />
        }
      >
        <React.Suspense
          fallback={
            <InternalMessagingLauncherButton
              key={user?.publicID}
              accountID={user?.publicID || ""}
              unreadCount={status.unreadCount}
              label={t("aria.open")}
              onOpen={open}
            />
          }
        >
          <LazyInternalMessagingWindow initiallyOpen initialStatus={status} />
        </React.Suspense>
      </MessagingWindowBoundary>
    );
  }
  if (!status?.enabled || !user) return null;

  return (
    <InternalMessagingLauncherButton
      key={user.publicID}
      accountID={user.publicID}
      unreadCount={status.unreadCount}
      label={t("aria.open")}
      onOpen={open}
    />
  );
}

function LauncherLoadFailure({
  errorMessage,
  retryLabel,
  ...buttonProps
}: React.ComponentProps<typeof InternalMessagingLauncherButton> & {
  errorMessage: string;
  retryLabel: string;
}) {
  return (
    <>
      <div
        className="fixed bottom-20 right-5 z-[70] max-w-64 rounded-md border bg-background p-3 text-sm shadow-lg"
        role="alert"
      >
        <p>{errorMessage}</p>
        <p className="mt-1 text-xs text-muted-foreground">{retryLabel}</p>
      </div>
      <InternalMessagingLauncherButton {...buttonProps} label={retryLabel} />
    </>
  );
}
