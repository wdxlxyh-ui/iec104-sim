<template>
  <el-container style="height: 100vh; flex-direction: column">
    <header class="app-header">
      <h2>IEC104 模拟器管理系统 v2.1</h2>
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
        </el-menu>
      </el-aside>
      <el-main class="app-main">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { Setting, Monitor } from '@element-plus/icons-vue'
import { getStatus, type GlobalStatus } from './api'

const route = useRoute()
const currentRoute = computed(() => route.path)
const status = ref<GlobalStatus | null>(null)

onMounted(async () => {
  try {
    status.value = await getStatus()
  } catch {}
})
</script>

<style>
body { margin: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
.el-menu-item { font-size: 14px; }
</style>
