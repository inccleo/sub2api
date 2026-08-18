import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import DailyCheckinButton from '@/components/common/DailyCheckinButton.vue'
import type { DailyCheckinResult, DailyCheckinStatus } from '@/api/dailyCheckin'

const mocks = vi.hoisted(() => ({
  getStatus: vi.fn(),
  checkin: vi.fn(),
  getHistory: vi.fn(),
  refreshUser: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/dailyCheckin', () => ({
  dailyCheckinAPI: {
    getStatus: mocks.getStatus,
    checkin: mocks.checkin,
    getHistory: mocks.getHistory,
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
    locale: { value: 'en' },
  }),
}))

vi.mock('@/components/common/BaseDialog.vue', () => ({
  default: defineComponent({
    name: 'BaseDialog',
    props: {
      show: { type: Boolean, default: false },
      title: { type: String, default: '' },
    },
    emits: ['close'],
    setup(props, { slots }) {
      return () =>
        props.show
          ? h('div', { 'data-testid': 'checkin-dialog' }, slots.default?.())
          : null
    },
  }),
}))

vi.mock('@/utils/apiError', () => ({
  extractApiErrorMessage: (
    err: unknown,
    fallback: string,
    i18nMap?: Record<string, string>,
  ) => {
    if (i18nMap && err && typeof err === 'object' && 'reason' in err) {
      const reason = String((err as { reason?: unknown }).reason ?? '')
      if (reason && i18nMap[reason]) return i18nMap[reason]
    }
    return fallback
  },
}))

function status(overrides: Partial<DailyCheckinStatus> = {}): DailyCheckinStatus {
  return {
    enabled: true,
    eligible: true,
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

async function openDialog(wrapper: Awaited<ReturnType<typeof mountButton>>) {
  await wrapper.get('button').trigger('click')
  await flushPromises()
  expect(wrapper.find('[data-testid="checkin-dialog"]').exists()).toBe(true)
}

describe('DailyCheckinButton', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.refreshUser.mockResolvedValue(undefined)
    mocks.getHistory.mockResolvedValue([])
  })

  it('stays hidden when daily check-in is disabled', async () => {
    mocks.getStatus.mockResolvedValue(status({ enabled: false }))

    const wrapper = await mountButton()

    expect(wrapper.find('button').exists()).toBe(false)
  })

  it('opens the calendar dialog for already-checked-in users', async () => {
    mocks.getStatus.mockResolvedValue(
      status({ checked_in_today: true, today_reward: 0.1 }),
    )

    const wrapper = await mountButton()
    await openDialog(wrapper)

    expect(mocks.getHistory).toHaveBeenCalled()
    expect(wrapper.get('[data-testid="daily-checkin-action"]').attributes('disabled')).toBeDefined()
  })

  it('checks in once from the calendar dialog and preserves completed state when refreshes fail', async () => {
    // mount + openDialog both call getStatus before check-in; only post-check-in should flip state
    mocks.getStatus
      .mockResolvedValueOnce(status())
      .mockResolvedValueOnce(status())
      .mockResolvedValue(status({ checked_in_today: true, today_reward: 0.1, current_streak: 3 }))
    mocks.checkin.mockResolvedValue(result())
    mocks.refreshUser.mockRejectedValue(new Error('user refresh failed'))
    const wrapper = await mountButton()

    await openDialog(wrapper)
    await wrapper.get('[data-testid="daily-checkin-action"]').trigger('click')
    await flushPromises()

    expect(mocks.checkin).toHaveBeenCalledTimes(1)
    expect(mocks.showSuccess).toHaveBeenCalledTimes(1)
    expect(mocks.showError).not.toHaveBeenCalled()
  })

  it('shows the entry but disables claiming when the user is not eligible', async () => {
    mocks.getStatus.mockResolvedValue(status({ eligible: false }))
    const wrapper = await mountButton()

    expect(wrapper.find('button').exists()).toBe(true)
    await openDialog(wrapper)

    expect(wrapper.text()).toContain('profile.dailyCheckin.notEligible')
    expect(wrapper.get('[data-testid="daily-checkin-action"]').attributes('disabled')).toBeDefined()
    expect(mocks.checkin).not.toHaveBeenCalled()
  })

  it('shows an error and allows retry after the check-in request fails', async () => {
    mocks.getStatus.mockResolvedValue(status())
    mocks.checkin.mockRejectedValue(new Error('request failed'))
    const wrapper = await mountButton()

    await openDialog(wrapper)
    await wrapper.get('[data-testid="daily-checkin-action"]').trigger('click')
    await flushPromises()

    expect(mocks.showError).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-testid="daily-checkin-action"]').attributes('disabled')).toBeUndefined()
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

    await openDialog(wrapper)
    const action = wrapper.get('[data-testid="daily-checkin-action"]')
    await Promise.all([action.trigger('click'), action.trigger('click')])
    expect(mocks.checkin).toHaveBeenCalledTimes(1)

    resolveCheckin(result())
    await flushPromises()
  })
})
