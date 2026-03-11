import { ref } from 'vue'
import type { AuditReport, RunRequest } from '../types/audit'

export type RunStatus = 'idle' | 'running' | 'done' | 'error'

export function useAuditRunner() {
  const status = ref<RunStatus>('idle')
  const logLines = ref<string[]>([])
  const result = ref<AuditReport | null>(null)
  const error = ref<string | null>(null)

  function reset() {
    status.value = 'idle'
    logLines.value = []
    result.value = null
    error.value = null
  }

  async function run(request: RunRequest): Promise<void> {
    reset()
    status.value = 'running'

    let response: Response
    try {
      response = await fetch('/api/run', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(request),
      })
    } catch (err) {
      status.value = 'error'
      error.value = err instanceof Error ? err.message : 'Network error: could not connect to server'
      return
    }

    if (!response.ok) {
      status.value = 'error'
      error.value = `Server error: ${response.status} ${response.statusText}`
      return
    }

    const body = response.body
    if (!body) {
      status.value = 'error'
      error.value = 'No response body from server'
      return
    }

    const reader = body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''

    try {
      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })

        // Parse SSE events from buffer
        const events = buffer.split('\n\n')
        // Keep the last incomplete chunk in buffer
        buffer = events.pop() ?? ''

        for (const eventBlock of events) {
          if (!eventBlock.trim()) continue

          const lines = eventBlock.split('\n')
          let eventType = 'message'
          let dataStr = ''

          for (const line of lines) {
            if (line.startsWith('event: ')) {
              eventType = line.slice(7).trim()
            } else if (line.startsWith('data: ')) {
              dataStr = line.slice(6).trim()
            }
          }

          if (!dataStr) continue

          try {
            const parsed = JSON.parse(dataStr)

            switch (eventType) {
              case 'log':
                if (parsed.line) {
                  logLines.value.push(parsed.line)
                }
                break

              case 'result':
                result.value = parsed as AuditReport
                break

              case 'done':
                status.value = 'done'
                break

              case 'error':
                status.value = 'error'
                error.value = parsed.message ?? 'Unknown error'
                break
            }
          } catch {
            // Non-JSON data line — treat as raw log
            logLines.value.push(dataStr)
          }
        }
      }
    } catch (err) {
      if (status.value === 'running') {
        status.value = 'error'
        error.value = err instanceof Error ? err.message : 'Stream read error'
      }
    } finally {
      reader.releaseLock()
      if (status.value === 'running') {
        // Stream ended without a done event
        status.value = result.value ? 'done' : 'error'
        if (!result.value && !error.value) {
          error.value = 'Connection closed without result'
        }
      }
    }
  }

  return { status, logLines, result, error, run, reset }
}
