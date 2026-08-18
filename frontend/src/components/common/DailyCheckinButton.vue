<template>
  <div v-if="status?.enabled" class="contents">
    <button
      type="button"
      class="relative flex h-9 items-center justify-center gap-1.5 rounded-lg px-2 text-sm font-medium transition-all hover:scale-105 sm:px-2.5"
      :class="
        status.checked_in_today
          ? 'bg-emerald-50 text-emerald-600 hover:bg-emerald-100 dark:bg-emerald-900/20 dark:text-emerald-400 dark:hover:bg-emerald-900/35'
          : 'bg-amber-50 text-amber-700 hover:bg-amber-100 dark:bg-amber-900/20 dark:text-amber-300 dark:hover:bg-amber-900/35'
      "
      :aria-label="buttonLabel"
      :title="buttonTitle"
      @click="openDialog"
    >
      <Icon
        :name="status.checked_in_today ? 'checkCircle' : 'gift'"
        size="sm"
      />
      <span class="hidden sm:inline">{{ buttonLabel }}</span>
      <span
        v-if="!status.checked_in_today && status.eligible"
        class="absolute right-0.5 top-0.5 h-1.5 w-1.5 rounded-full bg-amber-500"
      ></span>
    </button>

    <BaseDialog
      :show="dialogOpen"
      :title="t('profile.dailyCheckin.title')"
      width="narrow"
      :close-on-click-outside="true"
      @close="dialogOpen = false"
    >
      <div class="checkin-dialog space-y-3">
        <!-- Compact header: month nav + streak -->
        <div class="flex items-center justify-between gap-2">
          <button
            type="button"
            class="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-dark-800"
            :aria-label="t('profile.dailyCheckin.prevMonth')"
            @click="shiftMonth(-1)"
          >
            <Icon name="chevronLeft" size="sm" />
          </button>
          <div class="min-w-0 text-center">
            <div class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ monthLabel }}
            </div>
            <div class="text-[11px] text-gray-500 dark:text-gray-400">
              {{ t('profile.dailyCheckin.streak', { count: status.current_streak }) }}
              <span class="mx-1 text-gray-300 dark:text-dark-600">·</span>
              {{ t('profile.dailyCheckin.daysUntilBonus', { count: status.days_until_bonus }) }}
            </div>
          </div>
          <button
            type="button"
            class="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 disabled:opacity-40 dark:text-gray-400 dark:hover:bg-dark-800"
            :disabled="!canGoNextMonth"
            :aria-label="t('profile.dailyCheckin.nextMonth')"
            @click="shiftMonth(1)"
          >
            <Icon name="chevronRight" size="sm" />
          </button>
        </div>

        <!-- Weekday headers -->
        <div class="grid grid-cols-7 gap-0.5 text-center text-[11px] font-medium text-gray-400 dark:text-gray-500">
          <div v-for="label in weekdayLabels" :key="label" class="py-0.5">
            {{ label }}
          </div>
        </div>

        <!-- Calendar grid -->
        <div v-if="historyLoading" class="flex items-center justify-center py-8 text-xs text-gray-500">
          <span class="mr-2 h-3.5 w-3.5 animate-spin rounded-full border-2 border-amber-500 border-t-transparent"></span>
          {{ t('common.loading') }}
        </div>

        <div v-else class="grid grid-cols-7 gap-0.5">
          <div
            v-for="(cell, index) in calendarCells"
            :key="index"
            class="min-h-[2.75rem]"
          >
            <div
              v-if="cell"
              class="flex h-full min-h-[2.75rem] w-full flex-col items-center justify-center rounded-lg px-0.5 py-1"
              :class="dayCellClass(cell)"
              :title="dayTooltip(cell)"
            >
              <span class="text-[11px] leading-none" :class="dayNumberClass(cell)">
                {{ cell.day }}
              </span>
              <span
                v-if="cell.displayAmount !== null"
                class="mt-0.5 max-w-full truncate text-[9px] font-semibold leading-none"
                :class="amountClass(cell)"
              >
                <template v-if="cell.hasBonus">★</template>{{ formatCompact(cell.displayAmount) }}
              </span>
              <span
                v-else-if="cell.isMissed"
                class="mt-0.5 text-[9px] leading-none text-gray-300 dark:text-dark-600"
              >·</span>
            </div>
          </div>
        </div>

        <!-- Tiny legend -->
        <div class="flex flex-wrap items-center justify-center gap-x-3 gap-y-1 text-[10px] text-gray-400 dark:text-gray-500">
          <span class="inline-flex items-center gap-1">
            <span class="h-2 w-2 rounded-sm bg-emerald-400"></span>
            {{ t('profile.dailyCheckin.legendChecked') }}
          </span>
          <span class="inline-flex items-center gap-1">
            <span class="h-2 w-2 rounded-sm bg-amber-400"></span>
            {{ t('profile.dailyCheckin.legendBonus') }}
          </span>
          <span class="inline-flex items-center gap-1">
            <span class="h-2 w-2 rounded-sm border border-dashed border-primary-400 bg-primary-50 dark:bg-primary-900/30"></span>
            {{ t('profile.dailyCheckin.legendFuture') }}
          </span>
        </div>

        <p
          v-if="!status.eligible"
          class="rounded-lg bg-amber-50 px-3 py-2 text-center text-xs text-amber-700 dark:bg-amber-900/20 dark:text-amber-300"
        >
          {{ t('profile.dailyCheckin.notEligible') }}
        </p>

        <button
          type="button"
          data-testid="daily-checkin-action"
          class="btn btn-primary w-full !py-2 text-sm"
          :disabled="submitting || status.checked_in_today || !status.eligible"
          @click="performCheckin"
        >
          <span
            v-if="submitting"
            class="mr-2 h-3.5 w-3.5 animate-spin rounded-full border-2 border-white/40 border-t-white"
          ></span>
          <template v-if="status.checked_in_today">
            {{ t('profile.dailyCheckin.checkedIn') }}
          </template>
          <template v-else-if="!status.eligible">
            {{ t('profile.dailyCheckin.notEligibleAction') }}
          </template>
          <template v-else>
            {{ t('profile.dailyCheckin.action') }}
            <span v-if="todayProjected" class="ml-1 opacity-90">
              +{{ formatCompact(todayProjected.total) }}
            </span>
          </template>
        </button>
      </div>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  dailyCheckinAPI,
  type DailyCheckinHistoryItem,
  type DailyCheckinStatus,
} from '@/api/dailyCheckin'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'

