<template>
  <div
    class="bg-gray-50 dark:bg-dark-950"
    :class="fillHeight
      ? 'flex min-h-screen flex-col lg:h-dvh lg:min-h-0 lg:overflow-hidden'
      : 'min-h-screen'"
  >
    <!-- Background Decoration -->
    <div class="pointer-events-none fixed inset-0 bg-mesh-gradient"></div>

    <!-- Sidebar -->
    <AppSidebar />

    <!-- Main Content Area -->
    <div
      class="relative transition-all duration-300"
      :class="[
        sidebarCollapsed ? 'lg:ml-[72px]' : 'lg:ml-64',
        fillHeight
          ? 'flex min-h-0 flex-1 flex-col lg:overflow-hidden'
          : 'min-h-screen',
      ]"
    >
      <!-- Header -->
      <div class="shrink-0">
        <AppHeader />
      </div>

      <!-- Main Content -->
      <main
        :class="fillHeight
          ? 'flex min-h-0 flex-1 flex-col p-3 md:p-4 lg:overflow-hidden lg:p-4'
          : 'p-4 md:p-6 lg:p-8'"
      >
        <div
          :class="fillHeight
            ? 'flex min-h-0 min-w-0 flex-1 flex-col lg:overflow-hidden'
            : undefined"
        >
          <slot />
        </div>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import { computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'

const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const isAdmin = computed(() => authStore.user?.role === 'admin')
const fillHeight = computed(() => route.meta.fillHeight === true)

const { replayTour } = useOnboardingTour({
  storageKey: isAdmin.value ? 'admin_guide' : 'user_guide',
  autoStart: true
})

const onboardingStore = useOnboardingStore()

onMounted(() => {
  onboardingStore.setReplayCallback(replayTour)
})

defineExpose({ replayTour })
</script>
