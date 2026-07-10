<template>
  <div class="flex min-h-0 flex-1 flex-col overflow-hidden">
    <header class="flex shrink-0 items-center gap-2 px-3 pb-3 pt-6">
      <button
        type="button"
        class="rounded-full p-1.5 text-wings-mutedStrong hover:text-wings-text"
        aria-label="Назад"
        @click="$emit('close')"
      >
        <ChevronLeft :size="24" />
      </button>
      <h1 class="font-sharp text-[22px] font-bold text-white">Лицензии свободного ПО</h1>
    </header>

    <div class="min-h-0 flex-1 overflow-y-auto px-4 pb-8">
      <SamsungCard
        title="Лицензии свободного ПО"
        subtitle="Основные open-source компоненты, используемые приложением и встроенными runtime частями"
      />

      <SamsungCard kicker="Лицензии свободного ПО" class="mt-4">
        <div class="divide-y divide-wings-divider">
          <button
            v-for="lib in libraries"
            :key="lib.title"
            type="button"
            class="flex w-full items-center gap-3.5 py-3.5 text-left"
            @click="open(lib.url)"
          >
            <AvatarCircle :initials="lib.initials" :color="lib.color" :size="44" />
            <span class="flex min-w-0 flex-1 flex-col">
              <span class="truncate text-[17px] text-wings-text">{{ lib.title }}</span>
              <span class="mt-0.5 text-sm text-wings-muted">{{ lib.summary }}</span>
            </span>
            <ChevronRight :size="18" class="shrink-0 text-wings-muted" />
          </button>
        </div>
      </SamsungCard>
    </div>
  </div>
</template>

<script setup>
import { ChevronLeft, ChevronRight } from 'lucide-vue-next';
import { Browser } from '@wailsio/runtime';
import SamsungCard from '@/components/layout/SamsungCard.vue';
import AvatarCircle from '@/components/layout/AvatarCircle.vue';

defineEmits(['close']);

// Open-source components actually bundled by the desktop port (not the Android runtime
// parts). Kept in the same visual shape as the Android licenses screen.
const libraries = [
  {
    title: 'Wails',
    summary: 'MIT License • Go + системный WebView desktop-фреймворк',
    initials: 'WL',
    color: '#C24A2B',
    url: 'https://github.com/wailsapp/wails',
  },
  {
    title: 'Vue',
    summary: 'MIT License • фреймворк интерфейса',
    initials: 'VU',
    color: '#159E6B',
    url: 'https://github.com/vuejs/core',
  },
  {
    title: 'WireGuard Go',
    summary: 'MIT License • userspace WireGuard (Windows) и wgctrl (Linux)',
    initials: 'WG',
    color: '#596574',
    url: 'https://git.zx2c4.com/wireguard-go/',
  },
  {
    title: 'netlink',
    summary: 'Apache License 2.0 • kernel WireGuard, маршруты и правила (Linux)',
    initials: 'NL',
    color: '#4F748D',
    url: 'https://github.com/vishvananda/netlink',
  },
  {
    title: 'vk-turn-proxy',
    summary: 'GNU GPL v3 • WINGS-N/vk-turn-proxy (форк cacggghp/vk-turn-proxy)',
    initials: 'VK',
    color: '#5C61D3',
    url: 'https://github.com/WINGS-N/vk-turn-proxy',
  },
  {
    title: 'pion',
    summary: 'MIT License • TURN/DTLS/SRTP стек для vk-turn-proxy',
    initials: 'PI',
    color: '#2F7DBB',
    url: 'https://github.com/pion/turn',
  },
  {
    title: 'gRPC-Go',
    summary: 'Apache License 2.0 • AppControl IPC к vkturn',
    initials: 'GR',
    color: '#2E9E8F',
    url: 'https://github.com/grpc/grpc-go',
  },
];

function open(url) {
  Browser.OpenURL(url).catch(() => {});
}
</script>
