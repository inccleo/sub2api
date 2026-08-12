<template>
  <AppLayout>
    <!-- 大屏：AppLayout fillHeight 锁死视口，整页不滚；仅右侧历史列表滚动 -->
    <div class="mx-auto flex w-full max-w-[1600px] flex-col gap-2 lg:h-full lg:min-h-0 lg:overflow-hidden">
      <div v-if="configError" class="shrink-0 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300">
        {{ configError }}
      </div>

      <div class="grid min-h-0 min-w-0 flex-1 grid-cols-1 gap-3 lg:grid-cols-[minmax(250px,290px)_minmax(0,1fr)_minmax(220px,260px)] lg:overflow-hidden">
        <!-- 左：提示词 + 默认展开的生成选项 -->
        <section class="min-h-0 min-w-0 rounded-lg border border-gray-200 bg-white p-3 shadow-sm dark:border-dark-700 dark:bg-dark-800 lg:overflow-y-auto lg:overscroll-contain">
          <form
            class="relative flex flex-col gap-2.5"
            :class="isDraggingReference ? 'ring-2 ring-primary-400 ring-offset-2 dark:ring-offset-dark-800' : ''"
            @submit.prevent="generate"
            @dragenter.prevent="onReferenceDragEnter"
            @dragover.prevent="onReferenceDragOver"
            @dragleave.prevent="onReferenceDragLeave"
            @drop.prevent="onReferenceDrop"
          >
            <input
              ref="fileInputRef"
              type="file"
              accept="image/png,image/jpeg,image/jpg,image/webp,image/gif"
              multiple
              class="hidden"
              @change="onReferenceFileInput"
            />

            <!-- 参考图（有图即进入编辑/图生图模式，对齐 chatgpt2api） -->
            <div>
              <div class="mb-1 flex items-center justify-between gap-2">
                <span class="text-sm font-medium text-gray-800 dark:text-gray-200">{{ t('imageWorkbench.referenceImages') }}</span>
                <button
                  type="button"
                  class="btn btn-ghost px-2 py-1 text-xs"
                  :disabled="submitting || !config || referenceImages.length >= maxImages"
                  @click="pickReferenceImages"
                >
                  <Icon name="upload" size="sm" />
                  {{ referenceImages.length ? t('imageWorkbench.addReference') : t('imageWorkbench.uploadReference') }}
                </button>
              </div>
              <div v-if="referenceImages.length" class="mb-1.5 flex flex-wrap gap-2">
                <div
                  v-for="(item, index) in referenceImages"
                  :key="item.id"
                  class="relative h-14 w-14 overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600"
                >
                  <img :src="item.previewUrl" :alt="item.file.name" class="h-full w-full object-cover" />
                  <button
                    type="button"
                    class="absolute right-0.5 top-0.5 flex h-5 w-5 items-center justify-center rounded-full bg-black/60 text-white hover:bg-black/80"
                    :title="t('imageWorkbench.removeReference')"
                    @click="removeReferenceImage(index)"
                  >
                    <Icon name="x" size="xs" />
                  </button>
                </div>
              </div>
              <p class="text-[11px] leading-4 text-gray-400">
                {{ t('imageWorkbench.referenceHint', { n: maxImages }) }}
              </p>
            </div>

            <label class="block">
              <span class="mb-1 block text-sm font-medium text-gray-800 dark:text-gray-200">{{ t('imageWorkbench.prompt') }}</span>
              <textarea
                v-model="prompt"
                rows="3"
                maxlength="32000"
                class="h-[4.75rem] w-full resize-y rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm leading-5 text-gray-900 outline-none transition focus:border-primary-500 focus:ring-2 focus:ring-primary-500/20 dark:border-dark-600 dark:bg-dark-900 dark:text-white"
                :placeholder="isEditMode ? t('imageWorkbench.promptPlaceholderEdit') : t('imageWorkbench.promptPlaceholder')"
                :disabled="submitting || !config"
                @paste="onPromptPaste"
              />
            </label>

            <div
              v-if="isDraggingReference"
              class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center rounded-lg border-2 border-dashed border-primary-500 bg-primary-50/80 text-sm font-medium text-primary-700 dark:bg-primary-950/70 dark:text-primary-200"
            >
              {{ t('imageWorkbench.dropReference') }}
            </div>

            <div class="rounded-lg border border-gray-200 dark:border-dark-600">
              <button
                type="button"
                class="flex w-full items-center justify-between gap-2 px-3 py-2 text-left"
                @click="settingsOpen = !settingsOpen"
              >
                <div class="min-w-0">
                  <div class="text-sm font-medium text-gray-900 dark:text-white">{{ t('imageWorkbench.settings') }}</div>
                  <div class="mt-0.5 truncate text-xs text-gray-500 dark:text-gray-400">{{ settingsSummary }}</div>
                </div>
                <Icon :name="settingsOpen ? 'chevronUp' : 'chevronDown'" size="sm" class="shrink-0 text-gray-400" />
              </button>

              <div v-show="settingsOpen" class="space-y-2.5 border-t border-gray-200 px-3 py-2.5 dark:border-dark-600">
                <div class="flex items-center justify-between gap-2">
                  <span class="text-xs font-medium text-gray-700 dark:text-gray-300">{{ t('imageWorkbench.model') }}</span>
                  <span class="truncate rounded-md bg-gray-100 px-2 py-0.5 text-xs text-gray-700 dark:bg-dark-900 dark:text-gray-200">{{ modelLabel }}</span>
                </div>

                <div>
                  <span class="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300">{{ t('imageWorkbench.quality') }}</span>
                  <div class="grid grid-cols-4 gap-1">
                    <button
                      v-for="option in qualityOptions"
                      :key="option"
                      type="button"
                      class="h-7 rounded-full border text-[11px] font-medium transition"
                      :class="quality === option
                        ? 'border-gray-900 bg-gray-900 text-white dark:border-white dark:bg-white dark:text-gray-900'
                        : 'border-gray-200 text-gray-600 hover:bg-gray-50 dark:border-dark-600 dark:text-gray-300 dark:hover:bg-dark-700'"
                      @click="quality = option"
                    >
                      {{ t(`imageWorkbench.qualityLabels.${option}`) }}
                    </button>
                  </div>
                </div>

                <div>
                  <span class="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300">{{ t('imageWorkbench.customSize') }}</span>
                  <div class="grid grid-cols-[1fr_auto_1fr] items-center gap-1">
                    <label class="flex items-center gap-1 rounded-md bg-gray-100 px-2 py-1 text-xs dark:bg-dark-900">
                      <span class="text-gray-500">{{ t('imageWorkbench.width') }}</span>
                      <input
                        v-model="customWidth"
                        type="number"
                        min="256"
                        max="4096"
                        inputmode="numeric"
                        class="w-full border-0 bg-transparent p-0 text-xs font-medium text-gray-900 outline-none dark:text-white"
                        :disabled="size === 'auto'"
                        @change="applyCustomSize"
                      />
                    </label>
                    <span class="text-gray-400">×</span>
                    <label class="flex items-center gap-1 rounded-md bg-gray-100 px-2 py-1 text-xs dark:bg-dark-900">
                      <span class="text-gray-500">{{ t('imageWorkbench.height') }}</span>
                      <input
                        v-model="customHeight"
                        type="number"
                        min="256"
                        max="4096"
                        inputmode="numeric"
                        class="w-full border-0 bg-transparent p-0 text-xs font-medium text-gray-900 outline-none dark:text-white"
                        :disabled="size === 'auto'"
                        @change="applyCustomSize"
                      />
                    </label>
                  </div>
                </div>

                <div>
                  <span class="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300">{{ t('imageWorkbench.aspectRatio') }}</span>
                  <div class="grid grid-cols-4 gap-1">
                    <button
                      v-for="option in aspectOptions"
                      :key="option.value"
                      type="button"
                      class="flex h-11 flex-col items-center justify-center gap-0.5 rounded-lg border px-0.5 text-center transition"
                      :class="size === option.value
                        ? 'border-gray-900 ring-1 ring-gray-900 dark:border-white dark:ring-white'
                        : 'border-gray-200 text-gray-600 hover:border-gray-300 dark:border-dark-600 dark:text-gray-300'"
                      @click="selectAspect(option)"
                    >
                      <span v-if="option.shape" class="block border border-current opacity-70" :class="option.shape" />
                      <span class="text-[10px] font-medium leading-none">{{ option.label }}</span>
                    </button>
                  </div>
                </div>

                <div>
                  <span class="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300">{{ t('imageWorkbench.count') }}</span>
                  <div class="grid grid-cols-5 gap-1">
                    <button
                      v-for="option in countOptions"
                      :key="option"
                      type="button"
                      class="h-7 rounded-full border text-[11px] font-medium transition"
                      :class="count === option
                        ? 'border-gray-900 bg-gray-900 text-white dark:border-white dark:bg-white dark:text-gray-900'
                        : 'border-gray-200 text-gray-600 hover:bg-gray-50 dark:border-dark-600 dark:text-gray-300 dark:hover:bg-dark-700'"
                      @click="count = option"
                    >
                      {{ option }}
                    </button>
                  </div>
                </div>
              </div>
            </div>

            <p v-if="submitError" class="text-sm text-red-600 dark:text-red-400">{{ submitError }}</p>

            <button type="submit" class="btn btn-primary w-full" :disabled="submitting || !config || !prompt.trim()">
              <Icon :name="submitting ? 'refresh' : 'sparkles'" size="sm" :class="submitting ? 'animate-spin' : ''" />
              {{
                submitting
                  ? (isEditMode ? t('imageWorkbench.editing') : t('imageWorkbench.generating'))
                  : (isEditMode ? t('imageWorkbench.editMode') : t('imageWorkbench.generateMode'))
              }}
            </button>
          </form>
        </section>

        <!-- 中：生成结果（大屏不滚） -->
        <section class="flex min-h-[280px] min-w-0 flex-col rounded-lg border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800 lg:min-h-0 lg:overflow-hidden">
          <div class="flex h-10 shrink-0 items-center justify-between gap-2 border-b border-gray-200 px-3 dark:border-dark-700">
            <div class="flex min-w-0 items-center gap-2">
              <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('imageWorkbench.result') }}</h2>
              <span v-if="resultImages.length > 1" class="text-xs text-gray-400">
                {{ t('imageWorkbench.resultCount', { n: resultImages.length }) }}
              </span>
            </div>
            <span v-if="activeTask" class="shrink-0 text-xs font-medium uppercase text-gray-400">{{ activeTask.status }}</span>
          </div>

          <div class="flex min-h-0 flex-1 flex-col p-2.5 lg:min-h-0">
            <div class="relative flex min-h-[200px] w-full flex-1 items-center justify-center overflow-hidden rounded-lg bg-gray-100 dark:bg-dark-900 lg:min-h-0">
              <!-- 多图网格 / 单图 -->
              <div
                v-if="activeTask?.status === 'completed' && resultImages.length"
                class="grid h-full w-full content-center gap-2 overflow-hidden p-2"
                :class="resultGridClass"
              >
                <button
                  v-for="(url, index) in resultImages"
                  :key="`${url}-${index}`"
                  type="button"
                  class="group relative overflow-hidden rounded-lg bg-white/50 transition hover:opacity-95 dark:bg-black/20"
                  :title="t('imageWorkbench.zoomHint')"
                  @click="openLightbox(index)"
                >
                  <img
                    :src="url"
                    :alt="`${activeRecord?.prompt || t('imageWorkbench.result')} #${index + 1}`"
                    class="mx-auto max-h-full max-w-full object-contain"
                    :class="resultImages.length === 1 ? 'max-h-full' : 'max-h-40 sm:max-h-52 lg:max-h-56'"
                  />
                  <span class="pointer-events-none absolute inset-0 flex items-center justify-center bg-black/0 opacity-0 transition group-hover:bg-black/25 group-hover:opacity-100">
                    <span class="rounded-full bg-white/90 px-2.5 py-1 text-xs font-medium text-gray-800 shadow">
                      {{ t('imageWorkbench.zoomHint') }}
                    </span>
                  </span>
                  <span
                    v-if="resultImages.length > 1"
                    class="absolute left-1.5 top-1.5 rounded bg-black/55 px-1.5 py-0.5 text-[10px] font-medium text-white"
                  >
                    {{ index + 1 }}
                  </span>
                </button>
              </div>

              <div v-else-if="activeTask?.status === 'processing'" class="max-w-sm px-6 text-center">
                <div class="mx-auto mb-4 h-10 w-10 animate-spin rounded-full border-2 border-gray-300 border-t-primary-600 dark:border-dark-600 dark:border-t-primary-400" />
                <p class="font-medium text-gray-900 dark:text-white">{{ t('imageWorkbench.processing') }}</p>
                <p class="mt-2 text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t('imageWorkbench.processingHint') }}</p>
              </div>
              <div v-else-if="activeTask?.status === 'failed'" class="max-w-sm px-6 text-center">
                <Icon name="exclamationCircle" size="xl" class="mx-auto text-red-500" />
                <p class="mt-4 font-medium text-gray-900 dark:text-white">{{ t('imageWorkbench.failed') }}</p>
                <p class="mt-2 break-words text-sm text-red-600 dark:text-red-400">{{ taskError(activeTask) }}</p>
              </div>
              <div v-else class="max-w-sm px-6 text-center">
                <Icon name="sparkles" size="xl" class="mx-auto text-gray-400" />
                <p class="mt-4 font-medium text-gray-900 dark:text-white">{{ t('imageWorkbench.ready') }}</p>
                <p class="mt-2 text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t('imageWorkbench.readyHint') }}</p>
              </div>
            </div>

            <div v-if="activeTask?.status === 'completed' && resultImages.length" class="mt-2.5 flex shrink-0 flex-wrap items-center justify-between gap-2">
              <p class="text-xs text-gray-400">{{ t('imageWorkbench.zoomHint') }}</p>
              <div class="flex flex-wrap justify-end gap-2">
                <button
                  v-if="resultImages.length > 1"
                  type="button"
                  class="btn btn-secondary"
                  :disabled="downloadingAll"
                  @click="downloadAllImages"
                >
                  <Icon :name="downloadingAll ? 'refresh' : 'download'" size="sm" :class="downloadingAll ? 'animate-spin' : ''" />
                  {{ t('imageWorkbench.downloadAll') }}
                </button>
                <button type="button" class="btn btn-primary" :disabled="downloadingOne" @click="downloadImageAt(0)">
                  <Icon :name="downloadingOne ? 'refresh' : 'download'" size="sm" :class="downloadingOne ? 'animate-spin' : ''" />
                  {{ resultImages.length > 1 ? t('imageWorkbench.downloadCurrent') + ' #1' : t('imageWorkbench.download') }}
                </button>
              </div>
            </div>
          </div>
        </section>

        <!-- 右：最近生成（大屏唯一滚动区） -->
        <section class="flex min-h-[240px] min-w-0 flex-col rounded-lg border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800 lg:min-h-0 lg:overflow-hidden">
          <div class="flex h-10 shrink-0 items-center justify-between border-b border-gray-200 px-3 dark:border-dark-700">
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('imageWorkbench.history') }}</h2>
            <button v-if="history.length" type="button" class="btn btn-ghost px-1.5 py-1 text-[11px]" @click="clearHistory">
              {{ t('imageWorkbench.clearHistory') }}
            </button>
          </div>
          <div class="min-h-0 flex-1 overflow-y-auto overscroll-contain p-2">
            <p v-if="!history.length" class="px-2 py-10 text-center text-xs text-gray-500 dark:text-gray-400">
              {{ t('imageWorkbench.noHistory') }}
            </p>
            <div v-else class="space-y-2">
              <article
                v-for="record in history"
                :key="record.id"
                class="overflow-hidden rounded-lg border transition"
                :class="record.id === activeTaskId
                  ? 'border-gray-400 bg-gray-50 dark:border-dark-500 dark:bg-dark-700/60'
                  : 'border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800'"
              >
                <div class="grid grid-cols-[72px_minmax(0,1fr)]">
                  <button type="button" class="flex h-full min-h-[72px] items-center justify-center bg-gray-100 dark:bg-dark-900" @click="selectRecord(record)">
                    <img v-if="record.task.image_url" :src="record.task.image_url" :alt="record.prompt" class="h-full w-full object-cover" />
                    <Icon
                      v-else
                      :name="record.task.status === 'failed' ? 'exclamationCircle' : 'clock'"
                      size="md"
                      :class="record.task.status === 'failed' ? 'text-red-400' : 'text-gray-400'"
                    />
                  </button>
                  <div class="flex min-w-0 flex-col p-2">
                    <button type="button" class="line-clamp-2 text-left text-xs font-medium leading-4 text-gray-900 dark:text-white" @click="selectRecord(record)">
                      {{ record.prompt }}
                    </button>
                    <div class="mt-auto flex items-center justify-between gap-1 pt-1.5">
                      <span class="truncate text-[10px] text-gray-400">{{ record.size }} · n={{ record.n || 1 }}</span>
                      <button
                        type="button"
                        class="rounded p-0.5 text-gray-400 hover:bg-gray-100 hover:text-red-500 dark:hover:bg-dark-700"
                        :title="t('imageWorkbench.remove')"
                        @click="removeRecord(record.id)"
                      >
                        <Icon name="trash" size="sm" />
                      </button>
                    </div>
                  </div>
                </div>
              </article>
            </div>
          </div>
        </section>
      </div>
    </div>

    <!-- 灯箱：点击放大，支持左右切换 / 下载当前 / 全部下载 -->
    <Teleport to="body">
      <div
        v-if="lightboxOpen && resultImages.length"
        class="fixed inset-0 z-[100] flex flex-col bg-black/85 backdrop-blur-sm"
        role="dialog"
        aria-modal="true"
        @click.self="closeLightbox"
      >
        <div class="flex shrink-0 items-center justify-between gap-3 px-4 py-3 text-white">
          <div class="min-w-0 text-sm">
            <span class="font-medium">{{ t('imageWorkbench.imageOf', { current: lightboxIndex + 1, total: resultImages.length }) }}</span>
            <span v-if="activeRecord?.prompt" class="ml-2 truncate text-white/60">{{ activeRecord.prompt }}</span>
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <a
              :href="resultImages[lightboxIndex]"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex items-center gap-1.5 rounded-lg bg-white/10 px-3 py-1.5 text-xs font-medium hover:bg-white/20"
            >
              <Icon name="externalLink" size="sm" />{{ t('imageWorkbench.openOriginal') }}
            </a>
            <button
              type="button"
              class="inline-flex items-center gap-1.5 rounded-lg bg-white/10 px-3 py-1.5 text-xs font-medium hover:bg-white/20"
              :disabled="downloadingOne"
              @click="downloadImageAt(lightboxIndex)"
            >
              <Icon :name="downloadingOne ? 'refresh' : 'download'" size="sm" :class="downloadingOne ? 'animate-spin' : ''" />
              {{ t('imageWorkbench.downloadCurrent') }}
            </button>
            <button
              v-if="resultImages.length > 1"
              type="button"
              class="inline-flex items-center gap-1.5 rounded-lg bg-white/10 px-3 py-1.5 text-xs font-medium hover:bg-white/20"
              :disabled="downloadingAll"
              @click="downloadAllImages"
            >
              <Icon :name="downloadingAll ? 'refresh' : 'download'" size="sm" :class="downloadingAll ? 'animate-spin' : ''" />
              {{ t('imageWorkbench.downloadAll') }}
            </button>
            <button
              type="button"
              class="inline-flex h-8 w-8 items-center justify-center rounded-lg bg-white/10 hover:bg-white/20"
              :aria-label="t('imageWorkbench.closePreview')"
              @click="closeLightbox"
            >
              <Icon name="x" size="sm" />
            </button>
          </div>
        </div>

        <div class="relative flex min-h-0 flex-1 items-center justify-center px-12 pb-6">
          <button
            v-if="resultImages.length > 1"
            type="button"
            class="absolute left-3 top-1/2 z-10 flex h-10 w-10 -translate-y-1/2 items-center justify-center rounded-full bg-white/15 text-white hover:bg-white/25"
            :aria-label="t('imageWorkbench.prevImage')"
            @click="stepLightbox(-1)"
          >
            <Icon name="chevronLeft" size="md" />
          </button>

          <img
            :src="resultImages[lightboxIndex]"
            :alt="`${activeRecord?.prompt || t('imageWorkbench.result')} #${lightboxIndex + 1}`"
            class="max-h-full max-w-full object-contain shadow-2xl"
            @click.stop
          />

          <button
            v-if="resultImages.length > 1"
            type="button"
            class="absolute right-3 top-1/2 z-10 flex h-10 w-10 -translate-y-1/2 items-center justify-center rounded-full bg-white/15 text-white hover:bg-white/25"
            :aria-label="t('imageWorkbench.nextImage')"
            @click="stepLightbox(1)"
          >
            <Icon name="chevronRight" size="md" />
          </button>
        </div>

        <div v-if="resultImages.length > 1" class="flex shrink-0 justify-center gap-2 overflow-x-auto px-4 pb-4">
          <button
            v-for="(url, index) in resultImages"
            :key="`thumb-${index}`"
            type="button"
            class="h-14 w-14 shrink-0 overflow-hidden rounded-md border-2 transition"
            :class="index === lightboxIndex ? 'border-white' : 'border-transparent opacity-60 hover:opacity-100'"
            @click="lightboxIndex = index"
          >
            <img :src="url" alt="" class="h-full w-full object-cover" />
          </button>
        </div>
      </div>
    </Teleport>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  collectImageWorkbenchURLs,
  getImageWorkbenchConfig,
  getImageWorkbenchTask,
  submitImageWorkbenchTask,
  type ImageWorkbenchConfig,
  type ImageWorkbenchTask,
} from '@/api/imageWorkbench'

