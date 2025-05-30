package cache

import (
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/authentication/user"
)

// mockWatchableCache implements WatchableCache for testing
type mockWatchableCache struct {
	objects  map[string]runtime.Object
	watchers []CacheWatcher
}

func newMockWatchableCache() *mockWatchableCache {
	return &mockWatchableCache{
		objects:  make(map[string]runtime.Object),
		watchers: make([]CacheWatcher, 0),
	}
}

func (m *mockWatchableCache) RemoveWatcher(watcher CacheWatcher) {
	for i, w := range m.watchers {
		if w == watcher {
			m.watchers = append(m.watchers[:i], m.watchers[i+1:]...)
			break
		}
	}
}

func (m *mockWatchableCache) ListObjects(user user.Info) (runtime.Object, error) {
	list := &metav1.List{
		TypeMeta: metav1.TypeMeta{
			Kind:       "List",
			APIVersion: "v1",
		},
	}

	for _, obj := range m.objects {
		list.Items = append(list.Items, runtime.RawExtension{Object: obj})
	}

	return list, nil
}

func (m *mockWatchableCache) Get(name string) (runtime.Object, error) {
	if obj, exists := m.objects[name]; exists {
		return obj, nil
	}
	return nil, nil
}

func (m *mockWatchableCache) ConvertResource(name string) runtime.Object {
	if obj, exists := m.objects[name]; exists {
		return obj
	}
	return &metav1.Status{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Status",
			APIVersion: "v1",
		},
		Status:  metav1.StatusFailure,
		Message: "Resource not found",
	}
}

// mockObject implements runtime.Object for testing
type mockObject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

func (m *mockObject) DeepCopyObject() runtime.Object {
	return &mockObject{
		TypeMeta: m.TypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:            m.Name,
			ResourceVersion: m.ResourceVersion,
		},
	}
}

func (m *mockWatchableCache) addObject(name, resourceVersion string) {
	obj := &mockObject{
		TypeMeta: metav1.TypeMeta{
			Kind:       "MockObject",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			ResourceVersion: resourceVersion,
		},
	}
	m.objects[name] = obj
}

func (m *mockWatchableCache) removeObject(name string) {
	delete(m.objects, name)
}

// mockUser implements user.Info for testing
type mockUser struct {
	name   string
	groups []string
}

func (m *mockUser) GetName() string {
	return m.name
}

func (m *mockUser) GetUID() string {
	return "test-uid"
}

func (m *mockUser) GetGroups() []string {
	return m.groups
}

func (m *mockUser) GetExtra() map[string][]string {
	return nil
}

func TestNewModernCacheWatcher(t *testing.T) {
	user := &mockUser{name: "test-user", groups: []string{"test-group"}}
	cache := newMockWatchableCache()

	// Add some test objects
	cache.addObject("resource1", "v1")
	cache.addObject("resource2", "v2")

	watcher := NewModernCacheWatcher(user, cache, true)

	if watcher == nil {
		t.Fatal("Expected watcher to be created, got nil")
	}

	if watcher.user != user {
		t.Errorf("Expected user to be %v, got %v", user, watcher.user)
	}

	if watcher.authCache != cache {
		t.Errorf("Expected authCache to be %v, got %v", cache, watcher.authCache)
	}

	if len(watcher.initialResources) != 2 {
		t.Errorf("Expected 2 initial resources, got %d", len(watcher.initialResources))
	}

	if watcher.GetKnownResourceCount() != 2 {
		t.Errorf("Expected 2 known resources, got %d", watcher.GetKnownResourceCount())
	}
}

func TestNewModernCacheWatcherWithOptions(t *testing.T) {
	user := &mockUser{name: "test-user", groups: []string{"test-group"}}
	cache := newMockWatchableCache()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := WatcherOptions{
		IncludeAllExistingResources: false,
		EventBufferSize:             500,
		EventTimeout:                10 * time.Second,
		Context:                     ctx,
	}

	watcher := NewModernCacheWatcherWithOptions(user, cache, opts)

	if watcher == nil {
		t.Fatal("Expected watcher to be created, got nil")
	}

	if watcher.eventBufferSize != 500 {
		t.Errorf("Expected event buffer size to be 500, got %d", watcher.eventBufferSize)
	}

	if watcher.eventTimeout != 10*time.Second {
		t.Errorf("Expected event timeout to be 10s, got %v", watcher.eventTimeout)
	}

	if len(watcher.initialResources) != 0 {
		t.Errorf("Expected 0 initial resources, got %d", len(watcher.initialResources))
	}
}

