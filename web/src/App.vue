<template>
  <el-container :class="[theme === 'light' ? 'theme-light' : 'theme-dark']" style="height: 100vh; flex-direction: column">
    <header class="app-header">
      <div style="display: flex; align-items: center; gap: 12px">
        <el-button text @click="sidebarCollapsed = !sidebarCollapsed" style="font-size: 18px; padding: 4px">
          <el-icon><Fold /></el-icon>
        </el-button>
        <h2>{{ t('app_title') }} <span style="font-size: 13px; font-weight: 400; color: var(--el-text-color-secondary)">v{{ version }}</span></h2>
      </div>
      <div style="display: flex; align-items: center; gap: 8px">
        <el-button size="small" :icon="theme === 'dark' ? Moon : Sunny" circle @click="toggleTheme" :title="theme === 'dark' ? t('theme_light') : t('theme_dark')" />
        <el-button size="small" @click="setLang(lang === 'zh' ? 'en' : 'zh')" style="width: 64px">{{ lang === 'zh' ? 'English' : '中文' }}</el-button>
        <el-tag v-if="status" class="header-status-tag">
          {{ t('nav_monitor') }} {{ status.running }}/{{ status.configured }}
        </el-tag>
      </div>
    </header>
    <el-container style="height: calc(100vh - var(--header-height))">
      <el-aside class="app-sidebar" :style="{ width: sidebarCollapsed ? '64px' : '200px' }"
        @mouseenter="sidebarCollapsed = false" @mouseleave="sidebarCollapsed = true">
        <el-menu :router="true" :default-active="currentRoute" :collapse="sidebarCollapsed">
          <el-menu-item index="/config">
            <el-icon><Setting /></el-icon>
            <span>{{ t('nav_config') }}</span>
          </el-menu-item>
          <el-menu-item index="/monitor">
            <el-icon><Monitor /></el-icon>
            <span>{{ t('nav_monitor') }}</span>
          </el-menu-item>
          <el-menu-item index="/trend">
            <el-icon><DataLine /></el-icon>
            <span>{{ t('nav_trend') }}</span>
          </el-menu-item>
        </el-menu>
      </el-aside>
      <el-main class="app-main">
        <router-view v-slot="{ Component }">
          <transition name="fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { Setting, Monitor, DataLine, Fold, Moon, Sunny } from '@element-plus/icons-vue'
import { getStatus, type GlobalStatus } from './api'
import { useI18n } from './composables/useI18n'
import { useTheme } from './composables/useTheme'

const { t, lang, setLang } = useI18n()
const { theme, toggleTheme } = useTheme()

const route = useRoute()
const sidebarCollapsed = ref(true)
const currentRoute = computed(() => {
  const path = route.path
  if (path.startsWith('/detail/')) return '/monitor'
  return path
})
const status = ref<GlobalStatus | null>(null)
const version = ref('2.2.0')

onMounted(async () => {
  try {
    status.value = await getStatus()
    if (status.value?.version) version.value = status.value.version
  } catch {}
})
</script>

