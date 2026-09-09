<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import {
  ArrowRightOnRectangleIcon,
  Bars3Icon,
  Cog6ToothIcon,
  DocumentTextIcon,
  HomeIcon,
  MapIcon,
  ServerStackIcon,
  UserCircleIcon,
  UserGroupIcon,
  UsersIcon,
} from '@heroicons/vue/24/outline'
import { setLocale } from '@/i18n'
import BrandLogo from '@/components/BrandLogo.vue'
import { urlForTab, type DashTab } from '@/lib/tabs'

const props = defineProps<{ tab: DashTab; staff: boolean }>()
const emit = defineEmits<{ 'update:tab': [DashTab]; logout: [] }>()
const { t, locale } = useI18n()

function go(next: DashTab) {
  emit('update:tab', next)
  const el = document.activeElement as HTMLElement | null
  el?.blur()
}

function toggleLang() {
  setLocale(locale.value === 'ru' ? 'en' : 'ru')
}

const itemClass = (id: DashTab) => (props.tab === id ? 'nav-active' : '')
</script>

<template>
  <header class="fixed top-0 inset-x-0 z-50 border-b border-base-content/10 bg-base-100/95 backdrop-blur-xl">
    <div class="grid grid-cols-[auto_1fr_auto] items-center gap-x-4 sm:gap-x-8 lg:gap-x-10 max-w-[90rem] mx-auto px-3 sm:px-5 min-h-14">
      <BrandLogo />

      <nav class="hidden lg:flex items-center justify-center min-w-0 px-2">
        <ul class="ovpn-nav menu menu-horizontal flex-nowrap px-0 py-0 text-sm gap-0.5">
          <template v-if="!props.staff">
            <li>
              <a class="gap-2" :href="urlForTab('home')" :class="itemClass('home')" @click.prevent="go('home')">
                <HomeIcon class="size-4 shrink-0" />{{ t('nav.home') }}
              </a>
            </li>
          </template>
          <template v-if="props.staff">
            <li>
              <a class="gap-2" :href="urlForTab('server')" :class="itemClass('server')" @click.prevent="go('server')">
                <ServerStackIcon class="size-4 shrink-0" />{{ t('nav.server') }}
              </a>
            </li>
            <li>
              <a class="gap-2" :href="urlForTab('clients')" :class="itemClass('clients')" @click.prevent="go('clients')">
                <UsersIcon class="size-4 shrink-0" />{{ t('nav.clients') }}
              </a>
            </li>
            <li>
              <a class="gap-2" :href="urlForTab('map')" :class="itemClass('map')" @click.prevent="go('map')">
                <MapIcon class="size-4 shrink-0" />{{ t('nav.map') }}
              </a>
            </li>
            <li>
              <a class="gap-2" :href="urlForTab('log')" :class="itemClass('log')" @click.prevent="go('log')">
                <DocumentTextIcon class="size-4 shrink-0" />{{ t('nav.log') }}
              </a>
            </li>
            <li>
              <a class="gap-2" :href="urlForTab('settings')" :class="itemClass('settings')" @click.prevent="go('settings')">
                <Cog6ToothIcon class="size-4 shrink-0" />{{ t('nav.settings') }}
              </a>
            </li>
            <li>
              <a class="gap-2" :href="urlForTab('users')" :class="itemClass('users')" @click.prevent="go('users')">
                <UserGroupIcon class="size-4 shrink-0" />{{ t('nav.users') }}
              </a>
            </li>
          </template>
          <li>
            <a class="gap-2" :href="urlForTab('profile')" :class="itemClass('profile')" @click.prevent="go('profile')">
              <UserCircleIcon class="size-4 shrink-0" />{{ t('nav.profile') }}
            </a>
          </li>
        </ul>
      </nav>

      <div class="flex items-center justify-end gap-1 sm:gap-2">
        <button class="btn btn-ghost btn-sm font-mono" type="button" @click="toggleLang">
          {{ locale === 'ru' ? t('lang.en') : t('lang.ru') }}
        </button>
        <button class="btn btn-ghost btn-sm gap-2 hidden lg:inline-flex" type="button" @click="emit('logout')">
          <ArrowRightOnRectangleIcon class="size-4 shrink-0" />
          {{ t('nav.logout') }}
        </button>
        <div class="dropdown dropdown-end lg:hidden">
          <div tabindex="0" role="button" class="btn btn-ghost btn-square btn-sm" :aria-label="t('nav.menu')">
            <Bars3Icon class="size-5" />
          </div>
          <ul
            tabindex="0"
            class="ovpn-nav menu menu-sm dropdown-content bg-base-100 rounded-box z-50 mt-3 w-56 p-2 shadow-lg border border-base-content/10"
          >
            <template v-if="!props.staff">
              <li>
                <a class="gap-2" :href="urlForTab('home')" :class="itemClass('home')" @click.prevent="go('home')">
                  <HomeIcon class="size-4" />{{ t('nav.home') }}
                </a>
              </li>
            </template>
            <template v-if="props.staff">
              <li>
                <a class="gap-2" :href="urlForTab('server')" :class="itemClass('server')" @click.prevent="go('server')">
                  <ServerStackIcon class="size-4" />{{ t('nav.server') }}
                </a>
              </li>
              <li>
                <a class="gap-2" :href="urlForTab('clients')" :class="itemClass('clients')" @click.prevent="go('clients')">
                  <UsersIcon class="size-4" />{{ t('nav.clients') }}
                </a>
              </li>
              <li>
                <a class="gap-2" :href="urlForTab('map')" :class="itemClass('map')" @click.prevent="go('map')">
                  <MapIcon class="size-4" />{{ t('nav.map') }}
                </a>
              </li>
              <li>
                <a class="gap-2" :href="urlForTab('log')" :class="itemClass('log')" @click.prevent="go('log')">
                  <DocumentTextIcon class="size-4" />{{ t('nav.log') }}
                </a>
              </li>
              <li>
                <a class="gap-2" :href="urlForTab('settings')" :class="itemClass('settings')" @click.prevent="go('settings')">
                  <Cog6ToothIcon class="size-4" />{{ t('nav.settings') }}
                </a>
              </li>
              <li>
                <a class="gap-2" :href="urlForTab('users')" :class="itemClass('users')" @click.prevent="go('users')">
                  <UserGroupIcon class="size-4" />{{ t('nav.users') }}
                </a>
              </li>
            </template>
            <li>
              <a class="gap-2" :href="urlForTab('profile')" :class="itemClass('profile')" @click.prevent="go('profile')">
                <UserCircleIcon class="size-4" />{{ t('nav.profile') }}
              </a>
            </li>
            <li>
              <a class="gap-2" @click.prevent="emit('logout')">
                <ArrowRightOnRectangleIcon class="size-4" />{{ t('nav.logout') }}
              </a>
            </li>
          </ul>
        </div>
      </div>
    </div>
  </header>
</template>
