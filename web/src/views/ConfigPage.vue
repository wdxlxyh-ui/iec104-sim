<template>
  <div>
    <el-card shadow="never" style="margin-bottom: 16px">
      <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 8px">
        <div style="display: flex; align-items: center; gap: 12px">
          <span style="font-size: 16px; font-weight: 600">实例配置</span>
          <span v-if="globalStatus" style="font-size: 13px; color: var(--el-text-color-secondary)">
            已配置 {{ globalStatus.configured }} / {{ globalStatus.max }} |
            运行中 <span style="color: #67c23a">{{ globalStatus.running }}</span> |
            已停止 <span style="color: #909399">{{ globalStatus.stopped }}</span>
          </span>
        </div>
        <div>
          <el-button @click="fetchData" :icon="Refresh" circle aria-label="刷新数据" />
          <el-button type="primary" @click="showAddDialog = true">添加实例</el-button>
        </div>
      </div>
    </el-card>

    <el-card shadow="never" v-loading="loading">
      <el-empty v-if="instances.length === 0 && !loading" description="暂无实例，点击上方按钮添加" />
      <el-table v-else :data="instances" stripe style="width: 100%">
        <el-table-column prop="id" label="ID" width="90" />
        <el-table-column prop="name" label="名称" min-width="140" />
        <el-table-column label="规约" width="120">
          <template #default="{ row }">
            <el-tag :type="protoTagType(row.protocol)">{{ protoLabel(row.protocol) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="端口" width="100">
          <template #default="{ row }">
            <el-tag>{{ displayPort(row) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="xlsx_file" label="点表文件" min-width="160" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag v-if="row.status === 'running'" type="success">运行中</el-tag>
            <el-tag v-else-if="row.status === 'error'" type="danger">错误</el-tag>
            <el-tag v-else type="info">已停止</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="统计" min-width="200">
          <template #default="{ row }">
            <span v-if="row.stats" style="font-size: 12px; color: var(--el-text-color-secondary)">
              <el-icon :color="row.stats.client_connected ? '#67c23a' : '#f56c6c'" style="vertical-align: middle; margin-right: 2px">
                <component :is="row.stats.client_connected ? SuccessFilled : CircleCloseFilled" />
              </el-icon>
              {{ row.stats.client_connected ? '在线' : '离线' }} |
              测点: {{ row.stats.total_points }} |
              运行: {{ fmtUptime(row.stats.uptime_seconds) }}
            </span>
            <span v-else style="color: var(--el-text-color-placeholder)">-</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="280" fixed="right">
          <template #default="{ row }">
            <el-button-group>
              <el-button v-if="row.status === 'running'" type="warning" size="small"
                :loading="actionLoading === row.id" @click="handleStop(row.id)">停止</el-button>
              <el-button v-else type="success" size="small"
                :loading="actionLoading === row.id" @click="handleStart(row.id)">启动</el-button>
              <el-button size="small" :disabled="row.status === 'running' || actionLoading === row.id" @click="handleEdit(row)">编辑</el-button>
              <el-button type="danger" size="small" :disabled="row.status === 'running' || actionLoading === row.id" @click="handleDelete(row.id)">删除</el-button>
            </el-button-group>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Add/Edit Dialog -->
    <el-dialog v-model="showAddDialog" :title="editing ? '编辑实例' : '添加实例'" width="560px">
      <el-form :model="form" label-width="110px" :rules="rules" ref="formRef">
        <el-form-item label="规约类型" prop="protocol">
          <el-radio-group v-model="form.protocol">
            <el-radio-button value="iec104">IEC104</el-radio-button>
            <el-radio-button value="modbus_tcp">Modbus TCP</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="实例名称" prop="name">
          <el-input v-model="form.name" placeholder="例如: 变电站A" />
        </el-form-item>
        <el-form-item :label="form.protocol === 'modbus_tcp' ? 'Modbus端口' : 'IEC104端口'" prop="iec104_port">
          <el-input-number v-model="form.iec104_port" :min="1" :max="65535" style="width: 100%" />
        </el-form-item>
        <el-form-item label="点表文件" prop="xlsx_file">
          <el-select v-model="form.xlsx_file" placeholder="选择或上传文件" style="width: 100%" allow-create filterable>
            <el-option v-for="f in availableFiles" :key="f.name" :label="f.name" :value="f.name" />
          </el-select>
          <div class="form-hint" style="font-size:12px;color:#999;margin-top:4px">{{ xlsxHint }}</div>
        </el-form-item>
        <el-form-item v-if="form.protocol === 'modbus_tcp'" label="从站地址">
          <el-input-number v-model="modbusSlaveId" :min="1" :max="247" style="width: 100%" />
        </el-form-item>
        <el-form-item v-if="form.protocol === 'modbus_tcp'" label="字节序">
          <el-select v-model="modbusByteOrder" style="width: 100%">
            <el-option label="ABCD (Big-Endian)" value="ABCD" />
            <el-option label="CDAB (Little-Endian)" value="CDAB" />
            <el-option label="BADC (Byte-Swapped)" value="BADC" />
            <el-option label="DCBA (Word-Swapped)" value="DCBA" />
          </el-select>
        </el-form-item>
        <el-form-item label="HTTP接口">
          <el-switch v-model="form.http_enabled" active-text="启用HTTP修改测点值" />
        </el-form-item>
        <el-form-item label="HTTP端口" v-if="form.http_enabled" prop="http_port">
          <el-input-number v-model="form.http_port" :min="1024" :max="65535" style="width: 100%" />
        </el-form-item>
        <el-form-item label="上传新文件">
          <el-upload :auto-upload="false" :show-file-list="false" accept=".xlsx" :on-change="handleFileChange">
            <el-button type="primary">选择 Excel 文件</el-button>
            <span v-if="selectedFile" style="margin-left: 10px; color: #67c23a">{{ selectedFile.name }}</span>
          </el-upload>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddDialog = false">取消</el-button>
        <el-button v-if="selectedFile" @click="handleUploadFirst" style="margin-right: 8px">先上传文件</el-button>
        <el-button type="primary" @click="handleSave" :loading="saving">{{ editing ? '保存' : '创建' }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { Refresh, SuccessFilled, CircleCloseFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance } from 'element-plus'
import {
  listInstances,
  createInstance,
  updateInstance,
  deleteInstance,
  startInstance,
  stopInstance,
  uploadExcel,
  listFiles,
  getStatus,
  type InstanceConfig,
  type InstanceState,
  type GlobalStatus,
} from '../api'

const loading = ref(false)
const instances = ref<InstanceState[]>([])
const globalStatus = ref<GlobalStatus | null>(null)
const showAddDialog = ref(false)
const editing = ref(false)
const saving = ref(false)
const formRef = ref<FormInstance>()
const availableFiles = ref<{ name: string; size: number; modtime: string }[]>([])
const selectedFile = ref<File | null>(null)
const actionLoading = ref('')

const form = ref<InstanceConfig>({
  name: '',
  iec104_port: 2404,
  xlsx_file: '',
  http_enabled: false,
  http_port: 8081,
  protocol: 'iec104',
})

const modbusSlaveId = ref(1)
const modbusByteOrder = ref('ABCD')

const rules = {
  name: [{ required: true, message: '请输入实例名称', trigger: 'blur' }],
  iec104_port: [{ required: true, message: '请填写端口号', trigger: 'blur' }],
  xlsx_file: [{ required: true, message: '请选择点表文件', trigger: 'change' }],
}

const xlsxHint = computed(() => {
  if (form.value.protocol === 'modbus_tcp') {
    return 'Modbus 格式: 名称 | IOA | 类型 | 类型 | 系数 | 基值 | 别名 | 寄存器地址 | 功能码 | 数据类型 | (额外列自动忽略)'
  }
  return 'IEC104 格式: 名称 | IOA | 数据类型 | 测点类型 | 系数 | 基值 | 别名'
})

async function fetchData() {
  loading.value = true
  try {
    instances.value = await listInstances()
    globalStatus.value = await getStatus()
  } catch (e: any) {
    ElMessage.error('获取实例列表失败: ' + (e?.response?.data?.error || e.message))
  } finally {
    loading.value = false
  }
}

async function fetchFiles() {
  try {
    availableFiles.value = await listFiles()
  } catch {}
}

function handleEdit(row: InstanceState) {
  editing.value = true
  form.value = {
    id: row.id,
    name: row.name,
    iec104_port: row.iec104_port,
    xlsx_file: row.xlsx_file,
    http_enabled: row.http_enabled ?? false,
    http_port: row.http_port ?? 8081,
    protocol: row.protocol || 'iec104',
  }
  modbusSlaveId.value = 1
  modbusByteOrder.value = 'ABCD'
  showAddDialog.value = true
}

async function handleSave() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  saving.value = true
  try {
    const data: InstanceConfig = { ...form.value }
    if (data.protocol === 'modbus_tcp') {
      data.modbus_config = {
        port: data.iec104_port,
        slave_id: modbusSlaveId.value,
        byte_order: modbusByteOrder.value,
      }
    }
    if (editing.value) {
      await updateInstance(data.id!, data)
      ElMessage.success('已更新')
    } else {
      const { id, ...createData } = data
      await createInstance(createData)
      ElMessage.success('已创建')
    }
    showAddDialog.value = false
    resetForm()
    await fetchData()
  } catch (e: any) {
    ElMessage.error((e?.response?.data?.error || e.message))
  } finally {
    saving.value = false
  }
}

async function handleStart(id: string) {
  actionLoading.value = id
  try {
    await startInstance(id)
    ElMessage.success('已启动')
    await fetchData()
  } catch (e: any) {
    ElMessage.error('启动失败: ' + (e?.response?.data?.error || e.message))
  } finally {
    actionLoading.value = ''
  }
}

async function handleStop(id: string) {
  actionLoading.value = id
  try {
    await stopInstance(id)
    ElMessage.success('已停止')
    await fetchData()
  } catch (e: any) {
    ElMessage.error('停止失败: ' + (e?.response?.data?.error || e.message))
  } finally {
    actionLoading.value = ''
  }
}

async function handleDelete(id: string) {
  try {
    await ElMessageBox.confirm('确定删除此实例？', '确认')
    await deleteInstance(id)
    ElMessage.success('已删除')
    await fetchData()
  } catch (e: any) {
    // Ignore cancel dialog, show other errors
    if (e !== 'cancel') {
      ElMessage.error('删除失败: ' + (e?.response?.data?.error || e.message))
    }
  }
}

function handleFileChange(file: any) {
  selectedFile.value = file.raw || file
}

async function handleUploadFirst() {
  if (!selectedFile.value) return
  saving.value = true
  try {
    const filename = await uploadExcel(selectedFile.value)
    ElMessage.success('上传成功: ' + filename)
    form.value.xlsx_file = filename
    selectedFile.value = null
    await fetchFiles()
  } catch (e: any) {
    ElMessage.error('上传失败: ' + (e?.response?.data?.error || e.message))
  } finally {
    saving.value = false
  }
}

function resetForm() {
  editing.value = false
  form.value = { name: '', iec104_port: 2404, xlsx_file: '', http_enabled: false, http_port: 8081, protocol: 'iec104' }
  modbusSlaveId.value = 1
  modbusByteOrder.value = 'ABCD'
  selectedFile.value = null
}

function protoLabel(proto?: string): string {
  if (proto === 'modbus_tcp') return 'Modbus TCP'
  return 'IEC104'
}

function protoTagType(proto?: string): 'success' | 'primary' | 'info' {
  if (proto === 'modbus_tcp') return 'success'
  return 'primary'
}

function displayPort(row: InstanceState): string {
  if (row.protocol === 'modbus_tcp' && row.iec104_port) return String(row.iec104_port)
  return String(row.iec104_port)
}

function fmtUptime(s: number): string {
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  return h > 0 ? `${h}h${m}m` : `${m}m`
}

onMounted(() => {
  fetchData()
  fetchFiles()
})
</script>
