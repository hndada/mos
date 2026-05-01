package event

import "sync"

// Kind identifies the category of an event.
type Kind int

const (
	KindLifecycle  Kind = iota // app window phase changes
	KindNavigation             // user navigation intent (back/home/recents)
	KindSystem                 // OS-wide setting or sensor change
	KindCustom                 // app-to-app messaging
)

// Event is the common interface for all events on the Bus.
type Event interface {
	Kind() Kind
}

// --- Lifecycle ---

// LifecyclePhase is a discrete step in a window's life.
type LifecyclePhase int

const (
	PhaseCreated   LifecyclePhase = iota // window instantiated
	PhaseResumed                          // Showing → Shown (fully open)
	PhasePaused                           // Shown → Hiding (close animation begins)
	PhaseDestroyed                        // purged from the window list
)

// Lifecycle is published by the windowing server at each app phase transition.
type Lifecycle struct {
	AppID string
	Phase LifecyclePhase
}

func (Lifecycle) Kind() Kind { return KindLifecycle }

// --- Navigation ---

// NavAction is the direction of a system navigation gesture or button press.
type NavAction int

const (
	NavBack    NavAction = iota
	NavHome
	NavRecents
)

// Navigation is published whenever GoBack / GoHome / GoRecents is invoked.
type Navigation struct{ Action NavAction }

func (Navigation) Kind() Kind { return KindNavigation }

// --- System ---

// SystemTopic names a system-wide state that can change at runtime.
type SystemTopic string

const (
	TopicDarkMode  SystemTopic = "dark_mode"  // Value: bool
	TopicFontScale SystemTopic = "font_scale" // Value: float64 (1.0 = normal)
	TopicLocale    SystemTopic = "locale"     // Value: string ("en", "ko", …)
	TopicBattery   SystemTopic = "battery"    // Value: int (0–100)
	TopicNetwork   SystemTopic = "network"    // Value: bool (connected)
)

// System is published when a system setting or sensor reading changes.
type System struct {
	Topic SystemTopic
	Value any
}

func (System) Kind() Kind { return KindSystem }

// --- Custom (app-to-app) ---

// Custom is a generic inter-app event. Topic should be namespaced by the sender
// (e.g. "com.example.myapp/refresh") to avoid collisions.
type Custom struct {
	Topic string
	Data  any
}

func (Custom) Kind() Kind { return KindCustom }

// --- Bus ---

// Handler is a callback invoked when an event is published.
type Handler func(Event)

// Bus delivers events synchronously to all registered handlers.
// It is safe for concurrent use; handlers are called with the internal
// lock released so they may themselves publish events.
type Bus struct {
	mu       sync.Mutex
	handlers map[Kind][]entry
	seq      int
}

type entry struct {
	id int
	h  Handler
}

// NewBus returns an initialised, empty Bus.
func NewBus() *Bus {
	return &Bus{handlers: make(map[Kind][]entry)}
}

// Subscribe registers h for every event of the given kind.
// It returns an unsubscribe function; call it to deregister h.
func (b *Bus) Subscribe(k Kind, h Handler) (unsubscribe func()) {
	b.mu.Lock()
	id := b.seq
	b.seq++
	b.handlers[k] = append(b.handlers[k], entry{id, h})
	b.mu.Unlock()

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		es := b.handlers[k]
		for i, e := range es {
			if e.id == id {
				b.handlers[k] = append(es[:i], es[i+1:]...)
				return
			}
		}
	}
}

// Publish dispatches e to all handlers registered for e.Kind().
// Handlers are called sequentially after the lock is released.
func (b *Bus) Publish(e Event) {
	b.mu.Lock()
	raw := b.handlers[e.Kind()]
	hs := make([]Handler, len(raw))
	for i, entry := range raw {
		hs[i] = entry.h
	}
	b.mu.Unlock()

	for _, h := range hs {
		h(e)
	}
}
