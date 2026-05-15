<template>
  <div>
    <!-- Description -->
    <el-card shadow="never" style="margin-bottom: 16px">
      <div style="font-size: 13px; color: #94a3b8; line-height: 1.8">
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

      <div v-if="traces.length === 0" style="text-align: center; padding: 80px 20px; color: #475569">
        <div style="font-size: 48px; margin-bottom: 12px">📊</div>
        <div>点击上方「+ 添加测点」选择要跟踪的数据</div>
      </div>

      <div v-else ref="chartRef" style="width: 100%; position: relative">
        <!-- Tooltip -->
        <div v-show="tooltip.show" :style="tooltip.style" style="position: absolute; z-index: 10; pointer-events: none;
          background: #1a1f2e; border: 1px solid #334155; border-radius: 6px; padding: 10px 14px; font-size: 12px;
          min-width: 140px; box-shadow: 0 8px 24px rgba(0,0,0,0.4);">
          <div style="color: #94a3b8; margin-bottom: 6px; font-size: 11px">{{ tooltip.time }}</div>
          <div v-for="(v, i) in tooltip.vals" :key="i"
            style="display: flex; justify-content: space-between; gap: 16px; padding: 2px 0">
            <span style="color: #94a3b8; white-space: nowrap">
              <span :style="{ display:'inline-block', width:'8px', height:'2px', background:COLORS[i%COLORS.length], marginRight:'6px', verticalAlign:'middle' }"></span>
              {{ v.name }}
            </span>
            <span style="font-family: monospace; font-weight: 600" :style="{ color: COLORS[i % COLORS.length] }">{{ v.val }}</span>
          </div>
        </div>

        <svg :viewBox="`0 0 ${SVG_W} ${SVG_H}`" preserveAspectRatio="xMidYMid meet"
          style="width: 100%; height: auto; display: block"
          @mousemove="onSvgMove" @mouseleave="tooltip.show = false">
          <line v-for="i in 6" :key="'g'+i"
            :x1="padL" :y1="padT + chartH - (i/6)*chartH"
            :x2="padL + chartW" :y2="padT + chartH - (i/6)*chartH"
            stroke="#1e293b" stroke-width="0.5" />
          <text v-for="i in 6" :key="'yl'+i"
            :x="padL - 6" :y="padT + chartH - (i/6)*chartH + 3"
            text-anchor="end" font-size="10" font-family="monospace" fill="#475569"
          >{{ yLabel(i/6) }}</text>
          <line :x1="padL" :y1="padT" :x2="padL" :y2="padT+chartH" stroke="#1e293b" stroke-width="1" />
          <line :x1="padL" :y1="padT+chartH" :x2="padL+chartW" :y2="padT+chartH" stroke="#1e293b" stroke-width="1" />
          <text v-for="i in 5" :key="'xl'+i"
            :x="padL + (i/5)*chartW" :y="padT + chartH + 16"
            text-anchor="middle" font-size="10" font-family="monospace" fill="#475569"
          >{{ xLabel(i/5) }}</text>
          <template v-for="(t, vi) in visibleTraces" :key="'trace'+vi">
            <path :d="areaPath(t)" :fill="`url(#grad-${t.colorIdx % COLORS.length})`" opacity="0.15" />
            <polyline :points="polylinePoints(t)" :stroke="COLORS[t.colorIdx % COLORS.length]"
              fill="none" stroke-width="1.5" stroke-linejoin="round" stroke-linecap="round" />
          </template>
          <rect :x="padL" y="0" :width="chartW" :height="SVG_H" fill="transparent"
            @mousemove="onSvgMove" @mouseleave="tooltip.show = false" />
          <!-- Gradient defs -->
          <defs>
            <linearGradient v-for="(c, i) in COLORS" :key="'lg'+i" :id="'grad-'+i" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" :stop-color="c" stop-opacity="0.3"/>
              <stop offset="100%" :stop-color="c" stop-opacity="0.02"/>
            </linearGradient>
          </defs>
        </svg>

        <!-- SVG gradient defs -->
        <svg style="position:absolute;width:0;height:0">
          <defs>
            <linearGradient v-for="(c, i) in COLORS" :key="'lg'+i" :id="'grad-'+i" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" :stop-color="c" stop-opacity="0.3" />
              <stop offset="100%" :stop-color="c" stop-opacity="0.02" />
            </linearGradient>
          </defs>
        </svg>
        <!-- Legend -->
        <div style="display: flex; flex-wrap: wrap; gap: 12px; margin-top: 16px; padding-top: 16px; border-top: 1px solid #1e293b">
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
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { listInstances, getPoints, readPoint, type PointSnapshot } from '../api'

const COLORS = ['#14b8a6','#f59e0b','#3b82f6','#a855f7','#ec4899','#22d3ee','#f97316','#8b5cf6']

interface Trace { instId:string; inst:string; ioa:number; name:string; unit:string; alias:string; colorIdx:number; data:number[] }

const SVG_W=700, SVG_H=280, padL=50, padT=20, chartW=SVG_W-padL-20, chartH=SVG_H-padT-36

const traces = ref<Trace[]>([])
const hiddenTraces = ref<Set<number>>(new Set())
const timeRange = ref(15)
const lastUpdate = ref('--')
const pointsCount = ref(0)
const dialogVisible = ref(false)
const allInstances = ref<{id:string;name:string}[]>([])
const formInst = ref(''); const formIoa = ref<number|null>(null)
const formAlias = ref(''); const formPoints = ref<PointSnapshot[]>([])
let pollTimer: ReturnType<typeof setInterval>|null = null

