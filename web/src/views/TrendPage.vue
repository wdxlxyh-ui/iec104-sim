<template>
  <div>
    <!-- Description -->
    <el-card shadow="never" style="margin-bottom: 16px">
      <div style="font-size: 13px; color: var(--el-text-color-secondary); line-height: 1.8">
        选取多个实例的测点，放在同一张图上对比趋势。
        每 5 秒轮询一次，最长保留 1 小时数据。
        <span v-if="traces.length === 0" style="color: var(--el-color-warning)">
          请先添加测点开始监控。
        </span>
      </div>
    </el-card>

    <!-- Selected points -->
    <el-card shadow="never" style="margin-bottom: 16px">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px">
        <span style="font-size: 14px; font-weight: 500">已选测点</span>
        <span style="font-size: 12px; color: var(--el-text-color-secondary)">
          {{ traces.length }} / 8 条线
        </span>
      </div>
      <div style="display: flex; flex-wrap: wrap; gap: 8px">
        <el-tag
          v-for="(t, i) in traces"
          :key="i"
          :color="COLORS[t.colorIdx % COLORS.length]"
          closable
          :disable-transitions="true"
          style="color: #fff; border: none"
          @close="removeTrace(i)"
        >
          {{ t.inst }} · {{ t.alias || t.name || 'IOA:' + t.ioa }}
        </el-tag>
        <el-button size="small" @click="openDialog">+ 添加测点</el-button>
      </div>
    </el-card>

    <!-- Chart area -->
    <el-card shadow="never">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; flex-wrap: wrap; gap: 8px">
        <span style="font-size: 15px; font-weight: 600">📈 实时趋势</span>
        <div style="display: flex; align-items: center; gap: 12px">
          <el-radio-group v-model="timeRange" size="small">
            <el-radio-button :value="5">5m</el-radio-button>
            <el-radio-button :value="15">15m</el-radio-button>
            <el-radio-button :value="30">30m</el-radio-button>
            <el-radio-button :value="60">1h</el-radio-button>
          </el-radio-group>
          <span style="font-size: 12px; color: var(--el-text-color-secondary)">
            {{ pointsCount }} 点 · 上次 {{ lastUpdate }}
          </span>
        </div>
      </div>

      <div v-if="traces.length === 0" style="text-align: center; padding: 80px 20px; color: var(--el-text-color-placeholder)">
        <div style="font-size: 48px; margin-bottom: 12px">📊</div>
        <div>点击上方「+ 添加测点」选择要跟踪的数据</div>
      </div>

      <div v-else ref="chartRef" style="width: 100%">
        <svg :viewBox="`0 0 ${SVG_W} ${SVG_H}`" preserveAspectRatio="xMidYMid meet" style="width: 100%; height: auto; display: block">
          <!-- Grid lines -->
          <line v-for="i in 6" :key="'g'+i"
            :x1="padL" :y1="padT + chartH - (i/6)*chartH"
            :x2="padL + chartW" :y2="padT + chartH - (i/6)*chartH"
            stroke="#e0e0e0" stroke-width="0.5" />
          <!-- Y labels -->
          <text v-for="i in 6" :key="'yl'+i"
            :x="padL - 6" :y="padT + chartH - (i/6)*chartH + 3"
            text-anchor="end" font-size="10" font-family="monospace" fill="#999"
          >{{ yLabel(i/6) }}</text>
          <!-- Axes -->
          <line :x1="padL" :y1="padT" :x2="padL" :y2="padT+chartH" stroke="#ccc" stroke-width="1" />
          <line :x1="padL" :y1="padT+chartH" :x2="padL+chartW" :y2="padT+chartH" stroke="#ccc" stroke-width="1" />
          <!-- X labels -->
          <text v-for="i in 5" :key="'xl'+i"
            :x="padL + (i/5)*chartW" :y="padT + chartH + 16"
            text-anchor="middle" font-size="10" font-family="monospace" fill="#999"
          >{{ xLabel(i/5) }}</text>
          <!-- Data lines -->
          <polyline
            v-for="(t, vi) in visibleTraces"
            :key="'line'+vi"
            :points="linePoints(t, vi)"
            :stroke="COLORS[t.colorIdx % COLORS.length]"
            fill="none" stroke-width="1.5" stroke-linejoin="round" stroke-linecap="round"
          />
        </svg>
        <!-- Legend -->
        <div style="display: flex; flex-wrap: wrap; gap: 12px; margin-top: 16px; padding-top: 16px; border-top: 1px solid var(--el-border-color-light)">
          <div
            v-for="(t, i) in traces"
            :key="i"
            style="display: flex; align-items: center; gap: 8px; font-size: 12px; cursor: pointer; padding: 4px 10px; border-radius: 4px;"
            :style="{ opacity: hiddenTraces.has(i) ? 0.35 : 1 }"
            @click="toggleTrace(i)"
          >
            <span style="width: 20px; height: 3px; border-radius: 2px; flex-shrink: 0"
              :style="{ background: COLORS[t.colorIdx % COLORS.length] }"></span>
            <span style="color: var(--el-text-color-regular)">{{ t.inst }} · {{ t.alias || t.name || 'IOA:' + t.ioa }}</span>
            <span style="font-family: monospace; font-size: 12px; font-weight: 600">{{ lastValue(t) }}</span>
            <span style="font-family: monospace; font-size: 10px; color: var(--el-text-color-secondary)">{{ t.unit }}</span>
          </div>
        </div>
      </div>
    </el-card>

    <!-- Add dialog -->
    <el-dialog v-model="dialogVisible" title="添加趋势测点" width="440px">
      <el-form label-width="80px">
        <el-form-item label="实例">
          <el-select v-model="formInst" filterable style="width: 100%" @change="onInstChange">
            <el-option v-for="inst in allInstances" :key="inst.id" :label="inst.name" :value="inst.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="测点">
          <el-select v-model="formIoa" filterable style="width: 100%">
            <el-option v-for="pt in formPoints" :key="pt.ioa" :label="pt.name + ' (IOA:' + pt.ioa + ')'" :value="pt.ioa" />
          </el-select>
        </el-form-item>
        <el-form-item label="别名">
          <el-input v-model="formAlias" placeholder="可选，如: 关口有功" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="addTrace">确认添加</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import { listInstances, getPoints, readPoint, type PointSnapshot } from '../api'

