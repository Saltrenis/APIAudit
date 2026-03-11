<script setup lang="ts">
import { computed } from 'vue'
import type { WizardState, RunRequest } from '../../types/audit'

const props = defineProps<{
  state: WizardState
}>()

const emit = defineEmits<{
  (e: 'update', patch: Partial<WizardState>): void
  (e: 'run'): void
}>()

const runRequest = computed((): RunRequest => {
  const flags: Record<string, string> = {}
  const boolFlags: string[] = []

  if (props.state.format && props.state.format !== 'table') {
    flags['format'] = props.state.format
  }
  if (props.state.output) {
    flags['output'] = props.state.output
  }
  if (props.state.beads) {
    boolFlags.push('beads')
  }
  if (props.state.aiAssist) {
    boolFlags.push('ai-assist')
  }

  if (props.state.command === 'audit') {
    if (props.state.skipFrontend) boolFlags.push('skip-frontend')
    if (props.state.skipGenerate) boolFlags.push('skip-generate')
    if (props.state.staticOnly) boolFlags.push('static-only')
  }

  if (props.state.command === 'generate') {
    if (props.state.title) flags['title'] = props.state.title
    if (props.state.description) flags['description'] = props.state.description
    if (props.state.apiVersion) flags['api-version'] = props.state.apiVersion
    if (props.state.jsonFormat) boolFlags.push('json')
  }

  if (props.state.command === 'annotate') {
    if (props.state.dryRun) boolFlags.push('dry-run')
  }

  return {
    command: props.state.command,
    dir: props.state.sourceType === 'local' ? props.state.dir : '',
    repo: props.state.sourceType === 'repo' ? props.state.repo : '',
    frontendDir: props.state.frontendDir,
    format: props.state.format,
    output: props.state.output,
    flags,
    boolFlags,
    beadsLimit: props.state.beads ? props.state.beadsLimit : 0,
  }
})

const cliPreview = computed((): string => {
  const req = runRequest.value
  const parts = ['apiaudit', req.command]

  if (req.dir) parts.push(req.dir)
  if (req.repo) {
    parts.push('--repo', req.repo)
  }
  if (req.frontendDir) {
    parts.push('--frontend-dir', req.frontendDir)
  }

  Object.entries(req.flags).forEach(([k, v]: [string, string]) => {
    parts.push(`--${k}`, v.includes(' ') ? `"${v}"` : v)
  })

  req.boolFlags.forEach((f: string) => {
    parts.push(`--${f}`)
  })

  if (req.beadsLimit > 0) {
    parts.push('--beads-limit', String(req.beadsLimit))
  }

  return parts.join(' ')
})
</script>

<template>
  <div class="step-content">
    <h2 class="step-title">Review & Run</h2>
    <p class="step-description">Review the command and configuration before running.</p>

    <div class="review-section">
      <div class="review-label">CLI Command Preview</div>
      <div class="cli-block">
        <span class="cli-prompt">$</span>
        <code class="cli-code">{{ cliPreview }}</code>
      </div>
    </div>

    <div class="review-section">
      <div class="review-label">API Payload (JSON)</div>
      <pre class="json-block">{{ JSON.stringify(runRequest, null, 2) }}</pre>
    </div>

    <div class="run-area">
      <button class="run-btn" @click="emit('run')">
        <svg viewBox="0 0 20 20" fill="currentColor" class="run-icon">
          <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z" clip-rule="evenodd"/>
        </svg>
        Run Audit
      </button>
      <p class="run-hint">This will send a POST request to <code>/api/run</code> and stream results in real time.</p>
    </div>
  </div>
</template>

<style scoped>
.step-content {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.step-title {
  font-size: 20px;
  color: var(--text-primary);
}

.step-description {
  font-size: 14px;
  color: var(--text-secondary);
  margin-top: -16px;
}

.review-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.review-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.07em;
}

.cli-block {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  background: #0a0d14;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  padding: 16px 18px;
  overflow-x: auto;
}

.cli-prompt {
  color: var(--success);
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 14px;
  user-select: none;
  flex-shrink: 0;
  padding-top: 1px;
}

.cli-code {
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 14px;
  color: #e2e8f0;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
  background: none;
}

.json-block {
  background: #0a0d14;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  padding: 16px 18px;
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 13px;
  color: var(--text-secondary);
  overflow-x: auto;
  line-height: 1.6;
  max-height: 300px;
  overflow-y: auto;
}

.run-area {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 24px;
  border: 1px dashed var(--border);
  border-radius: var(--radius-md);
  background: var(--bg-card);
}

.run-btn {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 36px;
  background: var(--accent);
  color: #fff;
  border: none;
  border-radius: var(--radius-md);
  font-size: 16px;
  font-weight: 700;
  cursor: pointer;
  transition: background 0.15s, transform 0.1s;
  letter-spacing: 0.02em;
}

.run-btn:hover {
  background: var(--accent-hover);
}

.run-btn:active {
  transform: scale(0.98);
}

.run-icon {
  width: 20px;
  height: 20px;
}

.run-hint {
  font-size: 12px;
  color: var(--text-muted);
  text-align: center;
}

.run-hint code {
  font-family: 'SF Mono', 'Fira Code', monospace;
  background: var(--bg-hover);
  padding: 1px 5px;
  border-radius: 3px;
  font-size: 11px;
}
</style>