// Tooltip state
const tooltip = reactive({ show:false, style:{left:'0px',top:'0px'}, time:'', vals:[] as {name:string;val:string}[] })

const visibleTraces = computed(() => traces.value.filter((_,i)=>!hiddenTraces.value.has(i)))

function chartBounds() {
  const all:number[]=[]; visibleTraces.value.forEach(t=>all.push(...sliceData(t)))
  if(!all.length) return {min:0,max:1,range:1}
  const min=Math.min(...all),max=Math.max(...all),range=max-min||1
  return {min,max,range}
}

function yLabel(r:number):string{const b=chartBounds();return(b.min+r*b.range).toFixed(1)}
function xLabel(r:number):string{const d=new Date(Date.now()-timeRange.value*60*1000*(1-r));return`${String(d.getHours()).padStart(2,'0')}:${String(d.getMinutes()).padStart(2,'0')}`}
function sliceData(t:Trace):number[]{const n=Math.floor(timeRange.value*60/5);return t.data.slice(-n)}

function polylinePoints(t:Trace):string{
  const sl=sliceData(t);if(sl.length<2)return''
  const b=chartBounds(),step=chartW/(sl.length-1)
  return sl.map((v,i)=>{const x=padL+i*step,y=padT+chartH-((v-b.min)/b.range)*chartH;return`${x},${y}`}).join(' ')
}

function areaPath(t:Trace):string{
  const pts=polylinePoints(t);if(!pts)return''
  const firstX=pts.split(',')[0],segs=pts.split(' '),lastX=segs[segs.length-1].split(',')[0]
  return`M${firstX},${padT+chartH} L${pts} L${lastX},${padT+chartH} Z`
}

function onSvgMove(e:MouseEvent){
  const svg=e.currentTarget as SVGElement,rect=svg.getBoundingClientRect()
  const mx=(e.clientX-rect.left)*(SVG_W/rect.width)
  if(mx<padL||mx>padL+chartW){tooltip.show=false;return}
  const maxPts=Math.floor(timeRange.value*60/5),step=chartW/(Math.max(maxPts,1)-1)
  const idx=Math.min(Math.round((mx-padL)/step),maxPts-1)
  if(idx<0){tooltip.show=false;return}
  const ptTime=new Date(Date.now()-(maxPts-1-idx)*(timeRange.value*60*1000/maxPts))
  const vals:{name:string;val:string}[]=[]
  visibleTraces.value.forEach(t=>{const s=sliceData(t);if(idx<s.length)vals.push({name:`${t.inst}·${t.alias||t.name}`,val:s[idx].toFixed(1)})})
  const p=svg.parentElement;if(p){const cr=p.getBoundingClientRect(),rx=e.clientX-cr.left+12,ry=e.clientY-cr.top-10;tooltip.style={left:`${Math.min(rx,cr.width-180)}px`,top:`${ry}px`}}
  tooltip.time=ptTime.toLocaleTimeString();tooltip.vals=vals;tooltip.show=true
}

function lastValue(t:Trace):string{return t.data.length>0?t.data[t.data.length-1].toFixed(1):'--'}
function toggleTrace(i:number){const s=new Set(hiddenTraces.value);if(s.has(i))s.delete(i);else s.add(i);hiddenTraces.value=s}
function removeTrace(i:number){traces.value.splice(i,1);hiddenTraces.value=new Set()}

async function fetchAllPoints(){
  if(!traces.value.length)return
  for(const t of traces.value){try{const pt=await readPoint(t.instId,t.ioa);t.data.push(pt.value??0);if(t.data.length>720)t.data.shift()}catch{}}
  lastUpdate.value=new Date().toLocaleTimeString()
  pointsCount.value=Math.min(Math.max(...traces.value.map(t=>t.data.length),0),Math.floor(timeRange.value*60/5))
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

async function openDialog() {
  try { const list = await listInstances(); allInstances.value = list.map(s => ({ id: s.id, name: s.name })) } catch {}
  formInst.value = ''; formIoa.value = null; formAlias.value = ''; formPoints.value = []; dialogVisible.value = true
}

async function onInstChange() {
  formIoa.value = null; formPoints.value = []
  if (!formInst.value) return
  try { const res = await getPoints(formInst.value); formPoints.value = res.points.filter(p => p.point_type !== 'AO' && p.point_type !== 'DO') } catch {}
}

function addTrace() {
  if (!formInst.value || formIoa.value === null) { ElMessage.warning('请选择实例和测点'); return }
  const inst = allInstances.value.find(i => i.id === formInst.value)
  const pt = formPoints.value.find(p => p.ioa === formIoa.value)
  if (!inst || !pt) return
  traces.value.push({ instId: formInst.value, inst: inst.name, ioa: formIoa.value, name: pt.name, unit: pt.unit || '', alias: formAlias.value, colorIdx: traces.value.length, data: [] })
  dialogVisible.value = false
}

onMounted(() => {
  loadFromLocal()
  if (traces.value.length > 0) { fetchAllPoints() }
  pollTimer = setInterval(fetchAllPoints, 5000)
})

onUnmounted(() => {
  saveToLocal()
  if (pollTimer) clearInterval(pollTimer)
})
</script>