const BONUS_INTERVAL = 7
/** How many months ahead users can browse for projected rewards */
const MAX_FUTURE_MONTHS = 1

interface CalendarCell {
  day: number
  dateKey: string
  isToday: boolean
  isFuture: boolean
  isPast: boolean
  isMissed: boolean
  hasBonus: boolean
  isProjected: boolean
  displayAmount: number | null
  streak: number | null
  record?: DailyCheckinHistoryItem
}

const { t, locale } = useI18n()
// Tests may mock useI18n without locale; keep calendar labels resilient.
const localeTag = computed(() => {
  const value = locale && typeof locale === 'object' && 'value' in locale ? locale.value : locale
  return (typeof value === 'string' && value) || undefined
})
const appStore = useAppStore()
const authStore = useAuthStore()

const status = ref<DailyCheckinStatus | null>(null)
const history = ref<DailyCheckinHistoryItem[]>([])
const submitting = ref(false)
const historyLoading = ref(false)
const dialogOpen = ref(false)

const viewYear = ref(new Date().getFullYear())
const viewMonth = ref(new Date().getMonth())

const historyByDate = computed(() => {
  const map = new Map<string, DailyCheckinHistoryItem>()
  for (const item of history.value) {
    map.set(item.checkin_date, item)
  }
  return map
})

const serverNow = computed(() => getServerNow(status.value?.server_timezone))
const todayKey = computed(() => formatDateKey(serverNow.value))

const canGoNextMonth = computed(() => {
  const now = serverNow.value
  const max = new Date(now.getFullYear(), now.getMonth() + MAX_FUTURE_MONTHS, 1)
  const next = new Date(viewYear.value, viewMonth.value + 1, 1)
  return next.getTime() <= max.getTime()
})

const monthLabel = computed(() => {
  const date = new Date(viewYear.value, viewMonth.value, 1)
  try {
    return new Intl.DateTimeFormat(localeTag.value, {
      year: 'numeric',
      month: 'long',
    }).format(date)
  } catch {
    return `${viewYear.value}-${String(viewMonth.value + 1).padStart(2, '0')}`
  }
})

const weekdayLabels = computed(() => {
  const formatter = new Intl.DateTimeFormat(localeTag.value, { weekday: 'short' })
  return Array.from({ length: 7 }, (_, i) => {
    const date = new Date(Date.UTC(2024, 0, 1 + i)) // Monday-first
    return formatter.format(date)
  })
})

const todayProjected = computed(() => {
  if (!status.value || status.value.checked_in_today || !status.value.eligible) return null
  return projectForDate(todayKey.value)
})

