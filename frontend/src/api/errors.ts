import type { ErrorDetail } from "./contracts";

export class ValidationApiError extends Error {
  constructor(public readonly details: ErrorDetail[]) {
    super("VALIDATION_ERROR");
    this.name = "ValidationApiError";
  }
}

export class CatalogUnavailableError extends Error {
  constructor() {
    super("CATALOG_UNAVAILABLE");
    this.name = "CatalogUnavailableError";
  }
}

export class NetworkApiError extends Error {
  constructor() {
    super("NETWORK_ERROR");
    this.name = "NetworkApiError";
  }
}

export class TimeoutApiError extends Error {
  constructor() {
    super("TIMEOUT");
    this.name = "TimeoutApiError";
  }
}

export class InvalidApiResponseError extends Error {
  constructor() {
    super("INVALID_API_RESPONSE");
    this.name = "InvalidApiResponseError";
  }
}

export class HttpApiError extends Error {
  constructor(public readonly status: number, public readonly code: string) {
    super(code);
    this.name = "HttpApiError";
  }
}

export function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === "AbortError";
}