interface HistoryRecord {
  id: string
  prompt: string
  size: string
  quality: string
  n: number
  task: ImageWorkbenchTask
}

interface AspectOption {
  value: string
  label: string
  width: string
  height: string
  shape?: string
}

interface ReferenceImageItem {
  id: string
  file: File
  previewUrl: string
}

const STORAGE_KEY = 'sub2api_image_workbench_history_v1'
const POLL_MS = 3000
const MIN_DIM = 256
const MAX_DIM = 4096
const DEFAULT_MAX_IMAGES = 4
const { t } = useI18n()

const config = ref<ImageWorkbenchConfig | null>(null)
const configError = ref('')
const submitError = ref('')
const submitting = ref(false)
const prompt = ref('')
const size = ref('1024x1024')
const quality = ref('auto')
const count = ref(1)
const customWidth = ref('1024')
const customHeight = ref('1024')
const settingsOpen = ref(true)
const history = ref<HistoryRecord[]>([])
const activeTaskId = ref('')
const lightboxOpen = ref(false)
const lightboxIndex = ref(0)
const downloadingOne = ref(false)
const downloadingAll = ref(false)
const referenceImages = ref<ReferenceImageItem[]>([])
const isDraggingReference = ref(false)
const fileInputRef = ref<HTMLInputElement | null>(null)
const pollTimers = new Map<string, number>()