const calendarCells = computed(() => {
  const year = viewYear.value
  const month = viewMonth.value
  const first = new Date(year, month, 1)
  const daysInMonth = new Date(year, month + 1, 0).getDate()
  const startOffset = (first.getDay() + 6) % 7
  const cells: Array<CalendarCell | null> = []
  const today = todayKey.value

  for (let i = 0; i < startOffset; i++) cells.push(null)

  for (let day = 1; day <= daysInMonth; day++) {
    const dateKey = `${year}-${String(month + 1).padStart(2, '0')}-${String(day).padStart(2, '0')}`
    const record = historyByDate.value.get(dateKey)
    const isToday = dateKey === today
    const isFuture = dateKey > today
    const isPast = dateKey < today

    let displayAmount: number | null = null
    let hasBonus = false
    let isProjected = false
    let streak: number | null = null
    let isMissed = false

    if (record) {
      displayAmount = record.total_reward
      hasBonus = record.bonus_reward > 0
      streak = record.streak_count
    } else if (isToday || isFuture) {
      const projected = projectForDate(dateKey)
      if (projected) {
        displayAmount = projected.total
        hasBonus = projected.bonus > 0
        streak = projected.streak
        isProjected = true
      }
    } else if (isPast) {
      isMissed = true
    }

    cells.push({
      day,
      dateKey,
      isToday,
      isFuture,
      isPast,
      isMissed,
      hasBonus,
      isProjected,
      displayAmount,
      streak,
      record,
    })
  }

  while (cells.length % 7 !== 0) cells.push(null)
  return cells
})

const buttonLabel = computed(() => {
  if (!status.value) return t('profile.dailyCheckin.action')
  if (status.value.checked_in_today) {
    if (status.value.current_streak > 0) {
      return t('profile.dailyCheckin.checkedInWithStreak', {
        count: status.value.current_streak,
      })
    }
    return t('profile.dailyCheckin.checkedIn')
  }
  return t('profile.dailyCheckin.action')
})

const buttonTitle = computed(() => {
  if (!status.value) return buttonLabel.value
  if (!status.value.eligible) return t('profile.dailyCheckin.notEligible')
  return t('profile.dailyCheckin.openHint')
})

/**
 * Project reward if user keeps checking in from today onward.
 * Mirrors backend: bonus when streak % 7 === 0.
 */
function projectForDate(dateKey: string): { streak: number; base: number; bonus: number; total: number } | null {
  if (!status.value?.eligible) return null
  const today = todayKey.value
  if (dateKey < today) return null

  const offset = daysBetween(today, dateKey)
  const streak = status.value.checked_in_today
    ? status.value.current_streak + offset
    : status.value.current_streak + 1 + offset

  if (streak <= 0) return null

  const base = status.value.daily_reward
  const bonus = streak % BONUS_INTERVAL === 0 ? status.value.weekly_bonus : 0
  return {
    streak,
    base,
    bonus,
    total: roundAmount(base + bonus),
  }
}

function daysBetween(fromKey: string, toKey: string): number {
  const from = parseDateKey(fromKey)
  const to = parseDateKey(toKey)
  return Math.round((to.getTime() - from.getTime()) / 86400000)
}

function parseDateKey(key: string): Date {
  const [y, m, d] = key.split('-').map(Number)
  return new Date(y, m - 1, d)
}

function roundAmount(value: number): number {
  return Math.round(value * 1e8) / 1e8
}

function formatCompact(value: number): string {
  // Compact money for tiny cells: $0.01 / $0.1 / $1
  if (value >= 1) return `$${value.toFixed(value % 1 === 0 ? 0 : 2)}`
  return `$${value.toFixed(2)}`
}

function getServerNow(timeZone?: string): Date {
  if (!timeZone) return new Date()
  try {
    const parts = new Intl.DateTimeFormat('en-CA', {
      timeZone,
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    }).formatToParts(new Date())
    const year = Number(parts.find((p) => p.type === 'year')?.value)
    const month = Number(parts.find((p) => p.type === 'month')?.value)
    const day = Number(parts.find((p) => p.type === 'day')?.value)
    if (!year || !month || !day) return new Date()
    return new Date(year, month - 1, day)
  } catch {
    return new Date()
  }
}