func TestModernCacheWatcher_GroupMembershipChanged(t *testing.T) {
	user := &mockUser{name: "test-user", groups: []string{"test-group"}}
	cache := newMockWatchableCache()

	// Add some test objects
	cache.addObject("resource1", "v1")
	cache.addObject("resource2", "v2")

	watcher := NewModernCacheWatcher(user, cache, false)
	watcher.Start()
	defer watcher.Stop()

	// Test case 1: User has access
	names := sets.NewString("resource1", "resource3")
	users := sets.NewString("test-user")
	groups := sets.NewString()

	// Add resource3 to cache
	cache.addObject("resource3", "v1")

	watcher.GroupMembershipChanged(names, users, groups)

	// Give some time for processing
	time.Sleep(100 * time.Millisecond)

	// Check that resource2 was removed (not in names)
	if _, exists := watcher.knownResources.Load("resource2"); exists {
		t.Error("Expected resource2 to be removed from known resources")
	}

	// Check that resource3 was added
	if _, exists := watcher.knownResources.Load("resource3"); !exists {
		t.Error("Expected resource3 to be added to known resources")
	}

	// Test case 2: User has no access
	names2 := sets.NewString("resource4")
	users2 := sets.NewString("other-user")
	groups2 := sets.NewString("other-group")

	initialCount := watcher.GetKnownResourceCount()
	watcher.GroupMembershipChanged(names2, users2, groups2)

	// Give some time for processing
	time.Sleep(100 * time.Millisecond)

	// Known resources should not change
	if watcher.GetKnownResourceCount() != initialCount {
		t.Errorf("Expected known resource count to remain %d, got %d",
			initialCount, watcher.GetKnownResourceCount())
	}
}

func TestModernCacheWatcher_WatchInterface(t *testing.T) {
	user := &mockUser{name: "test-user", groups: []string{"test-group"}}
	cache := newMockWatchableCache()

	// Add some test objects
	cache.addObject("resource1", "v1")

	watcher := NewModernCacheWatcher(user, cache, true)

	// Test ResultChan
	resultChan := watcher.ResultChan()
	if resultChan == nil {
		t.Fatal("Expected ResultChan to return a channel, got nil")
	}

	// Start the watcher
	watcher.Start()

	// Test that we can receive events
	select {
	case event := <-resultChan:
		if event.Type != watch.Added {
			t.Errorf("Expected first event to be Added, got %v", event.Type)
		}
	case <-time.After(1 * time.Second):
		t.Error("Expected to receive an event within 1 second")
	}

	// Test Stop
	if !watcher.IsStarted() {
		t.Error("Expected watcher to be started")
	}

	watcher.Stop()

	if watcher.IsStarted() {
		t.Error("Expected watcher to be stopped")
	}

	// Test that context is cancelled
	select {
	case <-watcher.Context().Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected context to be cancelled after Stop()")
	}
}