const aspectOptions: AspectOption[] = [
  { value: '1024x1024', label: '1:1', width: '1024', height: '1024', shape: 'h-4 w-4' },
  { value: '1024x1536', label: '2:3', width: '1024', height: '1536', shape: 'h-5 w-3.5' },
  { value: '1536x1024', label: '3:2', width: '1536', height: '1024', shape: 'h-3.5 w-5' },
  { value: '1024x1365', label: '3:4', width: '1024', height: '1365', shape: 'h-5 w-3.5' },
  { value: '1365x1024', label: '4:3', width: '1365', height: '1024', shape: 'h-3.5 w-5' },
  { value: '1088x1920', label: '9:16', width: '1088', height: '1920', shape: 'h-5 w-3' },
  { value: '1920x1088', label: '16:9', width: '1920', height: '1088', shape: 'h-3 w-5' },
  { value: 'auto', label: 'auto', width: '1024', height: '1024' },
]
const qualityOptions = ['auto', 'low', 'medium', 'high'] as const
const countOptions = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]

const activeRecord = computed(() => history.value.find((item) => item.id === activeTaskId.value) || null)
const activeTask = computed(() => activeRecord.value?.task || null)
const resultImages = computed(() => collectImageWorkbenchURLs(activeTask.value))
const modelLabel = computed(() => config.value?.models?.[0] || 'gpt-image-2')
const maxN = computed(() => Math.min(10, Math.max(1, config.value?.max_n || 10)))
const maxImages = computed(() => Math.min(10, Math.max(1, config.value?.max_images || DEFAULT_MAX_IMAGES)))
const isEditMode = computed(() => referenceImages.value.length > 0)
const settingsSummary = computed(() => {
  const qualityLabel = t(`imageWorkbench.qualityLabels.${quality.value}` as 'imageWorkbench.qualityLabels.auto')
  const sizeLabel = size.value === 'auto' ? 'auto' : size.value
  const base = t('imageWorkbench.settingsSummary', { quality: qualityLabel, size: sizeLabel, count: count.value })
  return isEditMode.value ? `${base} · edit` : base
})
const resultGridClass = computed(() => {
  const n = resultImages.value.length
  if (n <= 1) return 'grid-cols-1'
  if (n === 2) return 'grid-cols-2'
  if (n <= 4) return 'grid-cols-2'
  return 'grid-cols-2 sm:grid-cols-3'
})

