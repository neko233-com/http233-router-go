# http233-router-go Production Spec

## [S1] Problem

http233-router-go 目前是性能原型，缺少生产环境必须的能力。需要补齐中间件、路由组、错误管理、热重载等核心功能，并通过 100+ API 业务测试验证在复杂场景下的正确性和性能。

## [S2] Core Production Capabilities

### [S2.1] Middleware Chain
- `Use(middleware ...HandlerFunc)` 注册全局中间件
- 路由组级别中间件
- 中间件执行顺序：全局 → 组级 → 路由级
- 支持 `c.Next()` 和 `c.Abort()`
- Recovery 中间件（内置）

### [S2.2] Route Groups
- `Group(prefix string, middlewares ...HandlerFunc) *RouterGroup`
- 组可嵌套
- 组级中间件只作用于组内路由
- 组支持所有 HTTP 方法注册

### [S2.3] Error Management
- `c.Error(err)` 收集错误
- `c.Fail(code, msg)` 终止并返回错误
- `Recovery()` 中间件捕获 panic
- 自定义错误处理器 `router.OnError(handler)`

### [S2.4] HTTP Methods & Routing
- 完整支持：GET/POST/PUT/DELETE/HEAD/OPTIONS/PATCH
- `ANY(path, handler)` 注册所有方法
- `HandleMethodNotAllowed` 配置（返回 405 而非 404）
- `RedirectTrailingSlash` 配置
- `NotFound` / `NoMethod` 自定义处理器

### [S2.5] Context 增强
- `c.Query(key)` / `c.DefaultQuery(key, default)`
- `c.Param(key)` 已有，保持
- `c.BindJSON(obj)` / `c.ShouldBindJSON(obj)`
- `c.Set(key, val)` / `c.Get(key)` 请求级 KV 存储
- `c.AbortWithStatus(code)`
- `c.Redirect(code, url)`
- `c.File(path)` / `c.FileAttachment(path, name)`
- `c.HTML(code, name, obj)` 模板渲染
- `c.Data(code, contentType, data)`

### [S2.6] Static File Serving
- `router.Static(urlPrefix, rootDir)` 静态文件服务
- `router.StaticFS(urlPrefix, fs)` 自定义 FS
- `router.StaticFile(url, filepath)` 单文件绑定
- 目录列表可配置开关

## [S3] Hot-Reload Support

### [S3.1] Architecture
- 基于 `fsnotify` 监听文件变化
- 默认关闭（`router.SetHotReload(false)`）
- 开启后监听指定目录，自动清除响应缓存
- 适用场景：HTML/JS/CSS 等静态资源开发环境

### [S3.2] Interface
```go
router.EnableHotReload(dirs ...string)  // 开启并指定监听目录
router.DisableHotReload()               // 关闭
router.SetHotReloadCallback(fn func(events []fsnotify.Event)) // 变化回调
```

### [S3.3] Behavior
- 文件创建/修改/删除事件触发回调
- 静态文件服务响应时检查是否需要刷新
- 不影响路由匹配性能（关闭时零开销）
- 支持嵌套目录监听

## [S4] 100+ API Test Suite

### [S4.1] Business Scenario Coverage
模拟真实 RESTful API，覆盖以下资源：

| 资源 | 路由数 | 覆盖能力 |
|------|--------|---------|
| /api/v1/users | 15 | CRUD + 嵌套资源 + 参数 |
| /api/v1/posts | 12 | CRUD + 分页 + 搜索 |
| /api/v1/comments | 8 | 嵌套路由 + 批量操作 |
| /api/v1/auth | 6 | 登录/注册/刷新/注销 |
| /api/v1/admin | 10 | 权限中间件 + 组级中间件 |
| /api/v1/files | 8 | 上传/下载/静态文件 |
| /api/v1/search | 5 | 查询参数 + 通配符 |
| /static | 5 | 静态文件服务 |
| /health | 3 | 健康检查 + 就绪探针 |
| /ws | 2 | WebSocket 升级路径 |
| **总计** | **74+** | |

### [S4.2] 功能验证矩阵
- 路由匹配正确性（静态/参数/通配符）
- 中间件执行顺序
- 路由组隔离性
- 405 Method Not Allowed
- 404 Not Found
- 重定向（TrailingSlash / FixedPath）
- 并发安全性（100+ goroutine）
- Context 复用正确性（sync.Pool）
- 参数提取准确性
- 错误处理链
- 静态文件服务
- 热重载开关

### [S4.3] 性能验证
- 直接 TCP socket 基准测试
- 与 gin/httprouter 对比（如已安装）
- 内存分配追踪
- P99 延迟测量

## [S5] Architecture

```
http233-router-go/
├── router.go          # Router struct, 配置项, ServeHTTP
├── radix.go           # Radix tree 实现
├── tree_node.go       # Node 类型定义
├── route.go           # 路由注册, 路由组
├── context.go         # Context (pool + 增强方法)
├── middleware.go       # 中间件链, Recovery, 链式执行
├── group.go           # RouteGroup 实现
├── static.go          # 静态文件服务
├── hotreload.go       # fsnotify 热重载
├── router_test.go     # 单元测试
├── integration_test.go # 100+ API 业务测试
├── benchmark_test.go  # 性能基准测试
├── go.mod
└── docs/compose/
    ├── specs/
    └── plans/
```

## [S6] Implementation Order

1. **Phase 1**: Middleware chain + Recovery (基础)
2. **Phase 2**: Route groups (依赖 middleware)
3. **Phase 3**: Context 增强 + Error management
4. **Phase 4**: HTTP 方法完善 + 405/重定向
5. **Phase 5**: 静态文件服务
6. **Phase 6**: 热重载 (fsnotify)
7. **Phase 7**: 100+ API 集成测试
8. **Phase 8**: 性能基准测试
