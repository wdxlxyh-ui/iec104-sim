# IEC104 模拟器设计文档

## 1. 概述

基于 Go 语言的 IEC 60870-5-104 模拟器，纯命令行运行。

### 核心能力
- 通过 Excel 点表配置信息体地址、测点类型
- 响应总召（C_IC_NA_1）
- 值变更时立即变化上送（COT=3 spontaneous）
- HTTP API 外部修改数据并触发上送
- 接受 AO（遥调）/ DO（遥控）控制命令
- 支持遥测（YC）、遥信（YX）、遥脉（YM）
- 单进程 = 单端口 + 单客户端
- 前后台无界面，纯 CLI 操作

### 编译目标
| 系统 | 架构 | 备注 |
|------|------|------|
| Debian 11 | amd64 | 通用 Linux |
| Ubuntu 20.04+/22.04+ | amd64 | 通用 Linux |
| 银河麒麟 V10 | amd64 | 同 linux/amd64 |

## 2. 技术选型

| 层面 | 选择 | 理由 |
|------|------|------|
| 语言 | Go 1.21+ | 原生交叉编译，单二进制部署，CGO_ENABLED=0 纯静态 |
| IEC104 库 | github.com/wendy512/iec104 v1.0.4 | 维护活跃，服务端/客户端完备，支持总召/控制/变化上报 |
| Excel 解析 | github.com/xuri/excelize/v2 | 纯 Go 无依赖，支持 .xlsx |
| HTTP 路由 | 标准库 net/http + 内置 mux | 轻量无额外依赖 |
| 参数解析 | github.com/spf13/pflag | 支持 -p / --port 短长参数 |
| 日志 | golang.org/x/exp/slog 或 zerolog | 结构化控制台输出 |

## 3. 配置表格式

文件格式：`.xlsx`，默认文件名 `point.xlsx`

### 列定义

| 列 | 表头 | 类型 | 必填 | 说明 |
|----|------|------|------|------|
| A | point-name | string | 是 | 测点名称，如"母线电压" |
| B | point-number | uint32 | 是 | 信息体地址（IOA），全库唯一 |
| C | value-type | string | 是 | 数据类型：FLOAT / DOUBLE / INT / BIT |
| D | point-type | string | 是 | 测点类型：AI / DI / PI / DO / AO |
| E | efficient | float64 | 是 | 系数因子 |
| F | base-value | float64 | 是 | 基值/初始值 |
| G | alias | string | 否 | 别名或附加描述 |

### point-type → IEC104 类型映射

| point-type | 中文 | 功能 | IEC104 ASDU TypeID | 数据类型 |
|-----------|------|------|-------------------|---------|
| **AI** | 遥测 YC | 模拟量监视 | M_ME_NC_1 (13) | float32，值 = base × efficient |
| **DI** | 遥信 YX | 数字量监视 | M_SP_NA_1 (1) | bool，0/1 |
| **PI** | 遥脉 YM | 脉冲计数 | M_IT_NA_1 (15) | int32，计数值 |
| **DO** | 遥控 | 数字量控制 | C_SC_NA_1 (45) | 接受外部控制，更新 DI 点 |
| **AO** | 遥调 | 模拟量控制 | C_SE_NC_1 (48) | 接受外部控制，更新 AI 点 |

### value-type 处理规则

| value-type | AI(YC) 处理 | DI(YX) 处理 | PI(YM) 处理 |
|-----------|-------------|-------------|-------------|
| FLOAT | 直接存 float32 | - | - |
| DOUBLE | 转 float32 | - | - |
| INT | 转 float32 | - | 存 int32 |
| BIT | - | bool (0/1) | - |

