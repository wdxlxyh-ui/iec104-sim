# PRD: 多规约支持 — Modbus TCP 扩展

## 1. 背景与目标

### 1.1 现状
当前 IEC104 Simulator v2.3.0 仅支持 IEC 60870-5-104 规约。实例创建时固定使用 IEC104 协议栈，点表格式（Excel）按 IEC104 定义（AI/DI/PI/DO/AO + IOA + 系数 + 基值）。

### 1.2 目标
在**不影响现有 IEC104 规约**的前提下，扩展模拟器支持：
- **Modbus TCP** — 以太网传输，端口 502，MBAP 报文头，无 CRC

### 1.3 用户价值
- 一套模拟器同时覆盖 IEC104 和 Modbus TCP 两种主流工业协议
- 降低测试环境搭建成本（无需部署多套工具）
- 统一 Web 管理界面，降低运维复杂度

---

## 2. 需求分析

### 2.1 核心需求

| 编号 | 需求 | 优先级 | 说明 |
|------|------|--------|------|
| R1 | 新建实例时选择规约类型 | P0 | 创建实例时下拉选择 IEC104 / Modbus TCP |
| R2 | 点表扩展 Modbus 字段 | P0 | 点表增加功能码(Function Code)、寄存器地址(Register Address)列 |
| R3 | Modbus TCP 服务端 | P0 | 监听 TCP 端口，响应 Modbus TCP 请求 |
| R4 | 向前兼容 | P0 | 已有 IEC104 实例不受影响，旧版点表正常加载 |
| R5 | 详情页适配 | P0 | 实例详情页（测点查看/置数/自动变化）支持 Modbus 实例 |
| R6 | Excel 点表模板 | P0 | 提供 Modbus 点表模板，含功能码/寄存器地址列 |

### 2.2 协议对比

| 维度 | IEC104 | Modbus TCP |
|------|--------|------------|
| 地址标识 | IOA (信息体地址) | Register Address / Coil Address |
| 功能标识 | ASDU TypeID | Function Code (01/02/03/04/05/06/15/16) |
| 数据类型 | AI/DI/PI/DO/AO | Holding Register(4x)/Input Register(3x)/Coil(0x)/Discrete Input(1x) |
| 传输层 | TCP (自定义端口) | TCP (默认502) |
| 字节序 | IEEE 754 float | 可配置 (ABCD/CDAB/BADC/DCBA) |
| 系数/基值 | efficient / base-value | 同 IEC104（保持兼容） |

### 2.3 Modbus 点表列定义（扩展后）

| 列 | 表头 | 类型 | IEC104 | Modbus TCP | 必填 |
|----|------|------|--------|------------|------|
| A | point-name | string | ✅ | ✅ | 是 |
| B | point-number | uint32 | ✅ (IOA) | ✅ (作为内部ID) | 是 |
| C | value-type | string | ✅ | ✅ | 是 |
| D | point-type | string | ✅ (AI/DI/PI/AO/DO) | ✅ (AI/DI/PI/AO/DO) | 是 |
| E | efficient | float64 | ✅ | ✅ | 是 |
| F | base-value | float64 | ✅ | ✅ | 是 |
| G | alias | string | ✅ | ✅ | 否 |
| **H** | **function-code** | uint8 | ❌ (空) | ✅ (01-06,15,16) | **Modbus必填** |
| **I** | **register-address** | uint16 | ❌ (空) | ✅ (0-65535) | **Modbus必填** |
| **J** | **byte-order** | string | ❌ (默认ABCD) | ✅ (ABCD/CDAB/BADC/DCBA) | 否，默认ABCD |

### 2.4 功能码与测点类型映射

| 测点类型 | 读功能码 | 写功能码 | Modbus 区域 |
|----------|----------|----------|-------------|
| AI (遥测) | 04 (Read Input Registers) | — | 3x (Input Registers) |
| DI (遥信) | 02 (Read Discrete Inputs) | — | 1x (Discrete Inputs) |
| PI (遥脉) | 04 (Read Input Registers) | — | 3x (Input Registers, 32bit) |
| DO (遥控) | 01 (Read Coils) | 05 (Write Single Coil) / 15 (Write Multiple Coils) | 0x (Coils) |
| AO (遥调) | 03 (Read Holding Registers) | 06 (Write Single Register) / 16 (Write Multiple Registers) | 4x (Holding Registers) |

---

## 3. 非功能需求

| 编号 | 需求 | 说明 |
|------|------|------|
| N1 | 向后兼容 | 已有 IEC104 实例的 JSON 配置、Excel 点表无需修改即可正常加载 |
| N2 | 性能 | Modbus TCP 单实例支持 ≥10 个并发客户端连接 |
| N3 | 数据一致性 | 同一测点在 HTTP API 修改后，Modbus 客户端读取到的值同步更新 |
| N4 | 部署 | 单二进制文件，无需额外依赖 |

---

## 4. 范围外（Out of Scope）

- Modbus RTU 串口支持
- Modbus 安全认证/加密
- Modbus 网关/桥接模式
- 点表在线编辑（仍使用 Excel）

---

## 5. 验收标准

| 场景 | 验收条件 |
|------|----------|
| 创建 Modbus TCP 实例 | 选择规约→上传点表→启动→Modbus Poll 工具可连接并读取数据 |
| 旧 IEC104 实例 | 升级后原有实例正常启动、总召、变化上送不受影响 |
| 点表兼容性 | IEC104 点表（无 H/I/J 列）在 IEC104 实例中正常加载 |
| 详情页 | Modbus 实例的测点列表、置数、自动变化策略与 IEC104 实例操作一致 |
