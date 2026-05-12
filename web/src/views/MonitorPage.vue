<template>
  <div>
    <el-card shadow="never" style="margin-bottom: 16px">
      <div style="display: flex; justify-content: space-between; align-items: center">
        <span style="font-size: 16px; font-weight: 600">运行监控</span>
        <div>
          <span style="color: #666; font-size: 13px; margin-right: 12px">
            上次刷新: {{ lastRefresh }}
          </span>
          <el-button @click="fetchData" :icon="Refresh" :loading="loading">刷新</el-button>
        </div>
      </div>
    </el-card>

    <div v-if="instances.length === 0 && !loading" style="text-align: center; padding: 60px; color: #999">
      暂无实例，请先在"配置管理"页面添加
    </div>

    <el-row :gutter="16" v-loading="loading">
      <el-col v-for="inst in instances" :key="inst.id" :xs="24" :sm="12" :md="8" :lg="6" style="margin-bottom: 16px">
        <el-card shadow="never">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px">
            <span style="font-weight: 600; font-size: 15px">{{ inst.name }}</span>
            <el-tag v-if="inst.status === 'running'" type="success" size="small">运行中</el-tag>
            <el-tag v-else-if="inst.status === 'error'" type="danger" size="small">错误</el-tag>
            <el-tag v-else type="info" size="small">已停止</el-tag>
          </div>

          <template v-if="inst.stats">
            <el-descriptions :column="1" size="small" border style="margin-bottom: 12px">
              <el-descriptions-item label="IEC104端口">
                <el-tag size="small">{{ inst.iec104_port }}</el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="HTTP端口" v-if="inst.http_enabled">
                <el-tag size="small" type="warning">{{ inst.http_port }}</el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="客户端">
                <el-tag :type="inst.stats.client_connected ? 'success' : 'danger'" size="small">
                  {{ inst.stats.client_connected ? '已连接' : '未连接' }}
                </el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="测点数">{{ inst.stats.total_points }}</el-descriptions-item>
              <el-descriptions-item label="运行时间">{{ fmtUptime(inst.stats.uptime_seconds) }}</el-descriptions-item>
              <el-descriptions-item label="总召次数">{{ inst.stats.interrogations }}</el-descriptions-item>
              <el-descriptions-item label="变化上送">{{ inst.stats.spontaneous }}</el-descriptions-item>
            </el-descriptions>
          </template>
          <template v-else>
            <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 12px">
              <template #title>实例未运行</template>
            </el-alert>
          </template>

          <div style="display: flex; gap: 8px">
            <el-button
              v-if="inst.status === 'running'"
              type="warning"
              size="small"
              style="flex: 1"
              @click="handleRestart(inst.id)"
            >
              重启
            </el-button>
            <el-button
              v-else
              type="success"
              size="small"
              style="flex: 1"
              @click="handleStart(inst.id)"
            >
              启动
            </el-button>
            <el-button
              v-if="inst.status === 'running'"
              size="small"
              @click="handleStop(inst.id)"
            >
              停止
            </el-button>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import {
  listInstances,
  startInstance,
  stopInstance,
  restartInstance,
  type InstanceState,
} from '../api'

const loading = ref(false)
const instances = ref<InstanceState[]>([])
const lastRefresh = ref('')
let timer: ReturnType<typeof setInterval> | null = null

async function fetchData() {
  loading.value = true
  try {
    instances.value = await listInstances()
    lastRefresh.value = new Date().toLocaleTimeString()
  } catch (e: any) {
    // Silent fail on auto-refresh
  } finally {
    loading.value = false
  }
}

async function handleStart(id: string) {
  try {
    await startInstance(id)
    ElMessage.success('已启动')
    await fetchData()
  } catch (e: any) {
    ElMessage.error('启动失败: ' + (e?.response?.data?.error || e.message))
  }
}

async function handleStop(id: string) {
  try {
    await stopInstance(id)
    ElMessage.success('已停止')
    await fetchData()
  } catch (e: any) {
    ElMessage.error('停止失败: ' + (e?.response?.data?.error || e.message))
  }
}

async function handleRestart(id: string) {
  try {
    await restartInstance(id)
    ElMessage.success('已重启')
    await fetchData()
  } catch (e: any) {
    ElMessage.error('重启失败: ' + (e?.response?.data?.error || e.message))
  }
}

function fmtUptime(s: number): string {
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  if (h > 0) return `${h}h${m}m${sec}s`
  if (m > 0) return `${m}m${sec}s`
  return `${sec}s`
}

onMounted(() => {
  fetchData()
  // Auto-refresh every 5s
  timer = setInterval(fetchData, 5000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>
