import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import DailyCheckinButton from '@/components/common/DailyCheckinButton.vue'
import type { DailyCheckinResult, DailyCheckinStatus } from '@/api/dailyCheckin'

const mocks = vi.hoisted(() => ({
  getStatus: vi.fn(),
  checkin: vi.fn(),
  refreshUser: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/dailyCheckin', () => ({
  dailyCheckinAPI: {
    getStatus: mocks.getStatus,
    checkin: mocks.checkin,
  },
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    refreshUser: mocks.refreshUser,
  }),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess: mocks.showSuccess,
    showError: mocks.showError,
  }),
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

function status(overrides: Partial<DailyCheckinStatus> = {}): DailyCheckinStatus {
  return {
    enabled: true,
    checked_in_today: false,
    daily_reward: 0.1,
    weekly_bonus: 0.5,
    current_streak: 2,
    days_until_bonus: 5,
    server_timezone: 'Asia/Shanghai',
    ...overrides,
  }
}

function result(overrides: Partial<DailyCheckinResult> = {}): DailyCheckinResult {
  return {
    reward_amount: 0.1,
    base_reward: 0.1,
    bonus_reward: 0,
    new_balance: 0.1,
    current_streak: 3,
    checked_in_at: '2026-07-31T12:00:00+08:00',
    ...overrides,
  }
}

async function mountButton() {
  const wrapper = mount(DailyCheckinButton)
  await flushPromises()
  return wrapper
}

describe('DailyCheckinButton', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.refreshUser.mockResolvedValue(undefined)
  })

  it('stays hidden when daily check-in is disabled', async () => {
    mocks.getStatus.mockResolvedValue(status({ enabled: false }))

    const wrapper = await mountButton()

    expect(wrapper.find('button').exists()).toBe(false)
  })

  it('renders an already-checked-in disabled state', async () => {
    mocks.getStatus.mockResolvedValue(
      status({ checked_in_today: true, today_reward: 0.1 }),
    )

    const wrapper = await mountButton()

    expect(wrapper.get('button').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('profile.dailyCheckin.checkedIn')
  })

  it('checks in once and preserves the completed state when refreshes fail', async () => {
    mocks.getStatus
      .mockResolvedValueOnce(status())
      .mockRejectedValueOnce(new Error('status refresh failed'))
    mocks.checkin.mockResolvedValue(result())
    mocks.refreshUser.mockRejectedValue(new Error('user refresh failed'))
    const wrapper = await mountButton()

    await wrapper.get('button').trigger('click')
    await flushPromises()

    expect(mocks.checkin).toHaveBeenCalledTimes(1)
    expect(mocks.showSuccess).toHaveBeenCalledTimes(1)
    expect(mocks.showError).not.toHaveBeenCalled()
    expect(wrapper.get('button').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('profile.dailyCheckin.checkedIn')
  })

  it('shows an error and allows retry after the check-in request fails', async () => {
    mocks.getStatus.mockResolvedValue(status())
    mocks.checkin.mockRejectedValue(new Error('request failed'))
    const wrapper = await mountButton()

    await wrapper.get('button').trigger('click')
    await flushPromises()

    expect(mocks.showError).toHaveBeenCalledTimes(1)
    expect(wrapper.get('button').attributes('disabled')).toBeUndefined()
  })

  it('does not submit twice while a check-in is pending', async () => {
    mocks.getStatus.mockResolvedValue(status())
    let resolveCheckin!: (value: DailyCheckinResult) => void
    mocks.checkin.mockImplementation(
      () =>
        new Promise<DailyCheckinResult>((resolve) => {
          resolveCheckin = resolve
        }),
    )
    const wrapper = await mountButton()

    const button = wrapper.get('button')
    await Promise.all([button.trigger('click'), button.trigger('click')])
    expect(mocks.checkin).toHaveBeenCalledTimes(1)

    resolveCheckin(result())
    await flushPromises()
  })
})
