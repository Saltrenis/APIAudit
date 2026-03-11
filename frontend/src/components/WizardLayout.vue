<script setup lang="ts">
import { ref, computed } from 'vue'
import type { WizardStep, WizardState } from '../types/audit'
import { useAuditRunner } from '../composables/useAuditRunner'
import StepIndicator from './StepIndicator.vue'
import ProjectSourceStep from './steps/ProjectSourceStep.vue'
import CommandStep from './steps/CommandStep.vue'
import OptionsStep from './steps/OptionsStep.vue'
import ReviewStep from './steps/ReviewStep.vue'
import ResultsStep from './steps/ResultsStep.vue'

// ── Wizard state ────────────────────────────────────────────────────────────

const wizardState = ref<WizardState>({
  sourceType: 'local',
  dir: '',
  repo: '',
  frontendDir: '',
  command: 'audit',
  format: 'table',
  output: '',
  skipFrontend: false,
  skipGenerate: false,
  staticOnly: false,
  dryRun: false,
  jsonFormat: false,
  beads: false,
  beadsLimit: 10,
  aiAssist: false,
  title: '',
  description: '',
  apiVersion: '1.0.0',
})

function updateState(patch: Partial<WizardState>) {
  wizardState.value = { ...wizardState.value, ...patch }
}

// ── Step machine ─────────────────────────────────────────────────────────────

const STEP_ORDER: WizardStep[] = ['source', 'command', 'options', 'review', 'results']

const currentStep = ref<WizardStep>('source')
const completedSteps = ref<WizardStep[]>([])

function currentStepIndex(): number {
  return STEP_ORDER.indexOf(currentStep.value)
}

function markCompleted(step: WizardStep) {
  if (!completedSteps.value.includes(step)) {
    completedSteps.value.push(step)
  }
}

// ── Validation ────────────────────────────────────────────────────────────────

const validationError = ref<string | null>(null)

function validateCurrentStep(): boolean {
  validationError.value = null

  if (currentStep.value === 'source') {
    if (wizardState.value.sourceType === 'local' && !wizardState.value.dir.trim()) {
      validationError.value = 'Please enter a directory path.'
      return false
    }
    if (wizardState.value.sourceType === 'repo' && !wizardState.value.repo.trim()) {
      validationError.value = 'Please enter a repository URL.'
      return false
    }
  }

  if (currentStep.value === 'command') {
    if (!wizardState.value.command) {
      validationError.value = 'Please select a command.'
      return false
    }
  }

  return true
}

// ── Navigation ────────────────────────────────────────────────────────────────

function goNext() {
  if (!validateCurrentStep()) return

  const idx = currentStepIndex()
  if (idx < STEP_ORDER.length - 1) {
    markCompleted(currentStep.value)
    currentStep.value = STEP_ORDER[idx + 1]
  }
}

function goBack() {
  const idx = currentStepIndex()
  if (idx > 0) {
    validationError.value = null
    currentStep.value = STEP_ORDER[idx - 1]
  }
}

function goTo(step: WizardStep) {
  if (completedSteps.value.includes(step)) {
    validationError.value = null
    currentStep.value = step
  }
}

// ── Audit runner ──────────────────────────────────────────────────────────────

const { status, logLines, result, error, run, reset } = useAuditRunner()

async function handleRun() {
  markCompleted('review')
  currentStep.value = 'results'

  const s = wizardState.value
  const flags: Record<string, string> = {}
  const boolFlags: string[] = []

  if (s.format && s.format !== 'table') flags['format'] = s.format
  if (s.output) flags['output'] = s.output
  if (s.beads) boolFlags.push('beads')
  if (s.aiAssist) boolFlags.push('ai-assist')

  if (s.command === 'audit') {
    if (s.skipFrontend) boolFlags.push('skip-frontend')
    if (s.skipGenerate) boolFlags.push('skip-generate')
    if (s.staticOnly) boolFlags.push('static-only')
  }

  if (s.command === 'generate') {
    if (s.title) flags['title'] = s.title
    if (s.description) flags['description'] = s.description
    if (s.apiVersion) flags['api-version'] = s.apiVersion
    if (s.jsonFormat) boolFlags.push('json')
  }

  if (s.command === 'annotate') {
    if (s.dryRun) boolFlags.push('dry-run')
  }

  await run({
    command: s.command,
    dir: s.sourceType === 'local' ? s.dir : '',
    repo: s.sourceType === 'repo' ? s.repo : '',
    frontendDir: s.frontendDir,
    format: s.format,
    output: s.output,
    flags,
    boolFlags,
    beadsLimit: s.beads ? s.beadsLimit : 0,
  })
}

function handleRunAgain() {
  reset()
  completedSteps.value = []
  currentStep.value = 'source'
}

function handleRetry() {
  handleRun()
}

// ── UI state ──────────────────────────────────────────────────────────────────

