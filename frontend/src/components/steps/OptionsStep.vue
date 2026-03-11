<script setup lang="ts">
import type { WizardState } from '../../types/audit'

const props = defineProps<{
  state: WizardState
}>()

const emit = defineEmits<{
  (e: 'update', patch: Partial<WizardState>): void
}>()

function update(patch: Partial<WizardState>) {
  emit('update', patch)
}

const FORMAT_OPTIONS = [
  { value: 'table', label: 'Table' },
  { value: 'json', label: 'JSON' },
  { value: 'markdown', label: 'Markdown' },
]
</script>

<template>
  <div class="step-content">
    <h2 class="step-title">Configure Options</h2>
    <p class="step-description">Customize how the command runs.</p>

    <!-- Global Options -->
    <section class="options-section">
      <h3 class="section-title">Global Options</h3>

      <div class="options-grid">
        <div class="field">
          <label class="field-label" for="format-select">Output Format</label>
          <select
            id="format-select"
            class="field-select"
            :value="props.state.format"
            @change="update({ format: ($event.target as HTMLSelectElement).value })"
          >
            <option v-for="opt in FORMAT_OPTIONS" :key="opt.value" :value="opt.value">
              {{ opt.label }}
            </option>
          </select>
        </div>

        <div class="field">
          <label class="field-label" for="output-input">Output File <span class="optional">(optional)</span></label>
          <input
            id="output-input"
            class="field-input"
            type="text"
            placeholder="report.json"
            :value="props.state.output"
            @input="update({ output: ($event.target as HTMLInputElement).value })"
          />
        </div>
      </div>

      <div class="toggles-row">
        <label class="toggle-row">
          <div class="toggle-info">
            <span class="toggle-label">Beads Integration</span>
            <span class="toggle-hint">Create beads issues from findings</span>
          </div>
          <button
            class="toggle-btn"
            :class="{ 'toggle-btn--on': props.state.beads }"
            @click="update({ beads: !props.state.beads })"
            :aria-checked="props.state.beads"
            role="switch"
          >
            <span class="toggle-thumb" />
          </button>
        </label>

        <div v-if="props.state.beads" class="field field--inline">
          <label class="field-label" for="beads-limit">Beads Limit</label>
          <input
            id="beads-limit"
            class="field-input field-input--narrow"
            type="number"
            min="1"
            max="100"
            :value="props.state.beadsLimit"
            @input="update({ beadsLimit: parseInt(($event.target as HTMLInputElement).value) || 10 })"
          />
        </div>

        <label class="toggle-row">
          <div class="toggle-info">
            <span class="toggle-label">AI Assist</span>
            <span class="toggle-hint">Use AI to generate suggestions</span>
          </div>
          <button
            class="toggle-btn"
            :class="{ 'toggle-btn--on': props.state.aiAssist }"
            @click="update({ aiAssist: !props.state.aiAssist })"
            :aria-checked="props.state.aiAssist"
            role="switch"
          >
            <span class="toggle-thumb" />
          </button>
        </label>
      </div>
    </section>

    <!-- Audit Options -->
    <section v-if="props.state.command === 'audit'" class="options-section">
      <h3 class="section-title">Audit Options</h3>
      <div class="toggles-row">
        <label class="toggle-row">
          <div class="toggle-info">
            <span class="toggle-label">Skip Frontend</span>
            <span class="toggle-hint">Do not scan frontend directory</span>
          </div>
          <button
            class="toggle-btn"
            :class="{ 'toggle-btn--on': props.state.skipFrontend }"
            @click="update({ skipFrontend: !props.state.skipFrontend })"
            role="switch"
            :aria-checked="props.state.skipFrontend"
          >
            <span class="toggle-thumb" />
          </button>
        </label>

        <label class="toggle-row">
          <div class="toggle-info">
            <span class="toggle-label">Skip Generate</span>
            <span class="toggle-hint">Skip OpenAPI spec generation step</span>
          </div>
          <button
            class="toggle-btn"
            :class="{ 'toggle-btn--on': props.state.skipGenerate }"
            @click="update({ skipGenerate: !props.state.skipGenerate })"
            role="switch"
            :aria-checked="props.state.skipGenerate"
          >
            <span class="toggle-thumb" />
          </button>
        </label>

        <label class="toggle-row">
          <div class="toggle-info">
            <span class="toggle-label">Static Only</span>
            <span class="toggle-hint">Only static analysis, no runtime detection</span>
          </div>
          <button
            class="toggle-btn"
            :class="{ 'toggle-btn--on': props.state.staticOnly }"
            @click="update({ staticOnly: !props.state.staticOnly })"
            role="switch"
            :aria-checked="props.state.staticOnly"
          >
            <span class="toggle-thumb" />
          </button>
        </label>
      </div>
    </section>

    <!-- Generate Options -->
    <section v-if="props.state.command === 'generate'" class="options-section">
      <h3 class="section-title">Generate Options</h3>
      <div class="options-grid">
        <div class="field">
          <label class="field-label" for="title-input">API Title</label>
          <input
            id="title-input"
            class="field-input"
            type="text"
            placeholder="My API"
            :value="props.state.title"
            @input="update({ title: ($event.target as HTMLInputElement).value })"
          />
        </div>
        <div class="field">
          <label class="field-label" for="api-version-input">API Version</label>
          <input
            id="api-version-input"
            class="field-input"
            type="text"
            placeholder="1.0.0"
            :value="props.state.apiVersion"
            @input="update({ apiVersion: ($event.target as HTMLInputElement).value })"
          />
        </div>
        <div class="field field--full">
          <label class="field-label" for="description-input">Description <span class="optional">(optional)</span></label>
          <textarea
            id="description-input"
            class="field-textarea"
            placeholder="A brief description of your API"
            :value="props.state.description"
            @input="update({ description: ($event.target as HTMLTextAreaElement).value })"
          />
        </div>
      </div>
      <div class="toggles-row">
        <label class="toggle-row">
          <div class="toggle-info">
            <span class="toggle-label">JSON Format</span>
            <span class="toggle-hint">Output as JSON instead of YAML</span>
          </div>
          <button
            class="toggle-btn"
            :class="{ 'toggle-btn--on': props.state.jsonFormat }"
            @click="update({ jsonFormat: !props.state.jsonFormat })"
            role="switch"
            :aria-checked="props.state.jsonFormat"
          >
            <span class="toggle-thumb" />
          </button>
        </label>
      </div>
    </section>

    <!-- Annotate Options -->
    <section v-if="props.state.command === 'annotate'" class="options-section">
      <h3 class="section-title">Annotate Options</h3>
      <div class="toggles-row">
        <label class="toggle-row">
          <div class="toggle-info">
            <span class="toggle-label">Dry Run</span>
            <span class="toggle-hint">Preview changes without writing to files</span>
          </div>
          <button
            class="toggle-btn"
            :class="{ 'toggle-btn--on': props.state.dryRun }"
            @click="update({ dryRun: !props.state.dryRun })"
            role="switch"
            :aria-checked="props.state.dryRun"
          >
            <span class="toggle-thumb" />
          </button>
        </label>
      </div>
    </section>
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

