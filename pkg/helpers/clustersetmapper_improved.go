package helpers

import (
	"sync"

	"k8s.io/apimachinery/pkg/util/sets"
)

// ResourceType represents the type of resources being mapped
type ResourceType string

const (
	ResourceTypeCluster   ResourceType = "cluster"
	ResourceTypeNamespace ResourceType = "namespace"
)

// ClusterSetResourceMapper manages the mapping between ClusterSets and their associated resources
// This is a more specific and type-safe version of the original ClusterSetMapper
type ClusterSetResourceMapper struct {
	mutex        sync.RWMutex
	resourceType ResourceType
	// mapping: ClusterSet name -> Resource names
	setToResources map[string]sets.Set[string]
}

// NewClusterSetResourceMapper creates a new mapper for specific resource type
func NewClusterSetResourceMapper(resourceType ResourceType) *ClusterSetResourceMapper {
	return &ClusterSetResourceMapper{
		resourceType:   resourceType,
		setToResources: make(map[string]sets.Set[string]),
	}
}

// GetResourceType returns the type of resources this mapper manages
func (m *ClusterSetResourceMapper) GetResourceType() ResourceType {
	return m.resourceType
}

// UpdateClusterSetResources updates the complete resource set for a ClusterSet
func (m *ClusterSetResourceMapper) UpdateClusterSetResources(clusterSetName string, resources sets.Set[string]) {
	if clusterSetName == "" {
		return
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if resources.Len() == 0 {
		delete(m.setToResources, clusterSetName)
		return
	}
	m.setToResources[clusterSetName] = resources
}

// AddResourceToClusterSet adds a single resource to a ClusterSet
func (m *ClusterSetResourceMapper) AddResourceToClusterSet(resourceName, clusterSetName string) {
	if resourceName == "" || clusterSetName == "" {
		return
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.setToResources[clusterSetName]; !exists {
		m.setToResources[clusterSetName] = sets.New[string]()
	}
	m.setToResources[clusterSetName].Insert(resourceName)
}

// RemoveResourceFromAllClusterSets removes a resource from all ClusterSets
func (m *ClusterSetResourceMapper) RemoveResourceFromAllClusterSets(resourceName string) {
	if resourceName == "" {
		return
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	for clusterSetName, resources := range m.setToResources {
		if resources.Has(resourceName) {
			resources.Delete(resourceName)
			if resources.Len() == 0 {
				delete(m.setToResources, clusterSetName)
			}
		}
	}
}

// MoveResourceToClusterSet moves a resource from its current ClusterSet to a new one
func (m *ClusterSetResourceMapper) MoveResourceToClusterSet(resourceName, newClusterSetName string) {
	if resourceName == "" || newClusterSetName == "" {
		return
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Remove from all existing ClusterSets
	for clusterSetName, resources := range m.setToResources {
		if clusterSetName == newClusterSetName {
			continue
		}
		if resources.Has(resourceName) {
			resources.Delete(resourceName)
			if resources.Len() == 0 {
				delete(m.setToResources, clusterSetName)
			}
		}
	}

	// Add to new ClusterSet
	if _, exists := m.setToResources[newClusterSetName]; !exists {
		m.setToResources[newClusterSetName] = sets.New[string]()
	}
	m.setToResources[newClusterSetName].Insert(resourceName)
}

// GetResourcesInClusterSet returns all resources in a specific ClusterSet
func (m *ClusterSetResourceMapper) GetResourcesInClusterSet(clusterSetName string) sets.Set[string] {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if resources, exists := m.setToResources[clusterSetName]; exists {
		return resources.Clone() // Return a copy to prevent external modification
	}
	return sets.New[string]()
}

// GetClusterSetForResource returns the ClusterSet that contains the given resource
func (m *ClusterSetResourceMapper) GetClusterSetForResource(resourceName string) string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for clusterSetName, resources := range m.setToResources {
		if resources.Has(resourceName) {
			return clusterSetName
		}
	}
	return ""
}

// GetAllMappings returns a copy of all ClusterSet to resources mappings
func (m *ClusterSetResourceMapper) GetAllMappings() map[string]sets.Set[string] {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[string]sets.Set[string], len(m.setToResources))
	for clusterSetName, resources := range m.setToResources {
		result[clusterSetName] = resources.Clone()
	}
	return result
}

// RemoveClusterSet removes a ClusterSet and all its resource mappings
func (m *ClusterSetResourceMapper) RemoveClusterSet(clusterSetName string) {
	if clusterSetName == "" {
		return
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.setToResources, clusterSetName)
}

// Merge combines this mapper with another mapper of the same resource type
func (m *ClusterSetResourceMapper) Merge(other *ClusterSetResourceMapper) *ClusterSetResourceMapper {
	if m.resourceType != other.resourceType {
		// Cannot merge mappers of different resource types
		return m
	}

	currentMappings := m.GetAllMappings()
	otherMappings := other.GetAllMappings()

	if len(currentMappings) == 0 {
		return other
	}
	if len(otherMappings) == 0 {
		return m
	}

	merged := NewClusterSetResourceMapper(m.resourceType)

	// Add all current mappings
	for clusterSetName, resources := range currentMappings {
		if otherResources, exists := otherMappings[clusterSetName]; exists {
			// Merge resources for the same ClusterSet
			mergedResources := resources.Union(otherResources)
			merged.UpdateClusterSetResources(clusterSetName, mergedResources)
		} else {
			merged.UpdateClusterSetResources(clusterSetName, resources)
		}
	}

	// Add remaining mappings from other
	for clusterSetName, resources := range otherMappings {
		if _, exists := currentMappings[clusterSetName]; !exists {
			merged.UpdateClusterSetResources(clusterSetName, resources)
		}
	}

	return merged
}

// CopyFrom replaces all mappings with those from another mapper
func (m *ClusterSetResourceMapper) CopyFrom(source *ClusterSetResourceMapper) {
	if m.resourceType != source.resourceType {
		return // Cannot copy from mapper of different resource type
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Clear current mappings
	m.setToResources = make(map[string]sets.Set[string])

	// Copy all mappings from source
	sourceMappings := source.GetAllMappings()
	for clusterSetName, resources := range sourceMappings {
		m.setToResources[clusterSetName] = resources.Clone()
	}
}

// ClusterSetMappingManager manages multiple resource type mappers
type ClusterSetMappingManager struct {
	clusterMapper   *ClusterSetResourceMapper
	namespaceMapper *ClusterSetResourceMapper
}

// NewClusterSetMappingManager creates a new manager with separate mappers for different resource types
func NewClusterSetMappingManager() *ClusterSetMappingManager {
	return &ClusterSetMappingManager{
		clusterMapper:   NewClusterSetResourceMapper(ResourceTypeCluster),
		namespaceMapper: NewClusterSetResourceMapper(ResourceTypeNamespace),
	}
}

// GetClusterMapper returns the mapper for cluster resources
func (mgr *ClusterSetMappingManager) GetClusterMapper() *ClusterSetResourceMapper {
	return mgr.clusterMapper
}

// GetNamespaceMapper returns the mapper for namespace resources
func (mgr *ClusterSetMappingManager) GetNamespaceMapper() *ClusterSetResourceMapper {
	return mgr.namespaceMapper
}

// GetMapperForResourceType returns the appropriate mapper for the given resource type
func (mgr *ClusterSetMappingManager) GetMapperForResourceType(resourceType ResourceType) *ClusterSetResourceMapper {
	switch resourceType {
	case ResourceTypeCluster:
		return mgr.clusterMapper
	case ResourceTypeNamespace:
		return mgr.namespaceMapper
	default:
		return nil
	}
}
