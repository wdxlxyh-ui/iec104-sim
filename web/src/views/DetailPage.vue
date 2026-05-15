<template>
  <div>
    <el-card shadow="never" style="margin-bottom: 16px">
      <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 8px">
        <div style="display: flex; align-items: center; gap: 12px">
          <el-button text @click="goBack" style="font-size: 14px">← 返回</el-button>
          <span style="font-size: 16px; font-weight: 600">实例详情 — {{ instanceName }}</span>
          <el-tag v-if="instanceStatus === 'running'" type="success" size="small">运行中</el-tag>
          <el-tag v-else type="info" size="small">已停止</el-tag>
        </div>
        <div style="display: flex; align-items: center; gap: 8px">
          <el-switch
            v-model="pollingEnabled"
            size="small"
            active-text="刷新"
            inactive-text="停止"
            @change="togglePolling"
          />
          <template v-if="pollingEnabled">
            <span style="font-size: 13px; color: #666">频率:</span>
            <el-select v-model="refreshRate" size="small" style="width: 110px" @change="restartPolling">
              <el-option label="100ms" :value="100" />
              <el-option label="200ms" :value="200" />
              <el-option label="500ms" :value="500" />
              <el-option label="1000ms" :value="1000" />
            </el-select>
          </template>
          <span style="font-size: 12px; color: #999">{{ points.length }} 个测点</span>
        </div>
      </div>
    </el-card>

    <template v-if="instanceStatus === 'running'">
      <el-card shadow="never" style="margin-bottom: 16px">
        <div class="toolbar">
          <span style="font-size: 13px; color: #666; font-weight: 500">批量操作：</span>
          <el-button size="small" @click="openBatchModal">批量配置</el-button>
          <el-divider direction="vertical" />
          <el-button size="small" @click="exportAutoConfig">导出配置</el-button>
          <el-button size="small" @click="triggerImport">导入配置</el-button>
          <input ref="importInputRef" type="file" accept=".csv" style="display: none" @change="importAutoConfig" />
          <el-divider direction="vertical" />
          <el-button size="small" type="primary" @click="exportCSVData">导出 CSV 数据</el-button>
          <span style="font-size: 12px; color: #999; margin-left: auto" id="selectedCount">已选 {{ Object.keys(selectedIoas).length }} 个测点</span>
        </div>
      </el-card>

      <el-card shadow="never">
        <el-table :data="points" style="width: 100%" size="small" @selection-change="onSelectionChange" max-height="calc(100vh - 280px)">
          <el-table-column type="selection" width="40" />
          <el-table-column prop="ioa" label="信息体地址" width="90" />
          <el-table-column prop="name" label="测点名称" min-width="120" />
          <el-table-column label="测点类型" width="90">
            <template #default="{ row }">
              <el-tag :type="tagType(row.point_type)" size="small" effect="plain">
                {{ typeLabel(row.point_type) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="实时值" width="110">
            <template #default="{ row }">
              <span style="font-weight: 600; font-size: 14px">
                {{ displayValue(row) }}
              </span>
            </template>
          </el-table-column>
<el-table-column label="测点值更新时间" width="165">
             <template #default="{ row }">
               <span style="font-size: 12px; color: #666">
                 {{ formatTime(row.updated_at) }}
               </span>
             </template>
           </el-table-column>
          <el-table-column label="置数" width="150">
            <template #default="{ row }">
              <template v-if="row.point_type === 'AO' || row.point_type === 'DO'">
                <span style="color: #c0c4cc; font-size: 12px">—</span>
              </template>
              <template v-else-if="row.point_type === 'DI'">
                <el-switch
                   :model-value="!!setValues[row.ioa]"
                   :key="row.ioa"
                   @change="(val: boolean) => doSetValue(row, val ? 1 : 0)"
                   size="small"
                   active-text="ON"
                   inactive-text="OFF"
                 />
              </template>
              <template v-else>
                <div style="display: flex; gap: 4px">
<el-input-number
                     :model-value="setValues[row.ioa] ?? null"
                     size="small"
                     :step="row.point_type === 'PI' ? 1 : 0.1"
                     :controls="false"
                     style="width: 80px"
                     @update:model-value="(v: number | null) => { setValues[row.ioa] = (v ?? '') as string | number }"
                     @keydown.enter.prevent="() => doSetValue(row)"
                   />
                  <el-button size="small" type="primary" @click="doSetValue(row, undefined)">置数</el-button>
                </div>
              </template>
            </template>
          </el-table-column>
<el-table-column label="自动变化" width="120">
             <template #default="{ row }">
               <template v-if="row.point_type === 'AO' || row.point_type === 'DO'">
                 <span style="color: #c0c4cc; font-size: 12px">—</span>
               </template>
               <template v-else-if="row.point_type === 'DI'">
                 <span style="color: #c0c4cc; font-size: 12px">—</span>
               </template>
               <template v-else>
                 <el-button size="small" :type="autoStrategies[row.ioa] ? 'success' : 'default'" @click="openAutoModal(row)">
                   {{ autoStrategyLabel(row.ioa) }}
                 </el-button>
               </template>
             </template>
           </el-table-column>
        </el-table>

        <div style="font-size: 12px; color: #999; display: flex; gap: 16px; flex-wrap: wrap; margin-top: 12px; padding: 8px 0">
          <span><el-tag type="primary" size="small" effect="plain">AI</el-tag> 遥测 — 置数/自动变化均可用</span>
          <span><el-tag type="success" size="small" effect="plain">DI</el-tag> 遥信 — ON/OFF 开关置数</span>
          <span><el-tag type="warning" size="small" effect="plain">PI</el-tag> 遥脉 — 自动变化可用</span>
          <span><el-tag type="danger" size="small" effect="plain">AO/DO</el-tag> 置数和自动变化均不可用</span>
        </div>
      </el-card>
    </template>

    <template v-else>
      <el-card shadow="never">
        <el-empty description="实例未运行，请先在监控页面启动实例" />
      </el-card>
    </template>

    <!-- Auto-Change Config Dialog -->
    <el-dialog v-model="autoDialogVisible" title="自动变化配置" width="700px" :close-on-click-modal="false">
      <div style="margin-bottom: 12px; font-weight: 600; font-size: 14px">
        {{ autoDialogPointName }} (IOA: {{ autoDialogIOA }})
      </div>
      <el-tabs v-model="autoStrategyTab" type="card">
        <el-tab-pane label="递增" name="increment">
          <el-form label-width="100px" size="small">
            <el-form-item label="起始值">
              <el-input-number v-model="autoForm.start_value" :min="0" :step="1" style="width: 200px" />
            </el-form-item>
            <el-form-item label="步长">
              <el-input-number v-model="autoForm.step" :min="0.1" :step="0.1" style="width: 200px" />
            </el-form-item>
            <el-form-item label="变化周期(ms)">
              <el-input-number v-model="autoForm.period_ms" :min="100" :step="100" style="width: 200px" />
            </el-form-item>
            <el-form-item label="最大值">
              <el-input-number v-model="autoForm.max_value" :min="0" :step="1" style="width: 200px" />
            </el-form-item>
          </el-form>
          <div style="font-size: 12px; color: #999; margin-top: 8px">每个周期：最新值 = 上次值 + 步长，达最大值后从起始值重新开始</div>
        </el-tab-pane>
        <el-tab-pane label="随机" name="random">
          <el-form label-width="100px" size="small">
            <el-form-item label="变化周期(ms)">
              <el-input-number v-model="autoForm.period_ms" :min="100" :step="100" style="width: 200px" />
            </el-form-item>
            <el-form-item label="最小值">
              <el-input-number v-model="autoForm.min_value" :min="0" :step="1" style="width: 200px" />
            </el-form-item>
            <el-form-item label="最大值">
              <el-input-number v-model="autoForm.max_value_r" :min="0" :step="1" style="width: 200px" />
            </el-form-item>
            <el-form-item label="小数位数">
              <el-radio-group v-model="autoForm.decimal_places">
                <el-radio :value="0">整数</el-radio>
                <el-radio :value="1">1位小数</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-form>
        </el-tab-pane>
        <el-tab-pane label="CSV" name="csv">
          <el-form label-width="100px" size="small">
            <el-form-item label="CSV文件">
              <div style="display: flex; gap: 8px">
                <el-input v-model="autoForm.csv_file" placeholder="文件名" style="width: 200px" />
                <el-button size="small" @click="triggerCSVUpload">上传</el-button>
                <input ref="csvUploadRef" type="file" accept=".csv" style="display: none" @change="uploadCSVFile" />
              </div>
            </el-form-item>
            <el-form-item label="时间格式">
              <el-radio-group v-model="autoForm.time_format">
                <el-radio value="absolute">绝对时刻 (hh:mm:ss)</el-radio>
                <el-radio value="relative">相对时刻 (ms)</el-radio>
              </el-radio-group>
            </el-form-item>
            <el-form-item label="时间单位" v-if="autoForm.time_format === 'relative'">
              <el-radio-group v-model="autoForm.time_unit">
                <el-radio value="ms">毫秒</el-radio>
                <el-radio value="s">秒</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-form>
        </el-tab-pane>
        <el-tab-pane label="MAX" name="max">
          <el-form label-width="100px" size="small">
            <el-form-item label="ParaA(IOA列表)">
              <el-input v-model="autoForm.para_a" placeholder="多个IOA用分号隔开，如 16385;16386" />
            </el-form-item>
            <el-form-item label="ParaB(关联IOA)">
              <el-input v-model="autoForm.para_b" placeholder="关联IOA号（可选），为0时结果置0" />
            </el-form-item>
          </el-form>
          <div style="font-size: 12px; color: #999; margin-top: 8px">取 ParaA 中各 IOA 的最大值</div>
        </el-tab-pane>
        <el-tab-pane label="MIN" name="min">
          <el-form label-width="100px" size="small">
            <el-form-item label="ParaA(IOA列表)">
              <el-input v-model="autoForm.para_a" placeholder="多个IOA用分号隔开，如 16385;16386" />
            </el-form-item>
            <el-form-item label="ParaB(关联IOA)">
              <el-input v-model="autoForm.para_b" placeholder="关联IOA号（可选），为0时结果置0" />
            </el-form-item>
          </el-form>
          <div style="font-size: 12px; color: #999; margin-top: 8px">取 ParaA 中各 IOA 的最小值</div>
        </el-tab-pane>
        <el-tab-pane label="SOC计算" name="soc">
          <el-form label-width="120px" size="small">
            <el-form-item label="初始SOC(%)">
              <el-input-number v-model="autoForm.init_soc" :min="0" :max="100" :step="1" style="width: 200px" />
            </el-form-item>
            <el-form-item label="额定容量(kWh)">
              <el-input-number v-model="autoForm.rated_cap" :min="1" :step="1" style="width: 200px" />
            </el-form-item>
            <el-form-item label="功率AI点号">
              <el-input-number v-model="autoForm.power_ioa" :min="1" :step="1" style="width: 200px" />
            </el-form-item>
            <el-form-item label="积分周期(ms)">
              <el-input-number v-model="autoForm.integral_ms" :min="100" :step="100" style="width: 200px" />
            </el-form-item>
          </el-form>
        </el-tab-pane>
        <el-tab-pane label="电量计算" name="energy">
          <el-form label-width="120px" size="small">
            <el-form-item label="初始电量(kWh)">
              <el-input-number v-model="autoForm.init_energy" :min="0" :step="1" style="width: 200px" />
            </el-form-item>
            <el-form-item label="统计类别">
              <el-radio-group v-model="autoForm.stat_type">
                <el-radio :value="0">充电量</el-radio>
                <el-radio :value="1">放电量</el-radio>
              </el-radio-group>
            </el-form-item>
            <el-form-item label="功率AI点号">
              <el-input-number v-model="autoForm.energy_power_ioa" :min="1" :step="1" style="width: 200px" />
            </el-form-item>
            <el-form-item label="积分周期(ms)">
              <el-input-number v-model="autoForm.energy_period_ms" :min="100" :step="100" style="width: 200px" />
            </el-form-item>
          </el-form>
        </el-tab-pane>
        <el-tab-pane label="AO关联" name="aofollow">
          <el-form label-width="100px" size="small">
            <el-form-item label="关联AO点号">
              <el-input-number v-model="autoForm.follow_ao_ioa" :min="1" :step="1" style="width: 200px" />
            </el-form-item>
          </el-form>
          <div style="font-size: 12px; color: #999; margin-top: 8px">本 AI/DI/PI 点将跟随指定 AO 点的控制值变化</div>
        </el-tab-pane>
        <el-tab-pane label="接口更新" name="apiupdate">
          <el-form label-width="100px" size="small">
            <el-form-item label="初始值">
              <el-input-number v-model="autoForm.api_init_value" :min="0" :step="1" style="width: 200px" />
            </el-form-item>
          </el-form>
          <div style="font-size: 12px; color: #999; margin-top: 8px">此模式仅能通过 HTTP API 更新值，其他方式拒绝写入</div>
        </el-tab-pane>
<el-tab-pane label="手动置数" name="manual">
           <div style="padding: 20px; color: #999; font-size: 13px">
             此模式下引擎不自动计算值<br/>
             需通过 HTTP API 手动置数（PUT /api/v1/instances/{id}/points/{ioa}）<br/>
             适用于外部系统联调场景
           </div>
         </el-tab-pane>
         <el-tab-pane label="自定义公式" name="custom">
           <el-form label-width="100px" size="small">
             <el-form-item label="关联测点">
               <div style="display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 8px">
                 <el-tag
                   v-for="ioa in customSelectedIoas"
                   :key="ioa"
                   closable
                   :type="customTagType(ioa)"
                   @close="removeCustomIoa(ioa)"
                 >
                   {{ customIoaLabel(ioa) }}
                 </el-tag>
               </div>
               <div style="display: flex; gap: 8px">
                 <el-select
                   v-model="customPointSearch"
                   filterable
                   remote
                   reserve-keyword
                   placeholder="搜索测点名称或IOA"
                   :remote-method="queryCustomPoints"
                   :loading="customPointLoading"
                   style="width: 300px"
                   @change="addCustomIoa"
                 >
                   <el-option
                     v-for="pt in customPointOptions"
                     :key="pt.ioa"
                     :label="pt.name + ' (IOA: ' + pt.ioa + ')'"
                     :value="pt.ioa"
                   />
                 </el-select>
                 <el-button size="small" @click="clearCustomIoas">清空</el-button>
               </div>
               <div style="font-size: 12px; color: #999; margin-top: 4px">
                 已选 {{ customSelectedIoas.length }} 个测点（最少2个，最多50个）
               </div>
             </el-form-item>
             <el-form-item label="公式编辑">
               <div style="border: 1px solid #dcdfe6; border-radius: 4px; padding: 8px; min-height: 80px">
                 <div style="margin-bottom: 8px; min-height: 28px">
                   <el-tag v-for="(token, idx) in customFormulaTokens" :key="idx" size="small" style="margin-right: 4px">
                     {{ token }}
                   </el-tag>
                 </div>
                 <div style="display: flex; flex-wrap: wrap; gap: 4px">
                   <el-button v-for="op in formulaOperators" :key="op" size="small" @click="appendFormulaToken(op)">
                     {{ op }}
                   </el-button>
                   <el-button size="small" @click="appendFormulaToken('(')">(</el-button>
                   <el-button size="small" @click="appendFormulaToken(')')">)</el-button>
                   <el-button size="small" type="danger" plain @click="removeFormulaToken">退格</el-button>
                   <el-button size="small" type="warning" plain @click="clearFormulaTokens">清空</el-button>
                 </div>
                 <div style="margin-top: 8px; display: flex; flex-wrap: wrap; gap: 4px">
                   <el-button
                     v-for="pt in customSelectedPoints"
                     :key="pt.ioa"
                     size="small"
                     @click="appendFormulaToken('{' + getCustomIoaIndex(pt.ioa) + '}')"
                   >
                     {{ pt.name }}({{ pt.ioa }})
                   </el-button>
                 </div>
               </div>
               <div style="font-size: 12px; color: #999; margin-top: 4px">
                 点击参数按钮插入如 {0}、{1} 等占位符，再点击运算符和括号构造公式
               </div>
             </el-form-item>
             <el-form-item label="计算周期(ms)">
               <el-input-number v-model="customForm.period_ms" :min="100" :step="100" style="width: 200px" />
             </el-form-item>
             <el-form-item label="公式预览">
               <el-input :model-value="customFormulaPreview" disabled type="textarea" :rows="2" />
             </el-form-item>
           </el-form>
         </el-tab-pane>
      </el-tabs>
<template #footer>
         <div style="display: flex; justify-content: space-between; align-items: center">
           <span v-if="batchMode" style="font-size: 12px; color: #909399">已选 {{ Object.keys(selectedIoas).length }} 个测点，策略将应用到所有选中测点</span>
           <span v-else />
           <div style="display: flex; gap: 8px">
             <el-button @click="autoDialogVisible = false">取消</el-button>
             <el-button type="primary" @click="confirmAutoChange">确认启用</el-button>
           </div>
         </div>
       </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  getPoints, setPointValue, setAutoChange, batchAutoChange,
  exportAutoConfig as fetchExport, importAutoConfig as fetchImport,
  exportPointsCSV, uploadCSV, getInstance,
  type PointSnapshot, type InstanceState,
} from '../api'

const route = useRoute()
const router = useRouter()
const instanceId = computed(() => route.params.id as string)

const instanceName = ref('')
const instanceStatus = ref('')
const points = ref<PointSnapshot[]>([])
const refreshRate = ref(200)
const pollingEnabled = ref(true)
const setValues = reactive<Record<number, string | number>>({})
const autoStrategies = reactive<Record<number, string>>({})
const selectedIoas = reactive<Record<number, boolean>>({})
const importInputRef = ref<HTMLInputElement>()
const csvUploadRef = ref<HTMLInputElement>()

let pollTimer: ReturnType<typeof setInterval> | null = null

// Auto-change dialog
const autoDialogVisible = ref(false)
const autoDialogIOA = ref(0)
const autoDialogPointName = ref('')
const autoStrategyTab = ref('increment')
const batchMode = ref(false)

const autoForm = reactive({
  start_value: 0,
  step: 1,
  period_ms: 1000,
  max_value: 100,
  min_value: 0,
  max_value_r: 100,
  decimal_places: 0,
  csv_file: '',
  time_format: 'absolute',
  time_unit: 'ms',
  para_a: '',
  para_b: '',
  init_soc: 50,
  rated_cap: 100,
  power_ioa: 16385,
  integral_ms: 1000,
  init_energy: 0,
  stat_type: 0,
  energy_power_ioa: 16385,
  energy_period_ms: 1000,
  follow_ao_ioa: 20,
  api_init_value: 0,
})

// ---- 自定义公式相关状态 ----
const customSelectedIoas = ref<number[]>([])
const customPointSearch = ref('')
const customPointLoading = ref(false)
const customPointOptions = ref<PointSnapshot[]>([])
const customFormulaTokens = ref<string[]>([])
const formulaOperators = ['+', '-', '*', '/']

const customForm = reactive({
  period_ms: 1000,
})

const customFormulaPreview = computed(() => {
  return customFormulaTokens.value.join(' ')
})

const customSelectedPoints = computed(() => {
  return customSelectedIoas.value.map(ioa => points.value.find(p => p.ioa === ioa)).filter(Boolean) as PointSnapshot[]
})

function customTagType(ioa: number): string {
  const pt = points.value.find(p => p.ioa === ioa)
  if (!pt) return ''
  switch (pt.point_type) {
    case 'AI': return 'primary'
    case 'DI': return 'success'
    case 'PI': return 'warning'
    default: return 'info'
  }
}

function customIoaLabel(ioa: number): string {
  const pt = points.value.find(p => p.ioa === ioa)
  return pt ? `${pt.name} (${ioa})` : String(ioa)
}

function getCustomIoaIndex(ioa: number): number {
  return customSelectedIoas.value.indexOf(ioa)
}

function queryCustomPoints(query: string) {
  if (!query) {
    customPointOptions.value = []
    return
  }
  customPointLoading.value = true
  // 延迟搜索，避免频繁触发
  setTimeout(() => {
    const q = query.toLowerCase()
    customPointOptions.value = points.value.filter(p =>
      p.name.toLowerCase().includes(q) || String(p.ioa).includes(q)
    )
    customPointLoading.value = false
  }, 200)
}

function addCustomIoa(ioa: number) {
  if (customSelectedIoas.value.length >= 50) {
    ElMessage.warning('最多只能关联 50 个测点')
    return
  }
  if (!customSelectedIoas.value.includes(ioa)) {
    customSelectedIoas.value.push(ioa)
  }
  customPointSearch.value = ''
  customPointOptions.value = []
}

function removeCustomIoa(ioa: number) {
  const idx = customSelectedIoas.value.indexOf(ioa)
  if (idx >= 0) {
    customSelectedIoas.value.splice(idx, 1)
  }
}

function clearCustomIoas() {
  customSelectedIoas.value = []
}

function appendFormulaToken(token: string) {
  customFormulaTokens.value.push(token)
}

function removeFormulaToken() {
  customFormulaTokens.value.pop()
}

function clearFormulaTokens() {
  customFormulaTokens.value = []
}

function tagType(pt: string): string {
  switch (pt) {
    case 'AI': return 'primary'
    case 'DI': return 'success'
    case 'PI': return 'warning'
    case 'DO': return 'danger'
    case 'AO': return 'danger'
    default: return 'info'
  }
}

function typeLabel(pt: string): string {
  switch (pt) {
    case 'AI': return 'AI 遥测'
    case 'DI': return 'DI 遥信'
    case 'PI': return 'PI 遥脉'
    case 'DO': return 'DO 遥控'
    case 'AO': return 'AO 遥调'
    default: return pt
  }
}

function formatTime(ts: string): string {
  if (!ts) return ''
  return ts.replace('T', ' ').substring(0, 23)
}

function displayValue(p: PointSnapshot): string {
  if (p.point_type === 'DI') return p.bool_value ? 'ON' : 'OFF'
  if (p.point_type === 'AI') return p.value.toFixed(2)
  if (p.point_type === 'PI') return String(p.int_value)
  if (p.point_type === 'AO' || p.point_type === 'DO') return String(p.value)
  return String(p.value)
}

function autoStrategyLabel(ioa: number): string {
  return autoStrategies[ioa] || '配置'
}

// DI 用 setValues[ioa] 存储用户最近一次点击的值，
// 初始化时设为后端当前值，此后只随用户点击更新，不再被后端轮询覆盖

function goBack() {
  router.push('/monitor')
}

function onSelectionChange(rows: PointSnapshot[]) {
  Object.keys(selectedIoas).forEach(k => delete selectedIoas[+k])
  rows.forEach(r => selectedIoas[r.ioa] = true)
}

async function fetchPoints() {
   try {
     const res = await getPoints(instanceId.value)
     points.value = res.points
     // 仅初始化 AI/PI/DI 的置数输入框初值，AO/DO 无置数 UI 不初始化
     res.points.forEach(p => {
       if (p.ioa in setValues) return
       if (p.point_type === 'AI' || p.point_type === 'PI') {
         setValues[p.ioa] = p.value
       } else if (p.point_type === 'DI') {
         setValues[p.ioa] = p.bool_value ? 1 : 0
       }
     })
   } catch (e: any) {
     // silent on polling errors
   }
 }

function restartPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
  if (instanceStatus.value !== 'running' || !pollingEnabled.value) return
  pollTimer = setInterval(fetchPoints, refreshRate.value)
}