## 4. 整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        iec104-sim (main)                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────────────┐    ┌─────────────────┐                     │
│  │   Config Loader  │    │   Point Library  │                    │
│  │  (excelize .xlsx)│───▶│  map[uint32]Point │                    │
│  │  解析 → 校验      │    │  sync.RWMutex    │                    │
│  └─────────────────┘    └────────┬─────────┘                    │
│                                  │                                │
│                                 ▼                                │
│  ┌────────────────────────────────────────────────────────┐     │
│  │                  Core Engine                            │     │
│  │  ┌──────────────────┐  ┌────────────────────────┐      │     │
│  │  │  IEC104 Server    │  │   Change Publisher     │      │     │
│  │  │  (单客户端)        │  │   (值变更→立即组包→发送)│      │     │
│  │  │  - 总召响应        │  │                         │      │     │
│  │  │  - AO/DO 控制     │  │                         │      │     │
│  │  │  - 变化上送        │  │                         │      │     │
│  │  └──────────────────┘  └────────────────────────┘      │     │
│  └────────────────────────────────────────────────────────┘     │
│                                                                  │
│  ┌────────────────────────────────────────────────────────┐     │
│  │              HTTP API Server (net/http)                 │     │
│  │  GET  /api/points              — 列出所有点             │     │
│  │  GET  /api/points/{ioa}       — 查询单个点             │     │
│  │  PUT  /api/points/{ioa}       — 修改点值 + 触发上送     │     │
│  │  POST /api/points/batch       — 批量修改               │     │
│  │  PUT  /api/points/{ioa}/qds   — 修改品质描述           │     │
│  │  GET  /api/status             — 模拟器运行状态          │     │
│  └────────────────────────────────────────────────────────┘     │
│                                                                  │
│  CLI: iec104-sim -p 2404 -c point.xlsx -H :8080 -l info         │
└─────────────────────────────────────────────────────────────────┘
```

## 5. 数据模型

```go
type PointType string
const (
    TypeAI PointType = "AI" // 遥测 YC → M_ME_NC_1
    TypeDI PointType = "DI" // 遥信 YX → M_SP_NA_1
    TypePI PointType = "PI" // 遥脉 YM → M_IT_NA_1
    TypeDO PointType = "DO" // 遥控     → C_SC_NA_1
    TypeAO PointType = "AO" // 遥调     → C_SE_NC_1
)

type ValueType string
const (
    VTFloat  ValueType = "FLOAT"
    VTDouble ValueType = "DOUBLE"
    VTInt    ValueType = "INT"
    VTBit    ValueType = "BIT"
)

type QualityDescriptor struct {
    Invalid    bool `json:"invalid"`
    NotTopical bool `json:"not_topical"`
    Substituted bool `json:"substituted"`
    Overflow   bool `json:"overflow"`
    Blocked    bool `json:"blocked"`
}

type Point struct {
    IOA       uint32            `json:"ioa"`
    Name      string            `json:"name"`
    ValueType ValueType         `json:"value_type"`
    PointType PointType         `json:"point_type"`
    Value     float64           `json:"value"`       // AI/DO 用浮点
    BoolValue bool              `json:"bool_value"`  // DI 用布尔
    IntValue  int32             `json:"int_value"`   // PI 用整数
    Efficient float64           `json:"efficient"`
    BaseValue float64           `json:"base_value"`
    QDS       QualityDescriptor `json:"qds"`
    Alias     string            `json:"alias"`
    Timestamp time.Time         `json:"timestamp"`
    Changed   bool              `json:"-"` // 内部变化标记
}

// 存储结构
type Store struct {
    mu     sync.RWMutex
    points map[uint32]*Point     // IOA 索引
    byType map[PointType][]*Point // 类型索引（用于总召分组）
}
```

## 6. 核心流程详细设计

### 6.1 启动流程

```
main()
  ├── 解析 CLI 参数（pflag）
  ├── 加载 Excel 配置（excelize）
  │     ├── 读取 Sheet "point"
  │     ├── 逐行解析、校验（IOA 唯一性、point-type 合法性）
  │     ├── 初始化值 = base-value × efficient
  │     └── 构建 Store
  ├── 启动 HTTP API（goroutine）
  ├── 启动 IEC104 Server
  │     ├── 绑定 -p 端口
  │     └── 等待客户端连接（阻塞/后台）
  └── 输出启动日志
