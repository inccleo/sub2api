<template>
  <div class="space-y-4">
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
            'group relative min-h-[168px] overflow-hidden rounded-2xl border p-4 text-left transition-all duration-200 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-dark-900',
            modelValue === pkg.amount
              ? 'border-primary-500 bg-primary-50/80 shadow-lg shadow-primary-100/70 ring-1 ring-primary-500 dark:border-primary-400 dark:bg-primary-950/30 dark:shadow-none dark:ring-primary-400'
              : 'border-gray-200 bg-white hover:-translate-y-1 hover:border-primary-300 hover:shadow-lg dark:border-dark-600 dark:bg-dark-800 dark:hover:border-primary-700',
          ]"
          @click="selectAmount(pkg.amount)"
        >
          <span
            v-if="badgeKey(index)"
            class="absolute right-0 top-0 rounded-bl-xl bg-primary-600 px-3 py-1 text-[11px] font-bold text-white dark:bg-primary-500"
          >
            {{ t(badgeKey(index)) }}
          </span>

          <span class="block text-base font-bold text-gray-950 dark:text-white">
            {{ t(packageNameKey(pkg.amount)) }}
          </span>
          <span class="mt-1 block text-xs text-gray-400 dark:text-gray-500">
            {{ t(packageDescriptionKey(pkg.amount)) }}
          </span>
          <span class="mt-4 block text-3xl font-black tracking-tight text-gray-950 dark:text-white">
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

    <div>
      <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
        {{ t('payment.customAmount') }}
      </label>
      <div class="relative">
        <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-dark-500">
          $
        </span>
        <input
          type="text"
          inputmode="decimal"
          :value="customText"
          :placeholder="placeholderText"
          class="input w-full py-3 pl-8 pr-4"
          @input="handleInput"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { RechargePackage } from '@/types/payment'

const fallbackPackages: RechargePackage[] = [
  { amount: 50, bonus: 0 },
  { amount: 100, bonus: 20 },
  { amount: 500, bonus: 150 },
  { amount: 1000, bonus: 400 },
]

const props = withDefaults(defineProps<{
  packages?: RechargePackage[]
  modelValue: number | null
  min?: number
  max?: number
  currency?: string
  locale?: string
  creditMultiplier?: number
}>(), {
  packages: () => [
    { amount: 50, bonus: 0 },
    { amount: 100, bonus: 20 },
    { amount: 500, bonus: 150 },
    { amount: 1000, bonus: 400 },
  ],
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

const customText = ref('')

const normalizedMultiplier = computed(() =>
  Number.isFinite(props.creditMultiplier) && props.creditMultiplier > 0
    ? props.creditMultiplier
    : 1
)

const resolvedPackages = computed(() =>
  props.packages.length > 0 ? props.packages : fallbackPackages
)

const filteredPackages = computed(() =>
  resolvedPackages.value.filter((pkg) =>
    (props.min <= 0 || pkg.amount >= props.min)
    && (props.max <= 0 || pkg.amount <= props.max)
  )
)

const placeholderText = computed(() => {
  if (props.min > 0 && props.max > 0) return `${props.min} - ${props.max}`
  if (props.min > 0) return `≥ ${props.min}`
  if (props.max > 0) return `≤ ${props.max}`
  return t('payment.enterAmount')
})

const AMOUNT_PATTERN = /^\d*(\.\d{0,2})?$/

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

function selectAmount(amt: number) {
  customText.value = String(amt)
  emit('update:modelValue', amt)
}

function handleInput(e: Event) {
  const input = e.target as HTMLInputElement
  const val = input.value
  if (!AMOUNT_PATTERN.test(val)) {
    input.value = customText.value
    return
  }
  customText.value = val
  if (val === '') {
    emit('update:modelValue', null)
    return
  }
  const num = parseFloat(val)
  if (!isNaN(num) && num > 0) {
    emit('update:modelValue', num)
  } else {
    emit('update:modelValue', null)
  }
}

watch(() => props.modelValue, (v) => {
  if (v !== null && String(v) !== customText.value) {
    customText.value = String(v)
  }
}, { immediate: true })
</script>
