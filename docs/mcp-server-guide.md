# IEC104 模拟器 MCP Server

通过 MCP 协议让 AI 直接控制 IEC104 模拟器，实现自动化测试编排。

---

## 一分钟上手

### 1. 确认模拟器已启动

确保 IEC104 模拟器（`iec104-sim`）已在某台服务器上运行，HTTP API 端口默认为 `8989`。

### 2. 配置 MCP Server

编辑你的 AI 客户端的 MCP 配置文件：

**OpenCode** → `opencode.json`（项目根目录）或 `~/.opencode/config.json`：

```json
{
  "mcp": {
    "iec104-simulator": {
      "type": "local",
      "command": ["/你的绝对路径/mcp-server", "--mode", "both", "--simulator", "http://10.65.99.13:8989"],
      "enabled": true
    }
  }
}
```

**Claude Desktop** → `claude_desktop_config.json`：

```json
{
  "mcpServers": {
    "iec104-simulator": {
      "command": "/你的绝对路径/mcp-server",
      "args": ["--mode", "both", "--simulator", "http://10.65.99.13:8989"]
    }
  }
}
```

**Cursor** → `.cursor/mcp.json`（内容同上）。

### 3. 使用

配置好后重启 AI 客户端，你就可以直接对它说：

> *"帮我创建一个模拟器实例，端口 2405，用 samples/point.xlsx 点表，然后启动。再往 40001 写入 100，40002 写入 50。"*

AI 会自动调用 `create_instance` → `start_instance` → `write_points` 完成。

---

## 工作原理

```
┌──────────────────────────────────────────────────────────┐
│                   你的 AI 客户端                           │
│         (OpenCode / Claude Desktop / Cursor / Kiro)       │
└───────────────┬──────────────────────────────────────────┘
                │ MCP 协议 (stdio)
                ▼
┌──────────────────────────────────────────────────────────┐
│                    mcp-server                              │
│    ┌─────────────────┐  ┌────────────────────────────┐    │
│    │ Instance Manager │  │     Data Interface        │    │
│    │  • 创建/启停实例  │  │  • 读写测点                │    │
│    │  • 查询状态      │  │  • 批量写入 ⭐             │    │
│    │  共 9 个工具      │  │  • 配自动变化策略           │    │
│    └─────────────────┘  │  • CSV 导入导出             │    │
│                          │  共 15 个工具               │    │
│                          └────────────────────────────┘    │
└───────────────┬──────────────────────────────────────────┘
                │ HTTP API
                ▼
┌──────────────────────────────────────────────────────────┐
│              IEC104 模拟器 (iec104-sim)                    │
│        运行于 10.65.99.13:8989 (举例)                      │
└──────────────────────────────────────────────────────────┘
```

---

## 启动参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--simulator` | `http://localhost:8989` | IEC104 模拟器的 HTTP API 地址 |
| `--mode` | `both` | 服务模式: `instance` / `data` / `both` |
| `--listen` | (不设置) | HTTP SSE 监听地址，如 `:8081`，设置后以 HTTP 模式启动 |

### 三种启动方式

```bash
# 1. stdio 模式（默认，给 AI 客户端用）
./mcp-server --simulator http://10.65.99.13:8989 --mode both

# 2. HTTP SSE 模式（远程部署）
./mcp-server --simulator http://localhost:8989 --mode both --listen :8081

# 3. 单服务模式（只暴露部分工具）
./mcp-server --simulator http://10.65.99.13:8989 --mode instance  # 只有实例管理
./mcp-server --simulator http://10.65.99.13:8989 --mode data      # 只有数据操作
```

---

## 工具清单（共 24 个）

### Instance Manager（9 个）

| 工具 | 参数 | 说明 |
|------|------|------|
| `list_instances` | 无 | 列出所有已配置的实例 |
| `get_instance` | `instance_id` | 获取单个实例详情 |
| `create_instance` | `config`(JSON字符串) | 创建新实例 |
| `update_instance` | `instance_id`, `config` | 更新实例配置 |
| `delete_instance` | `instance_id` | 删除实例 |
| `start_instance` | `instance_id` | 启动实例 |
| `stop_instance` | `instance_id` | 停止实例 |
| `restart_instance` | `instance_id` | 重启实例 |
| `get_server_status` | 无 | 全局服务状态和实例健康度 |

### Data Interface（15 个）

