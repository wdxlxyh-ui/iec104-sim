# EGC AI 自动化测试方案 — IEC104 模拟器审查建议

> 审查日期：2026-05-13 | 基于：EGC AI 自动化测试方案 v1.0
> 审查范围：仅涉及 IEC104 模拟器相关部分

---

## 一、方案准确性修正

### 1.1 🔴 关键缺失：缺少批量写入端点

**方案原文**：
> MCP `write_points` — 一次调用写入多个测点，模拟同一时刻设备上报数据。

**实际情况**：
当前 IEC104 模拟器 v2.1.1 的 v1 API **不支持批量写入**。现有端点：

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/v1/instances/{id}/points/{ioa}` | PUT | 单点写入（仅支持 AI/DI/PI） |
| `/api/v1/instances/{id}/points/` | GET | 读取全部测点 |

每次单点写入独立加锁，多个测点无法在"同一时刻"原子写入。如果自动化引擎逐点调用，EGC 可能在写入中途触发控制周期，读到不完整数据。

**建议修正**：
在 Phase 1 前新增 `POST /api/v1/instances/{id}/points/batch` 端点，支持一次请求写入多个测点。

### 1.2 🟡 输出校验测点类型说明

**方案原文**：
> `pv_target_p(30001)`、`battery_target_p(30002)`、`grid_point_p(30003)` 作为输出校验点。

**实际情况**：
上述测点应为 **AO 类型**（遥调），由 EGC 通过 IEC104 协议写入。当前 HTTP API 对 AO 点**禁止写入**（`IsSetValueAllowed` 返回 false）。但自动化引擎只需**读取**这些 AO 值来做校验，读取操作完全支持。

**建议修正**：
在测点地址映射表中标注 AO 类型，MCP 的 `read_points` 读取这些点无任何阻碍。无需变更模拟器。

### 1.3 🟡 测点类型感知

**方案原文**：
> 所有测点值以 float32 抽象表示。

**实际情况**：
模拟器有 5 种测点类型，每种的值字段不同：

| 类型 | 含义 | 值字段 | JSON 示例 |
|------|------|--------|-----------|
| AI | 遥测 | float64 | `{"value": 50.0}` |
| DI | 遥信 | bool | `{"bool_value": true}` |
| PI | 遥脉 | int32 | `{"int_value": 123}` |
| AO | 遥调 | float64 | 只读（EGC 写入） |
| DO | 遥控 | bool | 只读（EGC 写入） |

**建议修正**：
方案/MCP 层需要感知测点类型，写入时选择正确的字段。MCP `write_point`/`write_points` 的 JSON Schema 应标明此限制。

### 1.4 🟡 自动变化写入保护

**方案原文**：
> 未提及自动变化策略对 API 写入的影响。

**实际情况**：
当某个测点已启用自动变化策略且策略**非 APIUpdate** 时，HTTP API 会**拒绝写入**：

```go
// CheckAPIWriteAllowed: 仅当策略为 APIUpdate 时才允许 API 写入
return cfg.Strategy == model.StrategyAPIUpdate
```

**建议修正**：
自动化测试中输入测点（40001-40007）应**不启用自动变化策略**，或使用 APIUpdate 策略。MCP 服务应在写入失败时返回明确错误信息指导用户排查。

---

## 二、架构优化建议

### 2.1 建议双 MCP 服务架构（已采纳）

原方案设计了 7 个独立的 MCP 工具，建议合并为 2 个 MCP 服务：

```
┌─────────────────────────────────────────────────┐
│                  AI Agent                        │
│           (自动化执行引擎 / 测试编排)              │
└──────────┬──────────────────────┬────────────────┘
           │ MCP                  │ MCP
           ▼                      ▼
┌──────────────────────┐ ┌──────────────────────────┐
│  MCP Instance Manager│ │  MCP Data Interface       │
│                      │ │                           │
│ • create_instance    │ │ • write_point             │
│ • delete_instance    │ │ • write_points  ⭐核心    │
│ • start_instance     │ │ • read_point              │
│ • stop_instance      │ │ • read_points             │
│ • restart_instance   │ │ • list_points             │
│ • list_instances     │ │ • config_auto_change      │
│ • get_status         │ │ • delete_auto_change      │
│                      │ │ • export_auto_changes     │
│ 通信: HTTP → 模拟器   │ │ • import_auto_changes     │
│                      │ │ • upload_csv              │
│                      │ │                           │
│                      │ │ 通信: HTTP → 模拟器        │
└──────────────────────┘ └──────────────────────────┘
```

**优势**：
- 职责清晰：管理面 vs 数据面
- 并行独立：实例管理不影响数据操作
- MCP 协议天然支持多服务注册

### 2.2 建议利用已有 CSV 回放策略

模拟器已内置 CSV 时间序列回放策略（`StrategyCSV`），可将 CSV 文件上传到模拟器自动按时间轴播放数据。对于**纯数据驱动**的测试场景（无需逐步骤校验），可以直接使用此功能，减少 MCP 调用次数：

```
PUT /api/v1/instances/{id}/points/auto-change/{ioa}
{
  "strategy": "csv",
  "enabled": true,
  "params": {
    "csv_file": "scenario.csv",
    "period_ms": 2000
  }
}
```

**混合模式建议**：
- **需要同步校验的场景** → MCP `write_points` + `read_points`（精确控制写-等-校验周期）
- **纯数据驱动场景** → 模拟器 CSV 回放（简化 MCP 链路）

### 2.3 建议增加 QDS 异常测试

模拟器已支持品质描述符（QDS），可在写入值时附加：

```json
{
  "value": 50.0,
  "qds": {"invalid": true, "substituted": true}
}
```

建议 MCP `write_point` 增加可选 `qds` 参数，支持验证 EGC 对异常品质数据的处理逻辑。

### 2.4 建议测点地址映射可配置化

建议用 YAML 配置文件管理逻辑名称 → IOA 地址的映射：

```yaml
# point_mapping.yaml
inputs:
  grid_meter_p: { ioa: 40001, type: AI, desc: "关口表有功功率" }
  grid_meter_q: { ioa: 40002, type: AI, desc: "关口表无功功率" }
  pv_power:     { ioa: 40003, type: AI, desc: "光伏发电功率" }
  battery_power:  { ioa: 40004, type: AI, desc: "储能充放电功率" }
  battery_soc:  { ioa: 40005, type: AI, desc: "储能 SOC" }