<style>
:root {
  --header-height: 52px;
  --app-font-size-lg: 16px;
  --app-font-size-base: 14px;
  --app-font-size-sm: 13px;
  --app-font-size-xs: 12px;
  --app-spacing-base: 16px;
  --app-spacing-sm: 8px;
}
body { margin: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
.el-menu-item { font-size: 14px; }
*:focus-visible { outline: 2px solid #409eff; outline-offset: 2px; }
.el-button:focus-visible { outline: 2px solid #409eff; outline-offset: 1px; }
.fade-enter-active, .fade-leave-active { transition: opacity 0.2s ease; }
.fade-enter-from, .fade-leave-to { opacity: 0; }

/* ===== Dark Theme (default) ===== */
[data-theme="dark"] {
  --bg-primary: #0a0e17;
  --bg-secondary: #111827;
  --bg-tertiary: #1a1f2e;
  --bg-card: #111827;
  --bg-main: #0f1520;
  --border: #1e293b;
  --text-primary: #e2e8f0;
  --text-secondary: #94a3b8;
  --text-muted: #475569;
  --accent: #f59e0b;
}
[data-theme="dark"] .app-header { background: #0a0e17; border-color: #1e293b; }
[data-theme="dark"] .app-header h2 { color: #e2e8f0; }
[data-theme="dark"] .app-sidebar { background: #111827; border-color: #1e293b; }
[data-theme="dark"] .app-sidebar .el-menu-item { color: #94a3b8; }
[data-theme="dark"] .app-sidebar .el-menu-item:hover { color: #e2e8f0; background: #1a1f2e !important; }
[data-theme="dark"] .app-sidebar .el-menu-item.is-active { color: #f59e0b; background: rgba(245,158,11,0.1) !important; }
[data-theme="dark"] .app-main { background: #0f1520; }
[data-theme="dark"] .app-main .el-card { background: #111827 !important; border-color: #1e293b !important; color: #e2e8f0; }
[data-theme="dark"] .app-main .el-radio-button__inner { background: #1a1f2e !important; border-color: #1e293b !important; color: #94a3b8 !important; }
[data-theme="dark"] .app-main .el-radio-button__original-radio:checked+.el-radio-button__inner { background: #f59e0b !important; color: #000 !important; }
[data-theme="dark"] .app-main .el-dialog { background: #111827 !important; border-color: #1e293b !important; }
[data-theme="dark"] .app-main .el-dialog__title { color: #e2e8f0 !important; }
[data-theme="dark"] .app-main .el-select-dropdown { background: #1a1f2e !important; border-color: #1e293b !important; }
[data-theme="dark"] .app-main .el-select-dropdown__item { color: #94a3b8 !important; }
[data-theme="dark"] .app-main .el-select-dropdown__item.selected { color: #f59e0b !important; }
[data-theme="dark"] .app-main .el-input__wrapper { background: #0a0e17 !important; border-color: #1e293b !important; }
[data-theme="dark"] .app-main .el-input__inner { color: #e2e8f0 !important; }

/* ===== Light Theme ===== */
[data-theme="light"] {
  --bg-primary: #ffffff;
  --bg-secondary: #f5f7fa;
  --bg-tertiary: #ebeef5;
  --bg-card: #ffffff;
  --bg-main: #f0f2f5;
  --border: #dcdfe6;
  --text-primary: #303133;
  --text-secondary: #606266;
  --text-muted: #c0c4cc;
  --accent: #409eff;
}
[data-theme="light"] .app-header { background: #fff; border-color: #dcdfe6; }
[data-theme="light"] .app-header h2 { color: #303133; }
[data-theme="light"] .app-sidebar { background: #fff; border-color: #dcdfe6; }
[data-theme="light"] .app-sidebar .el-menu-item { color: #606266; }
[data-theme="light"] .app-sidebar .el-menu-item:hover { color: #409eff; background: #ecf5ff !important; }
[data-theme="light"] .app-sidebar .el-menu-item.is-active { color: #409eff; background: #ecf5ff !important; }
[data-theme="light"] .app-main { background: #f0f2f5; }
[data-theme="light"] .app-main .el-card { background: #fff !important; border-color: #dcdfe6 !important; color: #303133; }
[data-theme="light"] .app-main .el-radio-button__inner { background: #fff !important; border-color: #dcdfe6 !important; color: #606266 !important; }
[data-theme="light"] .app-main .el-radio-button__original-radio:checked+.el-radio-button__inner { background: #409eff !important; color: #fff !important; }
[data-theme="light"] .app-main .el-dialog { background: #fff !important; border-color: #dcdfe6 !important; }
[data-theme="light"] .app-main .el-dialog__title { color: #303133 !important; }
[data-theme="light"] .app-main .el-select-dropdown { background: #fff !important; border-color: #dcdfe6 !important; }
[data-theme="light"] .app-main .el-select-dropdown__item { color: #606266 !important; }
[data-theme="light"] .app-main .el-select-dropdown__item.selected { color: #409eff !important; }
[data-theme="light"] .app-main .el-input__wrapper { background: #fff !important; border-color: #dcdfe6 !important; }
[data-theme="light"] .app-main .el-input__inner { color: #303133 !important; }

/* Shared layout */
.app-header {
  height: var(--header-height);
  display: flex; align-items: center; justify-content: space-between;
  padding: 0 20px; border-bottom: 1px solid; box-sizing: border-box;
}
.app-header h2 { margin: 0; font-size: var(--app-font-size-lg); font-weight: 600; }
.header-status-tag { font-size: 12px; }
.app-sidebar { border-right: 1px solid; transition: width 0.2s ease; overflow: hidden; }
.app-sidebar .el-menu { border-right: none; background: transparent; }
.app-sidebar .el-menu-item { background: transparent !important; transition: all 0.15s; }
.app-main { padding: 16px; overflow-y: auto; }
.app-main .el-card { transition: border-color 0.2s; }
.app-main .el-card__body { background: transparent; }
.app-main .el-tag { --el-tag-bg-color: transparent; }
.app-main .el-dialog__body { background: transparent !important; }
.app-main .el-select-dropdown__item.hover { background: var(--bg-tertiary) !important; }
.app-main .el-input__wrapper { box-shadow: none !important; }
</style>
