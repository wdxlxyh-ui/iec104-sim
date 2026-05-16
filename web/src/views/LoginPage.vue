<template>
  <div class="login-wrapper">
    <div class="login-card">
      <div class="login-header">
        <div class="login-logo"></div>
        <h2>IEC104 模拟器管理</h2>
        <p class="login-subtitle">请登录以继续</p>
      </div>
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        class="login-form"
        @keyup.enter="handleLogin"
      >
        <el-form-item prop="username">
          <el-input
            v-model="form.username"
            placeholder="用户名"
            :prefix-icon="User"
            size="large"
            autocomplete="username"
          />
        </el-form-item>
        <el-form-item prop="password">
          <el-input
            v-model="form.password"
            type="password"
            placeholder="密码"
            :prefix-icon="Lock"
            size="large"
            show-password
            autocomplete="current-password"
          />
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
            size="large"
            :loading="loading"
            style="width: 100%"
            @click="handleLogin"
          >
            {{ loading ? '登录中...' : '登 录' }}
          </el-button>
        </el-form-item>
        <div v-if="errorMsg" class="login-error">{{ errorMsg }}</div>
      </el-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { User, Lock } from '@element-plus/icons-vue'
import { login, setToken } from '../api'
import type { FormInstance, FormRules } from 'element-plus'

const router = useRouter()
const formRef = ref<FormInstance>()
const loading = ref(false)
const errorMsg = ref('')

const form = reactive({
  username: '',
  password: '',
})

const rules: FormRules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
}

async function handleLogin() {
  if (!formRef.value) return
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  loading.value = true
  errorMsg.value = ''

  try {
    const res = await login(form.username, form.password)
    setToken(res.token)
    router.push('/config')
  } catch (err: any) {
    errorMsg.value = err?.response?.data?.error || '登录失败，请检查用户名和密码'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-wrapper {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100vh;
  background: #0a0e17;
}

.login-card {
  width: 380px;
  padding: 40px 32px 32px;
  background: #111827;
  border: 1px solid #1e293b;
  border-radius: 12px;
  box-shadow: 0 8px 30px rgba(0, 0, 0, 0.5);
}

.login-header {
  text-align: center;
  margin-bottom: 32px;
}

.login-logo {
  width: 48px;
  height: 48px;
  margin: 0 auto 16px;
  background: #3b82f6;
  border-radius: 50%;
  box-shadow: 0 0 20px rgba(59, 130, 246, 0.2);
}

.login-header h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: #e2e8f0;
}

.login-subtitle {
  margin: 8px 0 0;
  font-size: 14px;
  color: #64748b;
}

.login-form {
  margin-top: 8px;
}

.login-form :deep(.el-input__wrapper) {
  background: #0a0e17;
  box-shadow: 0 0 0 1px #1e293b inset;
  border-radius: 8px;
  transition: box-shadow 0.2s;
}

.login-form :deep(.el-input__wrapper.is-focus) {
  box-shadow: 0 0 0 1px #3b82f6 inset;
}

.login-form :deep(.el-input__inner) {
  color: #e2e8f0;
  height: 42px;
}

.login-form :deep(.el-input__inner::placeholder) {
  color: #64748b;
}

.login-form :deep(.el-input__prefix-inner) {
  color: #64748b;
}

.login-form :deep(.el-button--primary) {
  height: 42px;
  font-size: 15px;
  letter-spacing: 2px;
  background: #3b82f6;
  border: none;
  border-radius: 8px;
  transition: background 0.2s, box-shadow 0.2s;
}

.login-form :deep(.el-button--primary:hover) {
  background: #2563eb;
  box-shadow: 0 0 16px rgba(59, 130, 246, 0.3);
}

.login-error {
  text-align: center;
  color: #ef4444;
  font-size: 13px;
  padding: 8px;
  background: rgba(239, 68, 68, 0.1);
  border-radius: 6px;
}
</style>
