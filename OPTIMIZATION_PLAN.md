# IEC 104 模拟器优化方案详细开发文档

> 版本: 1.0 | 日期: 2026-05-16 | 作者: Sisyphus

---

## 目录

1. [P0 紧急优化 - HTTP 安全配置](#p0-紧急优化---http-安全配置)
2. [P0 紧急优化 - 文件上传安全](#p0-紧急优化---文件上传安全)
3. [P1 重要优化 - 前端构建优化](#p1-重要优化---前端构建优化)
4. [P1 重要优化 - 认证机制设计](#p1-重要优化---认证机制设计)
5. [P1 重要优化 - MCP 接口认证](#p1-重要优化---mcp-接口认证)
6. [P2 计划优化 - CSV 缓存优化](#p2-计划优化---csv-缓存优化)
7. [P2 计划优化 - 自动变化引擎优化](#p2-计划优化---自动变化引擎优化)
8. [P2 计划优化 - 状态管理引入](#p2-计划优化---状态管理引入)
9. [P3 长期优化 - 测试与监控](#p3-长期优化---测试与监控)
10. [CI/CD 搭建方案](#cicd-搭建方案)

---

## P0 紧急优化 - HTTP 安全配置

### 问题描述

HTTP Server 缺少超时配置，存在以下风险：
- **Slowloris 攻击**: 攻击者发送不完整请求耗尽连接资源
- **内存泄漏**: 长时间未关闭的连接占用内存
- **资源耗尽**: 慢客户端占用服务器资源

### 影响范围

- `cmd/iec104-sim/main.go` 第 95 行（传统模式）
- `cmd/iec104-sim/main.go` 第 158 行（服务模式）

### 修复方案

#### 1. 新增 HTTP Server 工厂函数

```go
// cmd/iec104-sim/main.go

// newHTTPServer 创建配置安全的 HTTP Server
func newHTTPServer(addr string, handler http.Handler) *http.Server {
    return &http.Server{
        Addr:           addr,
        Handler:        handler,
        ReadTimeout:    15 * time.Second,   // 读取请求超时
        WriteTimeout:   15 * time.Second,   // 响应写入超时
        IdleTimeout:    60 * time.Second,   // 空闲连接超时
        MaxHeaderBytes: 1 << 20,             // 最大请求头 1MB
    }
}
```

#### 2. 应用到两个运行模式

```go
// 传统模式 - 约第 95 行
- httpSrv := &http.Server{Addr: httpAddr, Handler: mux}
+ httpSrv := newHTTPServer(httpAddr, mux)

// 服务模式 - 约第 158 行
- httpSrv := &http.Server{Addr: httpAddr, Handler: mux}
+ httpSrv := newHTTPServer(httpAddr, mux)
```

#### 3. 添加请求日志中间件（可选）

```go
// LoggingMiddleware 记录每个请求的处理时间
func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        ww := &statusWriter{ResponseWriter: w, statusCode: 200}
        next.ServeHTTP(ww, r)
        slog.Info("HTTP请求",
            "method", r.Method,
            "path", r.URL.Path,
            "status", ww.statusCode,
            "duration", time.Since(start).String(),
            "client", r.RemoteAddr,
        )
    })
}

type statusWriter struct {
    http.ResponseWriter
    statusCode int
}

func (w *statusWriter) WriteHeader(statusCode int) {
    w.statusCode = statusCode
    w.ResponseWriter.WriteHeader(statusCode)
}
```

### 验收标准

- [ ] HTTP Server 配置 ReadTimeout = 15s
- [ ] HTTP Server 配置 WriteTimeout = 15s
- [ ] HTTP Server 配置 IdleTimeout = 60s
- [ ] 慢客户端连接可被正确超时断开

### 预计工时

0.5 小时

---

## P0 紧急优化 - 文件上传安全

### 问题描述

当前文件上传接口存在以下安全漏洞：
- 无文件类型验证
- 无文件大小限制
- 无 MIME 类型检查
- 无文件名安全过滤（目录遍历攻击）

### 影响范围

- `cmd/iec104-sim/main.go` - `handleUpload` 函数
- `cmd/iec104-sim/main.go` - `handleUploadCSV` 函数

### 修复方案

#### 1. 新增上传配置结构体

```go
// cmd/iec104-sim/main.go

// UploadConfig 上传配置
type UploadConfig struct {
    MaxFileSize     int64            // 最大文件大小（字节）
    AllowedExts     map[string]bool  // 允许的扩展名
    AllowedMIME     []string         // 允许的 MIME 类型
    UploadDir       string           // 上传目录
    BlockExtensions []string         // 禁止的扩展名
}

var DefaultUploadConfig = UploadConfig{
    MaxFileSize: 10 * 1024 * 1024,  // 10MB
    AllowedExts: map[string]bool{
        ".xlsx": true,  // Excel 2007+
        ".xls":  true,  // Excel 97-2003
        ".csv":  true,  // CSV 文件（CSV 回放用）
    },
    AllowedMIME: []string{
        "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
        "application/vnd.ms-excel",
        "text/csv",
    },
    UploadDir:       "./config",
    BlockExtensions: []string{".exe", ".sh", ".php", ".js", ".html", ".sql", ".bat", ".cmd"},
}
```

#### 2. 重写 handleUpload 函数

```go
func (ws *webServer) handleUpload(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        writeError(w, http.StatusMethodNotAllowed, "method not allowed")
        return
    }

    // 1. 限制解析的表单大小
    if err := r.ParseMultipartForm(DefaultUploadConfig.MaxFileSize); err != nil {
        writeError(w, http.StatusBadRequest, "file too large or malformed")
        return
    }

    file, header, err := r.FormFile("file")
    if err != nil {
        writeError(w, http.StatusBadRequest, "no file provided")
        return
    }
    defer file.Close()

    // 2. 验证文件扩展名
    ext := strings.ToLower(filepath.Ext(header.Filename))
    if !DefaultUploadConfig.AllowedExts[ext] {
        writeError(w, http.StatusBadRequest, "file type not allowed: "+ext)
        return
    }

    // 3. 验证 MIME 类型
    contentType := header.Header.Get("Content-Type")
    allowedMIME := false
    for _, m := range DefaultUploadConfig.AllowedMIME {
        if contentType == m {
            allowedMIME = true
            break
        }
    }
    if !allowedMIME {
        writeError(w, http.StatusBadRequest, "invalid file content type")
        return
    }

    // 4. 检查禁止的扩展名
    baseName := strings.ToLower(header.Filename)
    for _, blocked := range DefaultUploadConfig.BlockExtensions {
        if strings.HasSuffix(baseName, blocked) {
            writeError(w, http.StatusBadRequest, "extension not allowed")
            return
        }
    }

    // 5. 生成安全文件名
    safeName := sanitizeFilename(header.Filename)
    dst := filepath.Join(ws.cfgDir, safeName)

    // 6. 检查目标文件是否存在
    if _, err := os.Stat(dst); err == nil {
        writeError(w, http.StatusConflict, "file already exists")
        return
    }

    // 7. 创建并写入文件
    dstFile, err := os.Create(dst)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to create file")
        return
    }
    defer dstFile.Close()

    if _, err := io.Copy(dstFile, file); err != nil {
        os.Remove(dst)
        writeError(w, http.StatusInternalServerError, "failed to save file")
        return
    }

    slog.Info("文件上传成功", "filename", safeName, "size", header.Size, "uploader", r.RemoteAddr)
    writeJSON(w, http.StatusOK, map[string]string{"status": "uploaded", "filename": safeName})
}

// sanitizeFilename 生成安全的文件名
func sanitizeFilename(name string) string {
    name = strings.Map(func(r rune) rune {
        if r < 32 || r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
            return -1
        }
        return r
    }, name)
    name = strings.ReplaceAll(name, "..", "")
    return name
}
```

#### 3. CSV 上传函数也需要同样验证

在 `internal/detail/handler.go` 中的 `HandleUploadCSV` 函数添加相同的验证逻辑。

### 验收标准

- [ ] 只允许上传 .xlsx, .xls, .csv 文件
- [ ] 文件大小限制 10MB
- [ ] 拒绝目录遍历攻击
- [ ] 拒绝伪装扩展名的文件

### 预计工时

1.5 小时

---

## P1 重要优化 - 前端构建优化

### 问题描述

前端构建产物未优化：
- 无代码分割
- 无 vendor 分离
- 无 gzip 压缩
- 单文件过大

### 影响范围

- `web/vite.config.ts`
- `web/package.json`

### 修复方案

#### 1. 安装压缩插件

```bash
cd web
npm install vite-plugin-compression -D
```

#### 2. 优化 vite.config.ts

```typescript
// web/vite.config.ts
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'
import compression from 'vite-plugin-compression'

export default defineConfig({
  plugins: [
    vue(),
    compression({
      algorithm: 'gzip',
      ext: '.gz',
      threshold: 1024,
    }),
  ],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        manualChunks: {
          'vendor-vue': ['vue', 'vue-router'],
          'vendor-element': ['element-plus'],
          'vendor-axios': ['axios'],
        },
        chunkFileNames: 'assets/js/[name]-[hash].js',
        entryFileNames: 'assets/js/[name]-[hash].js',
        assetFileNames: 'assets/[ext]/[name]-[hash].[ext]',
      },
    },
    sourcemap: false,
    minify: 'esbuild',
    chunkSizeWarningLimit: 500,
  },
  optimizeDeps: {
    include: ['vue', 'vue-router', 'element-plus', 'axios'],
  },
})
```

#### 3. 配置 Nginx gzip 传输

```nginx
# /etc/nginx/nginx.conf
gzip on;
gzip_types text/plain text/css application/json application/javascript text/xml application/xml application/xml+rss text/javascript;
gzip_min_length 1000;
gzip_proxied any;
gzip_vary on;
```

### 验收标准

- [ ] 构建产物包含多个 chunk
- [ ] vendor 库独立打包
- [ ] 生成 .gz 压缩文件
- [ ] 单个 JS 文件 < 200KB

### 预计工时

1 小时

---

## P1 重要优化 - 认证机制设计

### 问题描述

Web UI 和 API 无任何认证机制，存在以下风险：
- 任何人都可以修改测点值
- 恶意删除实例配置
- 上传恶意文件

### 影响范围

- `cmd/iec104-sim/main.go` - 所有 API 路由
- Web 前端所有页面

### 修复方案

#### 1. 认证架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                    认证流程                                  │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  客户端                                                     │
│     │                                                      │
│     ▼                                                      │
│  ┌─────────────────────┐                                   │
│  │  登录请求            │  POST /api/v1/auth/login          │
│  │  {username, password}│                                   │
│  └──────────┬──────────┘                                   │
│             │                                              │
│             ▼                                              │
│  ┌─────────────────────┐     ┌────────────────────────────┐ │
│  │  验证凭证           │─────▶│  用户数据库 (配置文件中)   │ │
│  └──────────┬──────────┘     └────────────────────────────┘ │
│             │                                              │
│     ┌───────┴───────┐                                      │
│     ▼               ▼                                      │
│  成功              失败                                      │
│     │               │                                      │
│     ▼               ▼                                      │
│  返回 JWT         返回 401                                  │
│     │                                                      │
│     ▼                                                      │
│  后续请求携带 Authorization: Bearer <token>                  │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

#### 2. 新增用户配置结构体

```go
// internal/model/user.go (新文件)

package model

// User 用户
type User struct {
    ID       string `json:"id"`
    Username string `json:"username"`
    // 存储 bcrypt 哈希后的密码
    PasswordHash string `json:"password_hash"`
    Role        string `json:"role"` // admin, operator, viewer
    CreatedAt int64  `json:"created_at"`
}

// UserConfig 用户配置
type UserConfig struct {
    Users []User `json:"users"`
}
```

#### 3. 新增认证中间件

```go
// pkg/middleware/auth.go (新文件)

package middleware

import (
    "log/slog"
    "net/http"
    "strings"
    "time"

    "github.com/golang-jwt/jwt/v5"
)

type AuthConfig struct {
    Secret     string
    TokenExpiry time.Duration
}

var (
    authConfig = &AuthConfig{
        Secret:     "your-secret-key-change-in-production",
        TokenExpiry: 24 * time.Hour,
    }
)

// AuthMiddleware 认证中间件
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 跳过登录和静态文件
        path := r.URL.Path
        if path == "/api/v1/auth/login" || path == "/" || strings.HasPrefix(path, "/assets") {
            next.ServeHTTP(w, r)
            return
        }

        // 获取 Authorization header
        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            http.Error(w, "missing authorization header", http.StatusUnauthorized)
            return
        }

        // 验证 Bearer token
        parts := strings.SplitN(authHeader, " ", 2)
        if len(parts) != 2 || parts[0] != "Bearer" {
            http.Error(w, "invalid authorization header", http.StatusUnauthorized)
            return
        }

        tokenString := parts[1]
        token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
            return []byte(authConfig.Secret), nil
        })

        if err != nil || !token.Valid {
            slog.Warn("invalid token", "error", err)
            http.Error(w, "invalid token", http.StatusUnauthorized)
            return
        }

        // 提取用户信息并注入到 context
        claims, ok := token.Claims.(jwt.MapClaims)
        if !ok {
            http.Error(w, "invalid token claims", http.StatusUnauthorized)
            return
        }

        ctx := r.Context()
        ctx = context.WithValue(ctx, "user_id", claims["user_id"])
        ctx = context.WithValue(ctx, "username", claims["username"])
        ctx = context.WithValue(ctx, "role", claims["role"])

        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// GenerateToken 生成 JWT token
func GenerateToken(user *model.User) (string, error) {
    claims := jwt.MapClaims{
        "user_id":  user.ID,
        "username": user.Username,
        "role":     user.Role,
        "exp":      time.Now().Add(authConfig.TokenExpiry).Unix(),
        "iat":      time.Now().Unix(),
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(authConfig.Secret))
}
```

#### 4. 添加用户配置文件

```json
// config/users.json
{
  "users": [
    {
      "id": "user-001",
      "username": "admin",
      "password_hash": "$2a$10$abcdefghijklmnopqrstuvwxyz",
      "role": "admin",
      "created_at": 1715846400
    },
    {
      "id": "user-002",
      "username": "operator",
      "password_hash": "$2a$10$zyxwvutsrqponmlkjihgfedcba",
      "role": "operator",
      "created_at": 1715846400
    }
  ]
}
```

#### 5. 添加登录 API

```go
// cmd/iec104-sim/main.go 新增

func (ws *webServer) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        writeError(w, http.StatusMethodNotAllowed, "method not allowed")
        return
    }

    var req struct {
        Username string `json:"username"`
        Password string `json:"password"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid JSON")
        return
    }

    // 加载用户配置
    userCfg := ws.loadUserConfig()
    var matchedUser *model.User
    for i := range userCfg.Users {
        if userCfg.Users[i].Username == req.Username {
            matchedUser = &userCfg.Users[i]
            break
        }
    }

    if matchedUser == nil {
        writeError(w, http.StatusUnauthorized, "invalid credentials")
        return
    }

    // 验证密码
    if !verifyPassword(matchedUser.PasswordHash, req.Password) {
        writeError(w, http.StatusUnauthorized, "invalid credentials")
        return
    }

    token, err := middleware.GenerateToken(matchedUser)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to generate token")
        return
    }

    writeJSON(w, http.StatusOK, map[string]interface{}{
        "token": token,
        "user": map[string]string{
            "id":       matchedUser.ID,
            "username": matchedUser.Username,
            "role":     matchedUser.Role,
        },
    })
}

func (ws *webServer) loadUserConfig() *model.UserConfig {
    // 加载 users.json
}
```

#### 6. 前端添加登录页面

在 `web/src/views/` 下新增 `LoginPage.vue`:

```vue
<template>
  <div class="login-container">
    <el-card class="login-card">
      <h2>IEC 104 模拟器</h2>
      <el-form :model="form" @submit.prevent="handleLogin">
        <el-form-item label="用户名">
          <el-input v-model="form.username" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.password" type="password" />
        </el-form-item>
        <el-button type="primary" native-type="submit" :loading="loading">
          登录
        </el-button>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'

const router = useRouter()
const form = ref({ username: '', password: '' })
const loading = ref(false)

const handleLogin = async () => {
  loading.value = true
  try {
    const { data } = await axios.post('/api/v1/auth/login', form.value)
    localStorage.setItem('token', data.token)
    localStorage.setItem('user', JSON.stringify(data.user))
    router.push('/')
  } catch (e) {
    ElMessage.error('登录失败')
  } finally {
    loading.value = false
  }
}
</script>
```

#### 7. 前端添加请求拦截器

```typescript
// web/src/utils/request.ts
import axios from 'axios'

const request = axios.create({
  baseURL: '/api',
  timeout: 10000,
})

request.interceptors.request.use(config => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

request.interceptors.response.use(
  response => response,
  error => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token')
      localStorage.removeItem('user')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

export default request
```

### 验收标准

- [ ] 用户配置文件管理用户
- [ ] 登录 API 返回 JWT token
- [ ] 所有 API 需要认证
- [ ] 前端登录页面可正常使用

### 预计工时

4 小时

---

## P1 重要优化 - MCP 接口认证

### 问题描述

MCP (Model Context Protocol) 接口可以远程调用工具修改测点值，缺乏认证会导致严重安全风险。

### 影响范围

- `internal/mcp/server.go`
- `internal/mcp/client.go`

### 修复方案

#### 1. 为 MCP Server 添加认证

```go
// internal/mcp/server.go

type MCPServer struct {
    authToken string
    store     *library.Store
    engine    *detail.Engine
}

func NewMCPServer(store *library.Store, engine *detail.Engine, authToken string) *MCPServer {
    return &MCPServer{
        authToken: authToken,
        store:     store,
        engine:    engine,
    }
}

func (m *MCPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 1. 验证 Authorization header
    token := r.Header.Get("Authorization")
    expectedToken := "Bearer " + m.authToken
    if token != expectedToken {
        slog.Warn("MCP unauthorized access attempt", "remote", r.RemoteAddr)
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    // 2. 记录审计日志
    slog.Info("MCP request", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)

    // 3. 处理 MCP 请求
    m.serveMCP(w, r)
}
```

#### 2. 配置 MCP 认证 Token

在配置文件中添加:

```json
// config/mcp-config.json
{
  "enabled": true,
  "auth_token": "your-mcp-secret-token-change-in-production",
  "allowed_tools": ["config_auto_change", "get_point", "set_point"]
}
```

#### 3. MCP 客户端添加认证

```go
// internal/mcp/client.go

type MCPClient struct {
    endpoint   string
    authToken  string
    httpClient *http.Client
}

func NewMCPClient(endpoint, authToken string) *MCPClient {
    return &MCPClient{
        endpoint:  endpoint,
        authToken: authToken,
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}

func (c *MCPClient) CallTool(toolName string, args map[string]interface{}) (interface{}, error) {
    // 添加 Authorization header
    req, _ := http.NewRequest("POST", c.endpoint+"/tools/call", nil)
    req.Header.Set("Authorization", "Bearer "+c.authToken)
    req.Header.Set("Content-Type", "application/json")

    body, _ := json.Marshal(map[string]interface{}{
        "name": toolName,
        "arguments": args,
    })
    req.Body = io.NopCloser(bytes.NewReader(body))

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    // ... 处理响应
}
```

### 验收标准

- [ ] MCP 请求需要 Authorization header
- [ ] 无效 token 被拒绝
- [ ] MCP 调用记录审计日志

### 预计工时

2 小时

---

## P2 计划优化 - CSV 缓存优化

### 问题描述

`loadCSVRows()` 在策略每次运行时被调用，绝对时间模式每次都重新解析整个 CSV 文件，造成 CPU 浪费。

### 影响范围

- `internal/detail/strategy.go` - `loadCSVRows` 函数
- `internal/detail/engine.go` - 策略调度逻辑

### 修复方案

#### 1. 修改 strategyState 结构体

```go
// internal/detail/strategy.go

type strategyState struct {
    csvRows      []csvRow
    csvIndex     int
    currentSOC   float64
    currentEnergy float64
    csvMTime    int64  // CSV 文件修改时间，用于检测变更
}
```

#### 2. 修改 loadCSVRows 为延迟加载

```go
func (sr *strategyRunner) ensureCSVRows(cfg *model.AutoChangeConfig, state *strategyState) bool {
    if state.csvRows != nil {
        // 检查文件是否变更
        csvPath := filepath.Join(sr.configDir, "csv", sr.instanceID, cfg.Params.CSVFileName)
        if _, err := os.Stat(csvPath); err == nil {
            info, _ := os.Stat(csvPath)
            if info.ModTime().Unix() == state.csvMTime {
                return true // 使用缓存
            }
        }
    }

    // 重新加载
    state.csvRows = sr.loadCSVRows(cfg)
    if state.csvRows == nil {
        return false
    }

    // 记录文件修改时间
    csvPath := filepath.Join(sr.configDir, "csv", sr.instanceID, cfg.Params.CSVFileName)
    if info, err := os.Stat(csvPath); err == nil {
        state.csvMTime = info.ModTime().Unix()
    }

    return true
}
```

#### 3. 修改 doCSV 使用缓存

```go
func (sr *strategyRunner) doCSV(cfg *model.AutoChangeConfig, state *strategyState) {
    if !sr.ensureCSVRows(cfg, state) {
        slog.Warn("CSV 回放: 文件加载失败", "ioa", cfg.PointIOA)
        return
    }
    // ... 原有逻辑
}
```

### 验收标准

- [ ] 相同 CSV 文件不重复解析
- [ ] CSV 文件变更时自动重新加载

### 预计工时

1 小时

---

## P2 计划优化 - 自动变化引擎优化

### 问题描述

当前引擎存在以下问题：
- goroutine 使用 channel 同步，可能阻塞
- 高频策略（100ms）创建大量 goroutine
- 无 goroutine 数量限制

### 影响范围

- `internal/detail/engine.go`

### 修复方案

#### 1. 使用 sync.WaitGroup 替代 channel 同步

```go
// internal/detail/engine.go

type Engine struct {
    mu           sync.RWMutex
    store        *library.Store
    pub          publisher
    strategy     *strategyRunner
    acStore      *AutoChangeStore
    tasks        map[uint32]*changeTask
    state        map[uint32]*strategyState
    instanceID   string
    cfgDir       string
    wg           sync.WaitGroup  // 新增
}

func (e *Engine) StopAll() {
    e.mu.Lock()
    defer e.mu.Unlock()

    for ioa, task := range e.tasks {
        task.cancel()
        e.wg.Done()  // 标记完成
        delete(e.tasks, ioa)
        delete(e.state, ioa)
    }
    e.wg.Wait()  // 等待所有 goroutine 退出
}
```

#### 2. 添加 goroutine 数量限制

```go
// 在 engine.go 中添加

const maxConcurrentTasks = 100  // 最大并发任务数

func (e *Engine) startTaskLocked(cfg *model.AutoChangeConfig) {
    // 检查并发数量
    if len(e.tasks) >= maxConcurrentTasks {
        slog.Error("已达到最大并发任务数", "max", maxConcurrentTasks, "current", len(e.tasks))
        return
    }

    // ... 原有逻辑
    e.wg.Add(1)
    go func() {
        defer e.wg.Done()  // 改为 WaitGroup
        // ...
    }()
}
```

### 验收标准

- [ ] 使用 WaitGroup 正确管理 goroutine 生命周期
- [ ] 并发任务数有上限保护

### 预计工时

1.5 小时

---

## P2 计划优化 - 状态管理引入

### 问题描述

当前前端无全局状态管理，组件间通信困难，代码重复。

### 影响范围

- `web/src/` - 所有 Vue 组件

### 修复方案

#### 1. 安装 Pinia

```bash
cd web
npm install pinia
```

#### 2. 创建 store 目录结构

```
web/src/
├── stores/
│   ├── auth.ts      # 认证状态
│   ├── instances.ts # 实例列表状态
│   └── points.ts    # 测点数据状态
```

#### 3. 实现 auth store

```typescript
// web/src/stores/auth.ts
import { defineStore } from 'pinia'
import request from '@/utils/request'

interface User {
  id: string
  username: string
  role: string
}

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem('token') || '',
    user: JSON.parse(localStorage.getItem('user') || 'null') as User | null,
  }),

  getters: {
    isLoggedIn: (state) => !!state.token,
    isAdmin: (state) => state.user?.role === 'admin',
  },

  actions: {
    async login(username: string, password: string) {
      const { data } = await request.post('/v1/auth/login', { username, password })
      this.token = data.token
      this.user = data.user
      localStorage.setItem('token', data.token)
      localStorage.setItem('user', JSON.stringify(data.user))
    },

    logout() {
      this.token = ''
      this.user = null
      localStorage.removeItem('token')
      localStorage.removeItem('user')
    },
  },
})
```

#### 4. 实现 instances store

```typescript
// web/src/stores/instances.ts
import { defineStore } from 'pinia'
import request from '@/utils/request'

interface Instance {
  id: string
  name: string
  iec104_port: number
  status: string
  // ...
}

export const useInstancesStore = defineStore('instances', {
  state: () => ({
    instances: [] as Instance[],
    loading: false,
  }),

  actions: {
    async fetchInstances() {
      this.loading = true
      try {
        const { data } = await request.get('/v1/instances')
        this.instances = data.instances
      } finally {
        this.loading = false
      }
    },

    async startInstance(id: string) {
      await request.post(`/v1/instances/${id}/start`)
      await this.fetchInstances()
    },

    async stopInstance(id: string) {
      await request.post(`/v1/instances/${id}/stop`)
      await this.fetchInstances()
    },
  },
})
```

#### 5. 在 main.ts 中注册 Pinia

```typescript
// web/src/main.ts
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'

const app = createApp(App)
app.use(createPinia())
app.mount('#app')
```

### 验收标准

- [ ] 安装 Pinia
- [ ] 创建 auth store 管理登录状态
- [ ] 创建 instances store 管理实例列表
- [ ] 组件使用 store 替代 props 传递

### 预计工时

2 小时

---

## P3 长期优化 - 测试与监控

### 测试覆盖率提升

#### 1. 新增测试文件

| 文件 | 覆盖范围 |
|------|----------|
| `internal/detail/engine_test.go` | 自动变化引擎启动/停止/状态 |
| `internal/detail/strategy_test.go` | 11 种策略计算逻辑 |
| `pkg/library/store_test.go` | 扩展更多边界用例 |
| `internal/model/user_test.go` | 用户模型验证 |

#### 2. 测试示例

```go
// internal/detail/engine_test.go

func TestEngine_StartTask(t *testing.T) {
    store := library.NewStore([]*config.Point{
        {IOA: 1, PointType: config.TypeAI, Value: 100},
    })
    engine := NewEngine("test", store, &mockPublisher{}, NewAutoChangeStore(), "./config", nil)

    cfg := &model.AutoChangeConfig{
        PointIOA:     1,
        Strategy:     model.StrategyIncrement,
        Enabled:      true,
        Params:       model.AutoChangeParams{StartValue: 0, Step: 1, PeriodMs: 1000},
    }

    if err := engine.StartOrUpdate(cfg); err != nil {
        t.Fatalf("启动任务失败: %v", err)
    }

    if !engine.IsRunning(1) {
        t.Error("任务未正常运行")
    }

    engine.StopAll()
}
```

### Prometheus 监控

#### 1. 添加 metrics 端点

```go
// pkg/metrics/metrics.go (新文件)

package metrics

import (
    "net/http"
    "sync/atomic"
)

var (
    httpRequestsTotal   int64
    httpRequestsSuccess int64
    iec104Connections   int64
    pointsModified      int64
)

func IncHTTPRequests() {
    atomic.AddInt64(&httpRequestsTotal, 1)
}

func IncHTTPSuccess() {
    atomic.AddInt64(&httpRequestsSuccess, 1)
}

// MetricsHandler 返回 Prometheus 格式的指标
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/plain; version=0.0.4")
    w.Write([]byte(`# HELP http_requests_total Total HTTP requests
# TYPE http_requests_total counter
http_requests_total ` + string(httpRequestsTotal) + `
# HELP iec104_connections Current IEC104 connections
# TYPE iec104_connections gauge
iec104_connections ` + string(iec104Connections) + `
`))
}
```

#### 2. 注册路由

```go
mux.HandleFunc("/metrics", metrics.MetricsHandler)
```

### 预计工时

3 小时

---

## CI/CD 搭建方案

### 概述

为项目搭建自动化 CI/CD 流程，实现：
- 代码提交自动运行测试
- 自动构建多平台二进制
- 自动生成发布包

### 技术选型

| 组件 | 选择 | 原因 |
|------|------|------|
| CI 平台 | GitHub Actions | 免费额度充足，与 GitHub 集成 |
| 构建工具 | Go + Node.js | 多阶段构建 |
| 发布目标 | GitHub Releases | 官方发布渠道 |

### 方案一：GitHub Actions（推荐）

#### 1. 创建工作流文件

```yaml
# .github/workflows/ci.yml

name: CI/CD

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

env:
  GO_VERSION: '1.24'
  NODE_VERSION: '20'

jobs:
  # 1. 代码检查
  lint:
    name: Lint
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}

      - name: Run Go fmt check
        run: |
          if [ -n "$(gofmt -l .)" ]; then
            echo "Code is not formatted, run 'gofmt -w .'"
            exit 1
          fi

      - name: Run Go vet
        run: go vet ./...

      - name: Run staticcheck
        run: |
          go install honnef.co/go/tools/cmd/staticcheck@latest
          staticcheck ./...

  # 2. 测试
  test:
    name: Test
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}

      - name: Run tests
        run: go test -v -race -coverprofile=coverage.out ./...

      - name: Upload coverage
        uses: actions/upload-artifact@v4
        with:
          name: coverage
          path: coverage.out

  # 3. 前端检查
  frontend:
    name: Frontend Build
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Node.js
        uses: actions/setup-node@v4
        with:
          node-version: ${{ env.NODE_VERSION }}
          cache: 'web/node_modules'

      - name: Install dependencies
        run: cd web && npm ci

      - name: Type check
        run: cd web && npx vue-tsc --noEmit

      - name: Build
        run: cd web && npm run build

  # 4. 构建发布
  build-release:
    name: Build Release
    runs-on: ubuntu-latest
    needs: [lint, test, frontend]
    if: github.event_name == 'push' && (github.ref == 'refs/heads/main' || github.ref == 'refs/heads/develop')
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}

      - name: Set up Node.js
        uses: actions/setup-node@v4
        with:
          node-version: ${{ env.NODE_VERSION }}

      - name: Build frontend
        run: |
          cd web
          npm ci
          npm run build

      - name: Build Linux AMD64
        run: |
          GOOS=linux GOARCH=amd64 go build -o bin/iec104-sim-linux-amd64 ./cmd/iec104-sim/
          tar -czf dist/iec104-sim-${GITHUB_REF_NAME}-linux-amd64.tar.gz -C .. iec104-sim-master/bin/iec104-sim-linux-amd64

      - name: Build Linux ARM64
        run: |
          GOOS=linux GOARCH=arm64 go build -o bin/iec104-sim-linux-arm64 ./cmd/iec104-sim/
          tar -czf dist/iec104-sim-${GITHUB_REF_NAME}-linux-arm64.tar.gz -C .. iec104-sim-master/bin/iec104-sim-linux-arm64

      - name: Build Windows AMD64
        run: |
          GOOS=windows GOARCH=amd64 go build -o bin/iec104-sim-windows-amd64.exe ./cmd/iec104-sim/
          cd dist && powershell -Command "Compress-Archive -Path ../bin/iec104-sim-windows-amd64.exe -DestinationPath iec104-sim-${GITHUB_REF_NAME}-windows-amd64.zip"

      - name: Create Release
        uses: softprops/action-gh-release@v1
        with:
          files: |
            dist/*.tar.gz
            dist/*.zip
          draft: false
          prerelease: ${{ github.ref != 'refs/heads/main' }}
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

#### 2. 添加工作流触发配置

```yaml
# .github/workflows/release.yml (可选，手动触发发布)

name: Manual Release

on:
  workflow_dispatch:
    inputs:
      version:
        description: 'Release version (e.g., v2.2.1)'
        required: true

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      # ... 类似上面的构建逻辑
```

#### 3. 添加依赖缓存加速

```yaml
# 在 setup-go 和 setup-node 步骤中已配置缓存
# 额外添加 action 缓存

- name: Cache Go modules
  uses: actions/cache@v4
  with:
    path: |
      ~/.cache/go-build
      ~/go/pkg/mod
    key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
    restore-keys: |
      ${{ runner.os }}-go-
```

### 方案二：本地 CI/CD（使用 Makefile）

如果无法使用 GitHub Actions，可以使用本地 Makefile:

```makefile
# Makefile

.PHONY: ci test build-all

ci: test lint

lint:
	@echo "Running linters..."
	go fmt ./...
	go vet ./...
	@cd web && npm run lint || true

test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	@cd web && npm test || true

build-all: build-linux-amd64 build-linux-arm64 build-windows

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -o bin/iec104-sim-linux-amd64 ./cmd/iec104-sim/

build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build -o bin/iec104-sim-linux-arm64 ./cmd/iec104-sim/

build-windows:
	GOOS=windows GOARCH=amd64 go build -o bin/iec104-sim-windows-amd64.exe ./cmd/iec104-sim/

release: build-all
	@echo "Creating release packages..."
	@mkdir -p dist
	cd bin && tar -czf ../dist/iec104-sim-$(VERSION)-linux-amd64.tar.gz iec104-sim-linux-amd64
	cd bin && tar -czf ../dist/iec104-sim-$(VERSION)-linux-arm64.tar.gz iec104-sim-linux-arm64
	cd bin && zip ../dist/iec104-sim-$(VERSION)-windows-amd64.zip iec104-sim-windows-amd64.exe
```

### CI/CD 验收标准

- [ ] 每次 push 自动运行 lint + test
- [ ] 前端自动 type check
- [ ] main 分支 push 自动构建发布包
- [ ] 自动生成 GitHub Release

### 预计工时

3 小时（GitHub Actions）

---

## 总结

### 工时汇总

| 阶段 | 优化项 | 预计工时 |
|------|--------|----------|
| P0 | HTTP 超时配置 | 0.5h |
| P0 | 文件上传安全 | 1.5h |
| P1 | 前端构建优化 | 1h |
| P1 | 认证机制设计 | 4h |
| P1 | MCP 接口认证 | 2h |
| P2 | CSV 缓存优化 | 1h |
| P2 | 引擎 goroutine 优化 | 1.5h |
| P2 | 状态管理引入 | 2h |
| P3 | 测试与监控 | 3h |
| - | CI/CD 搭建 | 3h |
| **总计** | | **19.5h** |

### 执行建议

1. **第 1 天**: 完成 P0 两项（HTTP 超时 + 文件上传安全）
2. **第 2-3 天**: 完成 P1 认证机制（最重要）
3. **第 4-5 天**: 完成 P1 前端优化 + MCP 认证
4. **第 6-7 天**: 完成 P2 优化项
5. **第 2 周**: 完成 P3 + CI/CD