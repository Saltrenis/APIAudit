<script setup lang="ts">
import type { WizardState } from '../../types/audit'

const props = defineProps<{
  state: WizardState
}>()

const emit = defineEmits<{
  (e: 'update', patch: Partial<WizardState>): void
}>()

interface CommandDef {
  id: string
  label: string
  description: string
  recommended?: boolean
  icon: string
}

const commands: CommandDef[] = [
  {
    id: 'audit',
    label: 'Full Audit',
    description: 'Detect → Scan → Generate → Analyze → Report',
    recommended: true,
    icon: '⚡',
  },
  {
    id: 'scan',
    label: 'Scan Routes',
    description: 'Extract all route definitions from source code',
    icon: '🔍',
  },
  {
    id: 'detect',
    label: 'Detect Framework',
    description: 'Identify language, framework, and project structure',
    icon: '🧠',
  },
  {
    id: 'generate',
    label: 'Generate OpenAPI',
    description: 'Create an OpenAPI 3.0 spec from routes',
    icon: '📄',
  },
  {
    id: 'annotate',
    label: 'Annotate Routes',
    description: 'Find and fix missing swagger annotations',
    icon: '✏️',
  },
  {
    id: 'init',
    label: 'Initialize',
    description: 'Create .apiaudit.json config file',
    icon: '🚀',
  },
]
</script>

<template>
  <div class="step-content">
    <h2 class="step-title">What do you want to run?</h2>
    <p class="step-description">Select a command to execute on your project.</p>

    <div class="command-grid">
      <button
        v-for="cmd in commands"
        :key="cmd.id"
        class="command-card"
        :class="{
          'command-card--active': props.state.command === cmd.id,
          'command-card--recommended': cmd.recommended,
        }"
        @click="emit('update', { command: cmd.id })"
      >
        <div class="command-card-top">
          <span class="command-icon">{{ cmd.icon }}</span>
          <span v-if="cmd.recommended" class="recommended-badge">Recommended</span>
        </div>
        <div class="command-card-title">{{ cmd.label }}</div>
        <div class="command-card-desc">{{ cmd.description }}</div>
        <span class="command-id">apiaudit {{ cmd.id }}</span>
      </button>
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

.command-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}

.command-card {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
  padding: 18px;
  background: var(--bg-card);
  border: 2px solid var(--border);
  border-radius: var(--radius-md);
  cursor: pointer;
  text-align: left;
  transition: all 0.15s;
}

.command-card:hover {
  border-color: var(--accent);
  background: var(--bg-hover);
}

.command-card--active {
  border-color: var(--accent);
  background: rgba(99, 102, 241, 0.1);
  box-shadow: 0 0 0 1px var(--accent);
}

.command-card--recommended {
  border-color: rgba(99, 102, 241, 0.4);
}

.command-card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}

.command-icon {
  font-size: 22px;
  line-height: 1;
}

.recommended-badge {
  font-size: 10px;
  font-weight: 700;
  padding: 2px 7px;
  border-radius: 999px;
  background: rgba(99, 102, 241, 0.15);
  color: var(--accent-hover);
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.command-card-title {
  font-size: 15px;
  font-weight: 700;
  color: var(--text-primary);
}

.command-card--active .command-card-title {
  color: var(--accent-hover);
}

.command-card-desc {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.5;
  flex: 1;
}

.command-id {
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 11px;
  color: var(--text-muted);
  background: var(--bg-hover);
  padding: 2px 7px;
  border-radius: 4px;
  align-self: flex-start;
}

.command-card--active .command-id {
  background: rgba(99, 102, 241, 0.15);
  color: var(--accent);
}
</style>
