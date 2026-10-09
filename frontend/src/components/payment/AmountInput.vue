<template>
  <div class="space-y-4">
    <div v-if="packageMode">
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
          :data-testid="`quick-amount-${pkg.amount}`"
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

    </div>
    <div v-else>
    <!-- Quick Amount Buttons -->
    <div>
      <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
        {{ t('payment.quickAmounts') }}
      </label>
      <div :class="['grid gap-4 pt-2', packageMode ? 'grid-cols-2 lg:grid-cols-4' : 'grid-cols-3']">
        <button
          v-for="amt in filteredAmounts"
          :key="amt"
          type="button"
          :class="[
            'relative rounded-lg border-2 px-3 py-3 text-center font-medium transition-colors',
            modelValue === amt
              ? 'border-primary-500 bg-primary-50 text-primary-700 dark:border-primary-400 dark:bg-primary-900/40 dark:text-primary-300'
              : quoteFor(amt).percent > 0
                ? 'border-red-200 bg-white text-gray-700 hover:border-red-300 dark:border-red-500/40 dark:bg-dark-800 dark:text-gray-200 dark:hover:border-red-400/60'
                : 'border-gray-200 bg-white text-gray-700 hover:border-gray-300 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200 dark:hover:border-dark-500',
          ]"
          :data-testid="`quick-amount-${amt}`"
          @click="selectAmount(amt)"
        >
          <!-- 促销价签（单行）：仅命中档位的金额显示；红底白字、内圈点线、右侧圆孔、整体旋转 -->
          <span
            v-if="quoteFor(amt).percent > 0"
            class="pointer-events-none absolute -right-2 -top-3 z-10 rotate-12"
            data-testid="quick-amount-bonus-badge"
          >
            <span
              class="relative flex items-center gap-1 whitespace-nowrap rounded bg-red-600 py-0.5 pl-1.5 pr-1 text-[11px] font-extrabold leading-tight tracking-tight text-white shadow-md ring-2 ring-white before:pointer-events-none before:absolute before:inset-[2px] before:rounded-sm before:border before:border-dotted before:border-white/70 dark:bg-red-500 dark:ring-dark-800"
            >
              <span>{{ badgeText(amt) }}</span>
              <span class="h-1 w-1 shrink-0 rounded-full bg-white"></span>
            </span>
          </span>
          <span class="block">{{ amt }}</span>
          <!-- 配置了优惠阶梯时，所有按钮都显示第二行，保持高度一致：赠金显示到账 USD，折扣显示折后实付 -->
          <span
            v-if="showSecondLine"
            :class="[
              'mt-0.5 block text-[11px] font-normal leading-tight',
              quoteFor(amt).percent > 0 ? 'text-red-600 dark:text-red-300' : 'text-gray-400 dark:text-gray-500',
            ]"
            data-testid="quick-amount-credited"
          >{{ secondLine(amt) }}</span>
        </button>
      </div>
    </div>

    <!-- Custom amounts are unavailable when fixed packages are supplied. -->
    <div v-if="!packageMode">
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
  </div>
</template><script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { RechargeBonusTier, RechargePackage } from '@/types/payment'
import { formatRechargeBonusNumber, quoteRechargeBonus, type RechargeBonusMode } from '@/utils/rechargeBonus'
import { formatPaymentAmount } from './currency'

const props = withDefaults(defineProps<{
  packages?: RechargePackage[]
  amounts?: number[]
  modelValue: number | null
  min?: number
  max?: number
  /** 充值优惠阶梯（按 min_amount 升序）；为空时不显示价签与第二行 */
  bonusTiers?: RechargeBonusTier[]
  /** 阶梯模式：bonus 赠金 / discount 折扣 */
  bonusMode?: RechargeBonusMode
  /** 充值倍率（1 支付币种 = multiplier USD），用于计算到账金额 */
  multiplier?: number
  /** 支付币种（折扣模式第二行实付金额的币种与精度） */
  currency?: string
}>(), {
  amounts: () => [10, 20, 50, 100, 200, 500, 1000, 2000, 5000],
  min: 0,
  max: 0,
  bonusTiers: () => [],
  bonusMode: 'bonus',
  multiplier: 1,
  currency: undefined,
})

