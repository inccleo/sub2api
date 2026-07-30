import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AuthLayout from '../AuthLayout.vue'

const fetchPublicSettings = vi.fn()

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    siteName: 'TOPAPI',
    siteLogo: '/configured-logo.png',
    cachedPublicSettings: {
      site_subtitle: 'AI API Platform'
    },
    publicSettingsLoaded: true,
    fetchPublicSettings
  })
}))

describe('AuthLayout branding', () => {
  beforeEach(() => {
    fetchPublicSettings.mockClear()
  })

  it('preserves the configured logo and title by default', () => {
    const wrapper = mount(AuthLayout)

    expect(wrapper.get('[data-testid="auth-brand-logo"]').attributes('src')).toBe(
      '/configured-logo.png'
    )
    expect(wrapper.get('[data-testid="auth-brand-title"]').text()).toBe('TOPAPI')
    expect(fetchPublicSettings).toHaveBeenCalledOnce()
  })

  it('supports a wide page-specific logo with the brand title hidden', () => {
    const wrapper = mount(AuthLayout, {
      props: {
        logoSrc: '/uploads/topopenai-logo-wide.png',
        showBrandTitle: false,
        wideLogo: true
      }
    })

    expect(wrapper.get('[data-testid="auth-brand-logo"]').attributes('src')).toBe(
      '/uploads/topopenai-logo-wide.png'
    )
    expect(wrapper.find('[data-testid="auth-brand-title"]').exists()).toBe(false)
    expect(
      wrapper.get('[data-testid="auth-brand-logo"]').element.parentElement?.classList.contains('w-64')
    ).toBe(true)
  })
})
