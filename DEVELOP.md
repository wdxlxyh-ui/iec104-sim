# IEC104 模拟器 — 开发规范

> 本文档记录项目开发中的关键约束和最佳实践，后续开发必须遵守。

---

## 1. 打包规范

### 1.1 Makefile

| 规范 | 说明 | 反例 |
|------|------|------|
| `mkdir` 逐行创建目录 | `/bin/sh` 不支持 brace expansion `{a,b,c}` | `mkdir -p dir/{bin,config}` ❌ |
| amd64 二进制无后缀 | `build-linux-amd64` 输出 `bin/iec104-sim` | `bin/iec104-sim-linux-amd64` ❌ |
| arm64 二进制加 `-arm64` 后缀 | `build-linux-arm64` 输出 `bin/iec104-sim-arm64` | — |
| Windows 输出 `.exe` | `build-windows` 输出 `bin/iec104-sim.exe` | — |
| `dist` 依赖 `web-build build-all` | 确保前端先构建，再打包二进制 | — |
| 发行包目录结构固定 | `{bin,config,logs,resources,web}/` | — |

### 1.2 发行包结构

```
iec104-sim-v{version}-linux-amd64/
├── bin/
│   ├── iec104-sim            ← 二进制（amd64 无后缀）
│   ├── start.sh
│   ├── stop.sh
│   └── restart.sh
├── config/
│   └── instances.json
├── logs/
│   └── .gitkeep
├── resources/
│   └── .gitkeep
└── web/                      ← 前端构建产物
    ├── index.html
    └── assets/
        ├── index-*.css
        ├── index-*.js
        └── DetailPage-*.js
```

### 1.3 前端路径解析

二进制通过 `filepath.Dir(exePath)` 定位到 `bin/` 目录，前端路径为：

```
filepath.Join(filepath.Dir(exePath), "..", "web", "dist")
```

**禁止**使用 `./web/dist`（依赖 CWD）或 `bin/web/dist`（路径错误）。

---

## 2. 前端代码规范（Vue 3 + TypeScript）

### 2.1 状态管理

| 规范 | 说明 |
|------|------|
| 表格数据用 `ref<T[]>([])` | 修改时替换整个数组，触发响应式更新 |
| 简单状态用 `reactive<Record>()` | 如 `setValues`, `autoStrategies` |
| 组件引用用 `ref<HTMLElement>()` | 配合 `nextTick` 访问 DOM |

### 2.2 表格行组件复用

**关键约束**：`el-table` 复用 DOM 行，切换 tab 或更新数据时组件状态可能残留。

**解决方案**：为有状态的组件（如 `el-switch`）添加 `:key="row.ioa"`：

```vue
<el-switch :model-value="!!setValues[row.ioa]" :key="row.ioa" />
```

### 2.3 置数值隔离

- `setValues[ioa]` 存储用户手动输入的值，与后端 `row.value` 完全隔离
- `fetchPoints` 初始化时用 `if (p.ioa in setValues) return` 保护用户输入
- `displayValue()` 只读后端数据，不读 `setValues`

### 2.4 事件处理

| 场景 | 推荐写法 |
|------|----------|
| 输入框值变更 | `@update:model-value="(v) => { setValues[row.ioa] = v }"` |
| Enter 确认 | `@keydown.enter.prevent="() => doSetValue(row)"` |
| 开关切换 | `@change="(val: boolean) => doSetValue(row, val ? 1 : 0)"` |

### 2.5 组件禁用逻辑

置数输入框的 `disabled` 状态：

```typescript
// ✅ 正确：仅 AO/DO 禁用（无置数 UI），其他均可编辑
// AI/PI/DI 无论策略如何都可置数
// AO/DO 永远显示 "—"，无输入框

// ❌ 禁止：根据 autoStrategies 禁用置数框
// 置数与自动变化策略是独立操作
```

### 2.6 API 调用

```typescript
// ✅ 正确：await 等待结果，try/catch 处理错误
try {
  await setPointValue(instanceId.value, ioa, body)
  ElMessage.success('操作成功')
} catch (e: any) {
  ElMessage.error('操作失败: ' + (e?.response?.data?.error || e.message))
}

// ❌ 禁止：.then()/.catch() 链式调用
// ❌ 禁止：忽略错误（空的 catch）
```

---

## 3. Go 后端代码规范

### 3.1 路径处理

```go
// ✅ 正确：使用 filepath.Join
webDir := filepath.Join(filepath.Dir(exePath), "..", "web", "dist")

// ❌ 禁止：硬编码斜杠
webDir := exeDir + "/../web/dist"
```

### 3.2 错误处理

```go
// ✅ 正确：检查错误并记录日志
if err := cfgStore.Load(); err != nil {
    slog.Warn("加载配置失败，使用空配置", "error", err)
}

// ❌ 禁止：忽略错误
cfgStore.Load()

// ❌ 禁止：panic
```

### 3.3 路由注册

```go
// ✅ 正确：通过 Register 方法注册
func (h *DetailHandler) Register(mux *http.ServeMux) {
    mux.HandleFunc("/api/v1/instances/", h.handleInstanceByID)
}

// ❌ 禁止：在全局变量中直接注册
```

### 3.4 策略模式

```go
// ✅ 正确：StrategyManual 为空逻辑，引擎不自动计算
case model.StrategyManual:
    // 空逻辑，值由用户 API 置数

// ✅ 正确：CheckAPIWriteAllowed 允许 manual 和 apiupdate
return cfg.Strategy == model.StrategyAPIUpdate || cfg.Strategy == model.StrategyManual
```

---

## 4. DI（遥信）组件规范

| 规范 | 说明 |
|------|------|
| DI 只有置数功能 | 无自动变化配置按钮 |
| 状态来源 | `row.bool_value`（后端实时值） |
| 状态变更 | 切换时调用 `doSetValue(row, val ? 1 : 0)` |
| 必须加 `:key` | `:key="row.ioa"` 防止组件复用导致状态残留 |

---

## 5. 版本号管理

| 规则 | 说明 |
|------|------|
| 主版本号 | 不兼容的 API 修改（尚未发生） |
| 次版本号 | 新功能发布（如 v2.1 引入详情页） |
| 修订号 | Bug 修复（如 v2.1.3 → v2.1.4） |
| 开发版本 | `-dev` 后缀（如 `2.1.5-dev`） |

版本号在以下位置同步更新：
1. `Makefile` → `VERSION := x.x.x`
2. `web/package.json` → `"version": "x.x.x"`
3. `cmd/iec104-sim/main.go` → `-ldflags="-X main.version=x.x.x"`
4. Git Tag → `v{x.x.x}`

---

## 6. 分支策略

| 分支 | 用途 |
|------|------|
| `main` | 稳定版本，仅接受 merge request |
| `develop` | 开发分支，所有新功能/修复在此完成 |

**流程**：
1. 在 `develop` 分支开发并测试
2. 确认无误后 merge 到 `main`
3. 打 tag 并打包发行