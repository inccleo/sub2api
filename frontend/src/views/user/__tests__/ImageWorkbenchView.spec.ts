import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ImageWorkbenchView from '../ImageWorkbenchView.vue'

const { getImageWorkbenchConfig, getImageWorkbenchTask, submitImageWorkbenchTask } = vi.hoisted(() => ({
  getImageWorkbenchConfig: vi.fn(),
  getImageWorkbenchTask: vi.fn(),
  submitImageWorkbenchTask: vi.fn(),
}))

vi.mock('@/api/imageWorkbench', () => ({
  collectImageWorkbenchURLs: () => [],
  getImageWorkbenchConfig,
  getImageWorkbenchTask,
  submitImageWorkbenchTask,
}))

vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

describe('ImageWorkbenchView generation controls', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
    getImageWorkbenchConfig.mockResolvedValue({
      ready: true,
      models: ['gpt-image-2'],
      sizes: ['1024x1024'],
      qualities: ['auto', 'low', 'medium', 'high'],
      max_n: 10,
      max_images: 4,
      supports_transparent_background: true,
    })
    submitImageWorkbenchTask.mockResolvedValue({
      id: 'imgtask_1',
      task_id: 'imgtask_1',
      object: 'image.generation.task',
      status: 'completed',
      created_at: 1,
      expires_at: 2,
    })
  })

  it('aligns custom dimensions to 16 and submits transparent background', async () => {
    const wrapper = mount(ImageWorkbenchView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })
    await flushPromises()

    const dimensionInputs = wrapper.findAll('input[type="number"]')
    expect(dimensionInputs).toHaveLength(2)
    await dimensionInputs[0].setValue('1025')
    await dimensionInputs[0].trigger('change')
    expect((dimensionInputs[0].element as HTMLInputElement).value).toBe('1040')

    const switches = wrapper.findAll('[role="switch"]')
    expect(switches).toHaveLength(2)
    await switches[0].trigger('click')
    await dimensionInputs[0].setValue('1025')
    await dimensionInputs[0].trigger('change')
    expect((dimensionInputs[0].element as HTMLInputElement).value).toBe('1025')

    await switches[1].trigger('click')
    await wrapper.get('textarea').setValue('transparent icon')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(submitImageWorkbenchTask).toHaveBeenCalledWith(expect.objectContaining({
      prompt: 'transparent icon',
      size: '1025x1024',
      background: 'transparent',
    }))
    wrapper.unmount()
  })
})
