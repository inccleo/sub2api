import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { reactive } from 'vue'
import DesktopAuthorizeView from '../DesktopAuthorizeView.vue'
import { getDesktopAuthorization, decideDesktopAuthorization } from '@/api/desktopAuth'

const route = reactive({ fullPath: '/oauth/authorize', query: {} as Record<string, string | string[]> })
vi.mock('vue-router', () => ({ useRoute: () => route }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/desktopAuth', () => ({ getDesktopAuthorization: vi.fn(), decideDesktopAuthorization: vi.fn() }))
vi.mock('@/components/layout/AuthLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))

const params = {
  client_id: 'sub2api-desktop', redirect_uri: 'sub2api://oauth/callback', response_type: 'code',
  scope: 'account', state: 's'.repeat(43), code_challenge: 'c'.repeat(43), code_challenge_method: 'S256'
}
const details = { client_name: 'Sub2API Desktop', redirect_uri: params.redirect_uri, scope: 'account', email: 'user@example.com', is_admin: false }
const assign = vi.fn()
const originalLocation = window.location
const wrappers: ReturnType<typeof mount>[] = []
function render() {
  const wrapper = mount(DesktopAuthorizeView, { global: { stubs: { AuthLayout: { template: '<main><slot /></main>' } } } })
  wrappers.push(wrapper)
  return wrapper
}

beforeEach(() => {
  vi.clearAllMocks()
  route.query = { ...params }
  route.fullPath = '/oauth/authorize'
  vi.mocked(getDesktopAuthorization).mockResolvedValue({ ...details })
  vi.mocked(decideDesktopAuthorization).mockResolvedValue(`${params.redirect_uri}?code=code&state=${params.state}`)
  Object.defineProperty(window, 'location', { configurable: true, value: { ...originalLocation, assign } })
})
afterEach(() => {
  wrappers.splice(0).forEach(w => w.unmount())
  Object.defineProperty(window, 'location', { configurable: true, value: originalLocation })
})

describe('Desktop consent', () => {
  it('shows the account, validates first, and requires an explicit click', async () => {
    const wrapper = render()
    expect(wrapper.findAll('button')).toHaveLength(0)
    await flushPromises()
    expect(wrapper.text()).toContain('user@example.com')
    expect(decideDesktopAuthorization).not.toHaveBeenCalled()
    await wrapper.findAll('button')[1].trigger('click')
    await flushPromises()
    expect(decideDesktopAuthorization).toHaveBeenCalledWith(params, 'approve')
    expect(assign).toHaveBeenCalledOnce()
    expect(wrapper.find('a').attributes('href')).toContain('sub2api://oauth/callback?')
    expect(wrapper.findAll('button')).toHaveLength(0)
  })

  it('warns for administrator permissions and supports denial', async () => {
    vi.mocked(getDesktopAuthorization).mockResolvedValue({ ...details, is_admin: true })
    const wrapper = render()
    await flushPromises()
    expect(wrapper.text()).toContain('desktopAuth.adminWarning')
    await wrapper.findAll('button')[0].trigger('click')
    await flushPromises()
    expect(decideDesktopAuthorization).toHaveBeenCalledWith(params, 'deny')
    expect(wrapper.text()).toContain('desktopAuth.denied')
  })

  it('never offers approval when validation fails or parameters repeat', async () => {
    vi.mocked(getDesktopAuthorization).mockRejectedValue(new Error('disabled'))
    const wrapper = render()
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    expect(wrapper.findAll('button')).toHaveLength(0)
    route.query = { ...params, state: ['a', 'b'] }
    route.fullPath = '/oauth/authorize?changed'
    await flushPromises()
    expect(getDesktopAuthorization).toHaveBeenCalledTimes(1)
    expect(decideDesktopAuthorization).not.toHaveBeenCalled()
  })

  it('does not navigate to an old callback when the request changes during approval', async () => {
    let complete!: (value: string) => void
    vi.mocked(decideDesktopAuthorization).mockImplementation(() => new Promise(resolve => { complete = resolve }))
    const wrapper = render()
    await flushPromises()
    await wrapper.findAll('button')[1].trigger('click')
    route.query = { ...params, state: 't'.repeat(43) }
    route.fullPath = '/oauth/authorize?new'
    await flushPromises()
    complete(`${params.redirect_uri}?code=old&state=${params.state}`)
    await flushPromises()
    expect(assign).not.toHaveBeenCalled()
    expect(wrapper.find('a').exists()).toBe(false)
  })
})
