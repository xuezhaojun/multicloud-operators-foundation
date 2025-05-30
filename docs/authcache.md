我来详细解释 `AuthCache` 的具体作用和工作原理。

## AuthCache 的核心作用

`AuthCache` 是一个**权限映射缓存系统**，它的主要目的是将 Kubernetes RBAC 规则转换为高效的用户-资源访问映射表。

## 1. AuthCache 解决的问题

```mermaid
graph TB
    subgraph "没有 AuthCache 的情况"
        A1[用户请求] --> B1[查询 ClusterRole]
        B1 --> C1[查询 ClusterRoleBinding]
        C1 --> D1[解析 RBAC 规则]
        D1 --> E1[检查权限]
        E1 --> F1[返回结果]

        style A1 fill:#ffcdd2
        style B1 fill:#ffcdd2
        style C1 fill:#ffcdd2
        style D1 fill:#ffcdd2
        style E1 fill:#ffcdd2
    end

    subgraph "有 AuthCache 的情况"
        A2[用户请求] --> B2[查询缓存]
        B2 --> C2[直接返回结果]

        style A2 fill:#c8e6c9
        style B2 fill:#c8e6c9
        style C2 fill:#c8e6c9
    end

    G[RBAC 变更] --> H[后台同步缓存]
    H --> B2
```

## 2. AuthCache 的数据结构

```mermaid
erDiagram
    AuthCache {
        map userSubjectRecordStore "用户权限存储"
        map groupSubjectRecordStore "组权限存储"
        sets knownResources "已知资源集合"
        sets knownUsers "已知用户集合"
        sets knownGroups "已知组集合"
        string lastState "上次同步状态"
    }

    SubjectRecord {
        string Subject "主体(用户/组)"
        sets Names "可访问的资源名称"
    }

    AuthCache ||--o{ SubjectRecord : "存储"
```

## 3. AuthCache 的工作流程

### 初始化和同步过程：

```mermaid
sequenceDiagram
    participant RBAC as RBAC Informers
    participant AC as AuthCache
    participant Store as Subject Stores
    participant Watcher as CacheWatchers

    Note over AC: 启动同步过程
    AC->>AC: synchronize()
    AC->>AC: 检查资源版本是否变化

    alt 有变化
        AC->>RBAC: 获取所有 ClusterRoleBinding
        RBAC-->>AC: 返回绑定列表

        loop 处理每个 ClusterRoleBinding
            AC->>RBAC: 获取对应的 ClusterRole
            RBAC-->>AC: 返回角色规则
            AC->>AC: 解析权限规则
            AC->>AC: 提取资源名称
            AC->>Store: 更新用户/组权限映射
        end

        AC->>Watcher: 通知权限变更
    else 无变化
        AC->>AC: 跳过同步
    end
```

### 权限查询过程：

```mermaid
sequenceDiagram
    participant User as 用户请求
    participant Cache as ClusterCache
    participant AC as AuthCache
    participant Store as Subject Stores

    User->>Cache: List(userInfo, selector)
    Cache->>AC: listNames(userInfo)

    AC->>Store: 查询用户权限记录
    Store-->>AC: 返回用户可访问资源

    loop 用户所属的每个组
        AC->>Store: 查询组权限记录
        Store-->>AC: 返回组可访问资源
    end

    AC->>AC: 合并用户和组权限
    AC-->>Cache: 返回授权资源名称集合
    Cache-->>User: 返回过滤后的资源列表
```

## 4. AuthCache 的具体实现逻辑

### 权限解析过程：

```mermaid
graph TD
    A[ClusterRoleBinding] --> B[提取 Subjects]
    B --> C{Subject 类型}
    C -->|User| D[添加到用户集合]
    C -->|Group| E[添加到组集合]

    A --> F[获取 ClusterRole]
    F --> G[解析 PolicyRules]
    G --> H{检查 API Group}
    H -->|匹配| I[提取 ResourceNames]
    H -->|不匹配| J[跳过]

    I --> K{ResourceNames 为空?}
    K -->|是| L[获取所有资源]
    K -->|否| M[使用指定资源]

    D --> N[更新用户权限映射]
    E --> O[更新组权限映射]
    L --> N
    M --> N
    L --> O
    M --> O
```

### 核心代码逻辑解析：

```go
// AuthCache 的核心方法
func (ac *AuthCache) listNames(userInfo user.Info) sets.String {
    keys := sets.String{}
    user := userInfo.GetName()
    groups := userInfo.GetGroups()

    // 1. 查询用户直接权限
    obj, exists, _ := ac.userSubjectRecordStore.GetByKey(user)
    if exists {
        SubjectRecord := obj.(*SubjectRecord)
        keys.Insert(SubjectRecord.Names.List()...)
    }

    // 2. 查询用户组权限
    for _, group := range groups {
        obj, exists, _ := ac.groupSubjectRecordStore.GetByKey(group)
        if exists {
            SubjectRecord := obj.(*SubjectRecord)
            keys.Insert(SubjectRecord.Names.List()...)
        }
    }

    return keys // 返回用户可访问的所有资源名称
}
```

## 5. AuthCache 的优势

### 性能对比：

```mermaid
graph LR
    subgraph "传统方式 - 每次查询"
        A1[请求] --> B1[查 RBAC]
        B1 --> C1[解析规则]
        C1 --> D1[检查权限]
        D1 --> E1[返回]

        style A1 fill:#ffcdd2
        style B1 fill:#ffcdd2
        style C1 fill:#ffcdd2
        style D1 fill:#ffcdd2
    end

    subgraph "AuthCache 方式"
        A2[请求] --> B2[查缓存]
        B2 --> C2[返回]

        style A2 fill:#c8e6c9
        style B2 fill:#c8e6c9
        style C2 fill:#c8e6c9
    end

    F[RBAC 变更] --> G[异步更新缓存]
    G --> B2
```

## 6. AuthCache 的实际应用场景

### 场景 1：多租户集群访问控制

```mermaid
graph TB
    subgraph "租户A"
        UA[用户A] --> CA[集群A, 集群B]
    end

    subgraph "租户B"
        UB[用户B] --> CB[集群C, 集群D]
    end

    subgraph "管理员"
        ADMIN[管理员] --> ALL[所有集群]
    end

    subgraph "AuthCache"
        AC[权限映射缓存]
        AC --> |用户A| CA
        AC --> |用户B| CB
        AC --> |管理员| ALL
    end
```

### 场景 2：动态权限更新

```mermaid
sequenceDiagram
    participant Admin as 管理员
    participant K8s as Kubernetes
    participant AC as AuthCache
    participant User as 普通用户

    Admin->>K8s: 创建新的 ClusterRoleBinding
    K8s->>AC: 触发 RBAC 变更事件
    AC->>AC: 重新同步权限映射
    AC->>AC: 更新用户权限缓存

    User->>AC: 请求集群列表
    AC-->>User: 返回更新后的权限集群
```

## 总结

**AuthCache 的核心价值：**

1. **性能优化**：将复杂的 RBAC 规则解析转换为简单的 O(1) 查找
2. **权限隔离**：确保用户只能看到被授权的资源
3. **实时更新**：RBAC 规则变更能及时反映到缓存中
4. **内存效率**：使用高效的数据结构存储权限映射
5. **并发安全**：支持多个用户同时查询权限

**简单来说，AuthCache 就是一个"权限查找表"**，它预先计算好了每个用户/组可以访问哪些资源，避免了每次请求都要重新解析复杂的 RBAC 规则，大大提升了系统性能。
