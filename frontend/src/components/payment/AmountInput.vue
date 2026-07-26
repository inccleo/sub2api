<template>
  <div>
    <div class="mb-3 flex items-end justify-between gap-3">
      <label class="block text-sm font-semibold text-gray-800 dark:text-gray-200">
        {{ t('payment.quickAmounts') }}
      </label>
      <span class="text-xs font-medium text-amber-600 dark:text-amber-400">
        {{ t('payment.moreRechargeMoreBonus') }}
      </span>
    </div>

    <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
      <button
        v-for="(pkg, index) in filteredPackages"
        :key="pkg.amount"
        type="button"
        :aria-label="t('payment.selectQuickAmount', { amount: formatMoney(pkg.amount) })"
        :aria-pressed="modelValue === pkg.amount"
        :class="[
          'group relative min-h-[206px] overflow-hidden rounded-2xl border p-5 text-left transition-all duration-200 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-dark-900',
          modelValue === pkg.amount
            ? 'border-primary-500 bg-primary-50/80 shadow-lg shadow-primary-100/70 ring-1 ring-primary-500 dark:border-primary-400 dark:bg-primary-950/30 dark:shadow-none dark:ring-primary-400'
            : 'border-gray-200 bg-white hover:-translate-y-1 hover:border-primary-300 hover:shadow-lg dark:border-dark-600 dark:bg-dark-800 dark:hover:border-primary-700',
        ]"
        @click="selectAmount(pkg.amount)"
      >
        <span
          v-if="badgeKey(index)"
          class="absolute right-0 top-0 rounded-bl-xl bg-primary-600 px-3 py-1.5 text-[11px] font-bold text-white dark:bg-primary-500"
        >
          {{ t(badgeKey(index)) }}
        </span>
        <span
          v-else-if="modelValue === pkg.amount"
          class="absolute right-4 top-4 flex h-6 w-6 items-center justify-center rounded-full bg-primary-600 text-white dark:bg-primary-500"
          aria-hidden="true"
        >
          <svg viewBox="0 0 20 20" fill="none" class="h-3.5 w-3.5" stroke="currentColor" stroke-width="2.5">
            <path d="m5 10 3 3 7-7" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
        </span>

        <span class="block text-base font-bold text-gray-950 dark:text-white">
          {{ t(packageNameKey(pkg.amount)) }}
        </span>
        <span class="mt-1 block text-xs text-gray-400 dark:text-gray-500">
          {{ t(packageDescriptionKey(pkg.amount)) }}
        </span>
        <span class="mt-5 block text-3xl font-black tracking-tight text-gray-950 dark:text-white">
          {{ formatMoney(pkg.amount) }}
        </span>

        <span
          v-if="pkg.bonus > 0"
          class="mt-3 inline-flex items-center rounded-full bg-orange-100 px-2.5 py-1 text-sm font-black text-orange-700 ring-1 ring-inset ring-orange-200 dark:bg-orange-950/60 dark:text-orange-300 dark:ring-orange-800"
        >
          {{ t('payment.bonusAmount', { amount: formatMoney(pkg.bonus) }) }}
        </span>
        <span v-else class="mt-3 block text-sm font-medium text-gray-500 dark:text-gray-400">
          {{ t('payment.noBonusStarter') }}
        </span>

        <span class="mt-3 block border-t border-gray-100 pt-3 text-xs text-gray-500 dark:border-dark-700 dark:text-gray-400">
          {{ t('payment.creditedAmount', { amount: formatCredit(creditedFor(pkg)) }) }}
        </span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { RechargePackage } from '@/types/payment'

const props = withDefaults(defineProps<{
  packages: RechargePackage[]
  modelValue: number | null
  min?: number
  max?: number
  currency?: string
  locale?: string
  creditMultiplier?: number
}>(), {
  min: 0,
  max: 0,
  currency: 'CNY',
  locale: undefined,
  creditMultiplier: 1,
})

const emit = defineEmits<{
  'update:modelValue': [value: number | null]
}>()

const { t } = useI18n()

const normalizedMultiplier = computed(() =>
  Number.isFinite(props.creditMultiplier) && props.creditMultiplier > 0
    ? props.creditMultiplier
    : 1
)

const filteredPackages = computed(() =>
  props.packages.filter((pkg) =>
    (props.min <= 0 || pkg.amount >= props.min)
    && (props.max <= 0 || pkg.amount <= props.max)
  )
)

function moneyFormatter(currency: string) {
  try {
    return new Intl.NumberFormat(props.locale || undefined, {
      style: 'currency',
      currency,
      minimumFractionDigits: 0,
      maximumFractionDigits: 2,
    })
  } catch {
    return new Intl.NumberFormat(undefined, {
      style: 'currency',
      currency: 'CNY',
      minimumFractionDigits: 0,
      maximumFractionDigits: 2,
    })
  }
}

function formatMoney(value: number) {
  return moneyFormatter(props.currency).format(value)
}

function formatCredit(value: number) {
  return `$${value.toFixed(2)}`
}

function creditedFor(pkg: RechargePackage) {
  return Math.round((pkg.amount + pkg.bonus) * normalizedMultiplier.value * 100) / 100
}

function packageNameKey(amount: number) {
  if (amount === 50) return 'payment.packageNames.trial'
  if (amount === 100) return 'payment.packageNames.standard'
  if (amount === 500) return 'payment.packageNames.advanced'
  return 'payment.packageNames.professional'
}

function packageDescriptionKey(amount: number) {
  if (amount === 50) return 'payment.packageDescriptions.trial'
  if (amount === 100) return 'payment.packageDescriptions.standard'
  if (amount === 500) return 'payment.packageDescriptions.advanced'
  return 'payment.packageDescriptions.professional'
}

function badgeKey(index: number) {
  if (index === filteredPackages.value.length - 1) return 'payment.bestValue'
  if (filteredPackages.value[index]?.amount === 500) return 'payment.popularChoice'
  return ''
}

function selectAmount(amount: number) {
  emit('update:modelValue', amount)
}
</script>
