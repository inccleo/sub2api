import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AmountInput from '../AmountInput.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, string>) => {
      if (key === 'payment.bonusAmount') return `Bonus ${params?.amount}`
      if (key === 'payment.creditedAmount') return `Receive ${params?.amount}`
      if (key === 'payment.selectQuickAmount') return `Select ${params?.amount}`
      return key
    },
  }),
}))

describe('AmountInput', () => {
  it('renders large selectable amount cards and the multiplier bonus', () => {
    const wrapper = mount(AmountInput, {
      props: {
        modelValue: 100,
        packages: [
          { amount: 50, bonus: 0 },
          { amount: 100, bonus: 20 },
          { amount: 500, bonus: 150 },
          { amount: 1000, bonus: 400 },
        ],
        currency: 'CNY',
        locale: 'zh-CN',
        creditMultiplier: 1,
      },
    })

    const cards = wrapper.findAll('button')
    expect(cards).toHaveLength(4)
    expect(cards[1].attributes('aria-pressed')).toBe('true')
    expect(cards[1].text()).toContain('Bonus ¥20')
    expect(cards[2].text()).toContain('Bonus ¥150')
    expect(cards[3].text()).toContain('Bonus ¥400')
  })

  it('applies the configured bonus before converting the credited balance', () => {
    const wrapper = mount(AmountInput, {
      props: {
        modelValue: null,
        packages: [{ amount: 100, bonus: 20 }],
        creditMultiplier: 0.14,
      },
    })

    expect(wrapper.text()).toContain('Receive $16.80')
    expect(wrapper.text()).toContain('Bonus')
  })

  it('selects a package card', async () => {
    const wrapper = mount(AmountInput, {
      props: {
        modelValue: null,
        packages: [
          { amount: 50, bonus: 0 },
          { amount: 100, bonus: 20 },
        ],
      },
    })

    await wrapper.findAll('button')[1].trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([100])
  })
})