watch(resultImages, () => {
  if (lightboxIndex.value >= resultImages.value.length) {
    lightboxIndex.value = Math.max(0, resultImages.value.length - 1)
  }
})

function isImageFile(file: File): boolean {
  if (file.type.startsWith('image/')) {
    return ['image/png', 'image/jpeg', 'image/jpg', 'image/webp', 'image/gif'].includes(file.type)
      || file.type === 'image/jpg'
  }
  return /\.(png|jpe?g|webp|gif)$/i.test(file.name)
}

function pickReferenceImages() {
  fileInputRef.value?.click()
}

function onReferenceFileInput(event: Event) {
  const input = event.target as HTMLInputElement
  void addReferenceFiles(Array.from(input.files || []))
  input.value = ''
}

async function addReferenceFiles(files: File[]) {
  const imageFiles = files.filter(isImageFile)
  if (!imageFiles.length) {
    if (files.length) submitError.value = t('imageWorkbench.invalidReferenceType')
    return
  }
  const room = maxImages.value - referenceImages.value.length
  if (room <= 0) {
    submitError.value = t('imageWorkbench.tooManyReferences', { n: maxImages.value })
    return
  }
  if (imageFiles.length > room) {
    submitError.value = t('imageWorkbench.tooManyReferences', { n: maxImages.value })
  } else {
    submitError.value = ''
  }
  const accepted = imageFiles.slice(0, room)
  const next: ReferenceImageItem[] = accepted.map((file) => ({
    id: `${Date.now()}-${Math.random().toString(16).slice(2)}`,
    file,
    previewUrl: URL.createObjectURL(file),
  }))
  referenceImages.value = [...referenceImages.value, ...next]
}

