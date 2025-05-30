package cache

import (
	"context"
	"errors"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/authentication/user"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
)

const (
	// Default buffer size for event channels
	defaultEventBufferSize = 1000
	// Default timeout for event processing
	defaultEventTimeout = 30 * time.Second
)

// ModernCacheWatcher is a modernized version of the cache watcher that combines
// official Kubernetes packages with custom permission-aware logic
type ModernCacheWatcher struct {
	user      user.Info
	authCache WatchableCache
	nsLister  corev1listers.NamespaceLister

	// Use sync.Map for thread-safe operations without explicit locking
	knownResources *sync.Map // map[string]string (name -> resourceVersion)

	// Context-based lifecycle management
	ctx    context.Context
	cancel context.CancelFunc

	// Simplified channel design - separate internal and external channels
	internalEvents chan watch.Event // Internal events from GroupMembershipChanged
	resultChan     chan watch.Event // External channel for ResultChan()
	errors         chan error

	// Configuration
	eventBufferSize int
	eventTimeout    time.Duration

	// State management
	started          bool
	initialResources []runtime.Object
	mu               sync.RWMutex

	// Event emission function (injectable for testing)
	emit func(watch.Event)
}

// NewModernCacheWatcher creates a new modernized cache watcher
func NewModernCacheWatcher(user user.Info, authCache WatchableCache, includeAllExistingResources bool) *ModernCacheWatcher {
	ctx, cancel := context.WithCancel(context.Background())

	w := &ModernCacheWatcher{
		user:            user,
		authCache:       authCache,
		knownResources:  &sync.Map{},
		ctx:             ctx,
		cancel:          cancel,
		internalEvents:  make(chan watch.Event, defaultEventBufferSize),
		resultChan:      make(chan watch.Event),
		errors:          make(chan error, 1),
		eventBufferSize: defaultEventBufferSize,
		eventTimeout:    defaultEventTimeout,
	}

	// Initialize known resources and initial resources
	w.initializeResources(includeAllExistingResources)

	// Set up default emit function
	w.emit = w.defaultEmit

	return w
}

// NewModernCacheWatcherWithOptions creates a new watcher with custom options
func NewModernCacheWatcherWithOptions(user user.Info, authCache WatchableCache, opts WatcherOptions) *ModernCacheWatcher {
	w := NewModernCacheWatcher(user, authCache, opts.IncludeAllExistingResources)

	if opts.EventBufferSize > 0 {
		w.eventBufferSize = opts.EventBufferSize
		// Recreate channel with new buffer size
		close(w.internalEvents)
		w.internalEvents = make(chan watch.Event, opts.EventBufferSize)
	}

	if opts.EventTimeout > 0 {
		w.eventTimeout = opts.EventTimeout
	}

	if opts.Context != nil {
		w.cancel() // Cancel the default context
		w.ctx, w.cancel = context.WithCancel(opts.Context)
	}

	return w
}

// WatcherOptions provides configuration options for the watcher
type WatcherOptions struct {
	IncludeAllExistingResources bool
	EventBufferSize             int
	EventTimeout                time.Duration
	Context                     context.Context
}

// initializeResources sets up the initial state of known resources
func (w *ModernCacheWatcher) initializeResources(includeAllExistingResources bool) {
	objectList, err := w.authCache.ListObjects(w.user)
	if err != nil {
		klog.Errorf("Failed to list objects for user %s: %v", w.user.GetName(), err)
		return
	}

	objs, err := meta.ExtractList(objectList)
	if err != nil {
		klog.Errorf("Failed to extract object list: %v", err)
		return
	}

	// Initialize known resources using sync.Map
	for _, object := range objs {
		accessor, err := meta.Accessor(object)
		if err != nil {
			klog.Errorf("Failed to get accessor for object: %v", err)
			continue
		}
		w.knownResources.Store(accessor.GetName(), accessor.GetResourceVersion())
	}

	// Set up initial resources if requested
	if includeAllExistingResources {
		w.initialResources = make([]runtime.Object, len(objs))
		copy(w.initialResources, objs)
	}
}

// GroupMembershipChanged implements the CacheWatcher interface
// This is the core permission-aware logic that cannot be replaced by standard packages
func (w *ModernCacheWatcher) GroupMembershipChanged(names, users, groups sets.String) {
	// Check if the user has access
	hasAccess := users.Has(w.user.GetName()) || groups.HasAny(w.user.GetGroups()...)
	if !hasAccess {
		return
	}

	// Handle resource deletions (permissions revoked)
	w.handleResourceDeletions(names)

	// Handle resource additions/modifications (permissions granted or resources updated)
	w.handleResourceUpdates(names)
}

