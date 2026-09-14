<template>
  <AppLayout>
    <!-- 大屏：AppLayout fillHeight 锁死视口，整页不滚；仅会话列表与结果区各自滚动 -->
    <div class="flex min-h-0 min-w-0 flex-1 gap-3 lg:overflow-hidden">
      <!-- 会话历史 -->
      <aside
        class="fixed inset-y-0 left-0 z-40 flex w-64 shrink-0 flex-col border-r border-gray-200 bg-white p-2 shadow-xl transition-transform duration-200 dark:border-dark-700 dark:bg-dark-800 lg:static lg:z-auto lg:h-full lg:min-h-0 lg:translate-x-0 lg:rounded-xl lg:border lg:shadow-sm"
        :class="mobileListOpen ? 'translate-x-0' : '-translate-x-full'"
      >
        <div class="flex items-center justify-between gap-1 border-b border-gray-200 px-1.5 pb-2 dark:border-dark-700">
          <h2 class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ t('imageWorkbench.conversations') }}</h2>
          <div class="flex shrink-0 items-center gap-0.5">
            <button
              type="button"
              class="btn btn-ghost px-1.5 py-1 text-[11px]"
              :disabled="!conversations.length"
              @click="confirmClearConversations = true"
            >
              {{ t('imageWorkbench.clearConversations') }}
            </button>
            <button type="button" class="icon-btn" :title="t('imageWorkbench.newConversation')" @click="newConversation">
              <Icon name="plus" size="sm" />
            </button>
          </div>
        </div>

        <div class="mt-2 min-h-0 flex-1 space-y-0.5 overflow-y-auto overscroll-contain px-0.5">
          <p v-if="!conversations.length" class="px-2 py-8 text-center text-xs text-gray-400">
            {{ t('imageWorkbench.emptyConversations') }}
          </p>
          <div
            v-for="conversation in conversations"
            :key="conversation.id"
            class="group flex items-center gap-1 rounded-lg px-1.5 py-1.5 transition"
            :class="conversation.id === activeConversationId
              ? 'bg-gray-100 dark:bg-dark-700'
              : 'hover:bg-gray-50 dark:hover:bg-dark-700/60'"
          >
            <button type="button" class="flex min-w-0 flex-1 flex-col items-start text-left" @click="selectConversation(conversation.id)">
              <span class="w-full truncate text-sm font-medium text-gray-800 dark:text-gray-100">{{ conversation.title }}</span>
              <span class="text-[11px] text-gray-400">
                {{ formatTime(conversation.updatedAt) }} · {{ t('imageWorkbench.turnCount', { n: conversation.turns.length }) }}
              </span>
            </button>
            <button
              type="button"
              class="shrink-0 rounded p-1 text-gray-400 opacity-0 transition hover:text-red-500 focus:opacity-100 group-hover:opacity-100"
              :title="t('imageWorkbench.deleteConversation')"
              @click.stop="removeConversation(conversation.id)"
            >
              <Icon name="trash" size="xs" />
            </button>
          </div>
        </div>
      </aside>
      <div v-if="mobileListOpen" class="fixed inset-0 z-30 bg-black/30 lg:hidden" @click="mobileListOpen = false" />

      <!-- 主工作区 -->
      <section class="flex min-h-0 min-w-0 flex-1 flex-col rounded-xl border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800 lg:overflow-hidden">
        <header class="flex shrink-0 items-center justify-between gap-2 border-b border-gray-200 px-3 py-2.5 dark:border-dark-700">
          <div class="flex min-w-0 items-center gap-2">
            <button type="button" class="icon-btn lg:hidden" :title="t('imageWorkbench.conversations')" @click="mobileListOpen = true">
              <Icon name="menu" size="sm" />
            </button>
            <div class="min-w-0">
              <h1 class="truncate text-sm font-semibold text-gray-900 dark:text-white">
                {{ activeConversation?.title || t('imageWorkbench.title') }}
              </h1>
              <p class="truncate text-[11px] text-gray-500 dark:text-gray-400">{{ t('imageWorkbench.description') }}</p>
            </div>
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <span
              v-if="processingCount > 0"
              class="hidden items-center gap-1 rounded-full bg-amber-50 px-2 py-1 text-[11px] font-medium text-amber-700 dark:bg-amber-950/40 dark:text-amber-300 sm:inline-flex"
            >
              <Icon name="refresh" size="xs" class="animate-spin" />
              {{ t('imageWorkbench.processingBadge', { n: processingCount }) }}
            </span>
            <button
              v-if="activeTurns.length"
              type="button"
              class="icon-btn"
              :title="t('imageWorkbench.clearConversation')"
              @click="confirmClearConversation = true"
            >
              <Icon name="trash" size="sm" />
            </button>
          </div>
        </header>

        <!-- 结果区 -->
        <div ref="scrollEl" class="min-h-0 flex-1 overflow-y-auto overscroll-contain px-3 py-4 md:px-5">
          <div v-if="!activeTurns.length" class="mx-auto flex h-full max-w-md flex-col items-center justify-center text-center">
            <div class="mb-3 flex h-12 w-12 items-center justify-center rounded-2xl bg-primary-50 text-primary-600 dark:bg-primary-900/30 dark:text-primary-300">
              <Icon name="sparkles" size="lg" />
            </div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('imageWorkbench.emptyTitle') }}</h2>
            <p class="mt-1.5 text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t('imageWorkbench.emptyHint') }}</p>
          </div>

          <div v-else class="mx-auto max-w-3xl space-y-4">
            <article
              v-for="turn in activeTurns"
              :key="turn.id"
              class="rounded-2xl border border-gray-200 bg-gray-50/60 p-3 dark:border-dark-700 dark:bg-dark-900/40"
            >
              <div class="flex flex-wrap items-start justify-between gap-2">
                <div class="flex min-w-0 items-start gap-2">
                  <span class="mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-lg bg-primary-600 text-white">
                    <Icon name="user" size="xs" />
                  </span>
                  <div class="min-w-0">
                    <p class="whitespace-pre-wrap break-words text-sm text-gray-900 dark:text-gray-100">{{ turn.prompt }}</p>
                    <div class="mt-1 flex flex-wrap items-center gap-1.5 text-[11px] text-gray-500 dark:text-gray-400">
                      <span class="rounded-full bg-white px-2 py-0.5 dark:bg-dark-800">{{ turn.model }}</span>
                      <span class="rounded-full bg-white px-2 py-0.5 dark:bg-dark-800">{{ t(`imageWorkbench.qualityLabels.${turn.quality}`) }}</span>
                      <span class="rounded-full bg-white px-2 py-0.5 dark:bg-dark-800">{{ turn.size }}</span>
                      <span class="rounded-full bg-white px-2 py-0.5 dark:bg-dark-800">{{ turn.n }} {{ t('imageWorkbench.countUnit') }}</span>
                      <span v-if="turn.mode === 'edit'" class="rounded-full bg-white px-2 py-0.5 dark:bg-dark-800">{{ t('imageWorkbench.modeEdit') }}</span>
                    </div>
                  </div>
                </div>
                <div class="flex shrink-0 items-center gap-0.5">
                  <button
                    v-if="turn.status !== 'processing'"
                    type="button"
                    class="btn btn-ghost px-2 py-1 text-[11px]"
                    @click="regenerate(turn)"
                  >
                    <Icon name="refresh" size="xs" />{{ t('imageWorkbench.regenerate') }}
                  </button>
                  <button
                    v-if="turn.urls.length"
                    type="button"
                    class="btn btn-ghost px-2 py-1 text-[11px]"
                    @click="continueEdit(turn)"
                  >
                    <Icon name="edit" size="xs" />{{ t('imageWorkbench.continueEdit') }}
                  </button>
                  <button type="button" class="icon-btn" :title="t('imageWorkbench.deleteTurn')" @click="removeTurn(turn.id)">
                    <Icon name="trash" size="xs" />
                  </button>
                </div>
              </div>

              <div v-if="turn.referenceImages.length" class="mt-2 flex flex-wrap gap-1.5">
                <img
                  v-for="(reference, index) in turn.referenceImages"
                  :key="`${turn.id}-reference-${index}`"
                  :src="reference.dataUrl"
                  :alt="reference.name"
                  class="h-12 w-12 rounded-lg border border-gray-200 object-cover dark:border-dark-600"
                />
              </div>

              <div class="mt-3">
                <div
                  v-if="turn.status === 'processing'"
                  class="flex flex-col items-center justify-center gap-2 rounded-xl bg-white/70 py-10 dark:bg-dark-800/60"
                >
                  <div class="h-8 w-8 animate-spin rounded-full border-2 border-gray-300 border-t-primary-600 dark:border-dark-600 dark:border-t-primary-400" />
                  <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('imageWorkbench.processingHint') }}</p>
                </div>
                <div
                  v-else-if="turn.status === 'failed'"
                  class="flex flex-col items-center justify-center gap-2 rounded-xl bg-red-50/70 px-4 py-8 text-center dark:bg-red-950/20"
                >
                  <Icon name="exclamationCircle" size="lg" class="text-red-500" />
                  <p class="break-words text-xs text-red-600 dark:text-red-400">{{ turn.error || t('imageWorkbench.taskFailed') }}</p>
                </div>
                <div v-else class="grid gap-2" :class="resultGridClass(turn.urls.length)">
                  <button
                    v-for="(url, index) in turn.urls"
                    :key="`${turn.id}-result-${index}`"
                    type="button"
                    class="group relative overflow-hidden rounded-xl bg-white transition hover:opacity-95 dark:bg-dark-900"
                    :title="t('imageWorkbench.zoomHint')"
                    @click="openLightbox(turn, index)"
                  >
                    <img
                      :src="url"
                      :alt="`${turn.prompt} #${index + 1}`"
                      class="mx-auto max-h-80 w-full object-contain"
                      loading="lazy"
                    />
                    <span class="absolute right-2 top-2 rounded-md bg-black/60 px-1.5 py-0.5 text-[11px] text-white opacity-0 transition group-hover:opacity-100">
                      {{ index + 1 }}
                    </span>
                  </button>
                </div>

                <div
                  v-if="turn.status === 'completed' && turn.urls.length"
                  class="mt-2 flex flex-wrap items-center justify-end gap-2"
                >
                  <button
                    v-if="turn.urls.length > 1"
                    type="button"
                    class="btn btn-secondary px-2.5 py-1 text-[11px]"
                    :disabled="downloadingAll"
                    @click="downloadAll(turn)"
                  >
                    <Icon name="download" size="xs" />{{ t('imageWorkbench.downloadAll') }}
                  </button>
                  <button type="button" class="btn btn-primary px-2.5 py-1 text-[11px]" @click="downloadOne(turn.urls[0], 0)">
                    <Icon name="download" size="xs" />{{ t('imageWorkbench.download') }}
                  </button>
                </div>
              </div>
            </article>
          </div>
        </div>

        <!-- 输入区 -->
        <form
          class="shrink-0 border-t border-gray-200 p-3 dark:border-dark-700"
          :class="isDraggingReference ? 'bg-primary-50/40 dark:bg-primary-950/10' : ''"
          @submit.prevent="send"
          @dragenter.prevent="onReferenceDragEnter"
          @dragover.prevent="onReferenceDragOver"
          @dragleave="onReferenceDragLeave"
          @drop.prevent="onReferenceDrop"
        >
          <div class="mx-auto w-full max-w-3xl">
            <div v-if="configError" class="mb-2 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300">
              {{ configError }}
            </div>

            <div v-if="referenceImages.length" class="mb-2 flex flex-wrap gap-2">
              <div
                v-for="(reference, index) in referenceImages"
                :key="reference.id"
                class="relative h-14 w-14 overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600"
              >
                <img :src="reference.dataUrl" :alt="reference.name" class="h-full w-full object-cover" />
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

            <input
              ref="fileInputRef"
              type="file"
              accept="image/png,image/jpeg,image/jpg,image/webp,image/gif"
              multiple
              class="hidden"
              @change="onReferenceFileInput"
            />

            <div
              class="rounded-2xl border border-gray-300 bg-white p-2 shadow-sm transition focus-within:border-primary-500 dark:border-dark-600 dark:bg-dark-900"
            >
              <textarea
                ref="textareaRef"
                v-model="prompt"
                rows="2"
                maxlength="32000"
                class="w-full resize-none border-0 bg-transparent px-2 py-1.5 text-sm leading-6 text-gray-900 outline-none placeholder:text-gray-400 dark:text-white"
                :placeholder="isEditMode ? t('imageWorkbench.promptPlaceholderEdit') : t('imageWorkbench.promptPlaceholder')"
                :disabled="submitting || !ready"
                @paste="onPromptPaste"
                @keydown.enter.exact.prevent="send"
              />
              <div class="mt-1 flex items-center justify-between gap-2">
                <div class="flex min-w-0 items-center gap-1.5">
                  <button
                    type="button"
                    class="icon-btn"
                    :title="referenceImages.length ? t('imageWorkbench.addReference') : t('imageWorkbench.uploadReference')"
                    :disabled="submitting || !config || referenceImages.length >= maxImages"
                    @click="pickReferenceImages"
                  >
                    <Icon name="upload" size="sm" />
                  </button>
                  <button
                    type="button"
                    class="flex h-8 min-w-0 items-center gap-1 rounded-full bg-gray-100 px-3 text-xs font-medium text-gray-700 transition hover:bg-gray-200 dark:bg-dark-800 dark:text-gray-200 dark:hover:bg-dark-700"
                    :title="t('imageWorkbench.settings')"
                    @click="settingsOpen = !settingsOpen"
                  >
                    <Icon :name="settingsOpen ? 'chevronDown' : 'cog'" size="xs" />
                    <span class="truncate">{{ settingsSummary }}</span>
                  </button>
                </div>
                <button
                  type="submit"
                  class="btn btn-primary h-9 shrink-0 px-3"
                  :disabled="submitting || !ready || !prompt.trim()"
                >
                  <Icon :name="submitting ? 'refresh' : 'arrowUp'" size="sm" :class="submitting ? 'animate-spin' : ''" />
                </button>
              </div>
            </div>

            <!-- 生成选项 -->
            <div
              v-show="settingsOpen"
              class="mt-2 space-y-3 rounded-2xl border border-gray-200 bg-gray-50/70 p-3 dark:border-dark-700 dark:bg-dark-900/50"
            >
              <div class="grid gap-3 sm:grid-cols-2">
                <label class="flex items-center justify-between gap-2">
                  <span class="text-xs font-medium text-gray-700 dark:text-gray-300">{{ t('imageWorkbench.model') }}</span>
                  <select
                    v-model="model"
                    class="input h-8 w-[60%] py-0 text-xs"
                    :disabled="submitting || !ready"
                  >
                    <option v-for="option in availableModels" :key="option" :value="option">{{ option }}</option>
                  </select>
                </label>

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
              </div>

              <div>
                <span class="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300">{{ t('imageWorkbench.aspectRatio') }}</span>
                <div class="grid grid-cols-4 gap-1 sm:grid-cols-7">
                  <button
                    v-for="option in aspectOptions"
                    :key="`${option.value}-${option.label}`"
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

              <div class="grid gap-3 sm:grid-cols-2">
                <div>
                  <div class="mb-1 flex items-center justify-between gap-2">
                    <span class="text-xs font-medium text-gray-700 dark:text-gray-300">{{ t('imageWorkbench.customSize') }}</span>
                    <div class="flex items-center gap-1.5" :title="t('imageWorkbench.align16Hint')">
                      <span class="text-[10px] text-gray-500 dark:text-gray-400">{{ t('imageWorkbench.align16') }}</span>
                      <Toggle v-model="alignTo16" />
                    </div>
                  </div>
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

              <div class="flex items-center justify-between gap-3 border-t border-gray-200 pt-2.5 dark:border-dark-700">
                <div class="min-w-0">
                  <span class="block text-xs font-medium text-gray-700 dark:text-gray-300">{{ t('imageWorkbench.transparentBackground') }}</span>
                  <p class="mt-0.5 text-[10px] leading-4 text-gray-400">{{ t('imageWorkbench.transparentBackgroundHint') }}</p>
                </div>
                <Toggle v-model="transparentBackground" />
              </div>

              <p v-if="referenceImages.length" class="text-[11px] leading-4 text-gray-400">
                {{ t('imageWorkbench.referenceHint', { n: maxImages }) }}
              </p>
            </div>

            <p v-if="submitError" class="mt-2 text-xs text-red-600 dark:text-red-400">{{ submitError }}</p>
          </div>
        </form>
      </section>
    </div>

    <!-- 灯箱 -->
    <Teleport to="body">
      <div
        v-if="lightboxOpen && lightboxUrls.length"
        class="fixed inset-0 z-[100] flex flex-col bg-black/90 backdrop-blur-sm"
        role="dialog"
        aria-modal="true"
        @click.self="closeLightbox"
      >
        <div class="flex shrink-0 items-center justify-between gap-3 px-4 py-3 text-white">
          <div class="min-w-0 text-sm">
            <span class="font-medium">{{ t('imageWorkbench.imageOf', { current: lightboxIndex + 1, total: lightboxUrls.length }) }}</span>
            <span v-if="lightboxPrompt" class="ml-2 truncate text-white/60">{{ lightboxPrompt }}</span>
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <a
              :href="lightboxUrls[lightboxIndex]"
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
              @click="downloadOne(lightboxUrls[lightboxIndex], lightboxIndex)"
            >
              <Icon name="download" size="sm" />{{ t('imageWorkbench.downloadCurrent') }}
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
            v-if="lightboxUrls.length > 1"
            type="button"
            class="absolute left-3 top-1/2 z-10 flex h-10 w-10 -translate-y-1/2 items-center justify-center rounded-full bg-white/15 text-white hover:bg-white/25"
            :aria-label="t('imageWorkbench.prevImage')"
            @click="stepLightbox(-1)"
          >
            <Icon name="chevronLeft" size="md" />
          </button>

          <img
            :src="lightboxUrls[lightboxIndex]"
            alt=""
            class="max-h-full max-w-full object-contain shadow-2xl"
            @click.stop
          />

          <button
            v-if="lightboxUrls.length > 1"
            type="button"
            class="absolute right-3 top-1/2 z-10 flex h-10 w-10 -translate-y-1/2 items-center justify-center rounded-full bg-white/15 text-white hover:bg-white/25"
            :aria-label="t('imageWorkbench.nextImage')"
            @click="stepLightbox(1)"
          >
            <Icon name="chevronRight" size="md" />
          </button>
        </div>

        <div v-if="lightboxUrls.length > 1" class="flex shrink-0 justify-center gap-2 overflow-x-auto px-4 pb-4">
          <button
            v-for="(url, index) in lightboxUrls"
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

    <!-- 确认弹窗 -->
    <Teleport to="body">
      <div
        v-if="pendingConfirm"
        class="fixed inset-0 z-[110] flex items-center justify-center bg-black/40 p-4"
        role="dialog"
        aria-modal="true"
        @click.self="pendingConfirm = null"
      >
        <div class="w-full max-w-sm rounded-2xl bg-white p-5 shadow-xl dark:bg-dark-800">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ pendingConfirm.title }}</h3>
          <p class="mt-2 text-sm leading-6 text-gray-500 dark:text-gray-400">{{ pendingConfirm.description }}</p>
          <div class="mt-4 flex justify-end gap-2">
            <button type="button" class="btn btn-secondary" @click="pendingConfirm = null">{{ t('imageWorkbench.cancel') }}</button>
            <button type="button" class="btn btn-danger" @click="confirmPendingAction">{{ t('imageWorkbench.confirmDelete') }}</button>
          </div>
        </div>
      </div>
    </Teleport>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import {
  collectImageWorkbenchURLs,
  getImageWorkbenchConfig,
  getImageWorkbenchModels,
  getImageWorkbenchTask,
  submitImageWorkbenchTask,
  type ImageWorkbenchConfig,
  type ImageWorkbenchTask,
  type ImageWorkbenchTaskStatus,
} from '@/api/imageWorkbench'