function togglePolling(val: boolean) {
  if (val) {
    restartPolling()
  } else {
    if (pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
  }
}

async function doSetValue(row: PointSnapshot, overrideVal?: number | undefined) {
  const ioa = row.ioa
  let val: number | undefined = overrideVal

  if (val === undefined) {
    const raw = setValues[ioa]
    if (raw !== undefined && raw !== '') {
      val = parseFloat(String(raw))
      if (isNaN(val)) val = undefined
    }
    if (val === undefined) {
      ElMessage.warning('请先输入要置数的值')
      return
    }
  }

  let body: any
  if (row.point_type === 'DI') {
    body = { bool_value: val !== 0 }
  } else if (row.point_type === 'PI') {
    body = { int_value: Math.round(val) }
  } else {
    body = { value: val }
  }

  try {
    await setPointValue(instanceId.value, ioa, body)
    setValues[ioa] = val
    ElMessage.success(`${row.name} 已置数为 ${val}`)
  } catch (e: any) {
    ElMessage.error('置数失败: ' + (e?.response?.data?.error || e.message))
  }
}

async function openAutoModal(row: PointSnapshot) {
  autoDialogIOA.value = row.ioa
  autoDialogPointName.value = row.name
  batchMode.value = false
  autoDialogVisible.value = true

  resetAutoForm()

  try {
    const cfg = await (await import('../api')).getAutoChange(instanceId.value, row.ioa)
    if (cfg) {
      autoStrategyTab.value = cfg.strategy
      Object.assign(autoForm, cfg.params)
      if (cfg.strategy === 'custom') {
        if (cfg.params.custom_ioas) {
          customSelectedIoas.value = cfg.params.custom_ioas.split(';').map(Number)
        }
        if (cfg.params.custom_formula) {
          customFormulaTokens.value = cfg.params.custom_formula.split(' ').filter(t => t !== '')
        }
        if (cfg.params.period_ms) {
          customForm.period_ms = cfg.params.period_ms
        }
      }
    }
  } catch {
    autoStrategyTab.value = 'increment'
  }
}

function openBatchModal() {
  if (Object.keys(selectedIoas).length === 0) {
    ElMessage.warning('请先勾选测点')
    return
  }
  autoDialogIOA.value = 0
  autoDialogPointName.value = '批量配置'
  batchMode.value = true
  resetAutoForm()
  autoStrategyTab.value = 'increment'
  autoDialogVisible.value = true
}

function resetAutoForm() {
   Object.assign(autoForm, {
     start_value: 0, step: 1, period_ms: 1000, max_value: 100,
     min_value: 0, max_value_r: 100, decimal_places: 0,
     csv_file: '', time_format: 'absolute', time_unit: 'ms',
     para_a: '', para_b: '',
     init_soc: 50, rated_cap: 100, power_ioa: 16385, integral_ms: 1000,
     init_energy: 0, stat_type: 0, energy_power_ioa: 16385, energy_period_ms: 1000,
     follow_ao_ioa: 20, api_init_value: 0,
   })
   customSelectedIoas.value = []
   customFormulaTokens.value = []
   customForm.period_ms = 1000
 }

async function confirmAutoChange() {
  const params: any = {}
  const strategy = autoStrategyTab.value

  switch (strategy) {
    case 'increment':
      params.start_value = autoForm.start_value
      params.step = autoForm.step
      params.period_ms = autoForm.period_ms
      params.max_value = autoForm.max_value
      break
    case 'random':
      params.period_ms = autoForm.period_ms
      params.min_value = autoForm.min_value
      params.max_value_r = autoForm.max_value_r
      params.decimal_places = autoForm.decimal_places
      break
    case 'csv':
      params.csv_file = autoForm.csv_file
      params.time_format = autoForm.time_format
      params.time_unit = autoForm.time_unit
      break
    case 'max':
    case 'min':
      params.para_a = autoForm.para_a
      params.para_b = autoForm.para_b
      break
    case 'soc':
      params.init_soc = autoForm.init_soc
      params.rated_cap = autoForm.rated_cap
      params.power_ioa = autoForm.power_ioa
      params.integral_ms = autoForm.integral_ms
      break
    case 'energy':
      params.init_energy = autoForm.init_energy
      params.stat_type = autoForm.stat_type
      params.energy_power_ioa = autoForm.energy_power_ioa
      params.energy_period_ms = autoForm.energy_period_ms
      break
case 'aofollow':
       params.follow_ao_ioa = autoForm.follow_ao_ioa
       break
     case 'apiupdate':
       params.api_init_value = autoForm.api_init_value
       break
     case 'manual':
       break
     case 'custom':
       params.custom_ioas = customSelectedIoas.value.join(';')
       params.custom_formula = customFormulaPreview.value
       params.period_ms = customForm.period_ms
       break
   }

  if (params.period_ms && params.period_ms < 100) {
    ElMessage.error('变化周期不能小于 100ms')
    return
  }

  const config = { strategy, enabled: true, params }

  try {
    if (batchMode.value && Object.keys(selectedIoas).length > 0) {
      const ioas = Object.keys(selectedIoas).map(Number)
      await batchAutoChange(instanceId.value, { ioas, config })
      ElMessage.success(`已批量配置 ${ioas.length} 个测点`)
      ioas.forEach(ioa => autoStrategies[ioa] = strategyLabel(strategy))
    } else if (autoDialogIOA.value) {
      await setAutoChange(instanceId.value, autoDialogIOA.value, config)
      ElMessage.success('自动变化配置已保存')
      autoStrategies[autoDialogIOA.value] = strategyLabel(strategy)
    }
    autoDialogVisible.value = false
  } catch (e: any) {
    ElMessage.error('配置失败: ' + (e?.response?.data?.error || e.message))
  }
}

function strategyLabel(s: string): string {
  const map: Record<string, string> = {
    increment: '递增', random: '随机', csv: 'CSV', max: 'MAX',
    min: 'MIN', soc: 'SOC', energy: '电量', aofollow: 'AO关联', apiupdate: '接口更新', manual: '手动',
  }
  return map[s] || s
}

async function exportAutoConfig() {
  try {
    const blob = await fetchExport(instanceId.value)
    downloadBlob(blob, `auto_changes_${instanceId.value}.csv`)
    ElMessage.success('自动变化配置已导出')
  } catch (e: any) {
    ElMessage.error('导出失败: ' + (e?.response?.data?.error || e.message))
  }
}

function triggerImport() {
  importInputRef.value?.click()
}

async function importAutoConfig(e: Event) {
  const input = e.target as HTMLInputElement
  if (!input.files?.length) return
  try {
    await fetchImport(instanceId.value, input.files[0])
    ElMessage.success(`已导入自动变化配置`)
    input.value = ''
  } catch (e: any) {
    ElMessage.error('导入失败: ' + (e?.response?.data?.error || e.message))
  }
}

async function exportCSVData() {
  try {
    const blob = await exportPointsCSV(instanceId.value)
    downloadBlob(blob, `points_${instanceId.value}.csv`)
    ElMessage.success('CSV 数据已导出')
  } catch (e: any) {
    ElMessage.error('导出失败: ' + (e?.response?.data?.error || e.message))
  }
}

function triggerCSVUpload() {
  csvUploadRef.value?.click()
}

async function uploadCSVFile(e: Event) {
  const input = e.target as HTMLInputElement
  if (!input.files?.length) return
  try {
    const res = await uploadCSV(instanceId.value, input.files[0])
    autoForm.csv_file = res.filename
    ElMessage.success('CSV 上传成功')
    input.value = ''
  } catch (e: any) {
    ElMessage.error('CSV 上传失败: ' + (e?.response?.data?.error || e.message))
  }
}

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

async function loadInstanceState() {
  try {
    const state: InstanceState = await getInstance(instanceId.value)
    instanceName.value = state.name
    instanceStatus.value = state.status
  } catch {
    instanceName.value = instanceId.value
    instanceStatus.value = 'stopped'
  }
}

onMounted(async () => {
  await loadInstanceState()
  if (instanceStatus.value === 'running') {
    await fetchPoints()
    pollingEnabled.value = true
    restartPolling()
  } else {
    pollingEnabled.value = false
  }
})

onUnmounted(() => {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
})
</script>