function formatDateKey(date: Date): string {
  const y = date.getFullYear()
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

function dayCellClass(cell: CalendarCell): string {
  if (cell.record) {
    if (cell.hasBonus) {
      return 'bg-amber-100 ring-1 ring-amber-300 dark:bg-amber-900/45 dark:ring-amber-700'
    }
    return 'bg-emerald-100 ring-1 ring-emerald-300 dark:bg-emerald-900/40 dark:ring-emerald-700'
  }
  if (cell.isProjected && cell.hasBonus) {
    return 'bg-amber-50/80 ring-1 ring-dashed ring-amber-300 dark:bg-amber-950/30 dark:ring-amber-700/70'
  }
  if (cell.isProjected) {
    return cell.isToday
      ? 'bg-primary-50 ring-2 ring-primary-400 dark:bg-primary-900/25 dark:ring-primary-500'
      : 'bg-gray-50 ring-1 ring-dashed ring-gray-200 dark:bg-dark-800/50 dark:ring-dark-600'
  }
  if (cell.isToday) {
    return 'bg-primary-50 ring-2 ring-primary-400 dark:bg-primary-900/25 dark:ring-primary-500'
  }
  return ''
}

function dayNumberClass(cell: CalendarCell): string {
  if (cell.record || cell.isProjected) {
    if (cell.hasBonus) return 'text-amber-900 dark:text-amber-100'
    if (cell.record) return 'text-emerald-900 dark:text-emerald-100'
    if (cell.isToday) return 'text-primary-700 dark:text-primary-200'
    return 'text-gray-500 dark:text-gray-400'
  }
  if (cell.isMissed) return 'text-gray-300 dark:text-dark-600'
  return 'text-gray-500 dark:text-gray-400'
}

function amountClass(cell: CalendarCell): string {
  if (cell.hasBonus) return 'text-amber-700 dark:text-amber-300'
  if (cell.record) return 'text-emerald-700 dark:text-emerald-300'
  if (cell.isToday) return 'text-primary-600 dark:text-primary-300'
  return 'text-gray-400 dark:text-gray-500'
}

function dayTooltip(cell: CalendarCell): string {
  const parts = [cell.dateKey]
  if (cell.record) {
    parts.push(`+${formatCompact(cell.record.total_reward)}`)
    parts.push(t('profile.dailyCheckin.streak', { count: cell.record.streak_count }))
    if (cell.record.bonus_reward > 0) {
      parts.push(
        t('profile.dailyCheckin.bonusIncluded', {
          amount: formatCompact(cell.record.bonus_reward),
        }),
      )
    }
    return parts.join(' · ')
  }
  if (cell.isProjected && cell.displayAmount !== null && cell.streak !== null) {
    parts.push(t('profile.dailyCheckin.projectedReward', { amount: formatCompact(cell.displayAmount) }))
    parts.push(t('profile.dailyCheckin.streak', { count: cell.streak }))
    if (cell.hasBonus) {
      parts.push(t('profile.dailyCheckin.legendBonus'))
    }
    return parts.join(' · ')
  }
  if (cell.isMissed) {
    parts.push(t('profile.dailyCheckin.missed'))
  }
  return parts.join(' · ')
}

function shiftMonth(delta: number) {
  const date = new Date(viewYear.value, viewMonth.value + delta, 1)
  const now = serverNow.value
  const max = new Date(now.getFullYear(), now.getMonth() + MAX_FUTURE_MONTHS, 1)
  // Don't go too far into the past (optional soft limit: 12 months)
  const min = new Date(now.getFullYear(), now.getMonth() - 12, 1)
  if (date.getTime() < min.getTime() || date.getTime() > max.getTime()) return
  viewYear.value = date.getFullYear()
  viewMonth.value = date.getMonth()
}

async function loadStatus() {
  try {
    status.value = await dailyCheckinAPI.getStatus()
  } catch (error) {
    console.debug('Failed to load daily check-in status:', error)
  }
}

async function loadHistory() {
  historyLoading.value = true
  try {
    history.value = await dailyCheckinAPI.getHistory(100)
  } catch (error: unknown) {
    history.value = []
    appStore.showError(
      extractApiErrorMessage(error, t('profile.dailyCheckin.historyLoadFailed')),
    )
  } finally {
    historyLoading.value = false
  }
}

async function openDialog() {
  dialogOpen.value = true
  await Promise.all([loadStatus(), loadHistory()])
  const now = getServerNow(status.value?.server_timezone)
  viewYear.value = now.getFullYear()
  viewMonth.value = now.getMonth()
}

async function performCheckin() {
  if (!status.value || !status.value.eligible || status.value.checked_in_today || submitting.value) return

  submitting.value = true
  try {
    const result = await dailyCheckinAPI.checkin()
    appStore.showSuccess(
      t('profile.dailyCheckin.success', {
        amount: formatCompact(result.reward_amount),
      }),
    )
    // Refresh best-effort: check-in already succeeded even if status/user refresh fails.
    await Promise.allSettled([loadStatus(), loadHistory(), authStore.refreshUser()])
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(error, t('profile.dailyCheckin.failed'), {
        DAILY_CHECKIN_NOT_ELIGIBLE: t('profile.dailyCheckin.notEligible'),
      }),
    )
  } finally {
    submitting.value = false
  }
}

onMounted(loadStatus)
</script>

<style scoped>
/* Tighten BaseDialog body padding for this compact calendar */
.checkin-dialog {
  margin: -0.25rem;
}
</style>
