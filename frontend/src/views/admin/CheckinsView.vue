<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Stats -->
      <div class="space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <p
            v-if="stats?.server_timezone"
            class="text-xs text-gray-500 dark:text-dark-400"
          >
            {{ t('admin.checkins.timezoneHint', { tz: stats.server_timezone }) }}
          </p>
          <button
            class="btn btn-secondary px-2 md:px-3"
            :disabled="statsLoading || loading"
            :title="t('common.refresh')"
            @click="refreshAll"
          >
            <Icon
              name="refresh"
              size="md"
              :class="statsLoading || loading ? 'animate-spin' : ''"
            />
          </button>
        </div>

        <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
          <button
            type="button"
            class="card p-4 text-left transition-all hover:border-primary-300 dark:hover:border-primary-700"
            :class="datePreset === 'today' ? 'ring-2 ring-primary-500 border-primary-500' : ''"
            @click="applyPreset('today')"
          >
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-emerald-100 p-2 dark:bg-emerald-900/30">
                <Icon name="check" size="md" class="text-emerald-600 dark:text-emerald-400" />
              </div>
              <div class="min-w-0">
                <p class="text-xs font-medium text-gray-500 dark:text-dark-400">
                  {{ t('admin.checkins.stats.todayCheckins') }}
                </p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ statsLoading ? '—' : formatCount(stats?.today_checkins) }}
                </p>
                <p class="truncate text-xs text-gray-400 dark:text-dark-500">
                  {{
                    t('admin.checkins.stats.rewardSuffix', {
                      amount: formatAmount(stats?.today_reward_total),
                    })
                  }}
                </p>
              </div>
            </div>
          </button>

          <button
            type="button"
            class="card p-4 text-left transition-all hover:border-primary-300 dark:hover:border-primary-700"
            :class="datePreset === 'yesterday' ? 'ring-2 ring-primary-500 border-primary-500' : ''"
            @click="applyPreset('yesterday')"
          >
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-blue-100 p-2 dark:bg-blue-900/30">
                <Icon name="clock" size="md" class="text-blue-600 dark:text-blue-400" />
              </div>
              <div class="min-w-0">
                <p class="text-xs font-medium text-gray-500 dark:text-dark-400">
                  {{ t('admin.checkins.stats.yesterdayCheckins') }}
                </p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ statsLoading ? '—' : formatCount(stats?.yesterday_checkins) }}
                </p>
                <p class="truncate text-xs text-gray-400 dark:text-dark-500">
                  {{
                    t('admin.checkins.stats.rewardSuffix', {
                      amount: formatAmount(stats?.yesterday_reward_total),
                    })
                  }}
                </p>
              </div>
            </div>
          </button>

          <button
            type="button"
            class="card p-4 text-left transition-all hover:border-primary-300 dark:hover:border-primary-700"
            :class="datePreset === 'last7Days' ? 'ring-2 ring-primary-500 border-primary-500' : ''"
            @click="applyPreset('last7Days')"
          >
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-purple-100 p-2 dark:bg-purple-900/30">
                <Icon name="chart" size="md" class="text-purple-600 dark:text-purple-400" />
              </div>
              <div class="min-w-0">
                <p class="text-xs font-medium text-gray-500 dark:text-dark-400">
                  {{ t('admin.checkins.stats.last7DaysCheckins') }}
                </p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ statsLoading ? '—' : formatCount(stats?.last_7_days_checkins) }}
                </p>
                <p class="truncate text-xs text-gray-400 dark:text-dark-500">
                  {{
                    t('admin.checkins.stats.rewardSuffix', {
                      amount: formatAmount(stats?.last_7_days_reward_total),
                    })
                  }}
                </p>
              </div>
            </div>
          </button>

          <button
            type="button"
            class="card p-4 text-left transition-all hover:border-primary-300 dark:hover:border-primary-700"
            :class="datePreset === 'all' ? 'ring-2 ring-primary-500 border-primary-500' : ''"
            @click="applyPreset('all')"
          >
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-amber-100 p-2 dark:bg-amber-900/30">
                <Icon name="users" size="md" class="text-amber-600 dark:text-amber-400" />
              </div>
              <div class="min-w-0">
                <p class="text-xs font-medium text-gray-500 dark:text-dark-400">
                  {{ t('admin.checkins.stats.totalCheckins') }}
                </p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ statsLoading ? '—' : formatCount(stats?.total_checkins) }}
                </p>
                <p class="truncate text-xs text-gray-400 dark:text-dark-500">
                  {{ t('admin.checkins.stats.uniqueUsers') }}:
                  {{ statsLoading ? '—' : formatCount(stats?.unique_users) }}
                  · ${{ formatAmount(stats?.total_reward) }}
                </p>
              </div>
            </div>
          </button>
        </div>

        <div class="grid grid-cols-2 gap-3 sm:grid-cols-3">
          <div class="card p-3">
            <p class="text-xs text-gray-500 dark:text-dark-400">
              {{ t('admin.checkins.stats.totalBonus') }}
            </p>
            <p class="mt-1 text-lg font-semibold text-amber-600 dark:text-amber-400">
              ${{ formatAmount(stats?.total_bonus_reward) }}
            </p>
          </div>
          <div class="card p-3">
            <p class="text-xs text-gray-500 dark:text-dark-400">
              {{ t('admin.checkins.stats.todayBonusCount') }}
            </p>
            <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">
              {{
                t('admin.checkins.stats.times', {
                  count: statsLoading ? '—' : formatCount(stats?.today_bonus_count),
                })
              }}
            </p>
          </div>
          <div class="card p-3 col-span-2 sm:col-span-1">
            <p class="text-xs text-gray-500 dark:text-dark-400">
              {{ t('admin.checkins.stats.totalReward') }}
            </p>
            <p class="mt-1 text-lg font-semibold text-emerald-600 dark:text-emerald-400">
              ${{ formatAmount(stats?.total_reward) }}
            </p>
          </div>
        </div>
      </div>

      <TablePageLayout>
        <template #filters>
          <div class="flex flex-col gap-3">
            <!-- Date presets -->
            <div class="flex flex-wrap items-center gap-2">
              <button
                v-for="preset in datePresets"
                :key="preset.key"
                type="button"
                class="text-xs px-3 py-1.5 rounded-lg border transition-all"
                :class="
                  datePreset === preset.key
                    ? 'bg-primary-500 text-white border-primary-500'
                    : 'border-gray-200 bg-white text-gray-700 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-200 hover:border-primary-300 dark:hover:border-dark-600'
                "
                @click="applyPreset(preset.key)"
              >
                {{ preset.label }}
              </button>
            </div>

            <div class="flex flex-wrap items-center gap-3">
              <div class="relative w-full md:w-80">
                <Icon
                  name="search"
                  size="md"
                  class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400"
                />
                <input
                  v-model="filters.search"
                  type="text"
                  class="input pl-10"
                  :placeholder="t('admin.checkins.searchPlaceholder')"
                  @input="debounceLoad"
                />
              </div>
              <input
                v-model="filters.start_date"
                type="date"
                class="input w-full sm:w-44"
                :title="t('admin.checkins.startDate')"
                @change="onCustomDateChange"
              />
              <input
                v-model="filters.end_date"
                type="date"
                class="input w-full sm:w-44"
                :title="t('admin.checkins.endDate')"
                @change="onCustomDateChange"
              />
              <button
                class="btn btn-secondary px-2 md:px-3"
                :disabled="loading"
                :title="t('common.refresh')"
                @click="loadRecords"
              >
                <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
              </button>
            </div>
          </div>
        </template>

        <template #table>
          <DataTable
            :columns="columns"
            :data="records"
            :loading="loading"
            default-sort-key="checkin_date"
            default-sort-order="desc"
          >
            <template #cell-user="{ row }">
              <div class="space-y-0.5">
                <div class="text-sm font-medium text-gray-900 dark:text-white">
                  {{ row.user_email || '-' }}
                </div>
                <div class="text-xs text-gray-500 dark:text-dark-400">
                  {{ row.username || '-' }} · #{{ row.user_id }}
                </div>
              </div>
            </template>
            <template #cell-checkin_date="{ value }">
              <span class="text-sm text-gray-900 dark:text-white">{{ value }}</span>
            </template>
            <template #cell-streak_count="{ value }">
              <span class="badge badge-info">{{ value }}</span>
            </template>
            <template #cell-base_reward="{ value }">
              <span class="text-sm text-gray-700 dark:text-gray-300">${{ formatAmount(value) }}</span>
            </template>
            <template #cell-bonus_reward="{ value }">
              <span
                :class="[
                  'text-sm font-medium',
                  value > 0
                    ? 'text-amber-600 dark:text-amber-400'
                    : 'text-gray-400 dark:text-dark-500',
                ]"
              >
                ${{ formatAmount(value) }}
              </span>
            </template>
            <template #cell-total_reward="{ value }">
              <span class="text-sm font-semibold text-emerald-600 dark:text-emerald-400">
                ${{ formatAmount(value) }}
              </span>
            </template>
            <template #cell-created_at="{ value }">
              <span class="text-sm text-gray-700 dark:text-gray-300">
                {{ formatDateTime(value) }}
              </span>
            </template>
          </DataTable>
        </template>

        <template #pagination>
          <Pagination
            v-if="pagination.total > 0"
            :page="pagination.page"
            :total="pagination.total"
            :page-size="pagination.page_size"
            @update:page="handlePageChange"
            @update:pageSize="handlePageSizeChange"
          />
        </template>
      </TablePageLayout>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { DailyCheckinAdminItem, DailyCheckinAdminStats } from '@/api/admin/checkins'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'

