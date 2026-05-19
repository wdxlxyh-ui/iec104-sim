# 开发设计文档: 多规约支持 (IEC104 + Modbus TCP)

## 1. 架构总览

### 1.1 设计原则

- **协议抽象层** — 将"规约"抽象为可插拔的 Protocol 接口，不同规约实现同一接口
- **数据层复用** — `library.Store`（内存点表）保持不变，所有规约共享同一数据源
- **配置向后兼容** — `InstanceConfig` 新增 `protocol` 字段，默认值为 `"iec104"`，旧配置无需迁移
- **点表向后兼容** — Excel 加载器对 Modbus 专属列（功能码/寄存器地址）做可选解析，IEC104 点表无这些列时正常加载

### 1.2 整体架构图

```
+-------------------------------------------------------------------------+
|                        iec104-sim (serve 模式)                           |
+-------------------------------------------------------------------------+
|  +-------------------------------------------------------------------+  |
|  |                      Manager (多实例管理)                          |  |
|  |  +------------------+  +------------------+                        |  |
|  |  | Instance #1      |  | Instance #2      |                        |  |
|  |  | Protocol: IEC104 |  | Protocol: Modbus |                        |  |
|  |  | +--------------+ |  | +--------------+ |                        |  |
|  |  | |IEC104 Server | |  | |ModbusTCPServer| |                       |  |
|  |  | +------+-------+ |  | +------+-------+ |                        |  |
|  |  |        |         |  |        |         |                        |  |
|  |  | +------v------+  |  | +------v------+  |                        |  |
|  |  | |   Store     |  |  | |   Store     |  |                        |  |
|  |  | +-------------+  |  | +-------------+  |                        |  |
|  |  +------------------+  +------------------+                        |  |
|  +-------------------------------------------------------------------+  |
|  +-------------------------------------------------------------------+  |
|  |              HTTP API + Web UI (Vue 3)                             |  |
|  +-------------------------------------------------------------------+  |
+-------------------------------------------------------------------------+
```

### 1.3 协议接口定义

```go
type Protocol interface {
    Name() string
    Start() error
    Stop()
    ClientConnected() bool
    ClientAddr() string
    Stats() (interrog, control, spont int64)
    Uptime() int64
    Publish(point *config.Point)
    SetStore(store *library.Store)
}
```

---

## 2. 数据模型变更

### 2.1 InstanceConfig 扩展

```go
type InstanceConfig struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    IEC104Port  int    `json:"iec104_port"`
    XLSXFile    string `json:"xlsx_file"`
    Enabled     bool   `json:"enabled"`
    HttpEnabled bool   `json:"http_enabled"`
    HttpPort    int    `json:"http_port"`
    Protocol    string `json:"protocol,omitempty"`
    ModbusConfig *ModbusInstanceConfig `json:"modbus_config,omitempty"`
}

type ModbusInstanceConfig struct {
    Port      int    `json:"port,omitempty"`
    ByteOrder string `json:"byte_order,omitempty"`
    SlaveID   uint8  `json:"slave_id,omitempty"`
}
```

向后兼容: `protocol` 为空时默认 `"iec104"`，`modbus_config` 为 nil 时使用默认值。

### 2.2 Point 数据模型扩展

```go
type Point struct {
    IOA       uint32            `json:"ioa"`
    Name      string            `json:"name"`
    ValueType ValueType         `json:"value_type"`
    PointType PointType         `json:"point_type"`
    Value     float64           `json:"value"`
    BoolValue bool              `json:"bool_value"`
    IntValue  int32             `json:"int_value"`
    Efficient float64           `json:"efficient"`
    BaseValue float64           `json:"base_value"`
    QDS       QualityDescriptor `json:"qds"`
    Alias     string            `json:"alias"`
    Timestamp time.Time         `json:"timestamp"`
    Changed   bool              `json:"-"`
    FunctionCode    uint8  `json:"function_code,omitempty"`
    RegisterAddress uint16 `json:"register_address,omitempty"`
    ByteOrder       string `json:"byte_order,omitempty"`
}
```

### 2.3 Excel 加载器变更

`LoadFromXLSX` 增加 `protocol` 参数。解析 H/I/J 列（索引 7/8/9），仅当 protocol 为 modbus 时校验必填。

---

## 3. 后端实现方案

### 3.1 目录结构变更

```
pkg/protocol/
  protocol.go        Protocol 接口定义
  factory.go         协议工厂
  iec104_wrapper.go  IEC104 适配器
  modbus/
    tcp_server.go    Modbus TCP 服务端
    handler.go       功能码分发处理
    converter.go     数据类型转换
```

### 3.2 Manager.StartInstance 变更

根据 `cfg.Protocol` 调用 `protocol.New(cfg)` 创建对应协议实例。

### 3.3 Instance 结构变更

`Server *iec104.Server` 替换为 `Protocol protocol.Protocol`。

### 3.4 Modbus TCP 服务端

- 监听 TCP 端口，单客户端连接
- 解析 MBAP header (7 bytes) + PDU
- 功能码分发: 01/02/03/04/05/06/15/16
- 从 Store 按功能码+寄存器地址查询测点
- 共享 Store，HTTP API 修改后 Modbus 读取同步

### 3.5 数据类型转换

Float32/Int32 <-> 2x16-bit registers，支持 ABCD/CDAB/BADC/DCBA 字节序。

### 3.6 Store 扩展

增加 `GetByFunctionCodeAndAddress(fc, addr)` 和 `GetByFunctionCodeRange(fc, startAddr, count)`。

### 3.7 API 变更

新增 `GET /api/v1/protocols` 返回 `["iec104", "modbus_tcp"]`。

---

## 4. 前端实现

- 规约卡片选择器 (IEC104 / Modbus TCP)
- 条件表单: IEC104 端口 vs Modbus TCP 端口 + 从站地址 + 字节序
- 实例列表新增"规约"列
- 详情页动态显示功能码/寄存器地址列

---

## 5. 向后兼容

| 场景 | 行为 |
|------|------|
| 旧 JSON 配置 | protocol 为空 -> 默认 iec104 |
| 旧 Excel 点表 | 无 H/I/J 列 -> 零值回退 |
| 旧版前端 | 不发送 protocol -> 后端默认 iec104 |

---

## 6. 开发阶段

| 阶段 | 内容 | 工时 |
|------|------|------|
| P0 | Protocol 接口 + 工厂 + IEC104 适配器 | 0.5d |
| P1 | Modbus TCP 服务端 | 2d |
| P2 | 数据模型 + Excel 加载器 | 0.5d |
| P3 | Manager 适配 | 0.5d |
| P4 | 前端规约选择 | 1d |
| P5 | 前端详情页 | 0.5d |
| P6 | 测试 + 文档 | 1d |

总计: 6 人天
