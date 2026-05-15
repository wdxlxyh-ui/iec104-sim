<template>
  <el-container style="height: 100vh; flex-direction: column">
    <header class="app-header">
      <h2>IEC104 模拟器管理系统 <span style="font-size: 13px; font-weight: 400; color: var(--el-text-color-secondary)">v{{ version }}</span></h2>
      <el-tag v-if="status" class="header-status-tag">
        运行 {{ status.running }} / 总计 {{ status.configured }}
      </el-tag>
    </header>
    <el-container style="height: calc(100vh - var(--header-height))">
      <el-aside class="app-sidebar" width="200px">
        <el-menu :router="true" :default-active="currentRoute">
          <el-menu-item index="/config">
            <el-icon><Setting /></el-icon>
            <span>配置管理</span>
          </el-menu-item>
          <el-menu-item index="/monitor">
            <el-icon><Monitor /></el-icon>
            <span>运行监控</span>
          </el-menu-item>
          <el-menu-item index="/trend">
            <el-icon><DataLine /></el-icon>
            <span>实时趋势</span>
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
import { Setting, Monitor, DataLine } from '@element-plus/icons-vue'
import { getStatus, type GlobalStatus } from './api'

const route = useRoute()
const currentRoute = computed(() => {
  const path = route.path
  // Detail page belongs to monitor group
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
  --app-color-text-primary: #303133;
  --app-color-text-regular: #606266;
  --app-color-text-secondary: #909399;
  --app-spacing-base: 16px;
  --app-spacing-sm: 8px;
}
body { margin: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
.el-menu-item { font-size: 14px; }

/* Focus ring for keyboard accessibility */
*:focus-visible {
  outline: 2px solid #409eff;
  outline-offset: 2px;
}
.el-button:focus-visible {
  outline: 2px solid #409eff;
  outline-offset: 1px;
}

/* Page transition */
.fade-enter-active, .fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from, .fade-leave-to {
  opacity: 0;
}

/* App layout */
.app-header {
  height: var(--header-height);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  background: #fff;
  border-bottom: 1px solid var(--el-border-color-light);
  box-sizing: border-box;
}
.app-header h2 {
  margin: 0;
  font-size: var(--app-font-size-lg);
  font-weight: 600;
  color: var(--app-color-text-primary);
}
.header-status-tag { font-size: 12px; }
.app-sidebar {
  border-right: 1px solid var(--el-border-color-light);
  background: #fff;
}
.app-main {
  background: var(--el-bg-color-page);
  padding: 16px;
  overflow-y: auto;
}
</style>
