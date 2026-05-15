<template>
  <div>
    <el-card shadow="never" style="margin-bottom: 16px">
      <div style="display: flex; justify-content: space-between; align-items: center">
        <span style="font-size: 16px; font-weight: 600">用户管理</span>
        <el-button type="primary" @click="showAdd = true">+ 添加用户</el-button>
      </div>
    </el-card>

    <el-card shadow="never" v-loading="loading">
      <el-table :data="users" stripe style="width: 100%">
        <el-table-column prop="username" label="用户名" width="160" />
        <el-table-column label="角色" width="100">
          <template #default="{ row }">
            <el-tag :type="row.role === 'admin' ? 'danger' : 'info'" size="small">{{ row.role }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'danger'" size="small">{{ row.enabled ? '启用' : '禁用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="180">
          <template #default="{ row }">{{ row.created_at?.substring(0, 16) || '-' }}</template>
        </el-table-column>
        <el-table-column label="最后登录" width="180">
          <template #default="{ row }">{{ row.last_login?.substring(0, 16) || '-' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="100">
          <template #default="{ row }">
            <el-button type="danger" size="small" :disabled="row.username === 'admin'"
              @click="handleDelete(row.username)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="showAdd" title="添加用户" width="400px">
      <el-form :model="form" label-width="80px" :rules="rules" ref="formRef">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="form.username" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="form.password" type="password" show-password />
        </el-form-item>
        <el-form-item label="角色" prop="role">
          <el-select v-model="form.role" style="width: 100%">
            <el-option label="管理员" value="admin" />
            <el-option label="只读用户" value="viewer" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAdd = false">取消</el-button>
        <el-button type="primary" @click="handleCreate" :loading="saving">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance } from 'element-plus'

const loading = ref(false)
const saving = ref(false)
const users = ref<any[]>([])
const showAdd = ref(false)
const formRef = ref<FormInstance>()
const form = ref({ username: '', password: '', role: 'viewer' })
const rules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
}

function getToken() { return localStorage.getItem('token') || '' }

async function fetchUsers() {
  loading.value = true
  try {
    const res = await fetch('/api/v1/users', { headers: { Authorization: 'Bearer ' + getToken() } })
    const data = await res.json()
    if (res.ok) users.value = data.users || []
    else ElMessage.error(data.error || '获取失败')
  } catch { ElMessage.error('获取用户列表失败') }
  finally { loading.value = false }
}

async function handleCreate() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  saving.value = true
  try {
    const res = await fetch('/api/v1/users', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + getToken() },
      body: JSON.stringify(form.value),
    })
    const data = await res.json()
    if (res.ok) {
      ElMessage.success('用户已创建')
      showAdd.value = false
      form.value = { username: '', password: '', role: 'viewer' }
      await fetchUsers()
    } else ElMessage.error(data.error || '创建失败')
  } catch { ElMessage.error('创建失败') }
  finally { saving.value = false }
}

async function handleDelete(username: string) {
  try {
    await ElMessageBox.confirm('确定删除用户 ' + username + '？', '确认')
    const res = await fetch('/api/v1/users/' + username, {
      method: 'DELETE',
      headers: { Authorization: 'Bearer ' + getToken() },
    })
    const data = await res.json()
    if (res.ok) { ElMessage.success('已删除'); await fetchUsers() }
    else ElMessage.error(data.error || '删除失败')
  } catch {}
}

onMounted(fetchUsers)
</script>
