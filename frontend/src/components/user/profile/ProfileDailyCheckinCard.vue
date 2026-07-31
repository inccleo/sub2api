<template>
  <section
    v-if="loading || status?.enabled"
    class="card overflow-hidden border border-amber-200/80 bg-gradient-to-br from-amber-50 via-white to-primary-50/70 dark:border-amber-900/40 dark:from-amber-950/25 dark:via-dark-900 dark:to-primary-950/20"
  >
    <div class="p-6">
      <div v-if="loading" class="flex items-center gap-3 text-sm text-gray-500 dark:text-gray-400">
        <span class="h-5 w-5 animate-spin rounded-full border-2 border-amber-500 border-t-transparent"></span>
        {{ t('common.loading') }}
      </div>

      <div v-else-if="status" class="flex flex-col gap-5 md:flex-row md:items-center md:justify-between">
        <div class="space-y-3">
          <div>
            <div class="flex flex-wrap items-center gap-2">
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('profile.dailyCheckin.title') }}
              </h3>
              <span v-if="status.checked_in_today" class="badge badge-success">
                {{ t('profile.dailyCheckin.completed') }}
              </span>
            </div>
            <p class="mt-1 text-sm text-gray-600 dark:text-gray-300">
              {{
                t('profile.dailyCheckin.rewardDescription', {
                  daily: formatAmount(status.daily_reward),
                  bonus: formatAmount(status.weekly_bonus),
                })
              }}
            </p>
          </div>

          <div class="flex flex-wrap gap-2 text-sm">
            <span class="rounded-full bg-white/80 px-3 py-1.5 text-gray-700 ring-1 ring-amber-200 dark:bg-dark-900/70 dark:text-gray-200 dark:ring-amber-900/50">
              {{ t('profile.dailyCheckin.streak', { count: status.current_streak }) }}
            </span>
            <span class="rounded-full bg-white/80 px-3 py-1.5 text-gray-700 ring-1 ring-primary-100 dark:bg-dark-900/70 dark:text-gray-200 dark:ring-primary-900/50">
              {{ t('profile.dailyCheckin.daysUntilBonus', { count: status.days_until_bonus }) }}
            </span>
            <span
              v-if="status.today_reward !== undefined"
              class="rounded-full bg-white/80 px-3 py-1.5 text-gray-700 ring-1 ring-emerald-100 dark:bg-dark-900/70 dark:text-gray-200 dark:ring-emerald-900/50"
            >
              {{ t('profile.dailyCheckin.todayReward', { amount: formatAmount(status.today_reward) }) }}
            </span>
          </div>
        </div>

        <button
          type="button"
          class="btn btn-primary min-w-32"
          :disabled="submitting || status.checked_in_today"
          @click="performCheckin"
        >
          <span
            v-if="submitting"
            class="mr-2 h-4 w-4 animate-spin rounded-full border-2 border-white/40 border-t-white"
          ></span>
          {{
            status.checked_in_today
              ? t('profile.dailyCheckin.checkedIn')
              : t('profile.dailyCheckin.action')
          }}
        </button>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { dailyCheckinAPI, type DailyCheckinStatus } from '@/api/dailyCheckin'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const loading = ref(true)
const submitting = ref(false)
const status = ref<DailyCheckinStatus | null>(null)

function formatAmount(value: number): string {
  return `$${value.toFixed(2)}`
}

async function loadStatus() {
  loading.value = true
  try {
    status.value = await dailyCheckinAPI.getStatus()
  } catch (error: unknown) {
    status.value = null
    appStore.showError(extractApiErrorMessage(error, t('profile.dailyCheckin.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function performCheckin() {
  if (!status.value || status.value.checked_in_today || submitting.value) return

  submitting.value = true
  try {
    const result = await dailyCheckinAPI.checkin()
    appStore.showSuccess(
      t('profile.dailyCheckin.success', { amount: formatAmount(result.reward_amount) }),
    )
    await Promise.all([loadStatus(), authStore.refreshUser()])
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('profile.dailyCheckin.failed')))
  } finally {
    submitting.value = false
  }
}

onMounted(loadStatus)
</script>
