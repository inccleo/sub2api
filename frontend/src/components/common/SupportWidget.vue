<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'

const { t } = useI18n()
const appStore = useAppStore()
const open = ref(false)
const panelRef = ref<HTMLElement | null>(null)

const qrCodeURL = computed(() => appStore.cachedPublicSettings?.support_qr_code_url?.trim() ?? '')
const contactInfo = computed(() => appStore.cachedPublicSettings?.contact_info?.trim() ?? '')
const visible = computed(() => qrCodeURL.value !== '')

function close() {
  open.value = false
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') close()
}

function onPointerDown(event: PointerEvent) {
  if (!open.value || panelRef.value?.contains(event.target as Node)) return
  close()
}

watch(open, (isOpen) => {
  if (isOpen) {
    document.addEventListener('keydown', onKeydown)
    document.addEventListener('pointerdown', onPointerDown)
    return
  }
  document.removeEventListener('keydown', onKeydown)
  document.removeEventListener('pointerdown', onPointerDown)
})

watch(visible, (isVisible) => {
  if (!isVisible) close()
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeydown)
  document.removeEventListener('pointerdown', onPointerDown)
})
</script>

<template>
  <Teleport to="body">
    <div v-if="visible" class="fixed bottom-5 right-5 z-40">
      <Transition name="support-panel">
        <section
          v-if="open"
          ref="panelRef"
          role="dialog"
          aria-modal="false"
          :aria-label="t('support.title')"
          class="absolute bottom-16 right-0 w-72 rounded-2xl border border-gray-200 bg-white p-4 text-center shadow-2xl dark:border-dark-700 dark:bg-dark-900"
        >
          <h2 class="text-base font-bold text-gray-950 dark:text-white">
            {{ t('support.title') }}
          </h2>
          <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400">
            {{ t('support.description') }}
          </p>
          <div class="mx-auto mt-3 w-fit rounded-xl border border-gray-200 bg-white p-2">
            <img
              :src="qrCodeURL"
              :alt="t('support.qrAlt')"
              class="h-44 w-44 rounded-lg object-contain"
            />
          </div>
          <p v-if="contactInfo" class="mt-3 break-all text-sm font-semibold text-gray-800 dark:text-gray-200">
            {{ contactInfo }}
          </p>
          <p class="mt-2 text-xs text-gray-400 dark:text-gray-500">
            {{ t('support.scanHint') }}
          </p>
        </section>
      </Transition>

      <button
        type="button"
        class="flex h-12 items-center gap-2 rounded-full bg-orange-600 px-4 text-sm font-semibold text-white shadow-lg transition hover:bg-orange-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-orange-500 focus-visible:ring-offset-2"
        :aria-expanded="open"
        :aria-label="t('support.button')"
        @click="open = !open"
      >
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.8">
          <path stroke-linecap="round" stroke-linejoin="round" d="M8.625 9.75a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H8.25m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H12m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0h-.375M21 12c0 4.556-4.03 8.25-9 8.25a9.764 9.764 0 0 1-2.555-.337A5.972 5.972 0 0 1 5.41 20.97a5.969 5.969 0 0 1-.474-.065 4.48 4.48 0 0 0 .978-2.025c.09-.457-.133-.901-.467-1.226C3.93 16.178 3 14.189 3 12c0-4.556 4.03-8.25 9-8.25s9 3.694 9 8.25Z" />
        </svg>
        {{ t('support.button') }}
      </button>
    </div>
  </Teleport>
</template>

<style scoped>
.support-panel-enter-active,
.support-panel-leave-active {
  transition: opacity 0.16s ease, transform 0.16s ease;
}

.support-panel-enter-from,
.support-panel-leave-to {
  opacity: 0;
  transform: translateY(8px);
}
</style>