// handleResourceDeletions processes resources that are no longer accessible
func (w *ModernCacheWatcher) handleResourceDeletions(accessibleNames sets.String) {
	w.knownResources.Range(func(key, value interface{}) bool {
		name := key.(string)
		if !accessibleNames.Has(name) {
			// Resource is no longer accessible, emit DELETE event
			w.knownResources.Delete(name)

			deleteEvent := watch.Event{
				Type:   watch.Deleted,
				Object: w.authCache.ConvertResource(name),
			}

			select {
			case w.internalEvents <- deleteEvent:
				klog.V(4).Infof("Emitted DELETE event for resource %s", name)
			case <-time.After(w.eventTimeout):
				// Handle timeout gracefully
				w.handleEventTimeout("delete", name)
				return false // Stop iteration
			case <-w.ctx.Done():
				return false // Stop iteration
			}
		}
		return true // Continue iteration
	})
}

// handleResourceUpdates processes accessible resources for additions/modifications
func (w *ModernCacheWatcher) handleResourceUpdates(accessibleNames sets.String) {
	for _, name := range accessibleNames.List() {
		object, err := w.authCache.Get(name)
		if err != nil {
			utilruntime.HandleError(err)
			continue
		}

		accessor, err := meta.Accessor(object)
		if err != nil {
			utilruntime.HandleError(err)
			continue
		}

		eventType := watch.Added
		currentResourceVersion := accessor.GetResourceVersion()

		// Check if this is a modification
		if lastResourceVersion, exists := w.knownResources.Load(name); exists {
			eventType = watch.Modified

			// Skip if we've already processed this resource version
			if lastResourceVersion.(string) == currentResourceVersion {
				continue
			}
		}

		// Update known resources
		w.knownResources.Store(name, currentResourceVersion)

		event := watch.Event{
			Type:   eventType,
			Object: object,
		}

		select {
		case w.internalEvents <- event:
			klog.V(4).Infof("Emitted %s event for resource %s", eventType, name)
		case <-time.After(w.eventTimeout):
			w.handleEventTimeout(string(eventType), name)
			return
		case <-w.ctx.Done():
			return
		}
	}
}

// handleEventTimeout handles timeout scenarios gracefully
func (w *ModernCacheWatcher) handleEventTimeout(eventType, resourceName string) {
	klog.Warningf("Event timeout for %s operation on resource %s, removing watcher", eventType, resourceName)
	w.authCache.RemoveWatcher(w)

	select {
	case w.errors <- errors.New("event notification timeout"):
	default:
		// Error channel is full, log the error
		klog.Errorf("Failed to send timeout error to error channel")
	}
}

// Start begins the watcher's operation using modern patterns
func (w *ModernCacheWatcher) Start() {
	w.mu.Lock()
	if w.started {
		w.mu.Unlock()
		return
	}
	w.started = true
	w.mu.Unlock()

	go w.run()
}

// run is the main event loop using official Kubernetes patterns
func (w *ModernCacheWatcher) run() {
	defer close(w.resultChan)
	defer func() {
		// Always remove the watcher from cache to avoid leaks
		w.authCache.RemoveWatcher(w)
	}()
	defer utilruntime.HandleCrash()

	// Emit initial resources
	w.emitInitialResources()

	// Main event processing loop
	for {
		select {
		case err := <-w.errors:
			w.emit(makeModernErrorEvent(err))
			return
		case event := <-w.internalEvents:
			// Monitor channel depth for performance tuning
			if curLen := int64(len(w.internalEvents)); watchChannelHWM.Update(curLen) {
				klog.V(2).Infof("watch: %v objects queued in managedCluster cache watching channel", curLen)
			}
			w.emit(event)
		case <-w.ctx.Done():
			return
		}
	}
}

// emitInitialResources sends all initial resources as ADD events
func (w *ModernCacheWatcher) emitInitialResources() {
	for _, resource := range w.initialResources {
		select {
		case err := <-w.errors:
			w.emit(makeModernErrorEvent(err))
			return
		case <-w.ctx.Done():
			return
		default:
		}

		w.emit(watch.Event{
			Type:   watch.Added,
			Object: resource.DeepCopyObject(),
		})
	}
}

// defaultEmit is the default event emission function
func (w *ModernCacheWatcher) defaultEmit(event watch.Event) {
	select {
	case w.resultChan <- event:
	case <-w.ctx.Done():
	}
}

// ResultChan implements watch.Interface
func (w *ModernCacheWatcher) ResultChan() <-chan watch.Event {
	return w.resultChan
}

// Stop implements watch.Interface with improved cleanup
func (w *ModernCacheWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.started {
		return
	}

	// Use context cancellation for clean shutdown
	w.cancel()
	w.started = false
}

// makeModernErrorEvent creates an error event with improved error handling
func makeModernErrorEvent(err error) watch.Event {
	return watch.Event{
		Type: watch.Error,
		Object: &metav1.Status{
			Status:  metav1.StatusFailure,
			Message: err.Error(),
			Code:    500,
		},
	}
}

// GetKnownResourceCount returns the number of currently known resources
// This is useful for monitoring and debugging
func (w *ModernCacheWatcher) GetKnownResourceCount() int {
	count := 0
	w.knownResources.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

// IsStarted returns whether the watcher has been started
func (w *ModernCacheWatcher) IsStarted() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.started
}

// Context returns the watcher's context for external coordination
func (w *ModernCacheWatcher) Context() context.Context {
	return w.ctx
}
