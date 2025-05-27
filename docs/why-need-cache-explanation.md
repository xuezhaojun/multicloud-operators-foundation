# 为什么需要 Cache 而不能直接使用 Lister？

## 核心问题：RBAC 权限过滤

### 直接使用 Lister 的问题

```go
// 如果直接使用 lister - 这样是错误的！
func ListManagedClusterSets(userInfo user.Info, selector labels.Selector) (*clusterv1beta2.ManagedClusterSetList, error) {
    // 这会返回所有的 ManagedClusterSet，不管用户是否有权限访问
    allClusterSets, err := clusterSetLister.List(selector)
    if err != nil {
        return nil, err
    }

    // 用户可能看到他们没有权限访问的资源！
    return &clusterv1beta2.ManagedClusterSetList{Items: allClusterSets}, nil
}
```

### 实际场景举例

假设集群中有以下 ManagedClusterSets：

- `production-clusters`
- `development-clusters`
- `staging-clusters`

用户权限配置：

```yaml
# 用户 alice 只能访问 development 环境
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: alice-dev-access
subjects:
  - kind: User
    name: alice
roleRef:
  kind: ClusterRole
  name: dev-clusterset-reader
  apiGroup: rbac.authorization.k8s.io

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: dev-clusterset-reader
rules:
  - apiGroups: ["cluster.open-cluster-management.io"]
    resources: ["managedclustersets"]
    resourceNames: ["development-clusters"] # 只能访问这一个
    verbs: ["get", "list"]
```

**如果直接使用 lister：**

```go
// 错误的做法 - 会返回所有 ClusterSets
clusterSets, _ := lister.List(labels.Everything())
// 结果：alice 会看到 production-clusters, development-clusters, staging-clusters
// 这违反了 RBAC 权限控制！
```

**使用 Cache 的正确做法：**

```go
// 正确的做法 - 只返回用户有权限的资源
clusterSets, _ := cache.List(aliceUserInfo, labels.Everything())
// 结果：alice 只会看到 development-clusters
```

## Cache 系统的工作原理

### 1. 权限索引构建

Cache 系统会分析所有的 ClusterRole 和 ClusterRoleBinding：

```go
// Cache 分析这个 ClusterRole
func analyzeClusterRole(role *rbacv1.ClusterRole) {
    for _, rule := range role.Rules {
        if rule.APIGroups[0] == "cluster.open-cluster-management.io" &&
           rule.Resources[0] == "managedclustersets" {
            // 提取用户可以访问的具体资源名称
            accessibleResources := rule.ResourceNames
            // 存储到权限索引中
        }
    }
}

// Cache 分析这个 ClusterRoleBinding
func analyzeClusterRoleBinding(binding *rbacv1.ClusterRoleBinding) {
    // 获取绑定的 ClusterRole
    role := getClusterRole(binding.RoleRef.Name)
    accessibleResources := analyzeClusterRole(role)

    // 为每个 Subject 建立权限映射
    for _, subject := range binding.Subjects {
        userPermissions[subject.Name] = accessibleResources
    }
}
```

### 2. 权限查询

当用户请求资源时：

```go
func (c *ClusterSetCache) List(userInfo user.Info, selector labels.Selector) (*clusterv1beta2.ManagedClusterSetList, error) {
    // 1. 从权限索引中获取用户可访问的资源名称
    accessibleNames := c.getAccessibleResourceNames(userInfo)
    // 例如：对于 alice，accessibleNames = ["development-clusters"]

    // 2. 只获取用户有权限的资源
    clusterSetList := &clusterv1beta2.ManagedClusterSetList{}
    for name := range accessibleNames {
        clusterSet, err := c.clusterSetLister.Get(name)  // 这里才使用 lister
        if err != nil {
            continue
        }
        if selector.Matches(labels.Set(clusterSet.Labels)) {
            clusterSetList.Items = append(clusterSetList.Items, *clusterSet)
        }
    }

    return clusterSetList, nil
}
```

## 为什么不能在每次请求时动态检查权限？

### 性能问题