const COLORS = ['#14b8a6', '#f59e0b', '#3b82f6', '#a855f7', '#ec4899', '#22d3ee', '#f97316', '#8b5cf6']

interface Trace {
  instId: string
  inst: string
  ioa: number
  name: string
  unit: string
  alias: string
  colorIdx: number
  data: number[]
}

// Chart constants
const SVG_W = 700
const SVG_H = 280
const padL = 50
const padT = 20
const chartW = SVG_W - padL - 20
const chartH = SVG_H - padT - 36

// State
const traces = ref<Trace[]>([])
const hiddenTraces = ref<Set<number>>(new Set())
const timeRange = ref(15)
const lastUpdate = ref('--')
const pointsCount = ref(0)
const dialogVisible = ref(false)
const allInstances = ref<{ id: string; name: string }[]>([])
const formInst = ref('')
const formIoa = ref<number | null>(null)
const formAlias = ref('')
const formPoints = ref<PointSnapshot[]>([])
let pollTimer: ReturnType<typeof setInterval> | null = null

const visibleTraces = computed(() => traces.value.filter((_, i) => !hiddenTraces.value.has(i)))

// SVG helpers
function yLabel(ratio: number): string {
  const all: number[] = []
  visibleTraces.value.forEach(t => {
    const sl = sliceData(t)
    all.push(...sl)
  })
  if (all.length === 0) return '0'
  const min = Math.min(...all)
  const max = Math.max(...all)
  const range = max - min || 1
  return ((min + ratio * range)).toFixed(1)
}

