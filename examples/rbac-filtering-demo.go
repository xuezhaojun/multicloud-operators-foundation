package main

import (
	"fmt"
	"strings"

	clusterv1beta2 "open-cluster-management.io/api/cluster/v1beta2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
)

// 演示：为什么不能直接使用 Lister

func main() {
	// 模拟集群中的数据
	setupDemo()
}

func setupDemo() {
	fmt.Println("=== RBAC 权限过滤演示 ===\n")

	// 1. 集群中的 ManagedClusterSets
	clusterSets := []*clusterv1beta2.ManagedClusterSet{
		{ObjectMeta: metav1.ObjectMeta{Name: "production-clusters"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "development-clusters"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "staging-clusters"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "testing-clusters"}},
	}

	// 2. 用户权限配置
	aliceUser := &user.DefaultInfo{
		Name:   "alice",
		Groups: []string{"developers"},
	}

	bobUser := &user.DefaultInfo{
		Name:   "bob",
		Groups: []string{"operators"},
	}

	// 3. RBAC 配置
	rbacConfig := map[string][]string{
		"alice": {"development-clusters", "testing-clusters"}, // alice 只能访问开发和测试环境
		"bob":   {"production-clusters", "staging-clusters"},  // bob 只能访问生产和预发环境
	}

	fmt.Println("集群中的 ManagedClusterSets:")
	for _, cs := range clusterSets {
		fmt.Printf("  - %s\n", cs.Name)
	}

	fmt.Println("\n用户权限配置:")
	for user, resources := range rbacConfig {
		fmt.Printf("  %s 可以访问: %v\n", user, resources)
	}

	fmt.Println("\n" + strings.Repeat("=", 50))

	// 演示直接使用 Lister 的问题
	fmt.Println("\n❌ 错误做法：直接使用 Lister")
	demonstrateDirectLister(clusterSets, aliceUser, bobUser)

	// 演示使用 Cache 的正确做法
	fmt.Println("\n✅ 正确做法：使用 RBAC Cache")
	demonstrateRBACCache(clusterSets, rbacConfig, aliceUser, bobUser)

	// 演示性能差异
	fmt.Println("\n⚡ 性能对比")
	demonstratePerformance()
}

// 错误的做法：直接使用 Lister
func demonstrateDirectLister(clusterSets []*clusterv1beta2.ManagedClusterSet, users ...user.Info) {
	fmt.Println("直接使用 lister.List() 的结果：")

	// 模拟 lister.List() - 返回所有资源
	allClusterSets := make([]clusterv1beta2.ManagedClusterSet, len(clusterSets))
	for i, cs := range clusterSets {
		allClusterSets[i] = *cs
	}

	for _, userInfo := range users {
		fmt.Printf("\n用户 %s 看到的资源:\n", userInfo.GetName())
		for _, cs := range allClusterSets {
			fmt.Printf("  - %s ⚠️  (可能没有权限访问!)\n", cs.Name)
		}
	}

	fmt.Println("\n🚨 安全问题：用户看到了所有资源，包括他们没有权限访问的！")
}

// 正确的做法：使用 RBAC Cache
func demonstrateRBACCache(clusterSets []*clusterv1beta2.ManagedClusterSet, rbacConfig map[string][]string, users ...user.Info) {
	fmt.Println("使用 RBAC Cache 的结果：")

	// 模拟 Cache 系统的权限过滤
	for _, userInfo := range users {
		userName := userInfo.GetName()
		allowedResources := rbacConfig[userName]

		fmt.Printf("\n用户 %s 看到的资源:\n", userName)
		for _, cs := range clusterSets {
			if contains(allowedResources, cs.Name) {
				fmt.Printf("  - %s ✅\n", cs.Name)
			}
			// 没有权限的资源不会显示
		}
	}

	fmt.Println("\n✅ 安全：用户只能看到有权限访问的资源！")
}

