package cache

import (
	"context"
	"fmt"
	"sync"

	clusterv1beta2 "open-cluster-management.io/api/cluster/v1beta2"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ControllerRuntimeClusterSetCache uses controller-runtime for modern Kubernetes integration
type ControllerRuntimeClusterSetCache struct {
	// Controller-runtime client and cache
	client client.Client
	cache  cache.Cache

	// RBAC permission index for fast lookups
	permissionIndex *RBACPermissionIndex

	// Resource name extraction function
	getResourceNamesFromClusterRole func(*rbacv1.ClusterRole, string, string) (sets.String, bool)

	// Watchers for real-time updates
	watchers    []CacheWatcher
	watcherLock sync.RWMutex

	// Context for lifecycle management
	ctx context.Context
}

// RBACPermissionIndex provides indexed RBAC permission lookups
type RBACPermissionIndex struct {
	// Indexed maps for O(1) lookups
	userToResources  map[string]sets.String
	groupToResources map[string]sets.String

	// Read-write mutex for concurrent access
	mu sync.RWMutex
}

// NewControllerRuntimeClusterSetCache creates a cache using controller-runtime
func NewControllerRuntimeClusterSetCache(
	mgr ctrl.Manager,
	getResourceNamesFromClusterRole func(*rbacv1.ClusterRole, string, string) (sets.String, bool),
) (*ControllerRuntimeClusterSetCache, error) {
	cache := &ControllerRuntimeClusterSetCache{
		client:                          mgr.GetClient(),
		cache:                           mgr.GetCache(),
		getResourceNamesFromClusterRole: getResourceNamesFromClusterRole,
		permissionIndex: &RBACPermissionIndex{
			userToResources:  make(map[string]sets.String),
			groupToResources: make(map[string]sets.String),
		},
		watchers: make([]CacheWatcher, 0),
		ctx:      context.Background(),
	}

	// Set up indexed fields for efficient queries
	if err := cache.setupIndexes(); err != nil {
		return nil, fmt.Errorf("failed to setup indexes: %w", err)
	}

	return cache, nil
}

// setupIndexes configures controller-runtime indexes for efficient queries
func (c *ControllerRuntimeClusterSetCache) setupIndexes() error {
	// Index ClusterRoleBindings by RoleRef for efficient lookups
	if err := c.cache.IndexField(c.ctx, &rbacv1.ClusterRoleBinding{}, "roleRef.name",
		func(obj client.Object) []string {
			binding := obj.(*rbacv1.ClusterRoleBinding)
			return []string{binding.RoleRef.Name}
		}); err != nil {
		return fmt.Errorf("failed to index ClusterRoleBinding by roleRef: %w", err)
	}

	// Index ClusterRoleBindings by Subject for efficient user/group lookups
	if err := c.cache.IndexField(c.ctx, &rbacv1.ClusterRoleBinding{}, "subjects",
		func(obj client.Object) []string {
			binding := obj.(*rbacv1.ClusterRoleBinding)
			subjects := make([]string, 0, len(binding.Subjects))
			for _, subject := range binding.Subjects {
				subjects = append(subjects, fmt.Sprintf("%s:%s", subject.Kind, subject.Name))
			}
			return subjects
		}); err != nil {
		return fmt.Errorf("failed to index ClusterRoleBinding by subjects: %w", err)
	}

	return nil
}

// Start initializes the cache and builds the permission index
func (c *ControllerRuntimeClusterSetCache) Start(ctx context.Context) error {
	c.ctx = ctx

	klog.V(2).Info("Starting controller-runtime ClusterSet cache")

	// Wait for cache to sync
	if !c.cache.WaitForCacheSync(ctx) {
		return fmt.Errorf("failed to sync controller-runtime cache")
	}

	// Build initial permission index
	if err := c.rebuildPermissionIndex(); err != nil {
		return fmt.Errorf("failed to build initial permission index: %w", err)
	}

	klog.V(2).Info("Controller-runtime ClusterSet cache started successfully")
	return nil
}

// List returns ManagedClusterSets accessible to the user using controller-runtime
func (c *ControllerRuntimeClusterSetCache) List(userInfo user.Info, selector labels.Selector) (*clusterv1beta2.ManagedClusterSetList, error) {
	// Get accessible resource names using indexed permissions
	accessibleNames := c.getAccessibleResourceNames(userInfo)

	clusterSetList := &clusterv1beta2.ManagedClusterSetList{}

	// Use controller-runtime client for consistent reads
	for name := range accessibleNames {
		clusterSet := &clusterv1beta2.ManagedClusterSet{}
		err := c.client.Get(c.ctx, client.ObjectKey{Name: name}, clusterSet)
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return nil, err
		}

		if !selector.Matches(labels.Set(clusterSet.Labels)) {
			continue
		}

		clusterSetList.Items = append(clusterSetList.Items, *clusterSet)
	}

	return clusterSetList, nil
}