type DatePreset = 'all' | 'today' | 'yesterday' | 'last7Days' | 'thisMonth' | 'custom'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const statsLoading = ref(false)
const records = ref<DailyCheckinAdminItem[]>([])
const stats = ref<DailyCheckinAdminStats | null>(null)
const datePreset = ref<DatePreset>('all')
const pagination = reactive({
  page: 1,
  page_size: 20,
  total: 0,
})
const filters = reactive({
  search: '',
  start_date: '',
  end_date: '',
})

const columns = [
  { key: 'user', label: t('admin.checkins.columns.user'), sortable: false },
  { key: 'checkin_date', label: t('admin.checkins.columns.checkinDate'), sortable: false },
  { key: 'streak_count', label: t('admin.checkins.columns.streak'), sortable: false },
  { key: 'base_reward', label: t('admin.checkins.columns.baseReward'), sortable: false },
  { key: 'bonus_reward', label: t('admin.checkins.columns.bonusReward'), sortable: false },
  { key: 'total_reward', label: t('admin.checkins.columns.totalReward'), sortable: false },
  { key: 'created_at', label: t('admin.checkins.columns.createdAt'), sortable: false },
]

const datePresets = computed(() => [
  { key: 'all' as const, label: t('admin.checkins.presets.all') },
  { key: 'today' as const, label: t('admin.checkins.presets.today') },
  { key: 'yesterday' as const, label: t('admin.checkins.presets.yesterday') },
  { key: 'last7Days' as const, label: t('admin.checkins.presets.last7Days') },
  { key: 'thisMonth' as const, label: t('admin.checkins.presets.thisMonth') },
  { key: 'custom' as const, label: t('admin.checkins.presets.custom') },
])