```

### 6.2 总召处理

```
收到 C_IC_NA_1 (COT=6 activation)
  ↓
Store.ByType 获取分组
  ├── AI 组 → 遍历 → 构造 M_ME_NC_1 ASDU (COT=7) → Send
  ├── DI 组 → 遍历 → 构造 M_SP_NA_1 ASDU (COT=7) → Send
  └── PI 组 → 遍历 → 构造 M_IT_NA_1 ASDU (COT=7) → Send
  ↓
发送 C_IC_NA_1 ACT_TERM (COT=10) → 总召结束
```

### 6.3 变化上送

```
触发条件：
  ├── HTTP PUT /api/points/{ioa} 修改值
  ├── HTTP POST /api/points/batch 批量修改
  ├── AO/DO 控制命令到达（写库后）
  └── 内部 SetValue() 调用
  ↓
Store.SetValue(ioa, newValue)
  ├── 加锁写入
  ├── Point.Changed = true
  ├── 更新 Timestamp
  └── 通过 channel/回调 通知 Publisher
  ↓
Publisher 立即构造 ASDU (COT=3 spontaneous)
  └── 通过当前连接发送
```

### 6.4 AO/DO 控制

```
接收 C_SC_NA_1 (DO) 或 C_SE_NC_1 (AO)
  ↓
解析 ASDU，提取 IOA + 新值
  ↓
验证：IOA 存在于 Store && point-type 匹配
  ↓
如果匹配：
  ├── Store.SetValue(ioa, receivedValue)
  ├── 发送 ACT_CON (COT=7 激活确认)
  ├── 执行后发送 ACT_TERM (COT=10 终止)
  └── 变化上送新值
  ↓
如果不匹配：
  └── 发送 ACT_CON + 负确认 (COT=44 unknown IOA)
```

### 6.5 单客户端管理

```
状态: waiting / connected

连接到达 → 状态 = connected？
   ├── 是 → 拒绝（发送断开 / 直接 close）
   └── 否 → 接受，状态 = connected，记录 remoteAddr

客户端断开 → 状态 = waiting，等待新连接
```

## 7. HTTP API 规范

### 7.1 获取所有点

```
GET /api/points

Response 200:
{
  "points": [
    {
      "ioa": 1001,
      "name": "AI_01",
      "value_type": "FLOAT",
      "point_type": "AI",
      "value": 0.0,
      "bool_value": false,
      "int_value": 0,
      "efficient": 1.0,
      "base_value": 0.0,
      "qds": {"invalid":false, "not_topical":false, "substituted":false, "overflow":false, "blocked":false},
      "timestamp": "2026-05-10T10:00:00Z"
    },
    ...
  ]
}
```

### 7.2 查询单个点

```
GET /api/points/{ioa}

Response 200: {单个 Point JSON}
Response 404: {"error": "point not found", "ioa": 1001}
```

### 7.3 修改点值（核心接口）

```
PUT /api/points/{ioa}
Content-Type: application/json

Request Body:
{
  "value": 235.5       // AI/AO 用
}

或
{
  "bool_value": true   // DI/DO 用
}

或
{
  "int_value": 12345   // PI 用
}

Response 200:
{
  "success": true,
  "ioa": 1001,
  "value": 235.5,
  "changed": true,
  "spontaneous_sent": true
}

Response 404: {"error": "point not found"}
```

**内部流程：**
1. 查找 IOA → Store.Get(ioa)
2. 根据 point-type 判断写入 Value / BoolValue / IntValue
3. Store.SetValue() → 标记 Changed → 通知 Publisher
4. Publisher 组包发送 ASDU(COT=3)
5. 返回 200

### 7.4 批量修改

```
POST /api/points/batch
Content-Type: application/json

Request Body:
{
  "points": [
    {"ioa": 1001, "value": 235.5},
    {"ioa": 2001, "bool_value": true},
    {"ioa": 3001, "int_value": 9999}
  ]
}

