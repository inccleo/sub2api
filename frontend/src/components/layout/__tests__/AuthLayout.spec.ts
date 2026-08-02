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

  it('preserves the configured logo, title and subtitle by default', () => {
    const wrapper = mount(AuthLayout)

    expect(wrapper.get('[data-testid="auth-brand-logo"]').attributes('src')).toBe(
      '/configured-logo.png'
    )
    expect(wrapper.get('[data-testid="auth-brand-title"]').text()).toBe('TOPAPI')
    expect(wrapper.get('[data-testid="auth-brand-subtitle"]').text()).toBe('AI API Platform')
    expect(fetchPublicSettings).toHaveBeenCalledOnce()
  })

  it('prefers the configured site logo over a page-level logoSrc override', () => {
    const wrapper = mount(AuthLayout, {
      props: {
        logoSrc: '/uploads/hardcoded-logo.png',
        showBrandTitle: false,
        showBrandSubtitle: false,
        wideLogo: true
      }
    })

    expect(wrapper.get('[data-testid="auth-brand-logo"]').attributes('src')).toBe(
      '/configured-logo.png'
    )
    expect(wrapper.find('[data-testid="auth-brand-title"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="auth-brand-subtitle"]').exists()).toBe(false)
    expect(
      wrapper.get('[data-testid="auth-brand-logo"]').element.parentElement?.classList.contains('w-64')
    ).toBe(true)
  })
})