interface StoredReference {
  name: string
  dataUrl: string
}

interface Turn {
  id: string
  prompt: string
  model: string
  size: string
  quality: string
  background?: 'transparent'
  n: number
  mode: 'generate' | 'edit'
  referenceImages: StoredReference[]
  taskId?: string
  status: ImageWorkbenchTaskStatus
  urls: string[]
  error?: string
  createdAt: number
}

interface Conversation {
  id: string
  title: string
  turns: Turn[]
  updatedAt: number
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
  name: string
  file: File
  dataUrl: string
}

interface PendingConfirm {
  title: string
  description: string
  run: () => void
}

const STORAGE_KEY = 'sub2api_image_studio_conversations_v1'
const SETTINGS_KEY = 'sub2api_image_studio_settings_v1'
const POLL_MS = 3000
const MIN_DIM = 256
const MAX_DIM = 4096
const DEFAULT_MAX_IMAGES = 4
const MAX_CONVERSATIONS = 30

const { t } = useI18n()

const config = ref<ImageWorkbenchConfig | null>(null)
const configError = ref('')
const submitError = ref('')
const ready = computed(() => Boolean(config.value?.ready))
const DEFAULT_IMAGE_MODELS = ['gpt-image-2', 'codex-gpt-image-2']
const availableModels = ref<string[]>([...DEFAULT_IMAGE_MODELS])

