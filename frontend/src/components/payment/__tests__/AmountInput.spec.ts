import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import AmountInput from '../AmountInput.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) =>
      params && 'amount' in params ? `${key} ${String(params.amount)}` : key,
  }),
}))
enableAutoUnmount(afterEach)

describe('recharge package selector', () => {
  it('filters packages by limits and converts the gift with the credit multiplier', () => {
    const wrapper = mount(AmountInput, { props: {
      modelValue: null, min: 100, max: 500, multiplier: 0.14,
      packages: [{ amount: 50, bonus: 0 }, { amount: 100, bonus: 20 }, { amount: 500, bonus: 150 }, { amount: 1000, bonus: 400 }],
    } })
    expect(wrapper.findAll('button')).toHaveLength(2)
    expect(wrapper.get('[data-testid="quick-amount-100"]').text()).toContain('$16.80')
    expect(wrapper.get('[data-testid="quick-amount-500"]').text()).toContain('$91.00')
  })

  it('uses explicit upstream tiers without stacking package gifts', () => {
    const wrapper = mount(AmountInput, { props: {
      modelValue: null, amounts: [100], packages: [{ amount: 100, bonus: 20 }],
      bonusTiers: [{ min_amount: 100, bonus_percent: 10 }],
    } })
    expect(wrapper.find('input').exists()).toBe(true)
    expect(wrapper.get('[data-testid="quick-amount-100"]').text()).toContain('$110.00')
  })

  it('offers only the configured packages and does not accept a custom amount', async () => {
    const wrapper = mount(AmountInput, {
      props: {
        modelValue: null,
        packages: [
          { amount: 50, bonus: 0 },
          { amount: 100, bonus: 20 },
        ],
      },
    })

    expect(wrapper.find('input').exists()).toBe(false)
    const buttons = wrapper.findAll('button')
    expect(buttons).toHaveLength(2)

    await buttons[1].trigger('click')
    expect(wrapper.emitted('update:modelValue')).toEqual([[100]])
  })
})

describe('recharge bonus hints on quick amounts', () => {
  const tiers = [
    { min_amount: 100, bonus_percent: 20 },
    { min_amount: 500, bonus_percent: 30 },
  ]

  it('renders no badge or second line when no tiers are configured', () => {
    const wrapper = mount(AmountInput, { props: { modelValue: null, amounts: [50, 100, 500] } })
    expect(wrapper.find('[data-testid="quick-amount-bonus-badge"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="quick-amount-credited"]').exists()).toBe(false)
  })

  it('bonus mode: price tag only on amounts that hit a tier, credited totals on every button', () => {
    const wrapper = mount(AmountInput, { props: { modelValue: null, amounts: [50, 100, 500], bonusTiers: tiers } })
    const below = wrapper.get('[data-testid="quick-amount-50"]')
    const first = wrapper.get('[data-testid="quick-amount-100"]')
    const second = wrapper.get('[data-testid="quick-amount-500"]')

    expect(below.find('[data-testid="quick-amount-bonus-badge"]').exists()).toBe(false)
    expect(below.get('[data-testid="quick-amount-credited"]').text()).toContain('$50.00')
    expect(first.get('[data-testid="quick-amount-bonus-badge"]').text()).toBe('+20%')
    expect(first.get('[data-testid="quick-amount-credited"]').text()).toContain('$120.00')
    expect(second.get('[data-testid="quick-amount-bonus-badge"]').text()).toBe('+30%')
    expect(second.get('[data-testid="quick-amount-credited"]').text()).toContain('$650.00')
  })

  it('bonus mode: matches tiers by the entered amount but credits by the multiplier', () => {
    const wrapper = mount(AmountInput, {
      props: { modelValue: null, amounts: [1000], bonusTiers: tiers, multiplier: 0.14 },
    })
    const button = wrapper.get('[data-testid="quick-amount-1000"]')
    expect(button.get('[data-testid="quick-amount-bonus-badge"]').text()).toBe('+30%')
    // 1000 × 0.14 = 140 base, +30% = 182
    expect(button.get('[data-testid="quick-amount-credited"]').text()).toContain('$182.00')
  })

  it('discount mode: tag reads N% OFF and the second line shows the discounted payment', () => {
    const wrapper = mount(AmountInput, {
      props: { modelValue: null, amounts: [50, 500], bonusTiers: tiers, bonusMode: 'discount', currency: 'USD' },
    })
    const below = wrapper.get('[data-testid="quick-amount-50"]')
    const hit = wrapper.get('[data-testid="quick-amount-500"]')
    expect(below.find('[data-testid="quick-amount-bonus-badge"]').exists()).toBe(false)
    expect(below.get('[data-testid="quick-amount-credited"]').text()).toContain('50.00')
    expect(hit.get('[data-testid="quick-amount-bonus-badge"]').text()).toBe('30% OFF')
    // 500 × (1 − 30%) = 350
    expect(hit.get('[data-testid="quick-amount-credited"]').text()).toContain('350.00')
  })
})
