# IEC 104 模拟器 MCP 使用指南

> 版本: 1.0 | 更新日期: 2026-05-16

## 概述

MCP (Model Context Protocol) 服务器提供 IEC 104 模拟器的程序化控制接口，支持：
- 实例管理（创建、启动、停止、删除）
- 测点数据读写（批量写入、策略配置）
- 文件上传（点表、CSV 回放文件）

## 快速开始

### 1. 编译 MCP 程序

```bash
# 编译 MCP 服务器
go build -o bin/mcp-server ./cmd/mcp-server/
```

### 2. 运行 MCP 服务器

```bash
# 方式一：使用 stdio 模式（推荐，用于 Claude Desktop 等）
./bin/mcp-server -simulator http://localhost:8989 -mode both

# 方式二：查看帮助
./bin/mcp-server -h
```

参数说明：
| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-simulator` | http://localhost:8989 | IEC104 模拟器 HTTP 地址 |
| `-mode` | both | 运行模式：instance(实例管理) / data(数据接口) / both(全部) |

## 可用工具

### 实例管理工具

| 工具名称 | 说明 |
|----------|------|
| `list_instances` | 列出所有已配置的模拟器实例 |
| `get_instance` | 获取单个实例的详细配置信息 |
| `create_instance` | 创建新的模拟器实例 |
| `update_instance` | 更新已有实例的配置 |
| `delete_instance` | 删除实例 |
| `start_instance` | 启动指定实例 |
| `stop_instance` | 停止指定实例 |
| `restart_instance` | 重启指定实例 |
| `get_server_status` | 获取模拟器全局状态 |

### 数据接口工具

| 工具名称 | 说明 |
|----------|------|
| `list_points` | 列出实例的所有测点及其当前值 |
| `read_point` | 读取单个测点的当前值 |
| `read_points` | 批量读取多个测点的当前值 |
| `write_point` | 写入单个测点的值 |
| `write_points` | **【核心】** 批量写入多个测点的值 |
| `config_auto_change` | 配置测点的自动变化策略 |
| `get_auto_change` | 查看测点的自动变化配置 |
| `delete_auto_change` | 删除测点的自动变化配置 |
| `export_auto_changes` | 导出实例所有自动变化配置为 CSV |
| `import_auto_changes` | 从 CSV 内容导入自动变化配置 |
| `upload_csv` | 上传 CSV 时间序列文件（用于 CSV 回放） |
| `upload_file` | 上传 .xlsx 点表文件 |

## 使用示例

### 1. 配置自动变化策略

```python
# 使用 MCP 工具配置测点自动变化
config_auto_change(
    instance_id="inst-001",
    ioa=16385,
    strategy="increment",
    enabled=True,
    params='{"start_value":0,"step":1,"period_ms":1000,"max_value":100}'
)
```

支持的策略类型：
- `increment` - 递增
- `random` - 随机
- `csv` - CSV 回放
- `max` - 取大
- `min` - 取小
- `soc` - SOC 计算
- `energy` - 电量计算
- `aofollow` - AO 关联
- `apiupdate` - 接口更新
- `manual` - 手动
- `custom` - 自定义公式

### 2. 批量写入测点

```python
# 一次写入多个测点，模拟真实设备同一时刻上报数据
write_points(
    instance_id="inst-001",
    points=[
        {"ioa": 16385, "value": 235.5},
        {"ioa": 16386, "value": 236.0},
        {"ioa": 16387, "bool_value": True}
    ]
)
```

### 3. 上传点表文件

```python
# 上传 .xlsx 点表文件（文件内容需 base64 编码）
upload_file(
    filename="固定验证-关口表.xlsx",
    content_base64="UEsFBgAAAA..."
)
```

## 一键安装包

使用项目根目录的 `iec104-autotester-pack.tar.gz` 快速部署：

```bash
# 解压
tar -xzf iec104-autotester-pack.tar.gz

# 进入目录
cd iec104-autotester-pack

# 查看内容
ls -la
```

包内包含：
- `iec104-sim` - IEC104 模拟器主程序
- `mcp-server` - MCP 服务器程序
- `config/` - 配置目录
- `samples/` - 示例点表

## 故障排除

### 连接失败

```
Error: dial tcp connection refused
```

检查：
1. IEC104 模拟器是否已启动
2. `-simulator` 参数地址是否正确
3. 模拟器 HTTP 端口是否可达

### 测点写入失败

```
错误: IOA xxx not found
```

检查：
1. 测点 IOA 是否存在于点表中
2. 实例是否已启动

### MCP 工具无响应

1. 检查模拟器日志
2. 确认点表文件已正确加载
3. 验证实例状态为 running

## 版本信息

- IEC104 Simulator: 2.2.0+
- MCP Server: 1.0.0