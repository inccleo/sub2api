<template>
  <AuthLayout>
    <div class="space-y-6">
      <div class="text-center">
        <h2 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('desktopAuth.title') }}</h2>
        <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ t('desktopAuth.subtitle') }}</p>
      </div>

      <p v-if="loading" role="status" class="text-center text-gray-500">{{ t('desktopAuth.loading') }}</p>
      <div v-else-if="callback" class="space-y-4 text-center" role="status">
        <p>{{ t(approved ? 'desktopAuth.approved' : 'desktopAuth.denied') }}</p>
        <a :href="callback" rel="noreferrer" class="btn btn-primary w-full">{{ t('desktopAuth.returnToApp') }}</a>
        <p class="text-sm text-gray-500">{{ t('desktopAuth.returnHint') }}</p>
      </div>
      <template v-else-if="details">
        <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-800">
          <p class="font-medium">{{ details.client_name }}</p>
          <p class="mt-1 break-all text-sm text-gray-500">{{ details.email }}</p>
        </div>
        <p class="text-sm text-gray-600 dark:text-dark-300">{{ t('desktopAuth.permissions') }}</p>
        <p v-if="details.is_admin" class="rounded-lg bg-amber-50 p-3 text-sm text-amber-900 dark:bg-amber-900/20 dark:text-amber-200">{{ t('desktopAuth.adminWarning') }}</p>
        <p class="text-sm text-gray-500">{{ t('desktopAuth.confirmHint') }}</p>
        <div class="flex gap-3">
          <button class="btn btn-secondary flex-1" :disabled="busy" @click="decide('deny')">{{ t('desktopAuth.deny') }}</button>
          <button class="btn btn-primary flex-1" :disabled="busy" @click="decide('approve')">{{ t(busy ? 'desktopAuth.working' : 'desktopAuth.approve') }}</button>
        </div>
      </template>
      <p v-if="error" role="alert" class="rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ error }}</p>
    </div>
  </AuthLayout>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AuthLayout from '@/components/layout/AuthLayout.vue'
import { getDesktopAuthorization, decideDesktopAuthorization, type DesktopAuthorizeRequest, type DesktopAuthorizationDetails } from '@/api/desktopAuth'

const route = useRoute()
const { t } = useI18n()
const details = ref<DesktopAuthorizationDetails | null>(null)
const loading = ref(true)
const busy = ref(false)
const error = ref('')
const callback = ref('')
const approved = ref(false)
let request: DesktopAuthorizeRequest | null = null
let generation = 0

// Keep consent bound to the request that was validated, including on same-route navigation.
watch(() => route.fullPath, async () => {
  const current = ++generation
  details.value = null
  request = null
  callback.value = ''
  error.value = ''
  loading.value = true
  busy.value = false
  const fields: (keyof DesktopAuthorizeRequest)[] = ['client_id', 'redirect_uri', 'response_type', 'scope', 'state', 'code_challenge', 'code_challenge_method']
  try {
    const params = {} as DesktopAuthorizeRequest
    for (const field of fields) {
      const value = route.query[field]
      if (typeof value !== 'string' || !value) throw new Error('Invalid authorization request')
      params[field] = value
    }
    const result = await getDesktopAuthorization(params)
    if (current !== generation) return
    request = params
    details.value = result
  } catch {
    if (current === generation) error.value = t('desktopAuth.invalidRequest')
  } finally {
    if (current === generation) loading.value = false
  }
}, { immediate: true })

async function decide(decision: 'approve' | 'deny') {
  if (!request || !details.value || busy.value) return
  const current = generation
  busy.value = true
  error.value = ''
  try {
    const url = await decideDesktopAuthorization(request, decision)
    if (current !== generation) return
    approved.value = decision === 'approve'
    callback.value = url
    // Keep a visible link: browsers may require a fresh click to launch a native app.
    window.location.assign(url)
  } catch {
    if (current === generation) error.value = t('desktopAuth.failed')
  } finally {
    if (current === generation) busy.value = false
  }
}
</script>