let debounceTimer: ReturnType<typeof setTimeout> | null = null
let abortController: AbortController | null = null
let statsAbortController: AbortController | null = null

function formatAmount(value?: number | null): string {
  return Number(value || 0).toFixed(2)
}

function formatCount(value?: number | null): string {
  return Number(value || 0).toLocaleString()
}

function parseYmd(ymd: string): Date {
  const [y, m, d] = ymd.split('-').map(Number)
  return new Date(y, (m || 1) - 1, d || 1)
}

function formatYmd(date: Date): string {
  const y = date.getFullYear()
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

function addDays(ymd: string, days: number): string {
  const date = parseYmd(ymd)
  date.setDate(date.getDate() + days)
  return formatYmd(date)
}

function monthStart(ymd: string): string {
  const date = parseYmd(ymd)
  date.setDate(1)
  return formatYmd(date)
}

/** Prefer server-provided dates so filters match stats timezone. */
function resolveToday(): string {
  if (stats.value?.today_date) return stats.value.today_date
  return formatYmd(new Date())
}

function resolveYesterday(): string {
  if (stats.value?.yesterday_date) return stats.value.yesterday_date
  return addDays(resolveToday(), -1)
}

function applyPreset(preset: DatePreset) {
  datePreset.value = preset
  const today = resolveToday()
  const yesterday = resolveYesterday()

  switch (preset) {
    case 'all':
      filters.start_date = ''
      filters.end_date = ''
      break
    case 'today':
      filters.start_date = today
      filters.end_date = today
      break
    case 'yesterday':
      filters.start_date = yesterday
      filters.end_date = yesterday
      break
    case 'last7Days':
      filters.start_date = addDays(today, -6)
      filters.end_date = today
      break
    case 'thisMonth':
      filters.start_date = monthStart(today)
      filters.end_date = today
      break
    case 'custom':
      // Keep current date inputs; user edits them manually.
      break
  }

  if (preset !== 'custom') {
    reloadFromFirstPage()
  }
}

function onCustomDateChange() {
  datePreset.value = 'custom'
  reloadFromFirstPage()
}

function debounceLoad() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    pagination.page = 1
    loadRecords()
  }, 300)
}