function removeReferenceImage(index: number) {
  const item = referenceImages.value[index]
  if (item) URL.revokeObjectURL(item.previewUrl)
  referenceImages.value = referenceImages.value.filter((_, i) => i !== index)
}

function clearReferenceImages() {
  referenceImages.value.forEach((item) => URL.revokeObjectURL(item.previewUrl))
  referenceImages.value = []
}

function onReferenceDragEnter(event: DragEvent) {
  if (!hasDraggedImages(event.dataTransfer)) return
  isDraggingReference.value = true
}

function onReferenceDragOver(event: DragEvent) {
  if (!hasDraggedImages(event.dataTransfer)) return
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
  isDraggingReference.value = true
}

function onReferenceDragLeave(event: DragEvent) {
  const next = event.relatedTarget
  if (next instanceof Node && (event.currentTarget as Node).contains(next)) return
  isDraggingReference.value = false
}

function onReferenceDrop(event: DragEvent) {
  isDraggingReference.value = false
  const files = Array.from(event.dataTransfer?.files || []).filter(isImageFile)
  void addReferenceFiles(files)
}

function hasDraggedImages(dataTransfer: DataTransfer | null): boolean {
  if (!dataTransfer) return false
  const items = Array.from(dataTransfer.items || [])
  if (items.length) {
    return items.some((item) => item.kind === 'file' && (item.type.startsWith('image/') || !item.type))
  }
  return Array.from(dataTransfer.files || []).some(isImageFile)
}