| 工具 | 参数 | 说明 |
|------|------|------|
| `list_points` | `instance_id` | 列出所有测点及当前值 |
| `read_point` | `instance_id`, `ioa` | 读取单个测点 |
| `read_points` | `instance_id`, `ioas`(可选) | 批量读取，不传`ioas`返回全部 |
| `write_point` | `instance_id`, `ioa`, `value`/`bool_value`/`int_value` | 写入单个测点 |
| `write_points` ⭐⭐ | `instance_id`, `points`(数组) | **批量写入多个测点** |
| `config_auto_change` | `instance_id`, `ioa`, `strategy`, `enabled`, `params` | 配置自动变化策略 |
| `get_auto_change` | `instance_id`, `ioa` | 查看自动变化配置 |
| `delete_auto_change` | `instance_id`, `ioa` | 删除自动变化配置 |
| `batch_auto_change` | `instance_id`, `configs`(JSON数组) | 批量配置自动变化 |
| `export_auto_changes` | `instance_id` | 导出自动变化 CSV |
| `import_auto_changes` | `instance_id`, `csv_content` | 从 CSV 导入自动变化 |
| `upload_csv` | `instance_id`, `csv_content` | 上传 CSV 回放文件 |
| `export_points_csv` | `instance_id` | 导出测点数据 CSV |

---

## 测点类型说明

写入时必须根据 **测点类型** 选择正确字段：

| 类型 | 中文 | 值字段 | 示例 |
|------|------|--------|------|
| AI | 遥测(YC) | `value` (float) | `{"ioa": 40001, "value": 50.0}` |
| DI | 遥信(YX) | `bool_value` (bool) | `{"ioa": 5, "bool_value": true}` |
| PI | 遥脉(YM) | `int_value` (int32) | `{"ioa": 100, "int_value": 1234}` |
| AO | 遥调 — **只读** | EGC 下发，MCP 不能写 |
| DO | 遥控 — **只读** | EGC 下发，MCP 不能写 |

---

## 典型测点地址

| 逻辑名 | IOA | 类型 | 说明 |
|--------|-----|------|------|
| `grid_meter_p` | 40001 | AI | 关口表有功功率（输入） |
| `grid_meter_q` | 40002 | AI | 关口表无功功率（输入） |
| `pv_power` | 40003 | AI | 光伏发电功率（输入） |
| `battery_power` | 40004 | AI | 储能充放电功率（输入） |
| `battery_soc` | 40005 | AI | 储能 SOC（输入） |
| `pv_target_p` | 30001 | AO | 光伏目标功率（EGC 下发，只读） |
| `battery_target_p` | 30002 | AO | 储能目标功率（EGC 下发，只读） |
| `grid_point_p` | 30003 | AO | 并网点功率（EGC 控制结果，只读） |

---

## 自动化测试编排流程

```
Phase 1: 准备
  1. create_instance       → 用测试点表创建实例(config: JSON字符串)
  2. start_instance        → 启动实例(instance_id)

Phase 2: 注入数据
  3. write_points          → 批量写入输入测点 40001-40007
          ↓
   EGC 控制周期触发，读取数据进行策略计算
          ↓
Phase 3: 校验结果
  4. read_points           → 读取 AO 点 30001-30003 验证 EGC 输出

Phase 4: 清理
  5. stop_instance         → 停止实例
  6. delete_instance       → 删除实例
```

## 多服务器控制

可以同时控制多台服务器上的模拟器：

```bash
# 开两个终端
终端1: ./mcp-server --simulator http://10.65.99.13:8989 --mode both
终端2: ./mcp-server --simulator http://10.65.100.50:8989 --mode both
```

在 AI 客户端中分别配置两个 MCP 服务（见 `config-examples/` 目录）。

---

## 注意事项

1. **自动变化策略冲突**：如果测点有非 `apiupdate` 策略，API 写入会被拒绝
2. **写入前确认类型**：AI / DI / PI 使用不同的值字段
3. **AO/DO 只读**：只能读取 EGC 的遥控/遥调结果，不能写入
4. **路径用绝对路径**：AI 客户端启动 MCP Server 时使用绝对路径
5. **--mode both**：推荐合并模式，暴露全部 24 个工具

---

## 与其它 AI 客户端通用

MCP 是开放标准协议，本服务与任何支持 MCP 的客户端兼容：
- OpenCode
- Claude Desktop
- Cursor
- Windsurf
- Continue (VS Code)
- Kiro (支持 MCP 版本的)
- 其他 MCP 兼容客户端

配置方式都是配一个 JSON 文件指向 `mcp-server` 二进制即可（参考 `config-examples/` 目录）。