func TestModernCacheWatcher_EventProcessing(t *testing.T) {
	user := &mockUser{name: "test-user", groups: []string{"test-group"}}
	cache := newMockWatchableCache()

	watcher := NewModernCacheWatcher(user, cache, false)
	watcher.Start()
	defer watcher.Stop()

	resultChan := watcher.ResultChan()

	// Test adding a resource
	cache.addObject("new-resource", "v1")
	names := sets.NewString("new-resource")
	users := sets.NewString("test-user")
	groups := sets.NewString()

	watcher.GroupMembershipChanged(names, users, groups)

	// Should receive an ADD event
	select {
	case event := <-resultChan:
		if event.Type != watch.Added {
			t.Errorf("Expected ADD event, got %v", event.Type)
		}
		accessor, err := meta.Accessor(event.Object)
		if err != nil {
			t.Fatalf("Failed to get accessor: %v", err)
		}
		if accessor.GetName() != "new-resource" {
			t.Errorf("Expected resource name 'new-resource', got %s", accessor.GetName())
		}
	case <-time.After(1 * time.Second):
		t.Error("Expected to receive ADD event within 1 second")
	}

	// Test modifying a resource
	cache.addObject("new-resource", "v2") // Update resource version
	watcher.GroupMembershipChanged(names, users, groups)

	// Should receive a MODIFIED event
	select {
	case event := <-resultChan:
		if event.Type != watch.Modified {
			t.Errorf("Expected MODIFIED event, got %v", event.Type)
		}
	case <-time.After(1 * time.Second):
		t.Error("Expected to receive MODIFIED event within 1 second")
	}

	// Test removing a resource
	emptyNames := sets.NewString()
	watcher.GroupMembershipChanged(emptyNames, users, groups)

	// Should receive a DELETED event
	select {
	case event := <-resultChan:
		if event.Type != watch.Deleted {
			t.Errorf("Expected DELETED event, got %v", event.Type)
		}
	case <-time.After(1 * time.Second):
		t.Error("Expected to receive DELETED event within 1 second")
	}
}

func TestModernCacheWatcher_ConcurrentAccess(t *testing.T) {
	user := &mockUser{name: "test-user", groups: []string{"test-group"}}
	cache := newMockWatchableCache()

	watcher := NewModernCacheWatcher(user, cache, false)
	watcher.Start()
	defer watcher.Stop()

	// Test concurrent access to known resources
	done := make(chan bool, 2)

	// Goroutine 1: Add resources
	go func() {
		defer func() { done <- true }()
		for i := 0; i < 100; i++ {
			cache.addObject("resource-"+string(rune(i)), "v1")
			names := sets.NewString("resource-" + string(rune(i)))
			users := sets.NewString("test-user")
			groups := sets.NewString()
			watcher.GroupMembershipChanged(names, users, groups)
		}
	}()

	// Goroutine 2: Check resource count
	go func() {
		defer func() { done <- true }()
		for i := 0; i < 100; i++ {
			count := watcher.GetKnownResourceCount()
			_ = count // Just access it to test for race conditions
			time.Sleep(time.Millisecond)
		}
	}()

	// Wait for both goroutines to complete
	<-done
	<-done

	// Test should complete without race conditions
}

func TestModernCacheWatcher_ContextCancellation(t *testing.T) {
	user := &mockUser{name: "test-user", groups: []string{"test-group"}}
	cache := newMockWatchableCache()

	ctx, cancel := context.WithCancel(context.Background())
	opts := WatcherOptions{
		Context: ctx,
	}

	watcher := NewModernCacheWatcherWithOptions(user, cache, opts)
	watcher.Start()

	if !watcher.IsStarted() {
		t.Error("Expected watcher to be started")
	}

	// Cancel the context
	cancel()

	// Give some time for the cancellation to propagate
	time.Sleep(200 * time.Millisecond)

	// The watcher should still be marked as started, but context should be done
	select {
	case <-watcher.Context().Done():
		// Expected
	default:
		t.Error("Expected context to be cancelled")
	}
}

func BenchmarkModernCacheWatcher_GroupMembershipChanged(b *testing.B) {
	user := &mockUser{name: "test-user", groups: []string{"test-group"}}
	cache := newMockWatchableCache()

	// Add many resources
	for i := 0; i < 1000; i++ {
		cache.addObject("resource-"+string(rune(i)), "v1")
	}

	watcher := NewModernCacheWatcher(user, cache, false)
	watcher.Start()
	defer watcher.Stop()

	names := sets.NewString()
	for i := 0; i < 500; i++ {
		names.Insert("resource-" + string(rune(i)))
	}
	users := sets.NewString("test-user")
	groups := sets.NewString()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		watcher.GroupMembershipChanged(names, users, groups)
	}
}
