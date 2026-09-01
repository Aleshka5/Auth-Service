import { Component, type ErrorInfo, type ReactNode } from 'react'

type Props = { children: ReactNode }

type State = { failed: boolean }

/**
 * Catches the stale-deploy case: the reader holds an old index.html and the
 * hashed chunk it points at is gone. Reloading picks up the new index.html.
 *
 * The caller gives this a React `key` of the node path, so navigating away from
 * a failed node remounts the boundary and clears the failure.
 */
export class NodeBoundary extends Component<Props, State> {
  state: State = { failed: false }

  static getDerivedStateFromError(): State {
    return { failed: true }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('Resume node failed to load', error, info.componentStack)
  }

  render() {
    if (!this.state.failed) return this.props.children
    return (
      <div className="node-error" role="alert">
        <p>This section could not be loaded — the site was updated.</p>
        <button type="button" className="btn btn-small" onClick={() => window.location.reload()}>
          Reload
        </button>
      </div>
    )
  }
}
