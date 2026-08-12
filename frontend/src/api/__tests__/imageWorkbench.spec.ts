import { beforeEach, describe, expect, it, vi } from 'vitest'

const get = vi.fn()
const post = vi.fn()

vi.mock('../client', () => ({ apiClient: { get, post } }))

describe('image workbench api', () => {
  beforeEach(() => vi.clearAllMocks())

  it('submits without exposing an API key', async () => {
    post.mockResolvedValue({ data: { id: 'imgtask_1', status: 'processing' } })
    const { submitImageWorkbenchTask } = await import('../imageWorkbench')
    const payload = { prompt: 'city', size: '1920x1088', quality: 'high', n: 3 }

    await expect(submitImageWorkbenchTask(payload)).resolves.toMatchObject({ id: 'imgtask_1' })
    expect(post).toHaveBeenCalledWith('/image-workbench/tasks', {
      prompt: 'city',
      size: '1920x1088',
      quality: 'high',
      n: 3,
    })
    expect(JSON.stringify(post.mock.calls)).not.toContain('api_key')
  })

  it('submits edits as multipart when reference images are present', async () => {
    post.mockResolvedValue({ data: { id: 'imgtask_edit', status: 'processing' } })
    const { submitImageWorkbenchTask } = await import('../imageWorkbench')
    const file = new File([new Uint8Array([1, 2, 3])], 'ref.png', { type: 'image/png' })

    await submitImageWorkbenchTask({
      prompt: 'make blue',
      size: '1024x1024',
      quality: 'auto',
      n: 2,
      images: [file],
    })

    expect(post).toHaveBeenCalledTimes(1)
    const [url, body, config] = post.mock.calls[0]
    expect(url).toBe('/image-workbench/tasks')
    expect(body).toBeInstanceOf(FormData)
    expect((body as FormData).get('prompt')).toBe('make blue')
    expect((body as FormData).get('n')).toBe('2')
    expect((body as FormData).get('image')).toBeInstanceOf(File)
    expect(config?.timeout).toBe(120000)
  })

  it('polls the JWT workbench endpoint', async () => {
    get.mockResolvedValue({ data: { id: 'imgtask_1', status: 'completed' } })
    const { getImageWorkbenchTask } = await import('../imageWorkbench')
    await getImageWorkbenchTask('imgtask_1')
    expect(get).toHaveBeenCalledWith('/image-workbench/tasks/imgtask_1')
  })

  it('collects all result image urls', async () => {
    const { collectImageWorkbenchURLs } = await import('../imageWorkbench')
    const urls = collectImageWorkbenchURLs({
      id: 'imgtask_1',
      task_id: 'imgtask_1',
      object: 'image.generation.task',
      status: 'completed',
      image_url: 'https://cdn.example/a.png',
      result: { data: [{ url: 'https://cdn.example/a.png' }, { url: 'https://cdn.example/b.png' }] },
      created_at: 1,
      expires_at: 2,
    })
    expect(urls).toEqual(['https://cdn.example/a.png', 'https://cdn.example/b.png'])
  })
})