function onPromptPaste(event: ClipboardEvent) {
  const files = Array.from(event.clipboardData?.files || []).filter(isImageFile)
  if (!files.length) return
  event.preventDefault()
  void addReferenceFiles(files)
}

function selectAspect(option: AspectOption) {
  size.value = option.value
  if (option.value !== 'auto') {
    customWidth.value = option.width
    customHeight.value = option.height
  }
}

function applyCustomSize() {
  const w = Number(customWidth.value)
  const h = Number(customHeight.value)
  if (!Number.isFinite(w) || !Number.isFinite(h) || w < MIN_DIM || h < MIN_DIM || w > MAX_DIM || h > MAX_DIM) {
    return
  }
  size.value = `${Math.round(w)}x${Math.round(h)}`
  customWidth.value = String(Math.round(w))
  customHeight.value = String(Math.round(h))
}

function clampCount(value: number): number {
  if (!Number.isFinite(value)) return 1
  return Math.min(maxN.value, Math.max(1, Math.floor(value)))
}

function persist() {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(history.value.slice(0, 24)))
}

function restore() {
  try {
    const parsed = JSON.parse(localStorage.getItem(STORAGE_KEY) || '[]')
    if (Array.isArray(parsed)) {
      history.value = parsed.slice(0, 24).map((item: HistoryRecord) => ({
        ...item,
        n: item.n || 1,
      }))
    }
  } catch {
    history.value = []
  }
  activeTaskId.value = history.value[0]?.id || ''
  const latest = history.value[0]
  if (latest) {
    prompt.value = latest.prompt
    size.value = latest.size
    quality.value = latest.quality
    count.value = latest.n || 1
    syncCustomFromSize(latest.size)
  }
}