// 性能对比演示
func demonstratePerformance() {
	fmt.Println("\n假设集群中有 1000 个 ManagedClusterSets，100 个用户：")

	fmt.Println("\n方法对比:")
	fmt.Println("┌─────────────────────┬──────────────┬─────────────┬──────────────┐")
	fmt.Println("│ 方法                │ 时间复杂度   │ 安全性      │ 推荐程度     │")
	fmt.Println("├─────────────────────┼──────────────┼─────────────┼──────────────┤")
	fmt.Println("│ 直接 Lister         │ O(1)         │ ❌ 不安全   │ ❌ 不推荐    │")
	fmt.Println("│ 动态 RBAC 检查      │ O(n×m×k)     │ ✅ 安全     │ ❌ 太慢      │")
	fmt.Println("│ RBAC Cache          │ O(1)         │ ✅ 安全     │ ✅ 推荐      │")
	fmt.Println("└─────────────────────┴──────────────┴─────────────┴──────────────┘")

	fmt.Println("\n具体性能数据 (每次请求):")
	fmt.Println("  直接 Lister:      ~1ms   (但不安全)")
	fmt.Println("  动态 RBAC 检查:   ~500ms (安全但太慢)")
	fmt.Println("  RBAC Cache:       ~1ms   (安全且快速)")
}

// 辅助函数
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// 演示如果使用 SubjectAccessReview API 的简化方案
func demonstrateSubjectAccessReviewApproach() {
	fmt.Println("\n🔄 简化方案：使用 SubjectAccessReview API")

	// 伪代码演示
	pseudoCode := `
func ListWithSubjectAccessReview(userInfo user.Info) (*clusterv1beta2.ManagedClusterSetList, error) {
    // 1. 获取所有资源
    allClusterSets, _ := lister.List(labels.Everything())

    result := &clusterv1beta2.ManagedClusterSetList{}

    // 2. 对每个资源检查权限
    for _, clusterSet := range allClusterSets {
        sar := &authorizationv1.SubjectAccessReview{
            Spec: authorizationv1.SubjectAccessReviewSpec{
                User: userInfo.GetName(),
                ResourceAttributes: &authorizationv1.ResourceAttributes{
                    Group:    "cluster.open-cluster-management.io",
                    Resource: "managedclustersets",
                    Name:     clusterSet.Name,
                    Verb:     "get",
                },
            },
        }

        // 3. 调用 API Server 检查权限
        sar, err := kubeClient.AuthorizationV1().SubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
        if err == nil && sar.Status.Allowed {
            result.Items = append(result.Items, clusterSet)
        }
    }

    return result, nil
}
`

	fmt.Println(pseudoCode)

	fmt.Println("SubjectAccessReview 方案的特点:")
	fmt.Println("  ✅ 优点：代码简单，使用官方 API，权限检查准确")
	fmt.Println("  ❌ 缺点：每个资源都要调用 API Server，性能较差")
	fmt.Println("  📊 性能：1000 个资源需要 1000 次 API 调用")
	fmt.Println("  🎯 适用场景：小规模集群，资源数量少的情况")
}

// 实际的代码示例：展示当前 Cache 系统如何工作
func showCurrentCacheImplementation() {
	fmt.Println("\n📋 当前 Cache 系统的核心逻辑:")

	coreLogic := `
// 1. 权限索引构建 (启动时执行一次)
func (c *ClusterSetCache) rebuildPermissionCache() {
    // 分析所有 ClusterRoleBindings
    bindings, _ := c.clusterRoleBindingLister.List(labels.Everything())

    for _, binding := range bindings {
        // 获取对应的 ClusterRole
        role, _ := c.clusterRoleLister.Get(binding.RoleRef.Name)

        // 提取可访问的资源名称
        resourceNames := extractResourceNames(role, "managedclustersets")

        // 为每个用户/组建立权限映射
        for _, subject := range binding.Subjects {
            c.userPermissions[subject.Name] = resourceNames
        }
    }
}

// 2. 权限查询 (每次请求时执行)
func (c *ClusterSetCache) List(userInfo user.Info, selector labels.Selector) (*clusterv1beta2.ManagedClusterSetList, error) {
    // 从缓存中获取用户可访问的资源名称 - O(1)
    accessibleNames := c.getAccessibleResourceNames(userInfo)

    result := &clusterv1beta2.ManagedClusterSetList{}

    // 只获取有权限的资源
    for name := range accessibleNames {
        clusterSet, err := c.clusterSetLister.Get(name)  // 这里使用 lister
        if err == nil && selector.Matches(labels.Set(clusterSet.Labels)) {
            result.Items = append(result.Items, *clusterSet)
        }
    }

    return result, nil
}
`

	fmt.Println(coreLogic)

	fmt.Println("关键点:")
	fmt.Println("  🔄 权限索引在启动时构建，RBAC 变更时重建")
	fmt.Println("  ⚡ 查询时直接从内存索引获取权限信息，O(1) 复杂度")
	fmt.Println("  🎯 只对有权限的资源调用 lister.Get()")
	fmt.Println("  🔒 确保用户只能看到有权限访问的资源")
}
