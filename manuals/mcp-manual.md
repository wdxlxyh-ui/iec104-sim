# IEC104 模拟器 — MCP 服务器使用手册

> 版本: 1.1.0 | 更新日期: 2026-05-20

---

## 目录

1. [概述](#1-概述)
2. [安装](#2-安装)
3. [运行配置](#3-运行配置)
4. [工具参考](#4-工具参考)
5. [AI 客户端集成](#5-ai-客户端集成)
6. [使用场景与示例](#6-使用场景与示例)
7. [策略参数速查](#7-策略参数速查)
8. [故障排除](#8-故障排除)

---

## 1. 概述

MCP（Model Context Protocol）服务器为 AI Agent（如 Claude、Cursor 等）提供 IEC104 模拟器的程序化控制接口。AI 助手可以通过 MCP 协议直接操控模拟器，实现自动化测试、设备仿真和数据监控。

### 核心能力

- **实例管理**：创建、启动、停止、删除模拟器实例
- **测点数据读写**：单点和批量读取/写入测点值
- **自动变化策略配置**：配置和管理 11 种自动变化策略，支持批量配置
- **文件管理**：上传点表和 CSV 回放文件，列出可用文件
- **配置导出导入**：批量导出/导入自动变化配置
- **测点数据导出**：导出测点实时数据为 CSV
- **品质描述管理**：更新测点 QDS（无效/非当前/替代/溢出/闭锁）
- **协议查询**：查询模拟器支持的协议类型

### 两种运行模式

| 模式 | 说明 | 适用场景 |
|------|------|----------|
| `instance` | 仅实例管理工具 | 只需要管理实例生命周期 |
| `data` | 仅数据接口工具 | 只需要读写测点和策略配置 |
| `both` | 全部工具（推荐） | 完整控制模拟器 |

---

## 2. 安装

### 2.1 从发行包获取

发行包已包含 MCP 服务器程序：

```
iec104-sim-v2.2.0-linux-amd64/
└── bin/
    └── iec104-mcp          ← MCP 服务器
```

### 2.2 从源码编译

```bash
# 编译 MCP 服务器
cd iec104-sim-master
go build -o bin/iec104-mcp ./cmd/mcp-server/

# 交叉编译（其他平台）
GOOS=linux GOARCH=arm64 go build -o bin/iec104-mcp-arm64 ./cmd/mcp-server/
GOOS=windows GOARCH=amd64 go build -o bin/iec104-mcp.exe ./cmd/mcp-server/
```

### 2.3 验证安装

```bash
./bin/iec104-mcp -h
```

输出应显示：
```
Usage of ./bin/iec104-mcp:
  -mode string
        MCP 服务模式: instance / data / both (default "both")
  -simulator string
        IEC104 模拟器的 HTTP API 地址 (default "http://localhost:8989")
```

---

## 3. 运行配置

### 3.1 前提条件

MCP 服务器需要连接一个正在运行的 IEC104 模拟器（HTTP API 服务）。确保模拟器已启动：

```bash
# 启动模拟器（服务模式）
./iec104-sim serve --http :8989 --config-dir ./config --log-dir ./logs
```

### 3.2 启动 MCP 服务器

```bash
# 完整模式（推荐）
./bin/iec104-mcp -simulator http://localhost:8989 -mode both

# 仅实例管理
./bin/iec104-mcp -simulator http://localhost:8989 -mode instance

# 仅数据接口
./bin/iec104-mcp -simulator http://localhost:8989 -mode data
```

### 3.3 参数说明

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-simulator` | `http://localhost:8989` | IEC104 模拟器 HTTP API 地址 |
| `-mode` | `both` | 运行模式：instance / data / both |

### 3.4 通信协议

MCP 服务器使用 **stdio 协议**，通过标准输入/输出与 AI 客户端通信。这意味着：

- 不需要监听网络端口
- 由 AI 客户端（如 Claude Desktop）负责启动和管理 MCP 进程
- 适用于本地部署场景

---

## 4. 工具参考

### 4.1 实例管理工具

#### list_instances

列出所有已配置的模拟器实例及其运行状态。

**参数：** 无

**返回示例：**
```json
{
  "instances": [
    {
      "id": "a1b2c3d4e5f6",
      "name": "关口表",
      "iec104_port": 2404,
      "xlsx_file": "samples/point.xlsx",
      "status": "running",
      "stats": {
        "uptime_seconds": 3600,
        "total_points": 6,
        "client_connected": true,
        "interrogations": 5,
        "controls": 12,
        "spontaneous": 88
      }
    }
  ]
}
```

---

#### get_instance

获取单个实例的详细配置和状态信息。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| instance_id | string | 是 | 实例 ID |

---

#### create_instance

创建新的模拟器实例。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| config | string (JSON) | 是 | 实例配置 JSON |

**config 格式：**
```json
{
  "name": "变电站A",
  "iec104_port": 2404,
  "xlsx_file": "固定验证-关口表.xlsx"
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| name | 是 | 实例名称 |
| iec104_port | 是 | IEC104 端口号 (1-65535) |
| xlsx_file | 是 | 点表文件名（需先上传） |
| enabled | 否 | 是否启用（默认 false） |

---

#### update_instance

更新已有实例的配置。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| instance_id | string | 是 | 实例 ID |
| config | string (JSON) | 是 | 更新后的配置 |

支持部分更新，只需填写需要修改的字段。运行中的实例会先停止再更新。

---

#### delete_instance

删除实例配置。运行中的实例会先停止再删除。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| instance_id | string | 是 | 实例 ID |

---

#### start_instance

启动指定实例。实例需要已经创建且配置正确。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| instance_id | string | 是 | 实例 ID |

---

#### stop_instance

停止指定实例。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| instance_id | string | 是 | 实例 ID |

---

#### restart_instance

重启指定实例。如实例未运行则直接启动。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| instance_id | string | 是 | 实例 ID |

---

#### get_server_status

获取模拟器的全局状态信息。

**参数：** 无

**返回示例：**
```json
{
  "version": "2.2.0",
  "mode": "serve",
  "configured": 3,
  "running": 1,
  "stopped": 2,
  "max": 10
}
```

| 字段 | 说明 |
|------|------|
| version | 模拟器版本号 |
| mode | 运行模式（serve） |
| configured | 已配置的实例总数 |
| running | 运行中的实例数 |
| stopped | 已停止的实例数 |
| max | 最大实例数限制（10） |

---

### 4.2 数据接口工具

#### list_points

列出实例的所有测点及其当前值。按 AI 优先 + IOA 升序排列。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| instance_id | string | 是 | 实例 ID |

**返回示例：**
```json
{
  "points": [
    {
      "ioa": 16385,
      "name": "母线电压",
      "point_type": "AI",
      "value": 100.5,
      "bool_value": false,
      "int_value": 0,
      "updated_at": "2026-05-16T10:00:00.000Z",
      "unit": ""
    }
  ],
  "refreshed_at": "2026-05-16T10:00:00.000Z"
}
```

---

#### read_point

读取单个测点的当前值。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| instance_id | string | 是 | 实例 ID |
| ioa | number | 是 | 信息体地址 |

---

#### read_points

批量读取多个测点的当前值。不传 `ioas` 则返回全部测点。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| instance_id | string | 是 | 实例 ID |
| ioas | number[] | 否 | 要读取的 IOA 列表（不传则返回全部） |

---

#### write_point

写入单个测点的值。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| instance_id | string | 是 | 实例 ID |
| ioa | number | 是 | 信息体地址 |
| value | number | 否 | 浮点数值（AI 遥测使用），DI 也可用（非零=true） |
| bool_value | boolean | 否 | 布尔值（DI 遥信使用） |
| int_value | number | 否 | 整数值（PI 遥脉使用） |

根据测点类型传入对应字段：

| 测点类型 | 使用字段 |
|----------|----------|
| AI (遥测) | `value` |
| DI (遥信) | `bool_value` 或 `value`（非零=true） |
| PI (遥脉) | `int_value` |

---

#### write_points ⭐ （核心工具）

**批量写入多个测点的值。** 一次调用写入多个 IOA，模拟真实设备同一时刻上报数据。这是自动化测试的关键接口。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| instance_id | string | 是 | 实例 ID |
| points | array | 是 | 要写入的测点列表 |

**points 元素格式：**
```json
{
  "ioa": 16385,
  "value": 235.5
}
```

每个元素可包含：
- `ioa` (number, 必填）： 信息体地址
- `value` (number, 可选）： 浮点值
- `bool_value` (boolean, 可选）： 布尔值
- `int_value` (number, 可选）： 整数值

---

#### config_auto_change

配置测点的自动变化策略。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| instance_id | string | 是 | 实例 ID |
| ioa | number | 是 | 信息体地址 |
| strategy | string | 是 | 策略类型（见下方列表） |
| enabled | boolean | 是 | 是否启用 |
| params | string (JSON) | 是 | 策略参数 JSON 字符串 |

**支持的策略类型：**
| 策略 | 标识 | 说明 |
|------|------|------|
| 递增 | `increment` | 每周期 += 步长，达最大值后回起始值 |
| 随机 | `random` | 在 [min, max] 范围内随机取值 |
| CSV 回放 | `csv` | 按 CSV 文件时间序列播放 |
| 取大 | `max` | 取多个关联 IOA 的最大值 |
| 取小 | `min` | 取多个关联 IOA 的最小值 |
| SOC 计算 | `soc` | 基于功率积分计算电池荷电状态 |
| 电量统计 | `energy` | 基于功率积分计算累计电量 |
| AO 关联 | `aofollow` | 跟随指定 AO 点的遥控值变化 |
| 接口更新 | `apiupdate` | 仅允许 HTTP API 写入 |
| 手动 | `manual` | 不自动计算，需 API 置数 |
| 自定义公式 | `custom` | 四则运算公式计算 |

**各策略 params 示例：**

```json
// 递增
{"start_value":0, "step":1, "period_ms":1000, "max_value":100}

// 随机
{"min_value":0, "max_value_r":100, "period_ms":1000, "decimal_places":2}

// CSV 回放
{"csv_file":"data.csv", "time_format":"auto"}

// MAX/MIN
{"para_a":"16385,16386,16387"}

// SOC
{"init_soc":50, "rated_cap":1000, "power_ioa":16385, "integral_ms":1000}

// 电量
{"init_energy":0, "stat_type":0, "energy_power_ioa":16385, "energy_period_ms":1000}

// AO 关联
{"follow_ao_ioa":16390}

// 接口更新
{"api_init_value":0}

// 手动
{}

// 自定义公式
{"custom_ioas":"16385,16386,16387", "custom_formula":"({0}+{1})*{2}/2", "period_ms":1000}
```

---

#### get_auto_change

查看测点的自动变化配置。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| instance_id | string | 是 | 实例 ID |
| ioa | number | 是 | 信息体地址 |

---

#### delete_auto_change

删除测点的自动变化配置。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| instance_id | string | 是 | 实例 ID |
| ioa | number | 是 | 信息体地址 |

---

#### export_auto_changes

导出实例所有自动变化配置为 CSV 表格。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| instance_id | string | 是 | 实例 ID |

---

#### import_auto_changes

从 CSV 内容导入自动变化配置。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| instance_id | string | 是 | 实例 ID |
| csv_content | string | 是 | CSV 内容（格式见下方） |

**CSV 格式：**
```csv
信息体地址,测点名称,自动变化模式,A,B,C,D,E,F,G
,代码说明,1=递增 2=随机 3=CSV 4=取大 5=取小 6=SOC 7=电量 8=AO关联 9=接口更新 10=手动 11=自定义公式,,,,,,,
,递增(A~D),A=起始值 B=步长 C=周期(ms) D=最大值,,,,,,,
16385,母线电压,1,0,1,1000,100,,,,
16386,线路电流,2,0,100,1000,2,,,,
```

---

#### upload_csv

上传 CSV 时间序列文件，用于 CSV 回放策略。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| instance_id | string | 是 | 实例 ID |
| csv_content | string | 是 | CSV 文件内容 |

**CSV 格式要求：**
```csv
时间戳,值
2026-05-16 10:00:00,100.0
2026-05-16 10:00:01,101.5
2026-05-16 10:00:02,102.3
```

---

#### upload_file

上传 .xlsx 点表文件到模拟器 config 目录。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| filename | string | 是 | 文件名，如"固定验证-关口表.xlsx" |
| content_base64 | string | 是 | 文件内容的 base64 编码 |

---

#### export_points_csv

导出实例所有测点实时数据为 CSV 格式（信息体地址/名称/类型/值/时间）。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| instance_id | string | 是 | 实例 ID |

---

#### batch_config_auto_change

批量配置多个测点的自动变化策略。一次调用为多个 IOA 应用同一策略配置。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| instance_id | string | 是 | 实例 ID |
| ioas | number[] | 是 | 要配置的 IOA 列表 |
| strategy | string | 是 | 策略类型 |
| enabled | boolean | 是 | 是否启用 |
| params | string (JSON) | 否 | 策略参数 JSON 字符串 |

**使用示例：**
```python
batch_config_auto_change(
    instance_id="a1b2c3d4e5f6",
    ioas=[16385, 16386, 16387],
    strategy="increment",
    enabled=True,
    params='{"start_value":0,"step":1,"period_ms":1000,"max_value":100}'
)
```

---

#### update_qds

更新测点的品质描述 QDS（传统模式 API）。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| ioa | number | 是 | 信息体地址 |
| invalid | boolean | 否 | 无效标志 |
| not_topical | boolean | 否 | 非当前标志 |
| substituted | boolean | 否 | 替代标志 |
| overflow | boolean | 否 | 溢出标志 |
| blocked | boolean | 否 | 闭锁标志 |

**使用示例：**
```python
update_qds(
    ioa=16385,
    invalid=True,
    blocked=True
)
```

---

## 4.3 全局工具

#### list_files

列出 config 目录下所有 .xlsx 点表文件。

**参数：** 无

**返回示例：**
```json
{
  "files": [
    {
      "name": "固定验证-关口表.xlsx",
      "size": 8192,
      "modtime": "2026-05-20T10:00:00Z"
    }
  ]
}
```

---

#### get_protocols

查询模拟器支持的协议类型。

**参数：** 无

**返回示例：**
```json
{
  "protocols": ["iec104", "modbus_tcp"]
}
```

---

## 5. AI 客户端集成

### 5.1 Claude Desktop

在 Claude Desktop 配置文件中添加 MCP 服务器（`claude_desktop_config.json`）：

```json
{
  "mcpServers": {
    "iec104-sim": {
      "command": "/path/to/iec104-mcp",
      "args": ["-simulator", "http://localhost:8989", "-mode", "both"]
    }
  }
}
```

**配置步骤：**
1. 打开 Claude Desktop → Settings → Developer
2. 点击 Edit Config
3. 在 `claude_desktop_config.json` 中添加上述配置
4. 重启 Claude Desktop
5. 点击工具按钮 🔧 查看可用工具列表

### 5.2 Cursor

在 Cursor 的 MCP 配置中添加：

```json
{
  "mcpServers": {
    "iec104-sim": {
      "command": "/path/to/iec104-mcp",
      "args": ["-simulator", "http://localhost:8989", "-mode", "both"]
    }
  }
}
```

### 5.3 其他 MCP 客户端

任何支持 MCP stdio 协议的客户端均可使用，配置格式相同：

```
command: /path/to/iec104-mcp
args: [-simulator, http://localhost:8989, -mode, both]
```

---

## 6. 使用场景与示例

### 场景 1：自动化测试环境初始化

AI Agent 通过 MCP 自动搭建测试环境：

```
步骤：
1. upload_file → 上传点表文件
2. create_instance → 创建实例
3. start_instance → 启动实例
4. 验证 list_instances → 确认实例运行中
```

### 场景 2：批量置数模拟设备状态

```python
# 模拟变电站设备同一时刻上报数据
write_points(
    instance_id="a1b2c3d4e5f6",
    points=[
        {"ioa": 16385, "value": 235.5},   # 母线电压
        {"ioa": 16386, "value": 236.2},   # 另一段母线
        {"ioa": 16387, "value": 220.1},   # 出线电压
        {"ioa": 5, "bool_value": true},   # 断路器合闸
        {"ioa": 10, "int_value": 1500}    # 电度表读数
    ]
)
```

### 场景 3：配置自动变化策略

```python
# 配置关口表有功功率递增变化
config_auto_change(
    instance_id="a1b2c3d4e5f6",
    ioa=16385,
    strategy="increment",
    enabled=True,
    params='{"start_value":0,"step":1,"period_ms":1000,"max_value":100}'
)

# 配置储能 SOC 计算
config_auto_change(
    instance_id="a1b2c3d4e5f6",
    ioa=16390,
    strategy="soc",
    enabled=True,
    params='{"init_soc":50,"rated_cap":1000,"power_ioa":16385,"integral_ms":1000}'
)
```

### 场景 4：读取并分析测点数据

```
1. list_points(instance_id="...") → 获取所有测点
2. 分析测点数据 → 判断设备状态
3. 如有异常 → write_point 调整值
```

### 场景 5：CSV 回放真实数据

```python
# 上传录波数据
upload_csv(
    instance_id="a1b2c3d4e5f6",
    csv_content="""时间戳,值
2026-05-16 10:00:00,100.0
2026-05-16 10:00:01,101.5
2026-05-16 10:00:02,98.3
2026-05-16 10:00:03,102.1"""
)

# 配置 CSV 回放策略
config_auto_change(
    instance_id="a1b2c3d4e5f6",
    ioa=16385,
    strategy="csv",
    enabled=True,
    params='{"csv_file":"replay_data.csv"}'
)
```

### 场景 6：实例全生命周期管理

```
1. list_instances → 查看现有实例
2. stop_instance → 停止不需要的实例
3. update_instance → 修改配置（端口/名称）
4. start_instance → 重新启动
5. delete_instance → 删除废弃实例
```

---

## 7. 策略参数速查

### params JSON 字段速查表

| 策略 | 所需参数 |
|------|----------|
| increment | `start_value`, `step`, `period_ms`, `max_value` |
| random | `min_value`, `max_value_r`, `period_ms`, `decimal_places` |
| csv | `csv_file`, `time_format`(可选), `time_unit`(可选) |
| max | `para_a`(IOA列表,逗号分隔) |
| min | `para_a`(IOA列表,逗号分隔) |
| soc | `init_soc`, `rated_cap`, `power_ioa`, `integral_ms` |
| energy | `init_energy`, `stat_type`, `energy_power_ioa`, `energy_period_ms` |
| aofollow | `follow_ao_ioa` |
| apiupdate | `api_init_value`(可选) |
| manual | 无需参数 |
| custom | `custom_ioas`(关联IOA列表), `custom_formula`, `period_ms` |

### 通用参数说明

| 参数 | 适用策略 | 类型 | 说明 |
|------|----------|------|------|
| start_value | increment | float | 起始值 |
| step | increment | float | 每周期步长 |
| period_ms | increment, random, custom | int | 执行周期(毫秒, ≥100) |
| max_value | increment | float | 最大值 |
| min_value | random | float | 随机最小值 |
| max_value_r | random | float | 随机最大值 |
| decimal_places | random | int | 小数位数 |
| csv_file | csv | string | CSV 文件名 |
| time_format | csv | string | CSV 时间格式 |
| para_a | max, min | string | 关联 IOA 列表 |
| init_soc | soc | float | 初始 SOC(%) |
| rated_cap | soc | float | 额定容量(kWh) |
| power_ioa | soc | uint32 | 功率 AI 点号 |
| integral_ms | soc | int | 积分周期(ms) |
| init_energy | energy | float | 初始电量(kWh) |
| stat_type | energy | int | 统计类型(0充/1放) |
| energy_power_ioa | energy | uint32 | 功率 AI 点号 |
| energy_period_ms | energy | int | 积分周期(ms) |
| follow_ao_ioa | aofollow | uint32 | 关联 AO 点号 |
| api_init_value | apiupdate | float | 初始值 |
| custom_ioas | custom | string | 关联 IOA 列表(逗号分隔) |
| custom_formula | custom | string | 计算公式 |

---

## 8. 故障排除

### 8.1 连接失败

**错误信息：**
```
Error: dial tcp connection refused
```

**检查项：**
1. IEC104 模拟器是否已启动（HTTP API 端口是否监听）
2. `-simulator` 参数的地址和端口是否正确
3. 防火墙是否阻止了连接

### 8.2 测点写入失败

**错误信息：**
```
错误: point not found
```

**检查项：**
1. 测点 IOA 是否存在于实例的点表中
2. 实例是否已启动（运行中）
3. 对于有自动变化策略的测点，需先停止策略（`apiupdate`/`manual` 策略除外）

### 8.3 实例操作失败

**错误信息：**
```
HTTP 409: port already in use
```

**检查项：**
1. IEC104 端口是否被其他进程占用
2. 端口是否已被其他实例使用
3. 实例数量是否已达上限（最大 10 个）

### 8.4 自动变化不生效

**检查项：**
1. 确认 `enabled` 参数为 `true`
2. 确认 `period_ms` ≥ 100
3. 确认测点类型为 AI（AO/DO 不支持自动变化）
4. 检查模拟器日志中的任务启动/停止信息

### 8.5 CSV 导入/上传问题

**检查项：**
1. CSV 内容是否为纯文本格式（UTF-8 编码）
2. CSV 表头是否与导出格式一致
3. 文件名是否包含特殊字符
4. 文件大小是否在限制范围内

### 8.6 MCP 工具无响应

**检查项：**
1. 检查模拟器日志 `logs/output.log` 中的错误信息
2. 确认 MCP 进程是否仍在运行
3. 重启 MCP 服务器重试
4. AI 客户端中检查 MCP 工具是否已加载（工具按钮或命令面板）

---

## 附录：API 映射

MCP 工具与底层 HTTP API 的对应关系：

| MCP 工具 | HTTP API |
|----------|----------|
| list_instances | GET /api/v1/instances |
| get_instance | GET /api/v1/instances/{id} |
| create_instance | POST /api/v1/instances |
| update_instance | PUT /api/v1/instances/{id} |
| delete_instance | DELETE /api/v1/instances/{id} |
| start_instance | POST /api/v1/instances/{id}/start |
| stop_instance | POST /api/v1/instances/{id}/stop |
| restart_instance | POST /api/v1/instances/{id}/restart |
| get_server_status | GET /api/v1/status |
| list_points | GET /api/v1/instances/{id}/points |
| read_point | GET /api/v1/instances/{id}/points/{ioa} |
| read_points | GET /api/v1/instances/{id}/points（过滤） |
| write_point | PUT /api/v1/instances/{id}/points/{ioa} |
| write_points | POST /api/v1/instances/{id}/points/batch |
| config_auto_change | PUT /api/v1/instances/{id}/points/auto-change/{ioa} |
| get_auto_change | GET /api/v1/instances/{id}/points/auto-change/{ioa} |
| delete_auto_change | DELETE /api/v1/instances/{id}/points/auto-change/{ioa} |
| export_auto_changes | GET /api/v1/instances/{id}/points/auto-change/export |
| import_auto_changes | POST /api/v1/instances/{id}/points/auto-change/import |
| upload_csv | POST /api/v1/instances/{id}/upload-csv |
| upload_file | POST /api/v1/upload |
| export_points_csv | GET /api/v1/instances/{id}/points/export |
| batch_config_auto_change | PUT /api/v1/instances/{id}/points/auto-change/batch |
| update_qds | PUT /api/points/{ioa}/qds |
| list_files | GET /api/v1/files |
| get_protocols | GET /api/v1/protocols |