```go
// 这种做法性能很差
func ListWithDynamicRBACCheck(userInfo user.Info) (*clusterv1beta2.ManagedClusterSetList, error) {
    allClusterSets, _ := lister.List(labels.Everything())

    result := &clusterv1beta2.ManagedClusterSetList{}
    for _, clusterSet := range allClusterSets {
        // 每个资源都要做一次 RBAC 检查 - 非常慢！
        if hasPermission(userInfo, clusterSet.Name) {
            result.Items = append(result.Items, clusterSet)
        }
    }
    return result, nil
}

func hasPermission(userInfo user.Info, resourceName string) bool {
    // 需要查询所有 ClusterRoleBindings
    bindings, _ := rbacLister.List(labels.Everything())
    for _, binding := range bindings {
        // 需要查询对应的 ClusterRole
        role, _ := roleLister.Get(binding.RoleRef.Name)
        // 检查用户是否在 subjects 中
        // 检查 role 是否允许访问这个资源
        // ... 复杂的逻辑
    }
    // 这个过程对每个资源都要重复一遍！
}
```

### 性能对比

| 方法           | 每次请求的复杂度 | 说明                                     |
| -------------- | ---------------- | ---------------------------------------- |
| 直接 Lister    | O(1)             | 但是**不安全**，会泄露权限               |
| 动态 RBAC 检查 | O(n×m×k)         | n=资源数，m=绑定数，k=角色数，**非常慢** |
| Cache 系统     | O(1)             | **安全且快速**                           |

## Cache 系统的优势

### 1. 安全性

- 严格按照 RBAC 权限过滤资源
- 用户只能看到有权限的资源
- 防止权限泄露

### 2. 性能

- 权限检查结果被缓存，查询时间复杂度 O(1)
- 避免每次请求都做复杂的 RBAC 计算
- 支持大规模集群（数千个 ClusterSets）

### 3. 实时性

- 监听 ClusterRole 和 ClusterRoleBinding 变化
- 权限变更时自动更新缓存
- 保证权限的实时生效

## 简化版本的可能性

如果你觉得当前实现太复杂，可以考虑这些简化方案：

### 方案 1：使用 Kubernetes SubjectAccessReview API

```go
func ListWithSubjectAccessReview(userInfo user.Info, selector labels.Selector) (*clusterv1beta2.ManagedClusterSetList, error) {
    allClusterSets, _ := lister.List(selector)

    result := &clusterv1beta2.ManagedClusterSetList{}
    for _, clusterSet := range allClusterSets {
        // 使用官方 API 检查权限
        sar := &authorizationv1.SubjectAccessReview{
            Spec: authorizationv1.SubjectAccessReviewSpec{
                User: userInfo.GetName(),
                Groups: userInfo.GetGroups(),
                ResourceAttributes: &authorizationv1.ResourceAttributes{
                    Group:    "cluster.open-cluster-management.io",
                    Resource: "managedclustersets",
                    Name:     clusterSet.Name,
                    Verb:     "get",
                },
            },
        }

        sar, err := kubeClient.AuthorizationV1().SubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
        if err == nil && sar.Status.Allowed {
            result.Items = append(result.Items, clusterSet)
        }
    }
    return result, nil
}
```

**优点：** 代码简单，使用官方 API
**缺点：** 每个资源都要调用 API Server，性能较差

### 方案 2：简化的内存缓存

```go
type SimpleRBACCache struct {
    userToResources map[string]sets.String
    mu sync.RWMutex
}

func (c *SimpleRBACCache) rebuildCache() {
    // 简化的权限分析逻辑
    // 只处理核心场景，减少复杂性
}
```

## 总结

Cache 系统存在的根本原因是：

1. **安全需求**：必须按照 RBAC 权限过滤资源
2. **性能需求**：不能每次请求都做复杂的权限计算
3. **实时需求**：权限变更要及时生效

直接使用 lister 无法满足安全需求，而动态权限检查无法满足性能需求。Cache 系统通过预计算和缓存权限映射，在安全性和性能之间找到了平衡。

如果你的使用场景比较简单（比如用户数量少、权限变化不频繁），可以考虑使用 SubjectAccessReview API 的简化方案。但对于大规模生产环境，Cache 系统是必要的。
