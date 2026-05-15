<template>
  <div class="login-wrapper">
    <div class="login-card">
      <div class="login-header">
        <div class="login-logo">⚡</div>
        <h2>IEC104 模拟器</h2>
        <p style="color: var(--el-text-color-secondary); font-size: 13px; margin-top: 4px">请登录</p>
      </div>
      <el-form :model="form" :rules="rules" ref="formRef" @keydown.enter="handleLogin">
        <el-form-item prop="username">
          <el-input v-model="form.username" placeholder="用户名" size="large" :prefix-icon="User" />
        </el-form-item>
        <el-form-item prop="password">
          <el-input v-model="form.password" type="password" placeholder="密码" size="large" show-password :prefix-icon="Lock" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" size="large" style="width: 100%" :loading="loading" @click="handleLogin">
            登 录
          </el-button>
        </el-form-item>
      </el-form>
      <div style="text-align: center; font-size: 12px; color: var(--el-text-color-placeholder); margin-top: 8px">
        默认账号: admin / Test1234
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { User, Lock } from '@element-plus/icons-vue'
import type { FormInstance } from 'element-plus'

const router = useRouter()
const formRef = ref<FormInstance>()
const loading = ref(false)
const form = reactive({ username: '', password: '' })
const rules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
}

async function handleLogin() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  loading.value = true
  try {
    const res = await fetch('/api/v1/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(form),
    })
    const data = await res.json()
    if (!res.ok) {
      ElMessage.error(data.error || '登录失败')
      return
    }
    localStorage.setItem('token', data.token)
    localStorage.setItem('user', JSON.stringify(data.user))
    ElMessage.success('登录成功')
    router.push('/config')
  } catch (e: any) {
    ElMessage.error('登录失败: ' + (e.message || '网络错误'))
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-wrapper {
  display: flex; align-items: center; justify-content: center;
  min-height: 100vh; background: #0a0e17;
}
.login-card {
  width: 380px; padding: 40px 32px 32px;
  background: #111827; border-radius: 8px;
  border: 1px solid #1e293b;
}
.login-header { text-align: center; margin-bottom: 28px; }
.login-logo { font-size: 48px; margin-bottom: 12px; }
.login-header h2 { margin: 0; color: #e2e8f0; font-size: 20px; font-weight: 600; }
</style>