// getAccessibleResourceNames efficiently retrieves accessible resources using indexed permissions
func (c *ControllerRuntimeClusterSetCache) getAccessibleResourceNames(userInfo user.Info) sets.String {
	c.permissionIndex.mu.RLock()
	defer c.permissionIndex.mu.RUnlock()

	accessibleNames := sets.NewString()

	// Check user permissions using index
	if userPerms, exists := c.permissionIndex.userToResources[userInfo.GetName()]; exists {
		accessibleNames = accessibleNames.Union(userPerms)
	}

	// Check group permissions using index
	for _, group := range userInfo.GetGroups() {
		if groupPerms, exists := c.permissionIndex.groupToResources[group]; exists {
			accessibleNames = accessibleNames.Union(groupPerms)
		}
	}

	return accessibleNames
}

// rebuildPermissionIndex efficiently rebuilds the permission index using controller-runtime
func (c *ControllerRuntimeClusterSetCache) rebuildPermissionIndex() error {
	c.permissionIndex.mu.Lock()
	defer c.permissionIndex.mu.Unlock()

	// Clear existing index
	c.permissionIndex.userToResources = make(map[string]sets.String)
	c.permissionIndex.groupToResources = make(map[string]sets.String)

	// List all ClusterRoleBindings using controller-runtime
	bindingList := &rbacv1.ClusterRoleBindingList{}
	if err := c.client.List(c.ctx, bindingList); err != nil {
		return fmt.Errorf("failed to list ClusterRoleBindings: %w", err)
	}

	// Process each binding efficiently
	for _, binding := range bindingList.Items {
		if err := c.processClusterRoleBinding(&binding); err != nil {
			klog.Errorf("Failed to process ClusterRoleBinding %s: %v", binding.Name, err)
			continue
		}
	}

	klog.V(2).Infof("Permission index rebuilt with %d users and %d groups",
		len(c.permissionIndex.userToResources), len(c.permissionIndex.groupToResources))

	return nil
}

// processClusterRoleBinding efficiently processes a single ClusterRoleBinding using controller-runtime
func (c *ControllerRuntimeClusterSetCache) processClusterRoleBinding(binding *rbacv1.ClusterRoleBinding) error {
	// Get the ClusterRole using controller-runtime client
	clusterRole := &rbacv1.ClusterRole{}
	err := c.client.Get(c.ctx, client.ObjectKey{Name: binding.RoleRef.Name}, clusterRole)
	if err != nil {
		return err
	}

	// Extract resource names using the provided function
	resourceNames, hasAll := c.getResourceNamesFromClusterRole(clusterRole, "cluster.open-cluster-management.io", "managedclustersets")
	if hasAll {
		// If user has access to all resources, get current list
		clusterSetList := &clusterv1beta2.ManagedClusterSetList{}
		if err := c.client.List(c.ctx, clusterSetList); err != nil {
			return fmt.Errorf("failed to list all ClusterSets: %w", err)
		}
		resourceNames = sets.NewString()
		for _, cs := range clusterSetList.Items {
			resourceNames.Insert(cs.Name)
		}
	}

	if resourceNames.Len() == 0 {
		return nil
	}

	// Process subjects efficiently with indexed storage
	for _, subject := range binding.Subjects {
		switch subject.Kind {
		case "User":
			if c.permissionIndex.userToResources[subject.Name] == nil {
				c.permissionIndex.userToResources[subject.Name] = sets.NewString()
			}
			c.permissionIndex.userToResources[subject.Name] = c.permissionIndex.userToResources[subject.Name].Union(resourceNames)

		case "Group":
			if c.permissionIndex.groupToResources[subject.Name] == nil {
				c.permissionIndex.groupToResources[subject.Name] = sets.NewString()
			}
			c.permissionIndex.groupToResources[subject.Name] = c.permissionIndex.groupToResources[subject.Name].Union(resourceNames)
		}
	}

	return nil
}

