import { API_BASE_URL } from '../config';

/** Mirrors the backend envelope: {"error":{"code":"...","message":"..."}} */
export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  let res: Response;
  try {
    res = await fetch(API_BASE_URL + path, {
      ...init,
      headers: { 'Content-Type': 'application/json', ...init.headers },
    });
  } catch (e) {
    if ((e as Error)?.name === 'AbortError') throw e;
    throw new ApiError(0, 'NETWORK_ERROR', `Cannot reach the server at ${API_BASE_URL}`);
  }

  if (res.status === 204) return undefined as T;

  const body = await res.json().catch(() => null);
  if (!res.ok) {
    const err = body?.error;
    throw new ApiError(
      res.status,
      typeof err?.code === 'string' ? err.code : 'UNKNOWN_ERROR',
      typeof err?.message === 'string' ? err.message : `Request failed (${res.status})`,
    );
  }
  return body as T;
}