.options-section {
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.section-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.08em;
  margin-bottom: 4px;
}

.options-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.field--full {
  grid-column: 1 / -1;
}

.field--inline {
  flex-direction: row;
  align-items: center;
  gap: 10px;
}

.field-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-secondary);
}

.optional {
  color: var(--text-muted);
  font-weight: 400;
  font-size: 12px;
}

.field-input {
  padding: 8px 12px;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  color: var(--text-primary);
  font-size: 14px;
  outline: none;
  transition: border-color 0.15s;
}

.field-input:focus {
  border-color: var(--accent);
}

.field-input::placeholder {
  color: var(--text-muted);
}

.field-input--narrow {
  width: 80px;
  text-align: center;
}

.field-select {
  padding: 8px 12px;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  color: var(--text-primary);
  font-size: 14px;
  outline: none;
  cursor: pointer;
  transition: border-color 0.15s;
  appearance: none;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 16 16'%3E%3Cpath fill='%2371717a' d='M4 6l4 4 4-4'/%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: right 10px center;
  background-size: 14px;
  padding-right: 32px;
}

.field-select:focus {
  border-color: var(--accent);
}

.field-textarea {
  padding: 8px 12px;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  color: var(--text-primary);
  font-size: 14px;
  outline: none;
  transition: border-color 0.15s;
  resize: vertical;
  min-height: 80px;
  line-height: 1.5;
}

.field-textarea:focus {
  border-color: var(--accent);
}

.field-textarea::placeholder {
  color: var(--text-muted);
}

.toggles-row {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.toggle-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: border-color 0.15s;
  gap: 12px;
}

.toggle-row:hover {
  border-color: var(--accent);
}

.toggle-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.toggle-label {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
}

.toggle-hint {
  font-size: 12px;
  color: var(--text-muted);
}

.toggle-btn {
  flex-shrink: 0;
  width: 42px;
  height: 24px;
  border-radius: 999px;
  border: none;
  background: var(--border);
  cursor: pointer;
  transition: background 0.2s;
  position: relative;
  padding: 0;
}

.toggle-btn--on {
  background: var(--accent);
}

.toggle-thumb {
  position: absolute;
  top: 3px;
  left: 3px;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: #fff;
  transition: transform 0.2s;
  display: block;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3);
}

.toggle-btn--on .toggle-thumb {
  transform: translateX(18px);
}
</style>
