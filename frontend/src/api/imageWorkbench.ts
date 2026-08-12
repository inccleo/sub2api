import { apiClient } from './client'

export type ImageWorkbenchTaskStatus = 'processing' | 'completed' | 'failed'

export interface ImageWorkbenchConfig {
  ready: boolean
  models: string[]
  sizes: string[]
  qualities: string[]
  max_n?: number
  max_images?: number
  supports_edit?: boolean
}

export interface ImageWorkbenchTask {
  id: string
  task_id: string
  object: string
  status: ImageWorkbenchTaskStatus
  http_status?: number
  image_url?: string
  result?: unknown
  error?: { type?: string; code?: string; message?: string } | string
  created_at: number
  completed_at?: number
  expires_at: number
}

export interface SubmitImageWorkbenchTask {
  prompt: string
  size: string
  quality: string
  n?: number
  /** Reference images: presence switches the workbench to image edit mode. */
  images?: File[]
}

export async function getImageWorkbenchConfig(): Promise<ImageWorkbenchConfig> {
  const response = await apiClient.get<ImageWorkbenchConfig>('/image-workbench/config')
  return response.data
}

export async function submitImageWorkbenchTask(payload: SubmitImageWorkbenchTask): Promise<ImageWorkbenchTask> {
  const images = (payload.images || []).filter(Boolean)
  if (images.length > 0) {
    const form = new FormData()
    form.append('prompt', payload.prompt)
    form.append('size', payload.size)
    form.append('quality', payload.quality)
    form.append('n', String(payload.n && payload.n > 0 ? payload.n : 1))
    for (const file of images) {
      form.append('image', file, file.name || 'reference.png')
    }
    const response = await apiClient.post<ImageWorkbenchTask>('/image-workbench/tasks', form, {
      timeout: 120000,
      headers: { 'Content-Type': 'multipart/form-data' },
      transformRequest: [
        (data, headers) => {
          // Let the browser set multipart boundary; default JSON content-type breaks FormData.
          if (data instanceof FormData && headers) {
            delete (headers as Record<string, unknown>)['Content-Type']
          }
          return data
        },
      ],
    })
    return response.data
  }

  const response = await apiClient.post<ImageWorkbenchTask>('/image-workbench/tasks', {
    prompt: payload.prompt,
    size: payload.size,
    quality: payload.quality,
    n: payload.n,
  })
  return response.data
}

export async function getImageWorkbenchTask(taskId: string): Promise<ImageWorkbenchTask> {
  const response = await apiClient.get<ImageWorkbenchTask>(`/image-workbench/tasks/${encodeURIComponent(taskId)}`)
  return response.data
}

/** Collect all image URLs from a completed task (primary image_url + result.data[].url). */
export function collectImageWorkbenchURLs(task: ImageWorkbenchTask | null | undefined): string[] {
  if (!task) return []
  const urls: string[] = []
  const push = (value: unknown) => {
    if (typeof value !== 'string') return
    const trimmed = value.trim()
    if (trimmed && !urls.includes(trimmed)) urls.push(trimmed)
  }
  push(task.image_url)
  const result = task.result as { data?: Array<{ url?: string }> } | undefined
  if (Array.isArray(result?.data)) {
    for (const item of result.data) push(item?.url)
  }
  return urls
}