Response 200:
{
  "success": true,
  "updated": 3,
  "failed": 0,
  "details": [
    {"ioa": 1001, "success": true},
    {"ioa": 2001, "success": true},
    {"ioa": 3001, "success": true}
  ]
}
```

### 7.5 修改品质描述

```
PUT /api/points/{ioa}/qds
Content-Type: application/json

Request Body:
{
  "invalid": false,
  "blocked": true
}

Response 200: {"success": true}
```

品质位变化不会触发变化上送（非值变化），但会在下次总召或变化上送时携带。

### 7.6 状态查询

```
GET /api/status

Response 200:
{
  "version": "1.0.0",
  "port": 2404,
  "client_connected": true,
  "client_addr": "192.168.1.100:54321",
  "total_points": 150,
  "uptime_seconds": 3600,
  "total_interrogations": 5,
  "total_controls": 12,
  "total_spontaneous": 48,
  "config_file": "point.xlsx"
}
```

## 8. CLI 参数与使用

```bash
# 基础用法
iec104-sim -p 2404 -c point.xlsx

# 指定 HTTP 端口
iec104-sim -p 2404 -c point.xlsx -H :9090

# 调试日志
iec104-sim -p 2404 -c point.xlsx -l debug

# 完整参数
iec104-sim --port 2404 --config point.xlsx --http :8080 --log info
```

| 参数 | 短形式 | 默认值 | 说明 |
|------|--------|--------|------|
| `--port` | `-p` | 2404 | IEC104 服务端 TCP 端口 |
| `--config` | `-c` | 必填 | 配置文件路径 (.xlsx) |
| `--http` | `-H` | `:8080` | HTTP API 监听地址 |
| `--log` | `-l` | `info` | 日志级别: debug / info / warn / error |

## 9. 日志规范

```
2026-05-10T10:00:00 INF 启动模拟器 port=2404 config=point.xlsx
2026-05-10T10:00:00 INF 点表加载完成 totalPoints=150 ai=120 di=25 pi=3 do=1 ao=1
2026-05-10T10:00:00 INF HTTP API 已启动 addr=:8080
2026-05-10T10:00:00 INF IEC104 服务端已启动 port=2404
2026-05-10T10:00:15 INF 客户端已连接 remote=192.168.1.100:54321
2026-05-10T10:00:20 INF 收到总召 remote=192.168.1.100 totalPoints=150
2026-05-10T10:01:00 INF 收到DO控制 ioa=2001 value=1 source=iec104
2026-05-10T10:01:00 INF 变化上送 ioa=2001 value=1 cot=3
2026-05-10T10:02:00 INF HTTP修改点值 ioa=1001 value=235.5 source=http
2026-05-10T10:02:00 INF 变化上送 ioa=1001 value=235.5 cot=3
2026-05-10T10:05:00 WRN 客户端断开 remote=192.168.1.100:54321
2026-05-10T10:05:30 INF 客户端已连接 remote=192.168.1.101:54322
```

## 10. 交叉编译

### 10.1 Makefile 目标

```makefile
.PHONY: build build-linux build-all clean

# 本地开发编译
build:
    go build -o bin/iec104-sim.exe .

# Linux amd64 (Debian/Ubuntu/银河麒麟共用)
build-linux:
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
    go build -ldflags="-s -w" -o bin/iec104-sim-linux-amd64 .

# 所有平台
build-all: build build-linux

# UPX 压缩（缩减体积~60%）
compress: build-linux
    upx --best bin/iec104-sim-linux-amd64 -o bin/iec104-sim-linux-amd64-upx

# 运行冒烟测试
smoke:
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
    go build -ldflags="-s -w" -o bin/iec104-sim-linux-amd64 .
    @echo "编译成功: bin/iec104-sim-linux-amd64"
    file bin/iec104-sim-linux-amd64

clean:
    rm -rf bin/
```

### 10.2 编译说明

```bash
# 1. 本地开发
go build -o bin/iec104-sim.exe .

