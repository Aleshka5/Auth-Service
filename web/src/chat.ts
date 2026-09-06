/**
 * The owl chat client. One question in, one streamed answer out — the server
 * keeps no history, so neither does this module.
 */

export const MAX_QUESTION_LENGTH = 255

const SESSION_KEY = 'owl-chat-session'

/**
 * A per-browser-session id, sent so the LiteLLM proxy can group one visitor's
 * questions in its logs. It is not a credential: the endpoint is public and
 * authorizes nothing on it. Signed-in visitors are identified by the gateway
 * from their cookie, never from anything the page sends.
 */
export function chatSessionId(): string {
  try {
    const existing = sessionStorage.getItem(SESSION_KEY)
    if (existing) return existing
    const fresh = crypto.randomUUID()
    sessionStorage.setItem(SESSION_KEY, fresh)
    return fresh
  } catch {
    // Private mode, or storage disabled — a per-call id still groups nothing
    // wrongly, it just groups less.
    return crypto.randomUUID()
  }
}

export class ChatError extends Error {}

type AskOptions = {
  onDelta: (delta: string) => void
  signal?: AbortSignal
}

async function readErrorMessage(res: Response): Promise<string> {
  try {
    const body = (await res.json()) as { error?: string }
    if (body.error) return body.error
  } catch {
    // fall through to the generic message
  }
  if (res.status === 429) return 'Too many questions right now. Try again in a minute.'
  return `The owl is not answering (${res.status}).`
}

/** Parses one `data:` line of the answer stream. */
function handleEvent(payload: string, onDelta: (delta: string) => void): void {
  if (payload === '[DONE]' || payload === '') return
  let event: { delta?: string; error?: string }
  try {
    event = JSON.parse(payload) as { delta?: string; error?: string }
  } catch {
    return
  }
  if (event.error) throw new ChatError(event.error)
  if (event.delta) onDelta(event.delta)
}

export async function askOwl(question: string, { onDelta, signal }: AskOptions): Promise<void> {
  const res = await fetch('/api/chat', {
    method: 'POST',
    credentials: 'include',
    signal,
    headers: {
      'Content-Type': 'application/json',
      Accept: 'text/event-stream',
      'X-Chat-Session': chatSessionId(),
    },
    body: JSON.stringify({ question }),
  })

  if (!res.ok) throw new ChatError(await readErrorMessage(res))
  if (!res.body) throw new ChatError('The owl is not answering.')

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  for (;;) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    // SSE frames are separated by a blank line; the tail may be a partial one.
    const frames = buffer.split('\n\n')
    buffer = frames.pop() ?? ''
    for (const frame of frames) {
      for (const line of frame.split('\n')) {
        if (line.startsWith('data:')) handleEvent(line.slice(5).trim(), onDelta)
      }
    }
  }
}