/**
 * Merge upstream catalogs into the picker, keeping the upstream ordering first
 * and never dropping an already-selected model. Older chatgpt2api builds and
 * the built-in fallback both feed through here.
 */
function mergeAvailableModels(models: string[]) {
  const merged: string[] = []
  for (const item of [...models, ...availableModels.value, ...DEFAULT_IMAGE_MODELS, model.value]) {
    const value = (item || '').trim()
    if (value && !merged.includes(value)) merged.push(value)
  }
  availableModels.value = merged
}

const prompt = ref('')
const model = ref('gpt-image-2')
const size = ref('1024x1024')
const quality = ref('auto')
const count = ref(1)
const customWidth = ref('1024')
const customHeight = ref('1024')
const alignTo16 = ref(true)
const transparentBackground = ref(false)
const settingsOpen = ref(false)
const submitting = ref(false)

const conversations = ref<Conversation[]>([])
const activeConversationId = ref('')
const mobileListOpen = ref(false)
const referenceImages = ref<ReferenceImageItem[]>([])
const isDraggingReference = ref(false)
const fileInputRef = ref<HTMLInputElement | null>(null)
const textareaRef = ref<HTMLTextAreaElement | null>(null)
const scrollEl = ref<HTMLElement | null>(null)
const pendingConfirm = ref<PendingConfirm | null>(null)