// GetAccessibleResourcesForUser returns all resources accessible to a specific user (for debugging/monitoring)
func (c *ControllerRuntimeClusterSetCache) GetAccessibleResourcesForUser(userName string) sets.String {
	c.permissionIndex.mu.RLock()
	defer c.permissionIndex.mu.RUnlock()

	if resources, exists := c.permissionIndex.userToResources[userName]; exists {
		return resources.Union(sets.NewString()) // Return a copy
	}
	return sets.NewString()
}

// GetAccessibleResourcesForGroup returns all resources accessible to a specific group (for debugging/monitoring)
func (c *ControllerRuntimeClusterSetCache) GetAccessibleResourcesForGroup(groupName string) sets.String {
	c.permissionIndex.mu.RLock()
	defer c.permissionIndex.mu.RUnlock()

	if resources, exists := c.permissionIndex.groupToResources[groupName]; exists {
		return resources.Union(sets.NewString()) // Return a copy
	}
	return sets.NewString()
}

// Watcher management (compatible with existing interface)

func (c *ControllerRuntimeClusterSetCache) AddWatcher(watcher CacheWatcher) {
	c.watcherLock.Lock()
	defer c.watcherLock.Unlock()
	c.watchers = append(c.watchers, watcher)
}

func (c *ControllerRuntimeClusterSetCache) RemoveWatcher(watcher CacheWatcher) {
	c.watcherLock.Lock()
	defer c.watcherLock.Unlock()

	for i, w := range c.watchers {
		if w == watcher {
			c.watchers = append(c.watchers[:i], c.watchers[i+1:]...)
			break
		}
	}
}

func (c *ControllerRuntimeClusterSetCache) notifyWatchers() {
	c.watcherLock.RLock()
	defer c.watcherLock.RUnlock()

	// Notify all watchers of changes
	for _, watcher := range c.watchers {
		// Create sets for notification
		names := sets.NewString()
		users := sets.NewString()
		groups := sets.NewString()

		// Populate from permission index
		c.permissionIndex.mu.RLock()
		for user, resources := range c.permissionIndex.userToResources {
			users.Insert(user)
			names = names.Union(resources)
		}
		for group, resources := range c.permissionIndex.groupToResources {
			groups.Insert(group)
			names = names.Union(resources)
		}
		c.permissionIndex.mu.RUnlock()

		watcher.GroupMembershipChanged(names, users, groups)
	}
}

// Interface compatibility methods

func (c *ControllerRuntimeClusterSetCache) ListObjects(userInfo user.Info) (runtime.Object, error) {
	return c.List(userInfo, labels.Everything())
}

func (c *ControllerRuntimeClusterSetCache) Get(name string) (runtime.Object, error) {
	clusterSet := &clusterv1beta2.ManagedClusterSet{}
	err := c.client.Get(c.ctx, client.ObjectKey{Name: name}, clusterSet)
	return clusterSet, err
}

func (c *ControllerRuntimeClusterSetCache) ConvertResource(name string) runtime.Object {
	clusterSet := &clusterv1beta2.ManagedClusterSet{}
	err := c.client.Get(c.ctx, client.ObjectKey{Name: name}, clusterSet)
	if err != nil {
		clusterSet = &clusterv1beta2.ManagedClusterSet{
			ObjectMeta: ctrl.ObjectMeta{Name: name},
		}
	}
	return clusterSet
}