outputs:
  pv_target_p:    { ioa: 30001, type: AO, desc: "光伏目标功率(EGC下发)" }
  battery_target_p: { ioa: 30002, type: AO, desc: "储能目标功率(EGC下发)" }
  grid_point_p:   { ioa: 30003, type: AO, desc: "并网点功率(EGC控制结果)" }
```

MCP 服务启动时加载此文件，暴露的逻辑名称供 AI Agent 使用，屏蔽底层 IOA 细节。

---

## 三、实施前置条件

### 需要先在模拟器中新增

| 优先级 | 端点 | 说明 | 状态 |
|--------|------|------|------|
| P0 | `POST /points/batch` | 批量写入端点，自动化测试的前提 | ⏳ 待实现 |

### 模拟器已有、可直接使用

| 端点 | 说明 |
|------|------|
| `GET /points` | 读取全部测点（AI优先+IOA升序，已排序） |
| `PUT /points/{ioa}` | 单点写入 |
| `GET/PUT/DELETE /points/auto-change/{ioa}` | 自动变化配置 |
| `PUT /points/auto-change/batch` | 批量自动变化配置 |
| `GET /points/auto-change/export` | 导出自动变化 CSV |
| `POST /points/auto-change/import` | 导入自动变化 CSV |
| `POST /upload-csv` | 上传 CSV 回放文件 |
| `GET /points/export` | 导出测点 CSV |

---

## 四、双 MCP 服务接口定义

### 4.1 MCP Instance Manager

管理模拟器实例的生命周期和健康监控。

| 工具名 | 描述 | 对应 HTTP API |
|--------|------|---------------|
| `list_instances` | 列出所有已配置实例 | `GET /api/v1/instances` |
| `get_instance` | 获取单个实例详情 | `GET /api/v1/instances/{id}` |
| `create_instance` | 创建新实例 | `POST /api/v1/instances` |
| `update_instance` | 更新实例配置 | `PUT /api/v1/instances/{id}` |
| `delete_instance` | 删除实例 | `DELETE /api/v1/instances/{id}` |
| `start_instance` | 启动实例 | `POST /api/v1/instances/{id}/start` |
| `stop_instance` | 停止实例 | `POST /api/v1/instances/{id}/stop` |
| `restart_instance` | 重启实例 | `POST /api/v1/instances/{id}/restart` |
| `get_server_status` | 全局服务状态与实例健康度 | `GET /api/v1/status` |

### 4.2 MCP Data Interface

所有测点数据操作 — 写入、读取、策略配置。

| 工具名 | 描述 | 对应 HTTP API | 状态 |
|--------|------|---------------|------|
| `read_point` | 读取单个测点 | `GET .../points/{ioa}` | ✓ 可用 |
| `read_points` | 批量读取测点 | `GET .../points` (MCP层过滤) | ✓ 可用 |
| `write_point` | 写入单个测点 | `PUT .../points/{ioa}` | ✓ 可用 |
| `write_points` ⭐ | **批量写入多个测点** | `POST .../points/batch` | ❌ 需新增 |
| `list_points` | 列出全部测点及当前值 | `GET .../points` | ✓ 可用 |
| `config_auto_change` | 配置自动变化策略 | `PUT .../auto-change/{ioa}` | ✓ 可用 |
| `get_auto_change` | 查看自动变化配置 | `GET .../auto-change/{ioa}` | ✓ 可用 |
| `delete_auto_change` | 删除自动变化配置 | `DELETE .../auto-change/{ioa}` | ✓ 可用 |
| `batch_auto_change` | 批量配置自动变化 | `PUT .../auto-change/batch` | ✓ 可用 |
| `export_auto_changes` | 导出全部自动变化配置(CSV) | `GET .../auto-change/export` | ✓ 可用 |
| `import_auto_changes` | 导入自动变化配置(CSV) | `POST .../auto-change/import` | ✓ 可用 |
| `upload_csv` | 上传CSV回放文件 | `POST .../upload-csv` | ✓ 可用 |
| `export_points_csv` | 导出测点数据(CSV) | `GET .../export` | ✓ 可用 |

---

## 五、实施步骤建议

```
Phase 0.5 — 模拟器增强（预计 1 天）
  └── 实现 POST /points/batch 批量写入端点

Phase 1 — MCP 服务实现（预计 2-3 天）
  ├── MCP Instance Manager Service（复用已有的实例管理 API）
  └── MCP Data Interface Service（包装数据接口）

Phase 2 — 自动化执行引擎（预计 2 天）
  ├── 测试数据生成器（测试用例 → 时序表）
  ├── 场景执行引擎（load_scenario / step_scenario / run_scenario）
  └── 报告生成器

Phase 3 — AI 集成（预计 2 天）
  ├── AI Agent 自动编排
  ├── 回归测试套件
  └── 失败智能分析
```
