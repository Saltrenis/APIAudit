export interface Framework {
  language: string
  framework: string
  version?: string
  entryPoints?: string[]
  hasFrontend: boolean
  frontendDir?: string
  hasSwagger: boolean
  confidence: number
}

export interface Schema {
  type?: string
  properties?: Record<string, Schema>
  items?: Schema
  required?: string[]
  $ref?: string
}

export interface Route {
  method: string
  path: string
  handler: string
  file: string
  line: number
  middleware?: string[]
  requestBody?: Schema
  response?: Schema
  hasSwagger: boolean
}

export interface Finding {
  category: string
  severity: string
  route?: Route
  message: string
  file?: string
  line?: number
  suggestion?: string
}

export interface AuditReport {
  framework: Framework
  summary: {
    totalRoutes: number
    documented: number
    coveragePct: number
    p1: number
    p2: number
    p3: number
    p4: number
  }
  routes: Route[]
  findings: Finding[]
}

export interface RunRequest {
  command: string
  dir: string
  repo: string
  frontendDir: string
  format: string
  output: string
  flags: Record<string, string>
  boolFlags: string[]
  beadsLimit: number
}

export type WizardStep = 'source' | 'command' | 'options' | 'review' | 'results'

export interface WizardState {
  // Step 1: Source
  sourceType: 'local' | 'repo'
  dir: string
  repo: string
  frontendDir: string

  // Step 2: Command
  command: string

  // Step 3: Options
  format: string
  output: string
  skipFrontend: boolean
  skipGenerate: boolean
  staticOnly: boolean
  dryRun: boolean
  jsonFormat: boolean
  beads: boolean
  beadsLimit: number
  aiAssist: boolean
  title: string
  description: string
  apiVersion: string
}
