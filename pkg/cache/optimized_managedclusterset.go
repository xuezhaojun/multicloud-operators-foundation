package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	clusterinformerv1beta2 "open-cluster-management.io/api/client/cluster/informers/externalversions/cluster/v1beta2"
	clusterv1beta2lister "open-cluster-management.io/api/client/cluster/listers/cluster/v1beta2"
	clusterv1beta2 "open-cluster-management.io/api/cluster/v1beta2"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// OptimizedClusterSetCache uses official Kubernetes client-go cache for better performance
type OptimizedClusterSetCache struct {
	// Core listers using official client-go cache
	clusterSetLister         clusterv1beta2lister.ManagedClusterSetLister
	clusterRoleLister        rbacv1listers.ClusterRoleLister
	clusterRoleBindingLister rbacv1listers.ClusterRoleBindingLister

	// Informer factory for consistent cache management
	informerFactory informers.SharedInformerFactory

	// RBAC permission cache with optimized indexing
	permissionCache *PermissionCache

	// Resource name extraction function
	getResourceNamesFromClusterRole func(*rbacv1.ClusterRole, string, string) (sets.String, bool)

	// Watchers for real-time updates
	watchers    []CacheWatcher
	watcherLock sync.RWMutex

	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

// PermissionCache provides optimized RBAC permission caching
type PermissionCache struct {
	// Indexed cache for fast lookups
	userPermissions  map[string]sets.String // user -> resource names
	groupPermissions map[string]sets.String // group -> resource names

	// Mutex for thread-safe access
	mu sync.RWMutex

	// Last sync resource version for efficient updates
	lastSyncResourceVersion string
}

// NewOptimizedClusterSetCache creates a new optimized cache using official Kubernetes packages
func NewOptimizedClusterSetCache(
	kubeClient kubernetes.Interface,
	clusterSetInformer clusterinformerv1beta2.ManagedClusterSetInformer,
	getResourceNamesFromClusterRole func(*rbacv1.ClusterRole, string, string) (sets.String, bool),
) *OptimizedClusterSetCache {
	ctx, cancel := context.WithCancel(context.Background())

	// Use official informer factory for better performance
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 10*time.Minute)

	cache := &OptimizedClusterSetCache{
		clusterSetLister:                clusterSetInformer.Lister(),
		clusterRoleLister:               informerFactory.Rbac().V1().ClusterRoles().Lister(),
		clusterRoleBindingLister:        informerFactory.Rbac().V1().ClusterRoleBindings().Lister(),
		informerFactory:                 informerFactory,
		getResourceNamesFromClusterRole: getResourceNamesFromClusterRole,
		permissionCache: &PermissionCache{
			userPermissions:  make(map[string]sets.String),
			groupPermissions: make(map[string]sets.String),
		},
		watchers: make([]CacheWatcher, 0),
		ctx:      ctx,
		cancel:   cancel,
	}

	// Set up event handlers for efficient cache updates
	cache.setupEventHandlers()

	return cache
}

// setupEventHandlers configures optimized event handlers using official informers
func (c *OptimizedClusterSetCache) setupEventHandlers() {
	// ClusterRole event handler
	c.informerFactory.Rbac().V1().ClusterRoles().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onClusterRoleAdd,
		UpdateFunc: c.onClusterRoleUpdate,
		DeleteFunc: c.onClusterRoleDelete,
	})

	// ClusterRoleBinding event handler
	c.informerFactory.Rbac().V1().ClusterRoleBindings().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onClusterRoleBindingAdd,
		UpdateFunc: c.onClusterRoleBindingUpdate,
		DeleteFunc: c.onClusterRoleBindingDelete,
	})
}