const lightboxUrls = ref<string[]>([])
const lightboxPrompt = ref('')
const lightboxIndex = ref(0)
const lightboxOpen = ref(false)
const downloadingOne = ref(false)
const downloadingAll = ref(false)

const pollTimers = new Map<string, number>()

const aspectOptions: AspectOption[] = [
  { value: '1024x1024', label: '1:1', width: '1024', height: '1024', shape: 'h-4 w-4' },
  { value: '1024x1536', label: '2:3', width: '1024', height: '1536', shape: 'h-5 w-3.5' },
  { value: '1536x1024', label: '3:2', width: '1536', height: '1024', shape: 'h-3.5 w-5' },
  { value: '1024x1360', label: '3:4', width: '1024', height: '1360', shape: 'h-5 w-3.5' },
  { value: '1360x1024', label: '4:3', width: '1360', height: '1024', shape: 'h-3.5 w-5' },
  { value: '1088x1920', label: '9:16', width: '1088', height: '1920', shape: 'h-5 w-3' },
  { value: '1920x1088', label: '16:9', width: '1920', height: '1088', shape: 'h-3 w-5' },
  { value: '2048x2048', label: '1:1 2K', width: '2048', height: '2048', shape: 'h-4 w-4' },
  { value: '2560x1440', label: '16:9 2K', width: '2560', height: '1440', shape: 'h-3 w-5' },
  { value: '1440x2560', label: '9:16 2K', width: '1440', height: '2560', shape: 'h-5 w-3' },
  { value: '3840x2160', label: '16:9 4K', width: '3840', height: '2160', shape: 'h-3 w-5' },
  { value: '2160x3840', label: '9:16 4K', width: '2160', height: '3840', shape: 'h-5 w-3' },
  { value: 'auto', label: 'auto', width: '1024', height: '1024' },
]
const qualityOptions = ['auto', 'low', 'medium', 'high'] as const
const countOptions = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]

