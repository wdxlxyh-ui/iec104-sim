# 用户登录与审计系统设计方案

## 一、后端 API

### 数据模型

```go
// internal/model/auth.go
type User struct {
    ID       string `json:"id"`
    Username string `json:"username"`
    Password string `json:"-"`          // bcrypt 哈希，不返回
    Role     string `json:"role"`       // "admin" / "viewer"
    Enabled  bool   `json:"enabled"`
    CreatedAt time.Time `json:"created_at"`
    LastLogin time.Time `json:"last_login,omitempty"`
}

type AuditLog struct {
    ID         string    `json:"id"`
    UserID     string    `json:"user_id"`
    Username   string    `json:"username"`
    Action     string    `json:"action"`     // "login", "create_instance", "write_point", ...
    Target     string    `json:"target"`     // "instance:abc123", "point:16385"
    Detail     string    `json:"detail"`
    IP         string    `json:"ip"`
    Timestamp  time.Time `json:"timestamp"`
    Success    bool      `json:"success"`
}
```

### API 端点

| 方法 | 路径 | 说明 | 权限 |
|------|------|------|------|
| `POST` | `/api/v1/auth/login` | 登录，返回 JWT token | 公开 |
| `POST` | `/api/v1/auth/logout` | 登出 | 已登录 |
| `GET` | `/api/v1/auth/me` | 获取当前用户信息 | 已登录 |
| `PUT` | `/api/v1/auth/password` | 修改密码 | 已登录 |
| `GET` | `/api/v1/users` | 列出所有用户 | admin |
| `POST` | `/api/v1/users` | 创建用户 | admin |
| `PUT` | `/api/v1/users/{id}` | 更新用户 | admin |
| `DELETE` | `/api/v1/users/{id}` | 删除用户 | admin |
| `GET` | `/api/v1/audit` | 查询审计日志 | admin |

### 默认用户

```
用户名: admin
密码:   Test1234
角色:   admin
```

密码使用 bcrypt 哈希存储，首次启动时自动创建默认用户。

### JWT 认证

```
请求头: Authorization: Bearer <token>
Token 有效期: 24 小时
```

### 审计日志自动记录

所有写操作（创建/修改/删除实例、置数、修改密码等）自动写入审计日志。

## 二、登录页 UI 原型

```
┌──────────────────────────────────────────────┐
│                                              │
│   ┌──────────────────────────────────────┐   │
│   │                                      │   │
│   │          IEC104 Simulator             │   │
│   │       ─────────────────────           │   │
│   │                                      │   │
│   │  用户名                               │   │
│   │  ┌──────────────────────────┐        │   │
│   │  │ admin                    │        │   │
│   │  └──────────────────────────┘        │   │
│   │                                      │   │
│   │  密码                                 │   │
│   │  ┌──────────────────────────┐        │   │
│   │  │ ••••••••••              │        │   │
│   │  └──────────────────────────┘        │   │
│   │                                      │   │
│   │  [         登  录          ]         │   │
│   │                                      │   │
│   │  提示: 默认账号 admin / Test1234      │   │
│   │                                      │   │
│   └──────────────────────────────────────┘   │
│                                              │
│  深色/浅色切换     English/中文               │
│                                              │
└──────────────────────────────────────────────┘
```

## 三、用户管理页 UI

```
┌──────────────────────────────────────────────┐
│ 用户管理                    [+ 添加用户]       │
├──────────────────────────────────────────────┤
│ ┌──────┬────────┬────────┬────────┬────────┐ │
│ │ 用户  │ 角色   │ 状态   │ 最后登录 │ 操作   │ │
│ ├──────┼────────┼────────┼────────┼────────┤ │
│ │ admin│ 管理员 │ ● 启用 │ 05-15   │ [编辑] │ │
│ │ user1│ 只读   │ ● 启用 │ 05-14   │ [编辑] │ │
│ └──────┴────────┴────────┴────────┴────────┘ │
│                                              │
│  ┌─── 添加用户 ──────────────────────┐       │
│  │ 用户名: [____________]            │       │
│  │ 密码:   [____________]            │       │
│  │ 角色:   [管理员 ▾]               │       │
│  │  [取消]  [创建]                  │       │
│  └────────────────────────────────────┘       │
└──────────────────────────────────────────────┘
```

## 四、审计日志页 UI

```
┌──────────────────────────────────────────────┐
│ 审计日志                    [筛选] [导出]      │
├──────────────────────────────────────────────┤
│ ┌──────┬────────┬────────┬────────┬────────┐ │
│ │ 时间  │ 用户   │ 操作   │ 目标    │ 结果  │ │
│ ├──────┼────────┼────────┼────────┼────────┤ │
│ │15:30 │ admin  │ 登录   │ -      │ ✅     │ │
│ │15:31 │ admin  │ 创建实例│ 关口表 │ ✅     │ │
│ │15:32 │ admin  │ 置数   │ 16385  │ ✅     │ │
│ └──────┴────────┴────────┴────────┴────────┘ │
└──────────────────────────────────────────────┘
```

## 五、前端实现清单

| 文件 | 说明 | 工时 |
|------|------|------|
| `LoginPage.vue` | 登录页 | 0.5天 |
| `UserManage.vue` | 用户 CRUD 页面 | 0.5天 |
| `AuditLog.vue` | 审计日志查看 | 0.3天 |
| `router/index.ts` | 路由守卫 + 登录跳转 | 0.2天 |
| `api/index.ts` | auth API 调用 | 0.2天 |
| **前端合计** | | **1.7天** |

## 六、后端实现清单

| 文件 | 说明 | 工时 |
|------|------|------|
| `internal/model/auth.go` | User + AuditLog 模型 | 0.3天 |
| `internal/auth/handler.go` | 登录/登出/me API | 0.5天 |
| `internal/auth/store.go` | 用户持久化(JSON) + bcrypt | 0.5天 |
| `internal/auth/jwt.go` | JWT 签发与验证 | 0.3天 |
| `internal/audit/store.go` | 审计日志存储 | 0.3天 |
| `cmd/iec104-sim/main.go` | 中间件集成 | 0.3天 |
| **后端合计** | | **2.2天** |

## 七、实施建议

1. **Phase 1**: 先做登录 + JWT + 路由守卫（前端+后端），无用户管理页，仅默认 admin 用户
2. **Phase 2**: 加用户管理 + 密码修改
3. **Phase 3**: 加审计日志
