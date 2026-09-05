import { authedFetch, authedRequest } from "@/shared/api/authed-client";
import type {
  ChatFilePolicyDTO,
  DeleteFileResult,
  FileEmbeddingSubmissionDTO,
  FileExtractDTO,
  FileListResult,
  FileObjectDTO,
  FileProcessingStatusDTO,
  UploadFileResult,
} from "@/shared/api/file.types";
import {
  ApiNetworkError,
  apiFetch,
  pathParam,
  resolveAbortError,
} from "@/shared/api/http-client";

type UploadFileOptions = {
  purpose?: string;
  signal?: AbortSignal;
};

type ListFilesParams = {
  page?: number;
  pageSize?: number;
  query?: string;
  kind?: string[];
  sort?: "created" | "name" | "size" | "last_used";
};

export type FileContentResult = {
  blob: Blob;
  contentType: string;
  disposition: string | null;
  contentLength: number | null;
};

export type RenameFileResult = FileObjectDTO;

function waitForUploadRetry(delayMs: number, signal?: AbortSignal): Promise<void> {
  const abortError = resolveAbortError(undefined, signal);
  if (abortError) {
    return Promise.reject(abortError);
  }

  return new Promise((resolve, reject) => {
    let timeoutID: ReturnType<typeof setTimeout>;
    const handleAbort = () => {
      clearTimeout(timeoutID);
      signal?.removeEventListener("abort", handleAbort);
      const error = resolveAbortError(undefined, signal) ?? new Error("The operation was aborted");
      error.name = "AbortError";
      reject(error);
    };

    timeoutID = setTimeout(() => {
      signal?.removeEventListener("abort", handleAbort);
      resolve();
    }, delayMs);
    signal?.addEventListener("abort", handleAbort, { once: true });
    if (signal?.aborted) {
      handleAbort();
    }
  });
}

export async function readFileContentResponse(response: Response): Promise<FileContentResult> {
  const blob = await response.blob();
  const rawContentLength = response.headers.get("content-length");
  const parsedContentLength = rawContentLength ? Number.parseInt(rawContentLength, 10) : Number.NaN;

  return {
    blob,
    contentType: response.headers.get("content-type") || blob.type || "application/octet-stream",
    disposition: response.headers.get("content-disposition"),
    contentLength: Number.isFinite(parsedContentLength) ? parsedContentLength : blob.size || null,
  };
}

// Upload
export async function uploadFile(
  accessToken: string,
  file: File,
  options: UploadFileOptions = {},
): Promise<UploadFileResult> {
  for (let attempt = 0; ; attempt += 1) {
    const formData = new FormData();
    formData.append("file", file);
    if (options.purpose) {
      formData.append("purpose", options.purpose);
    }

    try {
      return await authedRequest<UploadFileResult>(
        "/api/v1/files",
        {
          method: "POST",
          accessToken,
          body: formData,
          signal: options.signal,
        },
        true,
      );
    } catch (error) {
      const abortError = resolveAbortError(error, options.signal);
      if (abortError) {
        throw abortError;
      }
      if (!(error instanceof ApiNetworkError) || attempt >= 2) {
        throw error;
      }
      await waitForUploadRetry(
        250 * (2 ** attempt) + Math.floor(Math.random() * 150),
        options.signal,
      );
    }
  }
}

// File catalog and content
export async function listFiles(
  accessToken: string,
  params: ListFilesParams = {},
  signal?: AbortSignal,
): Promise<FileListResult> {
  const searchParams = new URLSearchParams();

  if (typeof params.page === "number") {
    searchParams.set("page", String(params.page));
  }
  if (typeof params.pageSize === "number") {
    searchParams.set("page_size", String(params.pageSize));
  }
  if (params.query?.trim()) {
    searchParams.set("q", params.query.trim());
  }
  if (params.kind && params.kind.length > 0) {
    searchParams.set("kind", params.kind.join(","));
  }
  if (params.sort) {
    searchParams.set("sort", params.sort);
  }

  const suffix = searchParams.toString();
  return authedRequest<FileListResult>(
    suffix ? `/api/v1/files?${suffix}` : "/api/v1/files",
    {
      method: "GET",
      accessToken,
      signal,
    },
    true,
  );
}

export async function deleteFile(accessToken: string, fileID: string): Promise<DeleteFileResult> {
  return authedRequest<DeleteFileResult>(
    `/api/v1/files/${pathParam(fileID)}`,
    {
      method: "DELETE",
      accessToken,
    },
    true,
  );
}

export async function renameFile(
  accessToken: string,
  fileID: string,
  fileName: string,
): Promise<RenameFileResult> {
  return authedRequest<RenameFileResult>(
    `/api/v1/files/${pathParam(fileID)}`,
    {
      method: "PATCH",
      accessToken,
      body: { fileName: fileName },
    },
    true,
  );
}

export async function updateFileRagOptOut(
  accessToken: string,
  fileID: string,
  ragOptOut: boolean,
): Promise<FileObjectDTO> {
  return authedRequest<FileObjectDTO>(
    `/api/v1/files/${pathParam(fileID)}`,
    {
      method: "PATCH",
      accessToken,
      body: { ragOptOut: ragOptOut },
    },
    true,
  );
}

export async function fetchFileContent(
  accessToken: string,
  fileID: string,
  signal?: AbortSignal,
): Promise<FileContentResult> {
  const response = await authedFetch(
    `/api/v1/files/${pathParam(fileID)}/content`,
    {
      method: "GET",
      accessToken,
      cache: "no-store",
      signal,
    },
    true,
  );

  return readFileContentResponse(response);
}

export async function fetchSharedFileContent(
  shareID: string,
  fileID: string,
  signal?: AbortSignal,
): Promise<FileContentResult> {
  const response = await apiFetch(
    `/api/v1/shared-conversations/${pathParam(shareID)}/files/${pathParam(fileID)}/content`,
    { signal },
  );

  return readFileContentResponse(response);
}

export async function fetchFileExtract(accessToken: string, fileID: string): Promise<FileExtractDTO> {
  return authedRequest<FileExtractDTO>(
    `/api/v1/files/${pathParam(fileID)}/extract`,
    {
      method: "GET",
      accessToken,
    },
    true,
  );
}

export async function getFileProcessingStatuses(
  accessToken: string,
  fileIDs: string[],
  signal?: AbortSignal,
): Promise<FileProcessingStatusDTO[]> {
  if (fileIDs.length === 0) {
    return [];
  }
  const requests: Promise<FileProcessingStatusDTO[]>[] = [];
  for (let index = 0; index < fileIDs.length; index += 100) {
    requests.push(authedRequest<FileProcessingStatusDTO[]>(
      "/api/v1/files/processing/statuses",
      {
        method: "POST",
        accessToken,
        body: { fileIDs: fileIDs.slice(index, index + 100) },
        signal,
      },
      true,
    ));
  }
  return (await Promise.all(requests)).flat();
}

export async function submitFileEmbeddings(
  accessToken: string,
  fileIDs: string[],
  signal?: AbortSignal,
): Promise<FileEmbeddingSubmissionDTO> {
  return authedRequest<FileEmbeddingSubmissionDTO>(
    "/api/v1/files/embeddings",
    {
      method: "POST",
      accessToken,
      body: { fileIDs },
      signal,
    },
    true,
  );
}

export async function getChatFilePolicy(accessToken: string, signal?: AbortSignal): Promise<ChatFilePolicyDTO> {
  return authedRequest<ChatFilePolicyDTO>(
    "/api/v1/runtime/chat-file-policy",
    {
      method: "GET",
      accessToken,
      signal,
    },
    true,
  );
}
