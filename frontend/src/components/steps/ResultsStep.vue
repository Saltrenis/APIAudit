<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import type { AuditReport } from '../../types/audit'
import type { RunStatus } from '../../composables/useAuditRunner'
import SummaryCard from '../results/SummaryCard.vue'
import RouteTable from '../results/RouteTable.vue'
import FindingsTable from '../results/FindingsTable.vue'

const props = defineProps<{
  status: RunStatus
  logLines: string[]
  result: AuditReport | null
  error: string | null
}>()

const emit = defineEmits<{
  (e: 'runAgain'): void
  (e: 'retry'): void
}>()

const activeTab = ref<'routes' | 'findings'>('findings')
const logContainer = ref<HTMLElement | null>(null)

watch(
  () => props.logLines.length,
  async () => {
    await nextTick()
    if (logContainer.value) {
      logContainer.value.scrollTop = logContainer.value.scrollHeight
    }
  }
)

function downloadReport() {
  if (!props.result) return
  const json = JSON.stringify(props.result, null, 2)
  const blob = new Blob([json], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'audit-report.json'
  a.click()
  URL.revokeObjectURL(url)
}
</script>

<template>
  <div class="results-step">
    <!-- Running state: streaming log -->
    <div v-if="props.status === 'running'" class="terminal-container">
      <div class="terminal-header">
        <div class="terminal-dots">
          <span class="dot dot--red" />
          <span class="dot dot--yellow" />
          <span class="dot dot--green" />
        </div>
        <span class="terminal-title">Running audit...</span>
        <span class="terminal-spinner">◌</span>
      </div>
      <div ref="logContainer" class="terminal-body">
        <div
          v-for="(line, i) in props.logLines"
          :key="i"
          class="log-line"
        >{{ line }}</div>
        <div class="log-cursor">▋</div>
      </div>
    </div>

    <!-- Error state -->
    <div v-else-if="props.status === 'error'" class="error-container">
      <div class="error-icon">✗</div>
      <h3 class="error-title">Audit Failed</h3>
      <p class="error-message">{{ props.error ?? 'An unknown error occurred.' }}</p>
      <div v-if="props.logLines.length > 0" class="error-log">
        <div class="error-log-label">Last output:</div>
        <div class="terminal-body terminal-body--short">
          <div
            v-for="(line, i) in props.logLines.slice(-20)"
            :key="i"
            class="log-line"
          >{{ line }}</div>
        </div>
      </div>
      <div class="error-actions">
        <button class="btn btn--secondary" @click="emit('retry')">Retry</button>
        <button class="btn btn--ghost" @click="emit('runAgain')">Change Settings</button>
      </div>
    </div>

    <!-- Done state: structured results -->
    <div v-else-if="props.status === 'done' && props.result" class="results-container">
      <!-- Summary -->
      <SummaryCard :report="props.result" />

      <!-- Tabs -->
      <div class="tabs">
        <button
          class="tab-btn"
          :class="{ 'tab-btn--active': activeTab === 'findings' }"
          @click="activeTab = 'findings'"
        >
          Findings
          <span class="tab-count" :class="{ 'tab-count--active': activeTab === 'findings' }">
            {{ props.result.findings?.length ?? 0 }}
          </span>
        </button>
        <button
          class="tab-btn"
          :class="{ 'tab-btn--active': activeTab === 'routes' }"
          @click="activeTab = 'routes'"
        >
          Routes
          <span class="tab-count" :class="{ 'tab-count--active': activeTab === 'routes' }">
            {{ props.result.routes?.length ?? 0 }}
          </span>
        </button>
      </div>

      <div class="tab-content">
        <FindingsTable v-if="activeTab === 'findings'" :findings="props.result.findings ?? []" />
        <RouteTable v-if="activeTab === 'routes'" :routes="props.result.routes ?? []" />
      </div>

      <!-- Actions -->
      <div class="results-actions">
        <button class="btn btn--primary" @click="downloadReport">
          <svg viewBox="0 0 20 20" fill="currentColor" class="btn-icon">
            <path fill-rule="evenodd" d="M3 17a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1zm3.293-7.707a1 1 0 011.414 0L9 10.586V3a1 1 0 112 0v7.586l1.293-1.293a1 1 0 111.414 1.414l-3 3a1 1 0 01-1.414 0l-3-3a1 1 0 010-1.414z" clip-rule="evenodd"/>
          </svg>
          Download Report
        </button>
        <button class="btn btn--secondary" @click="emit('runAgain')">Run Again</button>
      </div>
    </div>

    <!-- Done but no result -->
    <div v-else-if="props.status === 'done' && !props.result" class="error-container">
      <div class="error-icon">⚠</div>
      <h3 class="error-title">No Results</h3>
      <p class="error-message">The audit completed but returned no structured data.</p>
      <div v-if="props.logLines.length > 0" class="error-log">
        <div class="terminal-body terminal-body--short">
          <div v-for="(line, i) in props.logLines" :key="i" class="log-line">{{ line }}</div>
        </div>
      </div>
      <button class="btn btn--secondary" @click="emit('runAgain')">Start Over</button>
    </div>
  </div>
</template>

<style scoped>
.results-step {
  display: flex;
  flex-direction: column;
  gap: 0;
}

/* Terminal */
.terminal-container {
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  overflow: hidden;
  background: #0a0d14;
}

.terminal-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 16px;
  background: #12151f;
  border-bottom: 1px solid var(--border);
}

