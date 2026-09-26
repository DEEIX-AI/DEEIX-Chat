import type { Envelope } from "@deeix/api-contract";

export type ApiEnvelope<T> = Omit<Envelope, "data" | "details" | "errorMsg"> & {
  errorMsg: string;
  details?: unknown;
  data: T;
};

export type PagePayload<T> = {
  total: number;
  results: T[];
};