const canGoNext = computed(() => currentStep.value !== 'review' && currentStep.value !== 'results')
const canGoBack = computed(() => currentStepIndex() > 0 && currentStep.value !== 'results')
const isLastBeforeRun = computed(() => currentStep.value === 'review')
</script>

<template>
  <div class="wizard-page">
    <header class="wizard-header">
      <div class="wizard-header-inner">
        <div class="logo">
          <svg viewBox="0 0 28 28" fill="none" class="logo-icon">
            <rect width="28" height="28" rx="7" fill="#6366f1"/>
            <path d="M7 8h14M7 14h10M7 20h6" stroke="#fff" stroke-width="2.5" stroke-linecap="round"/>
            <circle cx="21" cy="20" r="3.5" fill="#22c55e" stroke="#6366f1" stroke-width="1.5"/>
          </svg>
          <span class="logo-name">APIAudit</span>
        </div>
        <span class="header-sub">API Contract Analyzer</span>
      </div>
    </header>

    <main class="wizard-main">
      <div class="wizard-container">
        <StepIndicator
          :current-step="currentStep"
          :completed-steps="completedSteps"
          @go-to="goTo"
        />

        <div class="wizard-body">
          <ProjectSourceStep
            v-if="currentStep === 'source'"
            :state="wizardState"
            @update="updateState"
          />

          <CommandStep
            v-else-if="currentStep === 'command'"
            :state="wizardState"
            @update="updateState"
          />

          <OptionsStep
            v-else-if="currentStep === 'options'"
            :state="wizardState"
            @update="updateState"
          />

          <ReviewStep
            v-else-if="currentStep === 'review'"
            :state="wizardState"
            @update="updateState"
            @run="handleRun"
          />

          <ResultsStep
            v-else-if="currentStep === 'results'"
            :status="status"
            :log-lines="logLines"
            :result="result"
            :error="error"
            @run-again="handleRunAgain"
            @retry="handleRetry"
          />
        </div>

        <!-- Validation error -->
        <div v-if="validationError" class="validation-error" role="alert">
          <svg viewBox="0 0 16 16" fill="currentColor" class="err-icon">
            <path d="M8 1a7 7 0 100 14A7 7 0 008 1zm.75 3.75a.75.75 0 00-1.5 0v3.5a.75.75 0 001.5 0v-3.5zm-.75 6a.75.75 0 100 1.5.75.75 0 000-1.5z"/>
          </svg>
          {{ validationError }}
        </div>

        <!-- Navigation buttons -->
        <div v-if="currentStep !== 'results'" class="wizard-nav">
          <button
            v-if="canGoBack"
            class="nav-btn nav-btn--back"
            @click="goBack"
          >
            ← Back
          </button>
          <div class="nav-spacer" />
          <button
            v-if="canGoNext && !isLastBeforeRun"
            class="nav-btn nav-btn--next"
            @click="goNext"
          >
            Continue →
          </button>
        </div>
      </div>
    </main>
  </div>
</template>

<style scoped>
.wizard-page {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--bg-primary);
}

/* Header */
.wizard-header {
  border-bottom: 1px solid var(--border);
  background: var(--bg-secondary);
  position: sticky;
  top: 0;
  z-index: 10;
}

.wizard-header-inner {
  max-width: 900px;
  margin: 0 auto;
  padding: 14px 24px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.logo {
  display: flex;
  align-items: center;
  gap: 10px;
}

.logo-icon {
  width: 28px;
  height: 28px;
}

.logo-name {
  font-size: 18px;
  font-weight: 800;
  color: var(--text-primary);
  letter-spacing: -0.01em;
}

.header-sub {
  font-size: 13px;
  color: var(--text-muted);
}

/* Main */
.wizard-main {
  flex: 1;
  padding: 32px 24px 60px;
}

.wizard-container {
  max-width: 900px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  gap: 24px;
}

/* Body */
.wizard-body {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  padding: 28px 32px;
  min-height: 400px;
}

/* Validation */
.validation-error {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  background: rgba(239, 68, 68, 0.1);
  border: 1px solid rgba(239, 68, 68, 0.3);
  border-radius: var(--radius-sm);
  color: var(--error);
  font-size: 13px;
}

.err-icon {
  width: 15px;
  height: 15px;
  flex-shrink: 0;
}

/* Navigation */
.wizard-nav {
  display: flex;
  align-items: center;
  gap: 12px;
}

.nav-spacer {
  flex: 1;
}

.nav-btn {
  padding: 9px 22px;
  border-radius: var(--radius-sm);
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s;
  border: 1px solid transparent;
}

.nav-btn--back {
  background: var(--bg-card);
  color: var(--text-secondary);
  border-color: var(--border);
}

.nav-btn--back:hover {
  border-color: var(--accent);
  color: var(--text-primary);
}

.nav-btn--next {
  background: var(--accent);
  color: #fff;
  border-color: var(--accent);
}

.nav-btn--next:hover {
  background: var(--accent-hover);
  border-color: var(--accent-hover);
}
</style>