function syncCustomFromSize(value: string) {
  if (value === 'auto') return
  const match = value.match(/^(\d+)x(\d+)$/)
  if (match) {
    customWidth.value = match[1]
    customHeight.value = match[2]
  }
}

function upsert(record: HistoryRecord) {
  history.value = [record, ...history.value.filter((item) => item.id !== record.id)].slice(0, 24)
  persist()
}

async function generate() {
  if (!prompt.value.trim()) {
    submitError.value = t('imageWorkbench.promptRequired')
    return
  }
  if (size.value !== 'auto') {
    const match = size.value.match(/^(\d+)x(\d+)$/)
    if (!match) {
      submitError.value = t('imageWorkbench.invalidSize')
      return
    }
    const w = Number(match[1])
    const h = Number(match[2])
    if (w < MIN_DIM || h < MIN_DIM || w > MAX_DIM || h > MAX_DIM) {
      submitError.value = t('imageWorkbench.invalidSize')
      return
    }
  }
  const n = clampCount(count.value)
  count.value = n
  submitError.value = ''
  submitting.value = true
  try {
    const task = await submitImageWorkbenchTask({
      prompt: prompt.value.trim(),
      size: size.value,
      quality: quality.value,
      n,
      images: referenceImages.value.map((item) => item.file),
    })
    const record: HistoryRecord = {
      id: task.id,
      prompt: prompt.value.trim(),
      size: size.value,
      quality: quality.value,
      n,
      task,
    }
    upsert(record)
    activeTaskId.value = task.id
    // Keep prompt for iteration, but clear references like chatgpt2api after send.
    clearReferenceImages()
    closeLightbox()
    schedulePoll(task.id, 0)
  } catch (error: any) {
    submitError.value = error?.message || t('imageWorkbench.taskFailed')
  } finally {
    submitting.value = false
  }
}

function schedulePoll(taskId: string, delay = POLL_MS) {
  if (pollTimers.has(taskId)) window.clearTimeout(pollTimers.get(taskId))
  const timer = window.setTimeout(() => poll(taskId), delay)
  pollTimers.set(taskId, timer)
}

