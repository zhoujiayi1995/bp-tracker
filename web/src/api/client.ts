export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    public readonly field?: string,
    public readonly reason?: string,
    public readonly retryAfter?: number,
  ) {
    super(code)
  }
}

export class NetworkError extends Error {}

let onUnauthorized: (() => void) | null = null

export function setUnauthorizedHandler(fn: () => void) {
  onUnauthorized = fn
}

type Method = 'GET' | 'POST' | 'PUT' | 'DELETE'

export async function request<T>(method: Method, path: string, body?: unknown): Promise<T> {
  // The server rejects state-changing calls without this header (CSRF guard).
  const headers: Record<string, string> = { 'X-Requested-With': 'bp' }
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
  }

  let res: Response
  try {
    res = await fetch(`/api${path}`, {
      method,
      headers,
      credentials: 'same-origin',
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } catch {
    throw new NetworkError()
  }

  if (res.status === 204) {
    return undefined as T
  }

  const data = await res.json().catch(() => null)
  if (!res.ok) {
    const e = data?.error ?? {}
    const err = new ApiError(res.status, e.code ?? 'internal', e.field, e.reason, e.retryAfter)
    if (res.status === 401 && onUnauthorized) {
      onUnauthorized()
    }
    throw err
  }
  return data as T
}
