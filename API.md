# IEC104 Simulator API 文档

Base URL: `http://localhost:8080`

---

## 目录

1. [管理接口 (Server Mode)](#1-管理接口-server-mode-apiv1)
2. [点表接口 (Legacy Mode)](#2-点表接口-legacy-mode-api)
3. [数据类型](#3-数据类型)
4. [错误处理](#4-错误处理)

---

## 1. 管理接口 (Server Mode) `/api/v1`

> 需要以 `serve` 子命令启动：
> ```bash
> ./bin/iec104-sim serve --http :8080 --config-dir ./config --log-dir ./logs
> ```

### 1.1 获取全局状态

```
GET /api/v1/status
```

**响应 200：**
```json
{
  "version": "2.1.3",
  "mode": "serve",
  "configured": 2,
  "running": 1,
  "stopped": 1,
  "max": 10
}
```

### 1.2 获取实例列表

```
GET /api/v1/instances
```

**响应 200：**
```json
{
  "instances": [
    {
      "id": "a1b2c3d4e5f6",
      "name": "变电站A",
      "iec104_port": 2404,
      "xlsx_file": "samples/point.xlsx",
      "enabled": false,
      "status": "running",
      "stats": {
        "uptime_seconds": 3600,
        "total_points": 7,
        "client_connected": true,
        "interrogations": 5,
        "controls": 12,
        "spontaneous": 88
      }
    }
  ]
}
```

### 1.3 创建实例

```
POST /api/v1/instances
Content-Type: application/json

{
  "name": "变电站A",
  "iec104_port": 2404,
  "xlsx_file": "samples/point.xlsx"
}
```

**响应 201：**
```json
{
  "id": "c35ce99c4264",
  "name": "变电站A",
  "iec104_port": 2404,
  "xlsx_file": "samples/point.xlsx",
  "enabled": false
}
```

### 1.4 获取实例详情

```
GET /api/v1/instances/{id}
```

**响应 200：**
```json
{
  "id": "c35ce99c4264",
  "name": "变电站A",
  "iec104_port": 2404,
  "xlsx_file": "samples/point.xlsx",
  "enabled": false,
  "status": "running",
  "stats": {
    "uptime_seconds": 120,
    "total_points": 7,
    "client_connected": false,
    "interrogations": 0,
    "controls": 0,
    "spontaneous": 0
  }
}
```

### 1.5 更新实例

```
PUT /api/v1/instances/{id}
Content-Type: application/json

{
  "name": "变电站A-修改",
  "iec104_port": 2405,
  "xlsx_file": "samples/point.xlsx"
}
```

支持**部分更新**——只需传需要修改的字段。如正在运行，会先停止再更新配置。

**响应 200：**
```json
{
  "id": "c35ce99c4264",
  "name": "变电站A-修改",
  "iec104_port": 2405,
  "xlsx_file": "samples/point.xlsx",
  "enabled": false
}
```

### 1.6 删除实例

```
DELETE /api/v1/instances/{id}
```

如正在运行，会先停止再删除。

**响应 200：**
```json
{
  "status": "deleted"
}
```

### 1.7 启动实例

```
POST /api/v1/instances/{id}/start
```

**响应 200：**
```json
{
  "status": "ok",
  "id": "c35ce99c4264"
}
```

### 1.8 停止实例

```
POST /api/v1/instances/{id}/stop
```

**响应 200：**
```json
{
  "status": "ok",
  "id": "c35ce99c4264"
}
```

### 1.9 重启实例

```
POST /api/v1/instances/{id}/restart
```

如未运行，则直接启动。

**响应 200：**
```json
{
  "status": "ok",
  "id": "c35ce99c4264"
}
```

### 1.10 上传点表文件

```
POST /api/v1/upload
Content-Type: multipart/form-data

file: <选择 .xlsx 文件>
```

**响应 200：**
```json
{
  "status": "uploaded",
  "filename": "my_points.xlsx"
}
```

### 1.11 获取已上传文件列表

```
GET /api/v1/files
```

**响应 200：**
```json
{
  "files": []
}
```

---

## 2. 点表接口 (Legacy Mode) `/api`

> 需要以 legacy 模式启动：
> ```bash
> ./bin/iec104-sim -p 2404 -c samples/point.xlsx -H :8080
> ```

### 2.1 获取所有点表

```
GET /api/points
```

**响应 200：**
```json
{
  "points": [
    {
      "ioa": 16385,
      "name": "母线电压",
      "value_type": "FLOAT",
      "point_type": "AI",
      "value": 100.5,
      "bool_value": false,
      "int_value": 0,
      "efficient": 1,
      "base_value": 100.5,
      "qds": {
        "invalid": false,
        "not_topical": false,
        "substituted": false,
        "overflow": false,
        "blocked": false
      },
      "alias": "",
      "timestamp": "2026-05-12T02:30:00Z"
    }
  ]
}
```

### 2.2 获取单个点

```
GET /api/points/{ioa}
```

### 2.3 更新遥测/遥调点值 (AI/AO)

```
PUT /api/points/{ioa}
Content-Type: application/json

{
  "value": 235.5
}
```

### 2.4 更新遥信/遥控点值 (DI/DO)

```
PUT /api/points/{ioa}
Content-Type: application/json

{
  "bool_value": true
}
```

### 2.5 更新遥脉点值 (PI)

```
PUT /api/points/{ioa}
Content-Type: application/json

{
  "int_value": 42
}
```

**点值更新响应 200：**
```json
{
  "success": true,
  "ioa": 16385,
  "changed": true
}
```

> 更新后会触发 IEC104 变化上送 (COT=3)，客户端会收到实时更新。

### 2.6 批量更新点值

```
POST /api/points
Content-Type: application/json

{
  "points": [
    { "ioa": 16385, "value": 999.99 },
    { "ioa": 5, "bool_value": false },
    { "ioa": 10, "int_value": 100 }
  ]
}
```

**响应 200：**
```json
{
  "success": true,
  "updated": 3,
  "failed": 0,
  "details": [
    { "ioa": 16385, "success": true },
    { "ioa": 5, "success": true },
    { "ioa": 10, "success": true }
  ]
}
```

### 2.7 更新质量描述 (QDS)

```
PUT /api/points/{ioa}/qds
Content-Type: application/json

{
  "invalid": true,
  "blocked": true
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `invalid` | bool | 无效 |
| `not_topical` | bool | 非当前 |
| `substituted` | bool | 取代 |
| `overflow` | bool | 溢出 |
| `blocked` | bool | 闭锁 |

**响应 200：**
```json
{
  "success": true
}
```

### 2.8 获取服务端状态 (Legacy)

```
GET /api/status
```

**响应 200：**
```json
{
  "connected": true,
  "client_addr": "192.168.1.100:51234",
  "uptime": 3600,
  "interrog": 5,
  "control": 12,
  "spont": 88
}
```

---

## 3. 数据类型

### InstanceConfig (实例配置)

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | string | 否 | 实例ID（创建时不填则自动生成） |
| `name` | string | 是 | 实例名称 |
| `iec104_port` | int | 是 | IEC104 端口号 (1-65535) |
| `xlsx_file` | string | 是 | 点表 xlsx 文件名 |
| `enabled` | bool | 否 | 是否启用 |

### InstanceState (实例状态)

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 实例ID |
| `name` | string | 实例名称 |
| `iec104_port` | int | IEC104 端口 |
| `xlsx_file` | string | 点表文件 |
| `status` | string | `running` / `stopped` / `error` |
| `stats.uptime_seconds` | int64 | 运行时长（秒） |
| `stats.total_points` | int | 测点总数 |
| `stats.client_connected` | bool | 客户端是否已连接 |
| `stats.interrogations` | int64 | 总召次数 |
| `stats.controls` | int64 | 遥控次数 |
| `stats.spontaneous` | int64 | 变化上送次数 |

### Point (点表)

| 字段 | 类型 | 说明 |
|------|------|------|
| `ioa` | uint32 | IOA 地址 |
| `name` | string | 点名 |
| `value_type` | string | `FLOAT` / `DOUBLE` / `INT` / `BIT` |
| `point_type` | string | `AI` / `DI` / `PI` / `DO` / `AO` |
| `value` | float64 | 浮点值（AI/AO） |
| `bool_value` | bool | 布尔值（DI/DO） |
| `int_value` | int32 | 整型值（PI） |
| `efficient` | float64 | 系数 |
| `base_value` | float64 | 基值 |
| `qds` | object | 质量描述 |
| `alias` | string | 别名 |
| `timestamp` | string | 更新时间 |

---

## 4. 错误处理

所有错误响应使用统一的 JSON 格式：

```json
{
  "error": "错误描述信息"
}
```

### HTTP 状态码说明

| 状态码 | 含义 |
|--------|------|
| 200 | 成功 |
| 201 | 创建成功 |
| 400 | 请求参数错误（缺少字段、JSON 格式错误） |
| 404 | 资源不存在 |
| 409 | 冲突（端口已被使用、实例已存在等） |
| 500 | 服务端内部错误 |
