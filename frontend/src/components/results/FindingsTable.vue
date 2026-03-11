<script setup lang="ts">
import { ref, computed } from 'vue'
import type { Finding } from '../../types/audit'

const props = defineProps<{
  findings: Finding[]
}>()

const expandedFindings = ref<Set<string>>(new Set())
const collapsedGroups = ref<Set<string>>(new Set())

const SEVERITY_ORDER = ['P1', 'P2', 'P3', 'P4']

const severityColors: Record<string, string> = {
  P1: 'var(--error)',
  P2: 'var(--warning)',
  P3: '#eab308',
  P4: 'var(--text-muted)',
}

const severityBg: Record<string, string> = {
  P1: 'rgba(239,68,68,0.12)',
  P2: 'rgba(245,158,11,0.12)',
  P3: 'rgba(234,179,8,0.12)',
  P4: 'rgba(113,113,122,0.12)',
}

function findingKey(f: Finding, idx: number): string {
  return `${f.severity}-${f.category}-${idx}`
}

const groupedFindings = computed(() => {
  const groups: Record<string, { finding: Finding; idx: number }[]> = {}

  SEVERITY_ORDER.forEach(s => {
    groups[s] = []
  })

  props.findings.forEach((f, idx) => {
    const sev = (f.severity ?? 'P4').toUpperCase()
    if (!groups[sev]) groups[sev] = []
    groups[sev].push({ finding: f, idx })
  })

  return SEVERITY_ORDER
    .filter(s => groups[s].length > 0)
    .map(s => ({ severity: s, items: groups[s] }))
})

function toggleGroup(severity: string) {
  if (collapsedGroups.value.has(severity)) {
    collapsedGroups.value.delete(severity)
  } else {
    collapsedGroups.value.add(severity)
  }
}

function toggleFinding(key: string) {
  if (expandedFindings.value.has(key)) {
    expandedFindings.value.delete(key)
  } else {
    expandedFindings.value.add(key)
  }
}

function isGroupCollapsed(severity: string): boolean {
  return collapsedGroups.value.has(severity)
}

function isFindingExpanded(key: string): boolean {
  return expandedFindings.value.has(key)
}
</script>

<template>
  <div class="findings-wrapper">
    <div v-if="props.findings.length === 0" class="empty-state">
      <span class="empty-icon">✓</span>
      <span>No findings — clean audit!</span>
    </div>

    <div
      v-for="group in groupedFindings"
      :key="group.severity"
      class="severity-group"
    >
      <button
        class="group-header"
        :style="{ borderLeftColor: severityColors[group.severity] }"
        @click="toggleGroup(group.severity)"
      >
        <div class="group-header-left">
          <span
            class="severity-badge"
            :style="{ color: severityColors[group.severity], background: severityBg[group.severity] }"
          >
            {{ group.severity }}
          </span>
          <span class="group-count-badge" :style="{ background: severityBg[group.severity], color: severityColors[group.severity] }">
            {{ group.items.length }}
          </span>
        </div>
        <span class="collapse-icon">{{ isGroupCollapsed(group.severity) ? '▶' : '▼' }}</span>
      </button>

      <div v-if="!isGroupCollapsed(group.severity)" class="group-items">
        <div
          v-for="{ finding, idx } in group.items"
          :key="findingKey(finding, idx)"
          class="finding-item"
        >
          <div
            class="finding-main"
            @click="finding.suggestion ? toggleFinding(findingKey(finding, idx)) : undefined"
            :class="{ clickable: !!finding.suggestion }"
          >
            <div class="finding-meta">
              <span class="category-badge">{{ finding.category }}</span>
              <span v-if="finding.file" class="finding-location">
                {{ finding.file }}<span v-if="finding.line">:{{ finding.line }}</span>
              </span>
            </div>
            <p class="finding-message">{{ finding.message }}</p>
            <span v-if="finding.suggestion" class="expand-hint">
              {{ isFindingExpanded(findingKey(finding, idx)) ? 'Hide suggestion ▲' : 'Show suggestion ▼' }}
            </span>
          </div>

          <div
            v-if="finding.suggestion && isFindingExpanded(findingKey(finding, idx))"
            class="finding-suggestion"
          >
            <span class="suggestion-label">Suggestion</span>
            <p class="suggestion-text">{{ finding.suggestion }}</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.findings-wrapper {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.empty-state {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 32px;
  text-align: center;
  justify-content: center;
  color: var(--success);
  font-size: 15px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--bg-card);
}

.empty-icon {
  font-size: 20px;
}

.severity-group {
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  overflow: hidden;
}

.group-header {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  background: var(--bg-card);
  border: none;
  border-left: 3px solid transparent;
  cursor: pointer;
  text-align: left;
  transition: background 0.15s;
}

.group-header:hover {
  background: var(--bg-hover);
}

.group-header-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.severity-badge {
  font-size: 12px;
  font-weight: 700;
  padding: 3px 10px;
  border-radius: 999px;
  letter-spacing: 0.05em;
}

.group-count-badge {
  font-size: 12px;
  font-weight: 700;
  padding: 2px 8px;
  border-radius: 999px;
}

.collapse-icon {
  font-size: 10px;
  color: var(--text-muted);
}

.group-items {
  border-top: 1px solid var(--border);
}

.finding-item {
  border-bottom: 1px solid var(--border);
}

.finding-item:last-child {
  border-bottom: none;
}

.finding-main {
  padding: 12px 16px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  transition: background 0.15s;
}

.finding-main.clickable {
  cursor: pointer;
}

.finding-main.clickable:hover {
  background: var(--bg-hover);
}

.finding-meta {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.category-badge {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  background: var(--bg-hover);
  color: var(--text-secondary);
  border: 1px solid var(--border);
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.finding-location {
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 12px;
  color: var(--text-muted);
}

.finding-message {
  font-size: 13px;
  color: var(--text-primary);
  line-height: 1.5;
  margin: 0;
}

.expand-hint {
  font-size: 11px;
  color: var(--accent);
  cursor: pointer;
}

.finding-suggestion {
  padding: 12px 16px;
  background: var(--bg-secondary);
  border-top: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.suggestion-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--accent);
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.suggestion-text {
  font-size: 13px;
  color: var(--text-secondary);
  line-height: 1.6;
  margin: 0;
}
</style>