const activeConversation = computed(() =>
  conversations.value.find((item) => item.id === activeConversationId.value) || null
)
const activeTurns = computed(() => activeConversation.value?.turns || [])
const processingCount = computed(() =>
  conversations.value.reduce(
    (total, conversation) => total + conversation.turns.filter((turn) => turn.status === 'processing').length,
    0
  )
)
const maxN = computed(() => Math.min(10, Math.max(1, config.value?.max_n || 10)))
const maxImages = computed(() => Math.min(10, Math.max(1, config.value?.max_images || DEFAULT_MAX_IMAGES)))
const isEditMode = computed(() => referenceImages.value.length > 0)
const settingsSummary = computed(() => {
  const qualityLabel = t(`imageWorkbench.qualityLabels.${quality.value}` as 'imageWorkbench.qualityLabels.auto')
  const sizeLabel = size.value === 'auto' ? 'auto' : size.value
  const base = t('imageWorkbench.settingsSummary', { quality: qualityLabel, size: sizeLabel, count: count.value })
  return isEditMode.value ? `${base} · ${t('imageWorkbench.modeEdit')}` : base
})

function resultGridClass(total: number): string {
  if (total <= 1) return 'grid-cols-1'
  if (total === 2) return 'grid-cols-1 sm:grid-cols-2'
  return 'grid-cols-2 sm:grid-cols-3'
}

function createId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') return crypto.randomUUID()
  return `${Date.now()}-${Math.random().toString(16).slice(2)}`
}

function buildTitle(text: string): string {
  const trimmed = text.trim()
  return trimmed.length <= 24 ? trimmed : `${trimmed.slice(0, 24)}…`
}

function formatTime(value: number): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat(undefined, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(date)
}

/* ---------------------------------- 持久化 ---------------------------------- */

function persist() {
  try {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify(conversations.value.slice(0, MAX_CONVERSATIONS))
    )
  } catch {
    // localStorage may be unavailable; ignore.
  }
}

function persistSettings() {
  try {
    localStorage.setItem(
      SETTINGS_KEY,
      JSON.stringify({
        model: model.value,
        size: size.value,
        quality: quality.value,
        count: count.value,
        customWidth: customWidth.value,
        customHeight: customHeight.value,
        alignTo16: alignTo16.value,
        transparentBackground: transparentBackground.value,
      })
    )
  } catch {
    // ignore
  }
}

function restoreSettings() {
  try {
    const parsed = JSON.parse(localStorage.getItem(SETTINGS_KEY) || 'null') as Record<string, unknown> | null
    if (!parsed || typeof parsed !== 'object') return
    if (typeof parsed.model === 'string' && parsed.model) model.value = parsed.model
    if (typeof parsed.size === 'string' && parsed.size) size.value = parsed.size
    if (typeof parsed.quality === 'string') quality.value = parsed.quality
    if (typeof parsed.count === 'number') count.value = clampCount(parsed.count)
    if (typeof parsed.customWidth === 'string') customWidth.value = parsed.customWidth
    if (typeof parsed.customHeight === 'string') customHeight.value = parsed.customHeight
    if (typeof parsed.alignTo16 === 'boolean') alignTo16.value = parsed.alignTo16
    if (typeof parsed.transparentBackground === 'boolean') transparentBackground.value = parsed.transparentBackground
  } catch {
    // ignore malformed settings
  }
}

