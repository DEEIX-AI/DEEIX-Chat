export type ApiEnvelope<T> = {
  errorMsg: string;
  errorCode?: string;
  details?: unknown;
  requestId?: string;
  data: T;
};

export type PagePayload<T> = {
  total: number;
  results: T[];
};
