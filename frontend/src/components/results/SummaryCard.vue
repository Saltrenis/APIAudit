<script setup lang="ts">
import type { AuditReport } from '../../types/audit'

const props = defineProps<{
  report: AuditReport
}>()
</script>

<template>
  <div class="summary-card">
    <div class="framework-row">
      <div class="framework-badge">
        <span class="fw-language">{{ props.report.framework.language }}</span>
        <span class="fw-separator">/</span>
        <span class="fw-name">{{ props.report.framework.framework }}</span>
        <span v-if="props.report.framework.version" class="fw-version">
          v{{ props.report.framework.version }}
        </span>
      </div>
      <div class="confidence">
        <span class="confidence-label">Confidence</span>
        <span class="confidence-value">{{ props.report.framework.confidence }}%</span>
      </div>
    </div>

    <div class="metrics-row">
      <div class="metric">
        <span class="metric-value">{{ props.report.summary.totalRoutes }}</span>
        <span class="metric-label">Total Routes</span>
      </div>
      <div class="metric">
        <span class="metric-value">{{ props.report.summary.documented }}</span>
        <span class="metric-label">Documented</span>
      </div>
      <div class="metric">
        <span
          class="metric-value"
          :class="{
            'color-success': props.report.summary.coveragePct >= 80,
            'color-warning': props.report.summary.coveragePct >= 50 && props.report.summary.coveragePct < 80,
            'color-error': props.report.summary.coveragePct < 50,
          }"
        >{{ props.report.summary.coveragePct.toFixed(1) }}%</span>
        <span class="metric-label">Coverage</span>
      </div>
      <div class="metric-divider" />
      <div class="metric severity-metric">
        <span class="metric-value color-error">{{ props.report.summary.p1 }}</span>
        <span class="metric-label severity-label p1">P1</span>
      </div>
      <div class="metric severity-metric">
        <span class="metric-value color-warning">{{ props.report.summary.p2 }}</span>
        <span class="metric-label severity-label p2">P2</span>
      </div>
      <div class="metric severity-metric">
        <span class="metric-value color-yellow">{{ props.report.summary.p3 }}</span>
        <span class="metric-label severity-label p3">P3</span>
      </div>
      <div class="metric severity-metric">
        <span class="metric-value color-muted">{{ props.report.summary.p4 }}</span>
        <span class="metric-label severity-label p4">P4</span>
      </div>
    </div>

    <div v-if="props.report.framework.hasFrontend" class="tags-row">
      <span class="tag">Has Frontend</span>
      <span v-if="props.report.framework.frontendDir" class="tag tag-secondary">
        Frontend: {{ props.report.framework.frontendDir }}
      </span>
      <span v-if="props.report.framework.hasSwagger" class="tag tag-success">Swagger</span>
    </div>
  </div>
</template>

<style scoped>
.summary-card {
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  padding: 20px 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.framework-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.framework-badge {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 15px;
  font-weight: 600;
}

.fw-language {
  color: var(--accent-hover);
}

.fw-separator {
  color: var(--text-muted);
}

.fw-name {
  color: var(--text-primary);
}

.fw-version {
  font-size: 12px;
  color: var(--text-muted);
  background: var(--bg-hover);
  padding: 2px 7px;
  border-radius: 999px;
  font-weight: 400;
}

.confidence {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
}

.confidence-label {
  color: var(--text-muted);
}

.confidence-value {
  color: var(--text-secondary);
  font-weight: 600;
}

.metrics-row {
  display: flex;
  align-items: center;
  gap: 24px;
}

.metric {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 3px;
}

.metric-value {
  font-size: 22px;
  font-weight: 700;
  color: var(--text-primary);
  line-height: 1;
}

.metric-label {
  font-size: 11px;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.metric-divider {
  width: 1px;
  height: 36px;
  background: var(--border);
  margin: 0 4px;
}

.severity-metric .metric-value {
  font-size: 20px;
}

.severity-label {
  font-weight: 700;
  font-size: 11px;
}

.severity-label.p1 { color: var(--error); }
.severity-label.p2 { color: var(--warning); }
.severity-label.p3 { color: #eab308; }
.severity-label.p4 { color: var(--text-muted); }

.color-success { color: var(--success); }
.color-warning { color: var(--warning); }
.color-error { color: var(--error); }
.color-yellow { color: #eab308; }
.color-muted { color: var(--text-muted); }

.tags-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.tag {
  font-size: 12px;
  padding: 3px 10px;
  border-radius: 999px;
  background: var(--bg-hover);
  border: 1px solid var(--border);
  color: var(--text-secondary);
}

.tag-success {
  background: rgba(34, 197, 94, 0.1);
  border-color: rgba(34, 197, 94, 0.3);
  color: var(--success);
}

.tag-secondary {
  color: var(--text-muted);
}
</style>
