<template>
  <el-container style="height: 100vh; flex-direction: column">
    <header class="app-header">
      <div style="display: flex; align-items: center; gap: 12px">
        <el-button text @click="sidebarCollapsed = !sidebarCollapsed" style="font-size: 18px; padding: 4px">
          <el-icon><Fold /></el-icon>
        </el-button>
        <h2>IEC104 模拟器管理系统 <span style="font-size: 13px; font-weight: 400; color: var(--el-text-color-secondary)">v{{ version }}</span></h2>
      </div>
      <el-tag v-if="status" class="header-status-tag">
        运行 {{ status.running }} / 总计 {{ status.configured }}
      </el-tag>
    </header>
    <el-container style="height: calc(100vh - var(--header-height))">
      <el-aside class="app-sidebar" :style="{ width: sidebarCollapsed ? '64px' : '200px' }"
        @mouseenter="sidebarCollapsed = false" @mouseleave="sidebarCollapsed = true">
        <el-menu :router="true" :default-active="currentRoute" :collapse="sidebarCollapsed">
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
          <el-menu-item index="/users">
            <el-icon><UserFilled /></el-icon>
            <span>用户管理</span>
          </el-menu-item>
        </el-menu>
        <div style="padding: 12px 0; text-align: center; border-top: 1px solid var(--el-border-color-light)">
          <el-popconfirm title="确定退出登录？" @confirm="handleLogout">
            <template #reference>
              <el-button text size="small" style="color: var(--el-text-color-secondary)">
                <el-icon><SwitchButton /></el-icon>
                <span v-if="!sidebarCollapsed" style="margin-left: 4px">退出</span>
              </el-button>
            </template>
          </el-popconfirm>
        </div>
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
import { Setting, Monitor, DataLine, Fold, UserFilled, SwitchButton } from '@element-plus/icons-vue'
import { useRouter } from 'vue-router'
import { getStatus, type GlobalStatus } from './api'

const router = useRouter()

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

function handleLogout() {
  localStorage.removeItem('token')
  localStorage.removeItem('user')
  router.push('/login')
}
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
*:focus-visible { outline: 2px solid #409eff; outline-offset: 2px; }
.el-button:focus-visible { outline: 2px solid #409eff; outline-offset: 1px; }
.fade-enter-active, .fade-leave-active { transition: opacity 0.2s ease; }
.fade-enter-from, .fade-leave-to { opacity: 0; }

/* Dark theme overrides */
.app-header {
  height: var(--header-height);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  background: #0a0e17;
  border-bottom: 1px solid #1e293b;
  box-sizing: border-box;
}
.app-header h2 {
  margin: 0;
  font-size: var(--app-font-size-lg);
  font-weight: 600;
  color: #e2e8f0;
}
.header-status-tag { font-size: 12px; }
.app-sidebar {
  border-right: 1px solid #1e293b;
  background: #111827;
  transition: width 0.2s ease;
  overflow: hidden;
}
.app-sidebar .el-menu {
  border-right: none;
  background: transparent;
}
.app-sidebar .el-menu-item {
  color: #94a3b8;
  background: transparent !important;
}
.app-sidebar .el-menu-item:hover {
  color: #e2e8f0;
  background: #1a1f2e !important;
}
.app-sidebar .el-menu-item.is-active {
  color: #f59e0b;
  background: rgba(245,158,11,0.1) !important;
}
.app-main {
  background: #0f1520;
  padding: 16px;
  overflow-y: auto;
}

/* TrendPage dark card overrides */
.app-main .el-card {
  background: #111827 !important;
  border: 1px solid #1e293b !important;
  color: #e2e8f0;
}
.app-main .el-card__body {
  background: transparent;
}
.app-main .el-radio-button__inner {
  background: #1a1f2e !important;
  border-color: #1e293b !important;
  color: #94a3b8 !important;
}
.app-main .el-radio-button__original-radio:checked + .el-radio-button__inner {
  background: #f59e0b !important;
  border-color: #f59e0b !important;
  color: #000 !important;
  box-shadow: none !important;
}
.app-main .el-tag {
  --el-tag-bg-color: transparent;
}
.app-main .el-dialog {
  background: #111827 !important;
  border: 1px solid #1e293b !important;
}
.app-main .el-dialog__title {
  color: #e2e8f0 !important;
}
.app-main .el-dialog__body {
  background: #111827 !important;
}
.app-main .el-select-dropdown {
  background: #1a1f2e !important;
  border: 1px solid #1e293b !important;
}
.app-main .el-select-dropdown__item {
  color: #94a3b8 !important;
}
.app-main .el-select-dropdown__item.hover {
  background: #1e293b !important;
  color: #e2e8f0 !important;
}
.app-main .el-select-dropdown__item.selected {
  color: #f59e0b !important;
}
.app-main .el-input__wrapper {
  background: #0a0e17 !important;
  border-color: #1e293b !important;
  box-shadow: none !important;
}
.app-main .el-input__inner {
  color: #e2e8f0 !important;
}
</style>
