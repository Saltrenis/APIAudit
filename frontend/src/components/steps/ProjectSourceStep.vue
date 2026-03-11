<script setup lang="ts">
import type { WizardState } from '../../types/audit'

const props = defineProps<{
  state: WizardState
}>()

const emit = defineEmits<{
  (e: 'update', patch: Partial<WizardState>): void
}>()

function setSourceType(type: 'local' | 'repo') {
  emit('update', { sourceType: type })
}
</script>

<template>
  <div class="step-content">
    <h2 class="step-title">Where is your project?</h2>
    <p class="step-description">Choose a local directory or a remote Git repository to audit.</p>

    <div class="source-cards">
      <button
        class="source-card"
        :class="{ 'source-card--active': props.state.sourceType === 'local' }"
        @click="setSourceType('local')"
      >
        <div class="source-card-icon">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <path d="M3 7a2 2 0 012-2h14a2 2 0 012 2v10a2 2 0 01-2 2H5a2 2 0 01-2-2V7z"/>
            <path d="M16 3v4M8 3v4M3 11h18"/>
          </svg>
        </div>
        <div class="source-card-body">
          <span class="source-card-title">Local Directory</span>
          <span class="source-card-sub">Audit code on your filesystem</span>
        </div>
        <span class="source-card-radio" :class="{ 'source-card-radio--active': props.state.sourceType === 'local' }" />
      </button>

      <button
        class="source-card"
        :class="{ 'source-card--active': props.state.sourceType === 'repo' }"
        @click="setSourceType('repo')"
      >
        <div class="source-card-icon">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <circle cx="12" cy="18" r="3"/>
            <circle cx="6" cy="6" r="3"/>
            <circle cx="18" cy="6" r="3"/>
            <path d="M18 9v1a2 2 0 01-2 2H8a2 2 0 01-2-2V9M12 15V9"/>
          </svg>
        </div>
        <div class="source-card-body">
          <span class="source-card-title">Git Repository</span>
          <span class="source-card-sub">Clone and audit from a URL</span>
        </div>
        <span class="source-card-radio" :class="{ 'source-card-radio--active': props.state.sourceType === 'repo' }" />
      </button>
    </div>

    <div class="fields">
      <div v-if="props.state.sourceType === 'local'" class="field">
        <label class="field-label" for="dir-input">Directory Path <span class="required">*</span></label>
        <input
          id="dir-input"
          class="field-input"
          type="text"
          placeholder="/path/to/your/project"
          :value="props.state.dir"
          @input="emit('update', { dir: ($event.target as HTMLInputElement).value })"
        />
        <span class="field-hint">Absolute path to your project root</span>
      </div>

      <div v-if="props.state.sourceType === 'repo'" class="field">
        <label class="field-label" for="repo-input">Repository URL <span class="required">*</span></label>
        <input
          id="repo-input"
          class="field-input"
          type="text"
          placeholder="https://github.com/owner/repo"
          :value="props.state.repo"
          @input="emit('update', { repo: ($event.target as HTMLInputElement).value })"
        />
        <span class="field-hint">HTTPS or SSH Git URL</span>
      </div>

      <div class="field">
        <label class="field-label" for="frontend-dir-input">Frontend Directory <span class="optional">(optional)</span></label>
        <input
          id="frontend-dir-input"
          class="field-input"
          type="text"
          placeholder="frontend/ or client/"
          :value="props.state.frontendDir"
          @input="emit('update', { frontendDir: ($event.target as HTMLInputElement).value })"
        />
        <span class="field-hint">Subdirectory containing the frontend code, if separate</span>
      </div>
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

.source-cards {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.source-card {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 16px 18px;
  background: var(--bg-card);
  border: 2px solid var(--border);
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: all 0.15s;
  text-align: left;
}

.source-card:hover {
  border-color: var(--accent);
  background: var(--bg-hover);
}

.source-card--active {
  border-color: var(--accent);
  background: rgba(99, 102, 241, 0.08);
}

.source-card-icon {
  flex-shrink: 0;
  width: 36px;
  height: 36px;
  border-radius: var(--radius-sm);
  background: var(--bg-hover);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-secondary);
}

.source-card--active .source-card-icon {
  background: rgba(99, 102, 241, 0.15);
  color: var(--accent);
}

.source-card-icon svg {
  width: 18px;
  height: 18px;
}

.source-card-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.source-card-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
}

.source-card-sub {
  font-size: 12px;
  color: var(--text-muted);
}

.source-card-radio {
  width: 16px;
  height: 16px;
  border-radius: 50%;
  border: 2px solid var(--border);
  flex-shrink: 0;
  transition: all 0.15s;
  position: relative;
}

.source-card-radio--active {
  border-color: var(--accent);
  background: var(--accent);
  box-shadow: inset 0 0 0 3px var(--bg-card);
}

.fields {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.field-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-secondary);
}

.required {
  color: var(--error);
}

.optional {
  color: var(--text-muted);
  font-weight: 400;
  font-size: 12px;
}

.field-input {
  padding: 9px 12px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  color: var(--text-primary);
  font-size: 14px;
  outline: none;
  transition: border-color 0.15s;
}

.field-input::placeholder {
  color: var(--text-muted);
}

.field-input:focus {
  border-color: var(--accent);
}

.field-hint {
  font-size: 12px;
  color: var(--text-muted);
}
</style>