.terminal-dots {
  display: flex;
  gap: 6px;
}

.dot {
  width: 11px;
  height: 11px;
  border-radius: 50%;
}

.dot--red { background: #ef4444; }
.dot--yellow { background: #f59e0b; }
.dot--green { background: #22c55e; }

.terminal-title {
  flex: 1;
  font-size: 12px;
  color: var(--text-muted);
  font-family: 'SF Mono', 'Fira Code', monospace;
}

.terminal-spinner {
  font-size: 14px;
  color: var(--accent);
  animation: spin 1.2s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.terminal-body {
  padding: 16px;
  font-family: 'SF Mono', 'Fira Code', 'Fira Mono', 'Roboto Mono', monospace;
  font-size: 13px;
  line-height: 1.6;
  color: #a8b0c0;
  height: 340px;
  overflow-y: auto;
}

.terminal-body--short {
  max-height: 200px;
  height: auto;
  margin-top: 8px;
  border-radius: var(--radius-sm);
}

.log-line {
  white-space: pre-wrap;
  word-break: break-all;
}

.log-cursor {
  display: inline-block;
  color: var(--accent);
  animation: blink 1s step-end infinite;
}

@keyframes blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0; }
}

/* Error state */
.error-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  padding: 40px 24px;
  text-align: center;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--bg-card);
}

.error-icon {
  font-size: 36px;
  color: var(--error);
  font-weight: 300;
  line-height: 1;
}

.error-title {
  font-size: 18px;
  color: var(--text-primary);
}

.error-message {
  font-size: 14px;
  color: var(--text-secondary);
  max-width: 480px;
}

.error-log {
  width: 100%;
  max-width: 600px;
  text-align: left;
}

.error-log-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.06em;
  margin-bottom: 6px;
}

.error-actions {
  display: flex;
  gap: 10px;
}

/* Results */
.results-container {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.tabs {
  display: flex;
  gap: 0;
  border-bottom: 1px solid var(--border);
}

.tab-btn {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 20px;
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  color: var(--text-muted);
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  margin-bottom: -1px;
  transition: all 0.15s;
}

.tab-btn:hover {
  color: var(--text-primary);
}

.tab-btn--active {
  color: var(--text-primary);
  border-bottom-color: var(--accent);
}

.tab-count {
  font-size: 11px;
  padding: 2px 7px;
  border-radius: 999px;
  background: var(--bg-hover);
  color: var(--text-muted);
  font-weight: 700;
}

.tab-count--active {
  background: rgba(99, 102, 241, 0.15);
  color: var(--accent-hover);
}

.tab-content {
  min-height: 200px;
}

.results-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding-top: 8px;
  border-top: 1px solid var(--border);
}

/* Buttons */
.btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 9px 20px;
  border-radius: var(--radius-sm);
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s;
  border: 1px solid transparent;
}

.btn-icon {
  width: 16px;
  height: 16px;
}

.btn--primary {
  background: var(--accent);
  color: #fff;
  border-color: var(--accent);
}

.btn--primary:hover {
  background: var(--accent-hover);
  border-color: var(--accent-hover);
}

.btn--secondary {
  background: var(--bg-card);
  color: var(--text-primary);
  border-color: var(--border);
}

.btn--secondary:hover {
  border-color: var(--accent);
  color: var(--accent-hover);
}

.btn--ghost {
  background: none;
  color: var(--text-secondary);
  border-color: transparent;
}

.btn--ghost:hover {
  color: var(--text-primary);
}
</style>
