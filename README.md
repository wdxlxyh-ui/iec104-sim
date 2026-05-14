# IEC 60870-5-104 模拟器

[![Go Version](https://img.shields.io/badge/Go-1.21%2B-blue)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)

**IEC104 从站模拟器**，用于变电站自动化测试。支持多实例并行运行，模拟 RTU 及间隔层设备，适用于 SCADA 系统开发和集成测试。

---

## 版本历史

| 版本 | 日期 | 说明 |
|------|------|------|
| **v2.1.3** | 2026-05 | **手动置数策略** — 新增第10种自动变化策略 "manual"，引擎不自动计算，值由用户 API 置数，适用于外部系统联调场景 |
| **v2.1** | 2026-05 | **重大更新** — 实例详情页，支持测点高频刷新、置数、10种自动变化策略、批量配置、导出/导入 |
| v2.0.1 | 2026-05 | HTTP 开关控制、API 文档、iptables 防火墙自动管理 |
| v2.0 | 2026-05 | 多实例管理模式（serve 子命令）、Vue 3 Web 管理界面 |
| v1.0 | - | 传统单实例模式 |

### v2.1.3 新特性

- **手动置数策略** — 新增第10种自动变化策略"手动"，该模式下引擎不自动计算值，需通过 API 手动置数，适用于外部系统联调场景
- 导出 CSV 时，code 10 对应"手动"策略
- 导入 CSV 时，支持解析 code 10 并配置 manual 策略
- MCP 工具 `config_auto_change` 策略参数支持 `manual`

### v2.1 新特性

- **实例详情页** — 测点值 100ms~1000ms 可调高频轮询刷新
- **置数操作** — AI 数值输入、DI 开关（ON/OFF）、PI 整数输入，置数成功绿色提示
- **自动变化引擎（10种策略）**
  - 递增 / 随机 / CSV 回放 / MAX / MIN / SOC 计算 / 电量统计
  - **新增** AO关联（AO被遥控时同步更新关联点）
  - **新增** 接口更新（仅允许 HTTP API 写入）
  - **新增** 手动（不自动计算，需 API 置数）
  - **新增** 接口更新（仅允许 HTTP API 写入，其余拒绝）
- **批量配置** — 勾选多个测点，批量应用自动变化策略
- **导出/导入** — 自动变化配置 JSON 导出/导入
- **CSV 导出** — 测点实时数据导出为 CSV（信息体地址/名称/类型/值/时间）
- **CSV 上传** — 为 CSV 回放策略上传时间序列文件
- **置数隔离** — 置数值独立存储，不随实时数据刷新覆盖

---

## 功能特性

- **双运行模式**
  - **传统模式** — 单进程 = 单端口 + 单客户端，纯 CLI
  - **服务模式** (`serve`) — 多实例生命周期管理 + Vue 3 Web 界面
- **Excel 点表配置** — 在 `.xlsx` 中定义 IOA、点类型、系数、初始值
- **完整 IEC 104 协议支持**
  - 遥测 AI (M_ME_NC_1)、遥信 DI (M_SP_NA_1)、遥脉 PI (M_IT_NA_1)
  - 遥控 DO (C_SC_NA_1)、遥调 AO (C_SE_NC_1)
  - 总召唤 (C_IC_NA_1)、电度召唤 (C_CI_NA_1)
  - 变化上送 (COT=3)
  - 品质描述 QDS：无效 / 非当前 / 替代 / 溢出 / 闭锁
- **RESTful HTTP API** — 查询、单点/批量更新、修改 QDS
- **Web 管理界面** — 实例增删改查、启停、运行监控（自动刷新）
- **运行统计** — 运行时长、总召唤次数、遥控次数、变化上送次数
- **单客户端限制** — 每个实例只接受一个客户端连接
- **跨平台编译** — Linux amd64/arm64、Windows amd64、`.deb` 打包
- **自动变更策略** — 9种内置策略，后台独立 goroutine 调度，实例启停自动管理

---

## 技术栈

| 层 | 选型 |
|------|-------|
| 语言 | Go 1.21+ |
| IEC 104 库 | [go-iecp5](https://github.com/wendy512/go-iecp5) |
| Excel 解析 | [excelize](https://github.com/xuri/excelize/v2) v2 |
| 前端 | Vue 3 + TypeScript + Element Plus |
| 构建工具 | Vite |
| 命令行 | [pflag](https://github.com/spf13/pflag) |

---

## 快速开始

### 方式一：下载压缩包（推荐）

```bash
tar xzf iec104-sim-v2.1.tar.gz
cd iec104-sim-v2.1
./start.sh
# 浏览器访问 http://localhost:8989
```

### 方式二：源码构建

```bash
# 构建前端（需要 Node.js）
cd web && npm install && npm run build && cd ..

# 构建后端
go build -o bin/iec104-sim ./cmd/iec104-sim/

# 启动服务
./bin/iec104-sim serve --http :8989 --config-dir ./config --log-dir ./logs

# 浏览器访问 http://localhost:8989
```

### 传统模式（单实例）

```bash
go build -o bin/iec104-sim ./cmd/iec104-sim/
./bin/iec104-sim -p 2404 -c samples/point.xlsx -H :8080 -l info
```

### 命令行参数

| 参数 | 缩写 | 默认值 | 说明 |
|------|------|--------|------|
| `--port` | `-p` | 2404 | IEC 104 服务端 TCP 端口（传统模式） |
| `--config` | `-c` | 必填 | `.xlsx` 配置文件路径（传统模式） |
| `--http` | `-H` | `:8989` | HTTP API 监听地址 |
| `--log` | `-l` | `info` | 日志级别: debug / info / warn / error |
| `--config-dir` | `-c` | `./config` | 配置文件目录（服务模式） |
| `--log-dir` | `-L` | `./logs` | 日志文件目录（服务模式） |

---

## 点表配置（Excel）

格式：`.xlsx`，工作表名称 `point`。

| 列 | 表头 | 类型 | 必填 | 说明 |
|--------|--------|------|----------|-------------|
| A | point-name | string | 是 | 测点名称，如"母线电压" |
| B | point-number | uint32 | 是 | IOA，同类型测点唯一 |
| C | value-type | string | 是 | 数据类型：FLOAT / DOUBLE / INT / BIT |
| D | point-type | string | 是 | 测点类型：AI / DI / PI / DO / AO |
| E | efficient | float64 | 是 | 系数 |
| F | base-value | float64 | 是 | 初始值 |
| G | alias | string | 否 | 别名或描述 |

### 测点类型说明

| 类型 | 中文 | 功能 | IEC 104 类型标识 | 数据 |
|------|---------|----------|----------------|------|
| AI | 遥测 YC | 模拟量监视 | M_ME_NC_1 (13) | float32, value = base × efficient |
| DI | 遥信 YX | 数字量监视 | M_SP_NA_1 (1) | bool 0/1 |
| PI | 遥脉 YM | 脉冲计数 | M_IT_NA_1 (15) | int32 |
| DO | 遥控 | 远程控制 | C_SC_NA_1 (45) | 接收外部控制，更新 DI 点 |
| AO | 遥调 | 远程调节 | C_SE_NC_1 (48) | 接收外部控制，更新 AI 点 |

---

## HTTP API

### 实例管理 API（服务模式）

| 方法 | 端点 | 说明 |
|--------|----------|-------------|
| `GET` | `/api/v1/instances` | 列出所有已配置实例 |
| `POST` | `/api/v1/instances` | 创建新实例配置 |
| `GET` | `/api/v1/instances/{id}` | 获取实例详情 |
| `PUT` | `/api/v1/instances/{id}` | 更新实例配置 |
| `DELETE` | `/api/v1/instances/{id}` | 删除实例配置 |
| `POST` | `/api/v1/instances/{id}/start` | 启动实例 |
| `POST` | `/api/v1/instances/{id}/stop` | 停止实例 |
| `POST` | `/api/v1/instances/{id}/restart` | 重启实例 |
| `GET` | `/api/v1/status` | 全局服务状态 |
| `POST` | `/api/v1/upload` | 上传 `.xlsx` 点表文件 |

### 详情页 API（v2.1 新增）

| 方法 | 端点 | 说明 |
|--------|----------|-------------|
| `GET` | `/api/v1/instances/{id}/points` | 获取所有测点实时快照 |
| `GET` | `/api/v1/instances/{id}/points/{ioa}` | 获取单个测点快照 |
| `PUT` | `/api/v1/instances/{id}/points/{ioa}` | 置数（写入点值） |
| `GET` | `/api/v1/instances/{id}/points/auto-change/{ioa}` | 获取自动变化配置 |
| `PUT` | `/api/v1/instances/{id}/points/auto-change/{ioa}` | 配置自动变化 |
| `DELETE` | `/api/v1/instances/{id}/points/auto-change/{ioa}` | 删除自动变化配置 |
| `PUT` | `/api/v1/instances/{id}/points/auto-change/batch` | 批量配置自动变化 |
| `GET` | `/api/v1/instances/{id}/points/auto-change/export` | 导出自动变化配置 |
| `POST` | `/api/v1/instances/{id}/points/auto-change/import` | 导入自动变化配置 |
| `GET` | `/api/v1/instances/{id}/points/export` | 导出测点 CSV 数据 |
| `POST` | `/api/v1/instances/{id}/upload-csv` | 上传 CSV 回放文件 |

### 实例级 API（传统模式）

| 方法 | 端点 | 说明 |
|--------|----------|-------------|
| `GET` | `/api/points` | 列出所有测点 |
| `GET` | `/api/points/{ioa}` | 获取单个测点 |
| `PUT` | `/api/points/{ioa}` | 更新测点值 + 触发变化上送 |
| `POST` | `/api/points` | 批量更新测点值 |
| `PUT` | `/api/points/{ioa}/qds` | 更新品质描述 QDS |
| `GET` | `/api/status` | 服务运行状态 |

### 置数示例

```bash
# 设置遥测值
curl -X PUT http://localhost:8989/api/v1/instances/{id}/points/16385 \
  -H 'Content-Type: application/json' \
  -d '{"value": 235.5}'

# 设置遥信值（开关量）
curl -X PUT http://localhost:8989/api/v1/instances/{id}/points/5 \
  -H 'Content-Type: application/json' \
  -d '{"bool_value": true}'

# 配置自动变化（递增策略）
curl -X PUT http://localhost:8989/api/v1/instances/{id}/points/auto-change/16385 \
  -H 'Content-Type: application/json' \
  -d '{"strategy":"increment","enabled":true,"params":{"start_value":0,"step":1,"period_ms":1000,"max_value":100}}'
```

---

## 项目结构

```
├── cmd/iec104-sim/        入口（传统模式 + 服务模式）
├── internal/
│   ├── detail/            v2.1 详情页模块
│   │   ├── engine.go      自动变化调度引擎
│   │   ├── strategy.go    9种策略计算逻辑
│   │   ├── handler.go     详情页 HTTP API
│   │   └── store.go       自动变化配置持久化
│   ├── manager/           多实例生命周期管理（最多10个）
│   ├── model/             数据模型（实例配置/状态/详情）
│   └── storage/           JSON 配置持久化
├── pkg/
│   ├── api/               HTTP API 处理器
│   ├── config/            Excel 加载器 + 测点数据模型
│   ├── iec104/            IEC 104 服务端
│   └── library/           并发安全内存点表
├── web/                   Vue 3 + Element Plus 前端
│   ├── src/views/
│   │   ├── ConfigPage.vue     配置管理
│   │   ├── MonitorPage.vue    运行监控
│   │   └── DetailPage.vue     v2.1 实例详情页
│   └── src/api/           Axios API 客户端
├── scripts/               启停脚本
├── config/                运行时配置
└── samples/               示例点表
```

---

## 编译构建

```bash
# 本地开发
go build -o bin/iec104-sim ./cmd/iec104-sim/

# Linux amd64
make build-linux-amd64

# Linux arm64
make build-linux-arm64

# Windows amd64
make build-windows

# 全平台
make build-all

# 完整构建（含 Web UI）
make build-full

# Debian 打包
make deb-amd64    # 或 deb-arm64

# UPX 压缩（减少约 60% 体积）
make compress
```

---

## 典型使用场景

### 变电站遥测仿真

```bash
# 启动模拟器（变电站 A，端口 2404）
./iec104-sim -p 2404 -c samples/point.xlsx -H :8080

# 通过 HTTP API 模拟电压变化
curl -X PUT http://localhost:8080/api/points/16385 \
  -H 'Content-Type: application/json' \
  -d '{"value": 235.5}'
# → IEC 104 客户端收到变化上送 (COT=3)
```

### 多实例部署

```bash
# 进程 1：220kV 变电站
./iec104-sim serve --http :8989 --config-dir ./config220

# 进程 2：110kV 变电站
./iec104-sim serve --http :8990 --config-dir ./config110
```

---

## 自动变化策略说明（v2.1）

| 策略 | 说明 | 适用场景 |
|--------|------|-------------|
| 递增 | 每周期 += 步长，达最大值后回起始值 | 模拟缓慢上升的遥测量 |
| 随机 | 在 [min, max] 范围内随机取值 | 模拟噪声或波动信号 |
| CSV 回放 | 按 CSV 文件定义的时间序列播放 | 回放真实录波数据 |
| MAX | 取多个 IOA 的最大值 | 联锁逻辑模拟 |
| MIN | 取多个 IOA 的最小值 | 联锁逻辑模拟 |
| SOC | 基于功率积分计算电池荷电状态 | 储能系统仿真 |
| 电量 | 基于功率积分计算累计电量 | 电能计量仿真 |
| AO关联 | 跟随指定 AO 点的遥控值变化 | 遥调联动模拟 |
| 接口更新 | 仅允许 HTTP API 写入，引擎不做计算 | 外部系统联调 |

---

## License

MIT
