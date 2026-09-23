import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import AmountInput from '../AmountInput.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)

describe('recharge package selector', () => {
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