const emit = defineEmits<{
  'update:modelValue': [value: number | null]
}>()

const { t } = useI18n()

const customText = ref('')

const packageMode = computed(() => props.bonusTiers.length === 0 && !!props.packages?.length)

const fallbackPackages: RechargePackage[] = [
  { amount: 50, bonus: 0 },
  { amount: 100, bonus: 20 },
  { amount: 500, bonus: 150 },
  { amount: 1000, bonus: 400 },
]

const resolvedPackages = computed(() => props.packages?.length ? props.packages : fallbackPackages)
const filteredPackages = computed(() => resolvedPackages.value.filter((pkg) =>
  (props.min <= 0 || pkg.amount >= props.min) && (props.max <= 0 || pkg.amount <= props.max),
))

// 0 = no limit
const filteredAmounts = computed(() =>
  (packageMode.value ? props.packages!.map(pkg => pkg.amount) : props.amounts).filter((a) => (props.min <= 0 || a >= props.min) && (props.max <= 0 || a <= props.max))
)

const showSecondLine = computed(() => props.bonusTiers.length > 0 || packageMode.value)

function currencyDigits(): number {
  if (!props.currency) return 2
  try {
    return new Intl.NumberFormat(undefined, { style: 'currency', currency: props.currency }).resolvedOptions().maximumFractionDigits ?? 2
  } catch {
    return 2
  }
}

function quoteFor(amt: number) {
  return quoteRechargeBonus(props.bonusTiers, amt, {
    packages: props.packages,
    multiplier: props.multiplier,
    mode: props.bonusMode,
    currencyDigits: currencyDigits(),
  })
}

function moneyFormatter(currency: string) {
  try {
    return new Intl.NumberFormat(undefined, { style: 'currency', currency, minimumFractionDigits: 0, maximumFractionDigits: 2 })
  } catch {
    return new Intl.NumberFormat(undefined, { style: 'currency', currency: 'CNY', minimumFractionDigits: 0, maximumFractionDigits: 2 })
  }
}

function formatMoney(value: number) { return moneyFormatter(props.currency || 'CNY').format(value) }
function formatCredit(value: number) { return `$${value.toFixed(2)}` }
function creditedFor(pkg: RechargePackage) {
  const multiplier = Number.isFinite(props.multiplier) && props.multiplier > 0 ? props.multiplier : 1
  return Math.round((pkg.amount + pkg.bonus) * multiplier * 100) / 100
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

// 价签文案：赠金「+20%」，折扣「20% OFF」
function badgeText(amt: number): string {
  if (packageMode.value) {
    const gift = props.packages!.find(pkg => pkg.amount === amt)?.bonus ?? 0
    return t('payment.bonusAmount', { amount: formatPaymentAmount(gift, props.currency) })
  }
  const percent = formatRechargeBonusNumber(quoteFor(amt).percent)
  return props.bonusMode === 'discount' ? `${percent}% OFF` : `+${percent}%`
}

function secondLine(amt: number): string {
  const quote = quoteFor(amt)
  if (quote.mode === 'discount') {
    return t('payment.rechargeBonus.payShort', { amount: formatPaymentAmount(quote.payBase, props.currency) })
  }
  return t('payment.rechargeBonus.creditedShort', { amount: '$' + quote.credited.toFixed(2) })
}

const placeholderText = computed(() => {
  if (props.min > 0 && props.max > 0) return `${props.min} - ${props.max}`
  if (props.min > 0) return `≥ ${props.min}`
  if (props.max > 0) return `≤ ${props.max}`
  return t('payment.enterAmount')
})

const AMOUNT_PATTERN = /^\d*(\.\d{0,2})?$/

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