async function poll(taskId: string) {
  pollTimers.delete(taskId)
  const record = history.value.find((item) => item.id === taskId)
  if (!record || record.task.status !== 'processing') return
  try {
    const task = await getImageWorkbenchTask(taskId)
    upsert({ ...record, task })
    if (task.status === 'processing') schedulePoll(taskId)
  } catch (error: any) {
    if (error?.status === 404) {
      removeRecord(taskId)
      return
    }
    schedulePoll(taskId, 5000)
  }
}

function selectRecord(record: HistoryRecord) {
  activeTaskId.value = record.id
  prompt.value = record.prompt
  size.value = record.size
  quality.value = record.quality
  count.value = record.n || 1
  syncCustomFromSize(record.size)
  closeLightbox()
}

function removeRecord(id: string) {
  const timer = pollTimers.get(id)
  if (timer) window.clearTimeout(timer)
  pollTimers.delete(id)
  history.value = history.value.filter((item) => item.id !== id)
  if (activeTaskId.value === id) activeTaskId.value = history.value[0]?.id || ''
  if (!history.value.some((item) => item.id === activeTaskId.value)) closeLightbox()
  persist()
}

function clearHistory() {
  pollTimers.forEach((timer) => window.clearTimeout(timer))
  pollTimers.clear()
  history.value = []
  activeTaskId.value = ''
  closeLightbox()
  persist()
}

function taskError(task: ImageWorkbenchTask): string {
  if (typeof task.error === 'string') return task.error
  return task.error?.message || t('imageWorkbench.taskFailed')
}

function openLightbox(index: number) {
  lightboxIndex.value = index
  lightboxOpen.value = true
}

function closeLightbox() {
  lightboxOpen.value = false
}

function stepLightbox(delta: number) {
  const total = resultImages.value.length
  if (total <= 0) return
  lightboxIndex.value = (lightboxIndex.value + delta + total) % total
}

function filenameFor(index: number): string {
  const base = (activeRecord.value?.id || 'image').replace(/[^\w-]+/g, '_')
  return `${base}-${index + 1}.png`
}

async function fetchAsBlob(url: string): Promise<Blob> {
  const response = await fetch(url)
  if (!response.ok) throw new Error(`HTTP ${response.status}`)
  return response.blob()
}

function triggerBlobDownload(blob: Blob, filename: string) {
  const objectUrl = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = objectUrl
  anchor.download = filename
  anchor.rel = 'noopener'
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  window.setTimeout(() => URL.revokeObjectURL(objectUrl), 1000)
}

async function downloadImageAt(index: number) {
  const url = resultImages.value[index]
  if (!url) return
  downloadingOne.value = true
  try {
    try {
      const blob = await fetchAsBlob(url)
      triggerBlobDownload(blob, filenameFor(index))
    } catch {
      // Cross-origin without CORS: fall back to opening the URL.
      window.open(url, '_blank', 'noopener,noreferrer')
    }
  } finally {
    downloadingOne.value = false
  }
}

async function downloadAllImages() {
  if (!resultImages.value.length) return
  downloadingAll.value = true
  try {
    for (let i = 0; i < resultImages.value.length; i += 1) {
      const url = resultImages.value[i]
      try {
        const blob = await fetchAsBlob(url)
        triggerBlobDownload(blob, filenameFor(i))
      } catch {
        window.open(url, '_blank', 'noopener,noreferrer')
      }
      // Small gap so browsers don't coalesce multi-download clicks.
      await new Promise((resolve) => window.setTimeout(resolve, 250))
    }
  } finally {
    downloadingAll.value = false
  }
}

function onKeydown(event: KeyboardEvent) {
  if (!lightboxOpen.value) return
  if (event.key === 'Escape') {
    event.preventDefault()
    closeLightbox()
  } else if (event.key === 'ArrowLeft') {
    event.preventDefault()
    stepLightbox(-1)
  } else if (event.key === 'ArrowRight') {
    event.preventDefault()
    stepLightbox(1)
  }
}

onMounted(async () => {
  restore()
  history.value.filter((item) => item.task.status === 'processing').forEach((item) => schedulePoll(item.id, 0))
  window.addEventListener('keydown', onKeydown)
  try {
    config.value = await getImageWorkbenchConfig()
  } catch (error: any) {
    configError.value = error?.status === 503 || error?.code === 'IMAGE_WORKBENCH_UNAVAILABLE'
      ? t('imageWorkbench.unavailable')
      : t('imageWorkbench.configFailed')
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  clearReferenceImages()
  pollTimers.forEach((timer) => window.clearTimeout(timer))
  pollTimers.clear()
})
</script>
