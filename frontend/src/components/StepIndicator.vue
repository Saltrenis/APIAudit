<script setup lang="ts">
import type { WizardStep } from '../types/audit'

const props = defineProps<{
  currentStep: WizardStep
  completedSteps: WizardStep[]
}>()

const emit = defineEmits<{
  (e: 'goTo', step: WizardStep): void
}>()

interface StepDef {
  id: WizardStep
  label: string
  num: number
}

const steps: StepDef[] = [
  { id: 'source', label: 'Source', num: 1 },
  { id: 'command', label: 'Command', num: 2 },
  { id: 'options', label: 'Options', num: 3 },
  { id: 'review', label: 'Review', num: 4 },
  { id: 'results', label: 'Results', num: 5 },
]

function isCompleted(step: WizardStep): boolean {
  return props.completedSteps.includes(step)
}

function isCurrent(step: WizardStep): boolean {
  return props.currentStep === step
}

function canClick(step: WizardStep): boolean {
  return isCompleted(step) && step !== props.currentStep
}

function handleClick(step: WizardStep) {
  if (canClick(step)) {
    emit('goTo', step)
  }
}
</script>

<template>
  <nav class="step-indicator" aria-label="Wizard steps">
    <div class="steps-track">
      <template v-for="(step, index) in steps" :key="step.id">
        <div
          class="step"
          :class="{
            'step--completed': isCompleted(step.id),
            'step--current': isCurrent(step.id),
            'step--clickable': canClick(step.id),
          }"
          @click="handleClick(step.id)"
          :aria-current="isCurrent(step.id) ? 'step' : undefined"
        >
          <div class="step-circle">
            <svg v-if="isCompleted(step.id) && !isCurrent(step.id)" viewBox="0 0 16 16" class="check-icon">
              <polyline points="3,8 6.5,11.5 13,4.5" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            <span v-else>{{ step.num }}</span>
          </div>
          <span class="step-label">{{ step.label }}</span>
        </div>

        <div
          v-if="index < steps.length - 1"
          class="step-connector"
          :class="{ 'step-connector--filled': isCompleted(step.id) }"
        />
      </template>
    </div>
  </nav>
</template>

<style scoped>
.step-indicator {
  padding: 20px 0 4px;
}

.steps-track {
  display: flex;
  align-items: center;
  justify-content: center;
}

.step {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  position: relative;
  z-index: 1;
}

.step--clickable {
  cursor: pointer;
}

.step--clickable:hover .step-circle {
  border-color: var(--accent-hover);
  background: rgba(99, 102, 241, 0.15);
}

.step--clickable:hover .step-label {
  color: var(--text-primary);
}

.step-circle {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  border: 2px solid var(--border);
  background: var(--bg-card);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-muted);
  transition: all 0.2s;
}

.step--completed .step-circle {
  border-color: var(--accent);
  background: rgba(99, 102, 241, 0.15);
  color: var(--accent);
}

.step--current .step-circle {
  border-color: var(--accent);
  background: var(--accent);
  color: #fff;
  box-shadow: 0 0 0 4px rgba(99, 102, 241, 0.2);
}

.check-icon {
  width: 14px;
  height: 14px;
}

.step-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
  transition: color 0.2s;
  white-space: nowrap;
}

.step--current .step-label {
  color: var(--text-primary);
  font-weight: 600;
}

.step--completed .step-label {
  color: var(--text-secondary);
}

.step-connector {
  flex: 1;
  max-width: 80px;
  height: 2px;
  background: var(--border);
  margin: 0 8px;
  margin-bottom: 22px;
  transition: background 0.3s;
}

.step-connector--filled {
  background: var(--accent);
}
</style>