function xLabel(ratio: number): string {
  const d = new Date(Date.now() - timeRange.value * 60 * 1000 * (1 - ratio))
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

function sliceData(t: Trace): number[] {
  const maxPts = Math.floor(timeRange.value * 60 / 5)
  return t.data.slice(-maxPts)
}

function linePoints(t: Trace, vi: number): string {
  const sl = sliceData(t)
  if (sl.length < 2) return ''
  const all: number[] = []
  visibleTraces.value.forEach(t => { all.push(...sliceData(t)) })
  if (all.length === 0) return ''
  const min = Math.min(...all)
  const max = Math.max(...all)
  const range = max - min || 1
  const step = chartW / (sl.length - 1)
  return sl.map((v, i) => {
    const x = padL + i * step
    const y = padT + chartH - ((v - min) / range) * chartH
    return `${x},${y}`
  }).join(' ')
}

function lastValue(t: Trace): string {
  return t.data.length > 0 ? t.data[t.data.length - 1].toFixed(1) : '--'
}

function toggleTrace(i: number) {
  const s = new Set(hiddenTraces.value)
  if (s.has(i)) s.delete(i)
  else s.add(i)
  hiddenTraces.value = s
}

function removeTrace(i: number) {
  traces.value.splice(i, 1)
  const s = new Set(hiddenTraces.value)
  s.clear()
  hiddenTraces.value = s
}

// Data fetching
async function fetchAllPoints() {
  if (traces.value.length === 0) return
  for (const t of traces.value) {
    try {
      const pt = await readPoint(t.instId, t.ioa)
      const val = pt.value ?? 0
      t.data.push(val)
      if (t.data.length > 720) t.data.shift()
    } catch {
      // Skip failed reads silently
    }
  }
  lastUpdate.value = new Date().toLocaleTimeString()
  const maxPts = Math.max(...traces.value.map(t => t.data.length), 0)
  pointsCount.value = Math.min(maxPts, Math.floor(timeRange.value * 60 / 5))
}

// Dialog
async function openDialog() {
  try {
    const list = await listInstances()
    allInstances.value = list.map(s => ({ id: s.id, name: s.name }))
  } catch {}
  formInst.value = ''
  formIoa.value = null
  formAlias.value = ''
  formPoints.value = []
  dialogVisible.value = true
}

async function onInstChange() {
  formIoa.value = null
  formPoints.value = []
  if (!formInst.value) return
  try {
    const res = await getPoints(formInst.value)
    formPoints.value = res.points.filter(p => p.point_type !== 'AO' && p.point_type !== 'DO')
  } catch {}
}

function addTrace() {
  if (!formInst.value || formIoa.value === null) {
    ElMessage.warning('请选择实例和测点')
    return
  }
  const inst = allInstances.value.find(i => i.id === formInst.value)
  const pt = formPoints.value.find(p => p.ioa === formIoa.value)
  if (!inst || !pt) return

  traces.value.push({
    instId: formInst.value,
    inst: inst.name,
    ioa: formIoa.value,
    name: pt.name,
    unit: pt.unit || '',
    alias: formAlias.value,
    colorIdx: traces.value.length,
    data: [],
  })
  dialogVisible.value = false
}

function saveToLocal() {
  const save = traces.value.map(t => ({
    instId: t.instId, ioa: t.ioa, alias: t.alias, colorIdx: t.colorIdx,
    inst: t.inst, name: t.name, unit: t.unit,
  }))
  localStorage.setItem('trend_traces', JSON.stringify(save))
}

function loadFromLocal() {
  try {
    const raw = localStorage.getItem('trend_traces')
    if (!raw) return
    const saved = JSON.parse(raw)
    if (!Array.isArray(saved)) return
    saved.forEach((s: any) => {
      traces.value.push({
        instId: s.instId, inst: s.inst, ioa: s.ioa,
        name: s.name || '', unit: s.unit || '',
        alias: s.alias || '', colorIdx: s.colorIdx || 0,
        data: [],
      })
    })
  } catch {}
}

onMounted(() => {
  loadFromLocal()
  if (traces.value.length > 0) {
    fetchAllPoints()
  }
  pollTimer = setInterval(fetchAllPoints, 5000)
})

onUnmounted(() => {
  saveToLocal()
  if (pollTimer) clearInterval(pollTimer)
})
</script>
