import { useEffect, useState } from 'react'
import { onState } from './agent'
import type { StateDTO } from './types'

/** Subscribes to agent state (live in the webview, mock in a browser). */
export function useAgentState(): StateDTO | null {
  const [state, setState] = useState<StateDTO | null>(null)
  useEffect(() => onState(setState), [])
  return state
}