// Start begins the optimized cache with official informers
func (c *OptimizedClusterSetCache) Start() error {
	klog.V(2).Info("Starting optimized ClusterSet cache with official Kubernetes informers")

	// Start all informers
	c.informerFactory.Start(c.ctx.Done())

	// Wait for cache sync using official cache sync mechanism
	if !cache.WaitForCacheSync(c.ctx.Done(),
		c.informerFactory.Rbac().V1().ClusterRoles().Informer().HasSynced,
		c.informerFactory.Rbac().V1().ClusterRoleBindings().Informer().HasSynced,
	) {
		return fmt.Errorf("failed to sync caches")
	}

	// Initial permission cache build
	c.rebuildPermissionCache()

	klog.V(2).Info("Optimized ClusterSet cache started successfully")
	return nil
}

// Stop gracefully shuts down the cache
func (c *OptimizedClusterSetCache) Stop() {
	klog.V(2).Info("Stopping optimized ClusterSet cache")
	c.cancel()
}

// List returns ManagedClusterSets accessible to the user with optimized performance
func (c *OptimizedClusterSetCache) List(userInfo user.Info, selector labels.Selector) (*clusterv1beta2.ManagedClusterSetList, error) {
	// Get accessible resource names using optimized permission cache
	accessibleNames := c.getAccessibleResourceNames(userInfo)

	clusterSetList := &clusterv1beta2.ManagedClusterSetList{}

	// Use consistent reads from cache (Kubernetes v1.31+ feature)
	for name := range accessibleNames {
		clusterSet, err := c.clusterSetLister.Get(name)
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

// getAccessibleResourceNames efficiently retrieves accessible resources using optimized cache
func (c *OptimizedClusterSetCache) getAccessibleResourceNames(userInfo user.Info) sets.String {
	c.permissionCache.mu.RLock()
	defer c.permissionCache.mu.RUnlock()

	accessibleNames := sets.NewString()

	// Check user permissions
	if userPerms, exists := c.permissionCache.userPermissions[userInfo.GetName()]; exists {
		accessibleNames = accessibleNames.Union(userPerms)
	}

	// Check group permissions
	for _, group := range userInfo.GetGroups() {
		if groupPerms, exists := c.permissionCache.groupPermissions[group]; exists {
			accessibleNames = accessibleNames.Union(groupPerms)
		}
	}

	return accessibleNames
}

// rebuildPermissionCache efficiently rebuilds the permission cache
func (c *OptimizedClusterSetCache) rebuildPermissionCache() {
	c.permissionCache.mu.Lock()
	defer c.permissionCache.mu.Unlock()

	// Clear existing cache
	c.permissionCache.userPermissions = make(map[string]sets.String)
	c.permissionCache.groupPermissions = make(map[string]sets.String)

	// Get all ClusterRoleBindings using official lister
	roleBindings, err := c.clusterRoleBindingLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list ClusterRoleBindings: %v", err)
		return
	}

	// Process each binding efficiently
	for _, binding := range roleBindings {
		c.processClusterRoleBinding(binding)
	}

	klog.V(2).Infof("Permission cache rebuilt with %d users and %d groups",
		len(c.permissionCache.userPermissions), len(c.permissionCache.groupPermissions))
}

// processClusterRoleBinding efficiently processes a single ClusterRoleBinding
func (c *OptimizedClusterSetCache) processClusterRoleBinding(binding *rbacv1.ClusterRoleBinding) {
	// Get the ClusterRole using official lister
	clusterRole, err := c.clusterRoleLister.Get(binding.RoleRef.Name)
	if err != nil {
		return
	}

	// Extract resource names using the provided function
	resourceNames, hasAll := c.getResourceNamesFromClusterRole(clusterRole, "cluster.open-cluster-management.io", "managedclustersets")
	if hasAll {
		// If user has access to all resources, get current list
		allClusterSets, err := c.clusterSetLister.List(labels.Everything())
		if err != nil {
			klog.Errorf("Failed to list all ClusterSets: %v", err)
			return
		}
		resourceNames = sets.NewString()
		for _, cs := range allClusterSets {
			resourceNames.Insert(cs.Name)
		}
	}

	if resourceNames.Len() == 0 {
		return
	}

	// Process subjects efficiently
	for _, subject := range binding.Subjects {
		switch subject.Kind {
		case "User":
			if c.permissionCache.userPermissions[subject.Name] == nil {
				c.permissionCache.userPermissions[subject.Name] = sets.NewString()
			}
			c.permissionCache.userPermissions[subject.Name] = c.permissionCache.userPermissions[subject.Name].Union(resourceNames)

		case "Group":
			if c.permissionCache.groupPermissions[subject.Name] == nil {
				c.permissionCache.groupPermissions[subject.Name] = sets.NewString()
			}
			c.permissionCache.groupPermissions[subject.Name] = c.permissionCache.groupPermissions[subject.Name].Union(resourceNames)
		}
	}
}

// Event handlers for efficient incremental updates

func (c *OptimizedClusterSetCache) onClusterRoleAdd(obj interface{}) {
	c.rebuildPermissionCache()
	c.notifyWatchers()
}

func (c *OptimizedClusterSetCache) onClusterRoleUpdate(oldObj, newObj interface{}) {
	c.rebuildPermissionCache()
	c.notifyWatchers()
}

func (c *OptimizedClusterSetCache) onClusterRoleDelete(obj interface{}) {
	c.rebuildPermissionCache()
	c.notifyWatchers()
}

func (c *OptimizedClusterSetCache) onClusterRoleBindingAdd(obj interface{}) {
	binding := obj.(*rbacv1.ClusterRoleBinding)
	c.permissionCache.mu.Lock()
	c.processClusterRoleBinding(binding)
	c.permissionCache.mu.Unlock()
	c.notifyWatchers()
}

func (c *OptimizedClusterSetCache) onClusterRoleBindingUpdate(oldObj, newObj interface{}) {
	c.rebuildPermissionCache()
	c.notifyWatchers()
}

func (c *OptimizedClusterSetCache) onClusterRoleBindingDelete(obj interface{}) {
	c.rebuildPermissionCache()
	c.notifyWatchers()
}

// Watcher management

func (c *OptimizedClusterSetCache) AddWatcher(watcher CacheWatcher) {
	c.watcherLock.Lock()
	defer c.watcherLock.Unlock()
	c.watchers = append(c.watchers, watcher)
}

func (c *OptimizedClusterSetCache) RemoveWatcher(watcher CacheWatcher) {
	c.watcherLock.Lock()
	defer c.watcherLock.Unlock()

	for i, w := range c.watchers {
		if w == watcher {
			c.watchers = append(c.watchers[:i], c.watchers[i+1:]...)
			break
		}
	}
}

func (c *OptimizedClusterSetCache) notifyWatchers() {
	c.watcherLock.RLock()
	defer c.watcherLock.RUnlock()

	// Notify all watchers of changes
	for _, watcher := range c.watchers {
		// Create sets for notification (simplified for this example)
		names := sets.NewString()
		users := sets.NewString()
		groups := sets.NewString()

		// Populate from permission cache
		c.permissionCache.mu.RLock()
		for user, resources := range c.permissionCache.userPermissions {
			users.Insert(user)
			names = names.Union(resources)
		}
		for group, resources := range c.permissionCache.groupPermissions {
			groups.Insert(group)
			names = names.Union(resources)
		}
		c.permissionCache.mu.RUnlock()

		watcher.GroupMembershipChanged(names, users, groups)
	}
}

// Interface compatibility methods

func (c *OptimizedClusterSetCache) ListObjects(userInfo user.Info) (runtime.Object, error) {
	return c.List(userInfo, labels.Everything())
}

func (c *OptimizedClusterSetCache) Get(name string) (runtime.Object, error) {
	return c.clusterSetLister.Get(name)
}

func (c *OptimizedClusterSetCache) ConvertResource(name string) runtime.Object {
	clusterSet, err := c.clusterSetLister.Get(name)
	if err != nil {
		clusterSet = &clusterv1beta2.ManagedClusterSet{ObjectMeta: metav1.ObjectMeta{Name: name}}
	}
	return clusterSet
}