function restoreConversations() {
  try {
    const parsed = JSON.parse(localStorage.getItem(STORAGE_KEY) || '[]')
    if (!Array.isArray(parsed)) return
    conversations.value = parsed
      .filter((item): item is Conversation => Boolean(item) && typeof item === 'object' && Array.isArray(item.turns))
      .slice(0, MAX_CONVERSATIONS)
      .map((conversation) => ({
        ...conversation,
        turns: conversation.turns.map((turn) => ({ ...turn, urls: Array.isArray(turn.urls) ? turn.urls : [] })),
      }))
  } catch {
    conversations.value = []
  }
  activeConversationId.value = conversations.value[0]?.id || ''
}

/* ---------------------------------- 会话操作 ---------------------------------- */

function newConversation() {
  const conversation: Conversation = { id: createId(), title: t('imageWorkbench.newConversation'), turns: [], updatedAt: Date.now() }
  conversations.value.unshift(conversation)
  activeConversationId.value = conversation.id
  mobileListOpen.value = false
  submitError.value = ''
  persist()
  nextTick(() => textareaRef.value?.focus())
}

function selectConversation(id: string) {
  activeConversationId.value = id
  mobileListOpen.value = false
}

function removeConversation(id: string) {
  clearTimersOfConversation(id)
  conversations.value = conversations.value.filter((item) => item.id !== id)
  if (activeConversationId.value === id) {
    activeConversationId.value = conversations.value[0]?.id || ''
  }
  persist()
}

function clearAllConversations() {
  pollTimers.forEach((timer) => window.clearTimeout(timer))
  pollTimers.clear()
  conversations.value = []
  activeConversationId.value = ''
  persist()
}

function clearActiveConversation() {
  const conversation = activeConversation.value
  if (!conversation) return
  conversation.turns.forEach((turn) => clearTimerOfTurn(turn))
  conversation.turns = []
  conversation.updatedAt = Date.now()
  persist()
}

function removeTurn(turnId: string) {
  const conversation = activeConversation.value
  if (!conversation) return
  const turn = conversation.turns.find((item) => item.id === turnId)
  if (turn) clearTimerOfTurn(turn)
  conversation.turns = conversation.turns.filter((item) => item.id !== turnId)
  conversation.updatedAt = Date.now()
  persist()
}

function ensureActiveConversation(): Conversation {
  const existing = activeConversation.value
  if (existing) return existing
  const conversation: Conversation = { id: createId(), title: t('imageWorkbench.newConversation'), turns: [], updatedAt: Date.now() }
  conversations.value.unshift(conversation)
  activeConversationId.value = conversation.id
  return conversation
}

/* ------------------------------------ 轮询 ------------------------------------ */

function clearTimerOfTurn(turn: Turn) {
  if (!turn.taskId) return
  const timer = pollTimers.get(turn.taskId)
  if (timer) window.clearTimeout(timer)
  pollTimers.delete(turn.taskId)
}

function clearTimersOfConversation(conversationId: string) {
  const conversation = conversations.value.find((item) => item.id === conversationId)
  conversation?.turns.forEach((turn) => clearTimerOfTurn(turn))
}

function findTurnByTaskId(taskId: string): { conversation: Conversation; turn: Turn } | null {
  for (const conversation of conversations.value) {
    const turn = conversation.turns.find((item) => item.taskId === taskId)
    if (turn) return { conversation, turn }
  }
  return null
}

function applyTaskToTurn(turn: Turn, task: ImageWorkbenchTask) {
  turn.taskId = task.id
  turn.status = task.status
  if (task.status === 'completed') {
    turn.urls = collectImageWorkbenchURLs(task)
    turn.error = undefined
  } else if (task.status === 'failed') {
    turn.urls = []
    turn.error = taskError(task)
  }
}

function schedulePoll(taskId: string, delay = POLL_MS) {
  const existing = pollTimers.get(taskId)
  if (existing) window.clearTimeout(existing)
  pollTimers.set(taskId, window.setTimeout(() => void poll(taskId), delay))
}

async function poll(taskId: string) {
  pollTimers.delete(taskId)
  const found = findTurnByTaskId(taskId)
  if (!found || found.turn.status !== 'processing') return
  try {
    const task = await getImageWorkbenchTask(taskId)
    applyTaskToTurn(found.turn, task)
    found.conversation.updatedAt = Date.now()
    persist()
    if (task.status === 'processing') schedulePoll(taskId)
  } catch (error: any) {
    if (error?.status === 404) {
      found.turn.status = 'failed'
      found.turn.error = t('imageWorkbench.taskFailed')
      persist()
      return
    }
    schedulePoll(taskId, 5000)
  }
}

function taskError(task: ImageWorkbenchTask): string {
  if (typeof task.error === 'string') return task.error
  return task.error?.message || t('imageWorkbench.taskFailed')
}

/* ------------------------------------ 发送 ------------------------------------ */

function alignImageDimension(value: number): number {
  const integer = Math.floor(value)
  return alignTo16.value ? Math.ceil(integer / 16) * 16 : integer
}

function clampCount(value: number): number {
  if (!Number.isFinite(value)) return 1
  return Math.min(maxN.value, Math.max(1, Math.floor(value)))
}

function applyCustomSize() {
  const w = Number(customWidth.value)
  const h = Number(customHeight.value)
  if (!Number.isFinite(w) || !Number.isFinite(h) || w < MIN_DIM || h < MIN_DIM || w > MAX_DIM || h > MAX_DIM) {
    return
  }
  const nextWidth = alignImageDimension(w)
  const nextHeight = alignImageDimension(h)
  size.value = `${nextWidth}x${nextHeight}`
  customWidth.value = String(nextWidth)
  customHeight.value = String(nextHeight)
}

