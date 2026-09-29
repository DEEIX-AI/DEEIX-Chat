import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  type ContentModerationEventDetail,
  type ModerationEvent,
  fetchContentModerationEventImage,
  getContentModerationEvent,
  listContentModerationEvents,
} from "@/features/admin/api/content-moderation";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

export type AdminModerationEventImage = { index: number; url: string };

/**
 * Loads one moderation event and its images as object URLs. URLs are revoked
 * when replaced, when the sheet closes and on unmount so blobs never leak.
 */
export function useAdminLogsModerationEventDetail({
  eventID,
  open,
  onClose,
}: {
  eventID: string;
  open: boolean;
  onClose: () => void;
}) {
  const t = useTranslations("adminLogs.moderation");
  const [loading, setLoading] = React.useState(false);
  const [detail, setDetail] = React.useState<ContentModerationEventDetail | null>(null);
  const [images, setImages] = React.useState<AdminModerationEventImage[]>([]);
  const requestRef = React.useRef(0);
  const imagesRef = React.useRef<AdminModerationEventImage[]>([]);

  const revokeImages = React.useCallback((items: AdminModerationEventImage[]) => {
    for (const item of items) URL.revokeObjectURL(item.url);
  }, []);

  const replaceImages = React.useCallback(
    (items: AdminModerationEventImage[]) => {
      revokeImages(imagesRef.current);
      imagesRef.current = items;
      setImages(items);
    },
    [revokeImages],
  );

  React.useEffect(() => {
    if (!open || !eventID) return;
    const requestID = ++requestRef.current;
    setLoading(true);
    setDetail(null);
    replaceImages([]);
    void (async () => {
      try {
        const token = await resolveAccessToken();
        if (!token) return;
        const loaded = await getContentModerationEvent(token, eventID);
        if (requestRef.current !== requestID) return;
        setDetail(loaded);
        if (loaded.imagesAvailable && Array.isArray(loaded.images) && loaded.images.length > 0) {
          const loadedImages: AdminModerationEventImage[] = [];
          for (const image of loaded.images) {
            try {
              const { blob } = await fetchContentModerationEventImage(token, eventID, image.index);
              const url = URL.createObjectURL(blob);
              if (requestRef.current !== requestID) {
                URL.revokeObjectURL(url);
                revokeImages(loadedImages);
                return;
              }
              loadedImages.push({ index: image.index, url });
            } catch {
              if (requestRef.current !== requestID) {
                revokeImages(loadedImages);
                return;
              }
              toast.error(t("imageLoadFailed"));
            }
          }
          if (requestRef.current === requestID) replaceImages(loadedImages);
          else revokeImages(loadedImages);
        }
      } catch (error) {
        if (requestRef.current === requestID) {
          toast.error(t("detailFailed"), { description: resolveAdminErrorMessage(error) });
          onClose();
        }
      } finally {
        if (requestRef.current === requestID) setLoading(false);
      }
    })();
    return () => {
      requestRef.current += 1;
    };
  }, [eventID, onClose, open, replaceImages, revokeImages, t]);

  React.useEffect(() => {
    if (open) return;
    revokeImages(imagesRef.current);
    imagesRef.current = [];
  }, [open, revokeImages]);

  React.useEffect(() => {
    return () => {
      requestRef.current += 1;
      revokeImages(imagesRef.current);
      imagesRef.current = [];
    };
  }, [revokeImages]);

  return { loading, detail, images };
}

/** Pages through moderation events; stale responses are dropped by request id. */
export function useAdminLogsModerationEvents({
  page,
  pageSize,
  searchQuery,
  resultFilter,
  directionFilter,
}: {
  page: number;
  pageSize: number;
  searchQuery: string;
  resultFilter: string;
  directionFilter: string;
}) {
  const t = useTranslations("adminLogs.moderation");
  const [loading, setLoading] = React.useState(true);
  const [items, setItems] = React.useState<ModerationEvent[]>([]);
  const [total, setTotal] = React.useState(0);
  const requestRef = React.useRef(0);

  const load = React.useCallback(async () => {
    const requestID = ++requestRef.current;
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      const res = await listContentModerationEvents(token, {
        page,
        pageSize,
        query: searchQuery || undefined,
        result: resultFilter.trim() || undefined,
        direction: directionFilter.trim() || undefined,
      });
      if (requestRef.current !== requestID) return;
      setItems(res.items ?? []);
      setTotal(res.total ?? 0);
    } catch (error) {
      if (requestRef.current === requestID) {
        toast.error(t("loadFailed"), { description: resolveAdminErrorMessage(error) });
      }
    } finally {
      if (requestRef.current === requestID) setLoading(false);
    }
  }, [directionFilter, page, pageSize, resultFilter, searchQuery, t]);

  React.useEffect(() => {
    void load();
    return () => {
      requestRef.current += 1;
    };
  }, [load]);

  return { loading, items, total, load };
}
