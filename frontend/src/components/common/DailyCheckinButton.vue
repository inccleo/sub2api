<template>
  <button
    v-if="status?.enabled"
    type="button"
    class="relative flex h-9 items-center justify-center gap-1.5 rounded-lg px-2 text-sm font-medium transition-all sm:px-2.5"
    :class="
      status.checked_in_today
        ? 'cursor-default bg-emerald-50 text-emerald-600 dark:bg-emerald-900/20 dark:text-emerald-400'
        : 'bg-amber-50 text-amber-700 hover:scale-105 hover:bg-amber-100 dark:bg-amber-900/20 dark:text-amber-300 dark:hover:bg-amber-900/35'
    "
    :disabled="submitting || status.checked_in_today"
    :aria-label="buttonLabel"
    :title="buttonTitle"
    @click="performCheckin"
  >
    <span
      v-if="submitting"
      class="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent"
    ></span>
    <Icon
      v-else
      :name="status.checked_in_today ? 'checkCircle' : 'gift'"
      size="sm"
    />
    <span class="hidden sm:inline">{{ buttonLabel }}</span>
    <span
      v-if="!status.checked_in_today"
      class="absolute right-0.5 top-0.5 h-1.5 w-1.5 rounded-full bg-amber-500"
    ></span>
  </button>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { dailyCheckinAPI, type DailyCheckinStatus } from '@/api/dailyCheckin'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const status = ref<DailyCheckinStatus | null>(null)
const submitting = ref(false)

const buttonLabel = computed(() =>
  status.value?.checked_in_today
    ? t('profile.dailyCheckin.checkedIn')
    : t('profile.dailyCheckin.action'),
)

const buttonTitle = computed(() => {
  if (!status.value) return buttonLabel.value
  if (status.value.checked_in_today) {
    return t('profile.dailyCheckin.todayReward', {
      amount: formatAmount(status.value.today_reward ?? 0),
    })
  }
  return t('profile.dailyCheckin.rewardDescription', {
    daily: formatAmount(status.value.daily_reward),
    bonus: formatAmount(status.value.weekly_bonus),
  })
})

function formatAmount(value: number): string {
  return `$${value.toFixed(2)}`
}

async function loadStatus() {
  try {
    status.value = await dailyCheckinAPI.getStatus()
  } catch (error) {
    // The entry stays hidden when the feature is disabled or the old backend
    // has not been restarted yet. Avoid showing a global error on every page.
    console.debug('Failed to load daily check-in status:', error)
  }
}

async function performCheckin() {
  if (!status.value || status.value.checked_in_today || submitting.value) return

  submitting.value = true
  try {
    const result = await dailyCheckinAPI.checkin()
    appStore.showSuccess(
      t('profile.dailyCheckin.success', {
        amount: formatAmount(result.reward_amount),
      }),
    )
    await Promise.all([loadStatus(), authStore.refreshUser()])
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(error, t('profile.dailyCheckin.failed')),
    )
  } finally {
    submitting.value = false
  }
}

onMounted(loadStatus)
</script>