function selectAspect(option: AspectOption) {
  size.value = option.value
  if (option.value !== 'auto') {
    customWidth.value = option.width
    customHeight.value = option.height
  }
}

function validateSize(): boolean {
  if (size.value === 'auto') return true
  const match = size.value.match(/^(\d+)x(\d+)$/)
  if (!match) {
    submitError.value = t('imageWorkbench.invalidSize')
    return false
  }
  const w = Number(match[1])
  const h = Number(match[2])
  if (w < MIN_DIM || h < MIN_DIM || w > MAX_DIM || h > MAX_DIM) {
    submitError.value = t('imageWorkbench.invalidSize')
    return false
  }
  return true
}

function buildTurn(text: string, files: File[], references: StoredReference[]): Turn {
  return {
    id: createId(),
    prompt: text,
    model: model.value,
    size: size.value,
    quality: quality.value,
    background: transparentBackground.value ? 'transparent' : undefined,
    n: clampCount(count.value),
    mode: files.length ? 'edit' : 'generate',
    referenceImages: references,
    status: 'processing',
    urls: [],
    createdAt: Date.now(),
  }
}

async function runTurn(conversation: Conversation, turn: Turn, files: File[]) {
  conversation.updatedAt = Date.now()
  persist()
  try {
    const task = await submitImageWorkbenchTask({
      prompt: turn.prompt,
      model: turn.model,
      size: turn.size,
      quality: turn.quality,
      background: turn.background,
      n: turn.n,
      images: files,
    })
    applyTaskToTurn(turn, task)
    conversation.updatedAt = Date.now()
    persist()
    if (task.status === 'processing') schedulePoll(task.id)
  } catch (error: any) {
    const message: string = error?.message || t('imageWorkbench.taskFailed')
    turn.status = 'failed'
    turn.error = message
    submitError.value = message
    persist()
  }
}

async function send() {
  if (submitting.value || !ready.value) return
  const text = prompt.value.trim()
  if (!text) {
    submitError.value = t('imageWorkbench.promptRequired')
    return
  }
  if (!validateSize()) return

  const files = referenceImages.value.map((item) => item.file)
  const references = referenceImages.value.map((item) => ({ name: item.name, dataUrl: item.dataUrl }))

  const conversation = ensureActiveConversation()
  const turn = buildTurn(text, files, references)
  conversation.turns.push(turn)
  conversation.title = conversation.turns.length === 1 ? buildTitle(text) : conversation.title

  clearComposerInputs()
  submitError.value = ''
  submitting.value = true
  scrollToLatest()
  try {
    await runTurn(conversation, turn, files)
  } finally {
    submitting.value = false
    scrollToLatest()
  }
}

async function regenerate(turn: Turn) {
  const conversation = activeConversation.value
  if (!conversation || submitting.value) return
  const files = await Promise.all(
    turn.referenceImages.map((reference) => dataUrlToFile(reference.dataUrl, reference.name))
  )
  const next: Turn = { ...turn, id: createId(), taskId: undefined, status: 'processing', urls: [], error: undefined, createdAt: Date.now() }
  conversation.turns.push(next)
  conversation.updatedAt = Date.now()
  submitting.value = true
  scrollToLatest()
  try {
    await runTurn(conversation, next, files)
  } finally {
    submitting.value = false
    scrollToLatest()
  }
}

/* ------------------------------- 参考图与继续编辑 ------------------------------- */

function clearComposerInputs() {
  referenceImages.value = []
  if (fileInputRef.value) fileInputRef.value.value = ''
}

function isImageFile(file: File): boolean {
  if (file.type) return ['image/png', 'image/jpeg', 'image/jpg', 'image/webp', 'image/gif'].includes(file.type)
  return /\.(png|jpe?g|webp|gif)$/i.test(file.name)
}

function readFileAsDataUrl(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result || ''))
    reader.onerror = () => reject(reader.error || new Error('read failed'))
    reader.readAsDataURL(file)
  })
}

function dataUrlToFile(dataUrl: string, fileName: string): File {
  const [header, content] = dataUrl.split(',', 2)
  const mimeType = header.match(/data:(.*?);base64/)?.[1] || 'image/png'
  const binary = atob(content || '')
  const bytes = new Uint8Array(binary.length)
  for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index)
  return new File([bytes], fileName, { type: mimeType })
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
  submitError.value = imageFiles.length > room ? t('imageWorkbench.tooManyReferences', { n: maxImages.value }) : ''
  const accepted = imageFiles.slice(0, room)
  const next = await Promise.all(
    accepted.map(async (file) => ({
      id: createId(),
      name: file.name || 'reference.png',
      file,
      dataUrl: await readFileAsDataUrl(file),
    }))
  )
  referenceImages.value = [...referenceImages.value, ...next]
}

function removeReferenceImage(index: number) {
  referenceImages.value = referenceImages.value.filter((_, current) => current !== index)
}

async function continueEdit(turn: Turn) {
  const url = turn.urls[0]
  if (!url) return
  if (referenceImages.value.length >= maxImages.value) {
    submitError.value = t('imageWorkbench.tooManyReferences', { n: maxImages.value })
    return
  }
  try {
    const response = await fetch(url)
    if (!response.ok) throw new Error(`HTTP ${response.status}`)
    const blob = await response.blob()
    const name = `result-${Date.now()}.png`
    const file = new File([blob], name, { type: blob.type || 'image/png' })
    referenceImages.value = [
      ...referenceImages.value,
      { id: createId(), name, file, dataUrl: await readFileAsDataUrl(file) },
    ]
    prompt.value = ''
    submitError.value = ''
    nextTick(() => textareaRef.value?.focus())
  } catch {
    submitError.value = t('imageWorkbench.downloadFailed')
  }
}

