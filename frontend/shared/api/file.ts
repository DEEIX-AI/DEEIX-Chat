import { authedFetch, authedRequest } from "@/shared/api/authed-client";
import { pathParam, resolveApiBaseURL } from "@/shared/api/http-client";
import type {
  ChatFilePolicyDTO,
  DeleteFileResult,
  FileExtractDTO,
  FileListResult,
  FileObjectDTO,
  FileProcessingStatusDTO,
  UploadFileResult,
} from "@/shared/api/file.types";

type UploadFileOptions = {
  purpose?: string;
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

// Upload
export async function uploadFile(
  accessToken: string,
  file: File,
  options: UploadFileOptions = {},
): Promise<UploadFileResult> {
  const formData = new FormData();
  formData.append("file", file);
  if (options.purpose) {
    formData.append("purpose", options.purpose);
  }

  return authedRequest<UploadFileResult>(
    "/api/v1/files",
    {
      method: "POST",
      accessToken,
      body: formData,
    },
    true,
  );
}

// File catalog and content
export async function listFiles(
  accessToken: string,
  params: ListFilesParams = {},
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

export async function fetchFileContent(accessToken: string, fileID: string): Promise<FileContentResult> {
  const response = await authedFetch(
    `/api/v1/files/${pathParam(fileID)}/content`,
    {
      method: "GET",
      accessToken,
      cache: "no-store",
    },
    true,
  );

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

export async function fetchSharedFileContent(shareID: string, fileID: string): Promise<FileContentResult> {
  const response = await fetch(
    `${resolveApiBaseURL()}/api/v1/shared-conversations/${pathParam(shareID)}/files/${pathParam(fileID)}/content`,
    {
      method: "GET",
      cache: "no-store",
      credentials: "include",
    },
  );

  if (!response.ok) {
    const message = response.headers.get("content-type")?.includes("application/json")
      ? ((await response.json()) as { errorMsg?: string }).errorMsg
      : await response.text();
    throw new Error(message?.trim() || "Failed to load file");
  }

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

// Processing and runtime policy
export async function getFileProcessingStatus(
  accessToken: string,
  fileID: string,
): Promise<FileProcessingStatusDTO> {
  return authedRequest<FileProcessingStatusDTO>(
    `/api/v1/files/${pathParam(fileID)}/processing`,
    {
      method: "GET",
      accessToken,
    },
    true,
  );
}

export async function getChatFilePolicy(accessToken: string): Promise<ChatFilePolicyDTO> {
  return authedRequest<ChatFilePolicyDTO>(
    "/api/v1/runtime/chat-file-policy",
    {
      method: "GET",
      accessToken,
    },
    true,
  );
}
