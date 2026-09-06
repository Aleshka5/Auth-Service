import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { askOwl, ChatError, chatSessionId, MAX_QUESTION_LENGTH } from './chat'
import { mockFetch } from './test/fixtures'

/** Builds an SSE body, optionally splitting frames across network chunks. */
function sseResponse(chunks: string[], status = 200): Response {
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      const encoder = new TextEncoder()
      for (const chunk of chunks) controller.enqueue(encoder.encode(chunk))
      controller.close()
    },
  })
  return new Response(stream, { status, headers: { 'Content-Type': 'text/event-stream' } })
}

beforeEach(() => {
  sessionStorage.clear()
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('chatSessionId', () => {
  it('is stable for the browser session', () => {
    const first = chatSessionId()
    expect(first).toBeTruthy()
    expect(chatSessionId()).toBe(first)
  })
})

describe('askOwl', () => {
  it('streams deltas in order and sends the session header', async () => {
    const fetchMock = mockFetch(() =>
      sseResponse([
        'data: {"delta":"Alexey "}\n\n',
        'data: {"delta":"builds agents."}\n\n',
        'data: [DONE]\n\n',
      ]),
    )

    const deltas: string[] = []
    await askOwl('Who is he?', { onDelta: (d) => deltas.push(d) })

    expect(deltas.join('')).toBe('Alexey builds agents.')
    const init = fetchMock.mock.calls[0][1] as RequestInit
    expect(fetchMock.mock.calls[0][0]).toBe('/api/chat')
    expect(init.method).toBe('POST')
    expect(init.credentials).toBe('include')
    expect(JSON.parse(String(init.body))).toEqual({ question: 'Who is he?' })
    expect((init.headers as Record<string, string>)['X-Chat-Session']).toBe(chatSessionId())
  })

  it('reassembles a frame split across chunks', async () => {
    mockFetch(() => sseResponse(['data: {"del', 'ta":"half"}\n\n', 'data: [DONE]\n\n']))
    const deltas: string[] = []
    await askOwl('q', { onDelta: (d) => deltas.push(d) })
    expect(deltas).toEqual(['half'])
  })

  it('turns a mid-stream error event into a ChatError', async () => {
    mockFetch(() => sseResponse(['data: {"delta":"partial"}\n\n', 'data: {"error":"upstream down"}\n\n']))
    await expect(askOwl('q', { onDelta: () => {} })).rejects.toThrow('upstream down')
  })

  it('reports the rate limit in words', async () => {
    mockFetch(() => new Response('{}', { status: 429 }))
    await expect(askOwl('q', { onDelta: () => {} })).rejects.toBeInstanceOf(ChatError)
  })

  it('surfaces the server error message on a 400', async () => {
    mockFetch(
      () =>
        new Response(JSON.stringify({ error: 'question is longer than 255 characters' }), {
          status: 400,
          headers: { 'Content-Type': 'application/json' },
        }),
    )
    await expect(askOwl('x'.repeat(MAX_QUESTION_LENGTH + 1), { onDelta: () => {} })).rejects.toThrow(
      'question is longer than 255 characters',
    )
  })
})