/* --------------------------------- 拖拽 / 粘贴 --------------------------------- */

function hasDraggedImages(dataTransfer: DataTransfer | null): boolean {
  if (!dataTransfer) return false
  const items = Array.from(dataTransfer.items || [])
  if (items.length) {
    return items.some((item) => item.kind === 'file' && (item.type.startsWith('image/') || !item.type))
  }
  return Array.from(dataTransfer.files || []).some(isImageFile)
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
  void addReferenceFiles(Array.from(event.dataTransfer?.files || []).filter(isImageFile))
}

function onPromptPaste(event: ClipboardEvent) {
  const files = Array.from(event.clipboardData?.files || []).filter(isImageFile)
  if (!files.length) return
  event.preventDefault()
  void addReferenceFiles(files)
}

/* ------------------------------------ 灯箱 ------------------------------------ */

function openLightbox(turn: Turn, index: number) {
  lightboxUrls.value = [...turn.urls]
  lightboxPrompt.value = turn.prompt
  lightboxIndex.value = Math.max(0, Math.min(index, lightboxUrls.value.length - 1))
  lightboxOpen.value = true
}

function closeLightbox() {
  lightboxOpen.value = false
}

function stepLightbox(delta: number) {
  const total = lightboxUrls.value.length
  if (total <= 0) return
  lightboxIndex.value = (lightboxIndex.value + delta + total) % total
}

/* ------------------------------------ 下载 ------------------------------------ */

function filenameFor(index: number): string {
  return `sub2api-image-${Date.now()}-${index + 1}.png`
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

async function downloadOne(url: string, index: number) {
  if (!url) return
  downloadingOne.value = true
  try {
    try {
      triggerBlobDownload(await fetchAsBlob(url), filenameFor(index))
    } catch {
      window.open(url, '_blank', 'noopener,noreferrer')
    }
  } finally {
    downloadingOne.value = false
  }
}

async function downloadAll(turn: Turn) {
  if (!turn.urls.length) return
  downloadingAll.value = true
  try {
    for (let index = 0; index < turn.urls.length; index += 1) {
      const url = turn.urls[index]
      try {
        triggerBlobDownload(await fetchAsBlob(url), filenameFor(index))
      } catch {
        window.open(url, '_blank', 'noopener,noreferrer')
      }
      await new Promise((resolve) => window.setTimeout(resolve, 250))
    }
  } finally {
    downloadingAll.value = false
  }
}

/* ------------------------------------ 确认弹窗 ----------------------------------- */

const confirmClearConversations = ref(false)
const confirmClearConversation = ref(false)

function confirmPendingAction() {
  const pending = pendingConfirm.value
  pendingConfirm.value = null
  pending?.run()
}

watch(
  () => confirmClearConversations.value,
  (value) => {
    if (!value) return
    confirmClearConversations.value = false
    pendingConfirm.value = {
      title: t('imageWorkbench.clearConversations'),
      description: t('imageWorkbench.confirmClearConversations'),
      run: clearAllConversations,
    }
  }
)

watch(
  () => confirmClearConversation.value,
  (value) => {
    if (!value) return
    confirmClearConversation.value = false
    pendingConfirm.value = {
      title: t('imageWorkbench.clearConversation'),
      description: t('imageWorkbench.confirmClearConversation'),
      run: clearActiveConversation,
    }
  }
)

/* ------------------------------------ 滚动 ------------------------------------ */

function scrollToLatest() {
  nextTick(() => {
    if (scrollEl.value) scrollEl.value.scrollTop = scrollEl.value.scrollHeight
  })
}

watch(
  () => activeConversationId.value,
  () => scrollToLatest()
)

watch(
  [model, size, quality, count, customWidth, customHeight, alignTo16, transparentBackground],
  persistSettings
)

watch(
  () => lightboxUrls.value.length,
  () => {
    if (lightboxIndex.value >= lightboxUrls.value.length) {
      lightboxIndex.value = Math.max(0, lightboxUrls.value.length - 1)
    }
  }
)

/* ------------------------------------ 生命周期 ----------------------------------- */

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
  restoreSettings()
  restoreConversations()
  if (!conversations.value.length) newConversation()
  window.addEventListener('keydown', onKeydown)

  try {
    config.value = await getImageWorkbenchConfig()
    if (config.value.models?.length) mergeAvailableModels(config.value.models)
  } catch (error: any) {
    configError.value = error?.status === 503 || error?.code === 'IMAGE_WORKBENCH_UNAVAILABLE'
      ? t('imageWorkbench.unavailable')
      : t('imageWorkbench.configFailed')
  }

  try {
    const result = await getImageWorkbenchModels()
    if (result.models?.length) mergeAvailableModels(result.models)
    if (result.source === 'fallback') configError.value = configError.value || t('imageWorkbench.modelsFallback')
  } catch {
    configError.value = configError.value || t('imageWorkbench.modelsUnavailable')
  }
  mergeAvailableModels([model.value])

  conversations.value
    .flatMap((conversation) => conversation.turns)
    .filter((turn) => turn.status === 'processing' && turn.taskId)
    .forEach((turn) => schedulePoll(turn.taskId!, 0))

  scrollToLatest()
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  pollTimers.forEach((timer) => window.clearTimeout(timer))
  pollTimers.clear()
})
</script>