# 2. Linux 部署二进制
make build-linux
# → bin/iec104-sim-linux-amd64  (~15MB, 静态编译无依赖)

# 3. 验证
file bin/iec104-sim-linux-amd64
# → ELF 64-bit LSB executable, x86-64, statically linked

# 4. 部署到目标系统
scp bin/iec104-sim-linux-amd64 user@debian11:~/iec104-sim
scp bin/iec104-sim-linux-amd64 user@kylin:~/iec104-sim

# 5. 在目标系统运行
chmod +x iec104-sim-linux-amd64
./iec104-sim-linux-amd64 -p 2404 -c point.xlsx
```

## 11. 项目目录结构

```
D:\AI\Claw\iec104-sim\
├── main.go                     # 入口：组装模块
├── go.mod
├── go.sum
├── Makefile                    # 编译目标
│
├── config/
│   ├── loader.go               # Excel 加载器
│   ├── model.go                # Point 数据结构定义
│   └── loader_test.go
│
├── library/
│   ├── store.go                # 内存数据存储（并发安全）
│   └── store_test.go
│
├── iec104/
│   ├── server.go               # IEC104 服务端封装
│   ├── handler_interrogation.go # 总召处理器
│   ├── handler_control.go       # AO/DO 控制处理器
│   ├── publisher.go            # 变化上送引擎
│   └── server_test.go
│
├── api/
│   ├── router.go               # HTTP 路由注册
│   ├── handler.go              # API 处理器
│   └── handler_test.go
│
└── samples/
    └── point.xlsx              # 示例点表
```

## 12. 开发阶段规划

| 阶段 | 内容 | 产出 |
|------|------|------|
| **P0** | 项目骨架 + Excel加载 + Point内存库 | `config/` `library/` 可加载配置并查询 |
| **P1** | IEC104服务端（连接管理 + 总召） | 客户端可连接并收到总召数据 |
| **P2** | HTTP API（查询/修改/批量/状态） | 可通过 HTTP 修改数据 |
| **P3** | 变化上送引擎 | HTTP修改值后客户端立即收到变化上送 |
| **P4** | AO/DO控制 | IEC104客户端可下发遥控遥调 |
| **P5** | 交叉编译 + Makefile + 测试 | 三平台二进制 + 冒烟测试 |
| **P6** | 补充文档 + 示例点表 | 完整交付 |

## 13. 依赖清单

```go
// go.mod
module iec104-sim

go 1.21

require (
    github.com/wendy512/iec104 v1.0.4
    github.com/xuri/excelize/v2 v2.8.1
    github.com/spf13/pflag v1.0.5
)
```

---

## 附录：典型使用场景

### 场景1：变电站遥测遥信仿真

```bash
# 终端1：启动模拟器（变电站A，端口2404）
./iec104-sim -p 2404 -c substation_a.xlsx -H :8080

# 终端2：接线员站连接（IEC104客户端工具）
iec104-client -p 2404

# 终端3：模拟遥测值变化触发告警
curl -X PUT http://localhost:8080/api/points/1001 \
  -H 'Content-Type: application/json' \
  -d '{"value": 235.5}'
# → 接线员站实时看到值从220.0跳变到235.5
```

### 场景2：遥控测试

```bash
# 通过 IEC104 客户端下发分闸命令 → 模拟器接收并确认
# 同时变化上送遥信状态 → 客户端收到断路器变位
```

### 场景3：多进程独立部署

```bash
# 进程1：220kV变电站
./iec104-sim -p 2404 -c 220kv_station.xlsx -H :8080

# 进程2：110kV变电站
./iec104-sim -p 2405 -c 110kv_station.xlsx -H :8081
```

### 场景4：品质位模拟（测试异常场景）

```bash
# 模拟遥测数据无效（品质位标记）
curl -X PUT http://localhost:8080/api/points/1001/qds \
  -H 'Content-Type: application/json' \
  -d '{"invalid": true, "blocked": true}'
# → 下次总召或变化上送时携带 QDS=invalid
```
