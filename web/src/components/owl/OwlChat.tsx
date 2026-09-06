import { useEffect, useRef, useState } from 'react'
import { askOwl, ChatError, MAX_QUESTION_LENGTH } from '../../chat'
import type { User } from '../../types'
import { renderMarkdown } from './markdown'

type Status = 'idle' | 'streaming'

/**
 * The owl chat panel. Deliberately not a conversation: one question is on
 * screen at a time, the next one replaces it, and nothing is stored. The
 * backend works the same way, so what you see is the whole context the model
 * gets.
 */
type OwlChatProps = {
  onClose: () => void
  /** Passed down rather than read from context, so a page the reader may not
      see is never linked to them. */
  user?: User | null
}

export function OwlChat({ onClose, user = null }: OwlChatProps) {
  const [question, setQuestion] = useState('')
  const [asked, setAsked] = useState('')
  const [answer, setAnswer] = useState('')
  const [error, setError] = useState('')
  const [status, setStatus] = useState<Status>('idle')
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const abortRef = useRef<AbortController | null>(null)

  useEffect(() => {
    // The panel is always opened on purpose, so the caret belongs in the field.
    inputRef.current?.focus()
    return () => abortRef.current?.abort()
  }, [])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [onClose])

  const send = async () => {
    const trimmed = question.trim()
    if (!trimmed || status === 'streaming') return

    abortRef.current?.abort()
    const controller = new AbortController()
    abortRef.current = controller

    // Each question replaces the last one — there is no thread to append to.
    setAsked(trimmed)
    setAnswer('')
    setError('')
    setQuestion('')
    setStatus('streaming')

    try {
      await askOwl(trimmed, {
        signal: controller.signal,
        onDelta: (delta) => setAnswer((text) => text + delta),
      })
    } catch (err) {
      if (controller.signal.aborted) return
      setError(err instanceof ChatError ? err.message : 'The owl is not answering. Try again.')
    } finally {
      if (!controller.signal.aborted) setStatus('idle')
    }
  }

  const remaining = MAX_QUESTION_LENGTH - question.length

  return (
    <div className="owl-chat" role="dialog" aria-modal="false" aria-label="Chat with the AI owl">
      <div className="owl-chat-head">
        <span className="owl-chat-title">AI owl</span>
        <button className="owl-chat-close" type="button" aria-label="Close chat" onClick={onClose}>
          ×
        </button>
      </div>

      <div className="owl-chat-body">
        {asked ? (
          <>
            <p className="owl-chat-question">{asked}</p>
            <div className="owl-chat-answer" aria-live="polite">
              {renderMarkdown(answer, user)}
              {status === 'streaming' && !answer ? <span className="owl-chat-wait">…</span> : null}
            </div>
          </>
        ) : (
          <p className="owl-chat-hint">
            One question, one answer. I remember nothing between them — ask me about Alexey's
            experience, his projects, or this site.
          </p>
        )}
        {error ? (
          <p className="owl-chat-error" role="alert">
            {error}
          </p>
        ) : null}
      </div>

      <form
        className="owl-chat-form"
        onSubmit={(e) => {
          e.preventDefault()
          void send()
        }}
      >
        <textarea
          ref={inputRef}
          className="owl-chat-input"
          placeholder="Ask me something"
          value={question}
          rows={2}
          maxLength={MAX_QUESTION_LENGTH}
          disabled={status === 'streaming'}
          onChange={(e) => setQuestion(e.target.value)}
          onKeyDown={(e) => {
            // Enter sends; Shift+Enter is a newline inside 255 characters.
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault()
              void send()
            }
          }}
        />
        <div className="owl-chat-actions">
          <span className={remaining < 30 ? 'owl-chat-count is-low' : 'owl-chat-count'}>
            {remaining}
          </span>
          <button
            className="btn btn-small"
            type="submit"
            disabled={status === 'streaming' || question.trim() === ''}
          >
            {status === 'streaming' ? 'Thinking…' : 'Send'}
          </button>
        </div>
      </form>
    </div>
  )
}

export default OwlChat
