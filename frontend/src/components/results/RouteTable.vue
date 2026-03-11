<script setup lang="ts">
import { ref, computed } from 'vue'
import type { Route } from '../../types/audit'

const props = defineProps<{
  routes: Route[]
}>()

const searchQuery = ref('')
const methodFilter = ref('ALL')
const sortKey = ref<keyof Route | null>(null)
const sortDir = ref<'asc' | 'desc'>('asc')

const HTTP_METHODS = ['ALL', 'GET', 'POST', 'PUT', 'PATCH', 'DELETE']

const methodColors: Record<string, string> = {
  GET: '#22c55e',
  POST: '#3b82f6',
  PUT: '#f59e0b',
  DELETE: '#ef4444',
  PATCH: '#a855f7',
  HEAD: '#71717a',
  OPTIONS: '#71717a',
}

function methodColor(method: string): string {
  return methodColors[method.toUpperCase()] ?? '#71717a'
}

function toggleSort(key: keyof Route) {
  if (sortKey.value === key) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortKey.value = key
    sortDir.value = 'asc'
  }
}

const filteredRoutes = computed(() => {
  let list = props.routes

  if (methodFilter.value !== 'ALL') {
    list = list.filter(r => r.method.toUpperCase() === methodFilter.value)
  }

  if (searchQuery.value.trim()) {
    const q = searchQuery.value.toLowerCase()
    list = list.filter(
      r =>
        r.path.toLowerCase().includes(q) ||
        r.handler.toLowerCase().includes(q) ||
        r.file.toLowerCase().includes(q)
    )
  }

  if (sortKey.value) {
    const key = sortKey.value
    list = [...list].sort((a, b) => {
      const av = String(a[key] ?? '')
      const bv = String(b[key] ?? '')
      const cmp = av.localeCompare(bv)
      return sortDir.value === 'asc' ? cmp : -cmp
    })
  }

  return list
})

function sortIcon(key: keyof Route): string {
  if (sortKey.value !== key) return '↕'
  return sortDir.value === 'asc' ? '↑' : '↓'
}
</script>

<template>
  <div class="route-table-wrapper">
    <div class="table-controls">
      <div class="method-filters">
        <button
          v-for="m in HTTP_METHODS"
          :key="m"
          class="method-filter-btn"
          :class="{ active: methodFilter === m }"
          @click="methodFilter = m"
        >
          {{ m }}
        </button>
      </div>
      <input
        v-model="searchQuery"
        class="search-input"
        type="text"
        placeholder="Search path, handler, file..."
      />
    </div>

    <div class="table-container">
      <table class="route-table">
        <thead>
          <tr>
            <th class="sortable" @click="toggleSort('method')">
              Method <span class="sort-icon">{{ sortIcon('method') }}</span>
            </th>
            <th class="sortable" @click="toggleSort('path')">
              Path <span class="sort-icon">{{ sortIcon('path') }}</span>
            </th>
            <th class="sortable" @click="toggleSort('handler')">
              Handler <span class="sort-icon">{{ sortIcon('handler') }}</span>
            </th>
            <th class="sortable" @click="toggleSort('file')">
              File <span class="sort-icon">{{ sortIcon('file') }}</span>
            </th>
            <th>Swagger</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="filteredRoutes.length === 0">
            <td colspan="5" class="empty-row">No routes match the current filters.</td>
          </tr>
          <tr v-for="route in filteredRoutes" :key="`${route.method}-${route.path}`">
            <td>
              <span
                class="method-badge"
                :style="{ background: methodColor(route.method) + '22', color: methodColor(route.method), borderColor: methodColor(route.method) + '55' }"
              >
                {{ route.method.toUpperCase() }}
              </span>
            </td>
            <td class="path-cell">{{ route.path }}</td>
            <td class="handler-cell">{{ route.handler }}</td>
            <td class="file-cell">
              <span class="file-name">{{ route.file }}</span>
              <span class="line-num">:{{ route.line }}</span>
            </td>
            <td class="swagger-cell">
              <span v-if="route.hasSwagger" class="swagger-yes" title="Has swagger annotation">✓</span>
              <span v-else class="swagger-no" title="Missing swagger annotation">✗</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="table-footer">
      Showing {{ filteredRoutes.length }} of {{ props.routes.length }} routes
    </div>
  </div>
</template>

<style scoped>
.route-table-wrapper {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.table-controls {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.method-filters {
  display: flex;
  gap: 4px;
}

.method-filter-btn {
  padding: 5px 12px;
  font-size: 12px;
  font-weight: 600;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--bg-card);
  color: var(--text-muted);
  cursor: pointer;
  transition: all 0.15s;
  letter-spacing: 0.03em;
}

.method-filter-btn:hover {
  border-color: var(--accent);
  color: var(--text-primary);
}

.method-filter-btn.active {
  background: var(--accent);
  border-color: var(--accent);
  color: #fff;
}

.search-input {
  flex: 1;
  min-width: 200px;
  padding: 6px 12px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  color: var(--text-primary);
  font-size: 13px;
  outline: none;
  transition: border-color 0.15s;
}

.search-input::placeholder {
  color: var(--text-muted);
}

.search-input:focus {
  border-color: var(--accent);
}

.table-container {
  overflow-x: auto;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
}

.route-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.route-table thead {
  background: var(--bg-secondary);
}

.route-table th {
  padding: 10px 14px;
  text-align: left;
  font-size: 11px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.06em;
  border-bottom: 1px solid var(--border);
  white-space: nowrap;
}

.route-table th.sortable {
  cursor: pointer;
  user-select: none;
}

.route-table th.sortable:hover {
  color: var(--text-primary);
}

.sort-icon {
  font-size: 11px;
  opacity: 0.6;
  margin-left: 4px;
}

.route-table td {
  padding: 9px 14px;
  border-bottom: 1px solid var(--border);
  vertical-align: middle;
}

.route-table tbody tr:last-child td {
  border-bottom: none;
}

.route-table tbody tr:hover td {
  background: var(--bg-hover);
}

.method-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.05em;
  border: 1px solid;
}

.path-cell {
  font-family: 'SF Mono', 'Fira Code', 'Fira Mono', 'Roboto Mono', monospace;
  color: var(--text-primary);
  word-break: break-all;
}

.handler-cell {
  color: var(--text-secondary);
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 12px;
}

.file-cell {
  white-space: nowrap;
}

.file-name {
  color: var(--text-muted);
  font-size: 12px;
  font-family: 'SF Mono', 'Fira Code', monospace;
}

.line-num {
  color: var(--text-muted);
  font-size: 12px;
  opacity: 0.6;
  font-family: 'SF Mono', 'Fira Code', monospace;
}

.swagger-cell {
  text-align: center;
}

.swagger-yes {
  color: var(--success);
  font-size: 15px;
  font-weight: 700;
}

.swagger-no {
  color: var(--error);
  font-size: 15px;
  font-weight: 700;
  opacity: 0.7;
}

.empty-row {
  text-align: center;
  color: var(--text-muted);
  padding: 32px !important;
  font-style: italic;
}

.table-footer {
  font-size: 12px;
  color: var(--text-muted);
  text-align: right;
}
</style>