function reloadFromFirstPage() {
  pagination.page = 1
  loadRecords()
}

function handlePageChange(page: number) {
  pagination.page = page
  loadRecords()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  loadRecords()
}

async function loadStats() {
  if (statsAbortController) {
    statsAbortController.abort()
  }
  statsAbortController = new AbortController()
  statsLoading.value = true
  try {
    stats.value = await adminAPI.checkins.getStats({ signal: statsAbortController.signal })
  } catch (error: unknown) {
    if ((error as { name?: string })?.name === 'CanceledError') return
    stats.value = null
    appStore.showError(extractApiErrorMessage(error, t('admin.checkins.statsLoadFailed')))
  } finally {
    statsLoading.value = false
  }
}

async function loadRecords() {
  if (abortController) {
    abortController.abort()
  }
  abortController = new AbortController()
  loading.value = true
  try {
    const response = await adminAPI.checkins.list(
      pagination.page,
      pagination.page_size,
      {
        search: filters.search.trim() || undefined,
        start_date: filters.start_date || undefined,
        end_date: filters.end_date || undefined,
      },
      { signal: abortController.signal },
    )
    records.value = response.items || []
    pagination.total = response.total || 0
    pagination.page = response.page || pagination.page
    pagination.page_size = response.page_size || pagination.page_size
  } catch (error: unknown) {
    if ((error as { name?: string })?.name === 'CanceledError') return
    records.value = []
    pagination.total = 0
    appStore.showError(extractApiErrorMessage(error, t('admin.checkins.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function refreshAll() {
  await Promise.all([loadStats(), loadRecords()])
}

onMounted(() => {
  void loadStats()
  void loadRecords()
})

onUnmounted(() => {
  if (debounceTimer) clearTimeout(debounceTimer)
  if (abortController) abortController.abort()
  if (statsAbortController) statsAbortController.abort()
})
</script>
