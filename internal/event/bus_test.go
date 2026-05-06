package event

import (
	"sync"
	"sync/atomic"
	"testing"
)

// ── Subscribe / Publish ───────────────────────────────────────────────────────

func TestBus_PublishReachesSubscriber(t *testing.T) {
	bus := NewBus()
	var got Event
	bus.Subscribe(KindSystem, func(e Event) { got = e })

	ev := System{Topic: TopicDarkMode, Value: true}
	bus.Publish(ev)

	if got == nil {
		t.Fatal("handler was not called")
	}
	se, ok := got.(System)
	if !ok {
		t.Fatalf("got %T, want System", got)
	}
	if se.Topic != TopicDarkMode {
		t.Errorf("topic = %v, want %v", se.Topic, TopicDarkMode)
	}
	if se.Value != true {
		t.Errorf("value = %v, want true", se.Value)
	}
}

func TestBus_WrongKindNotDelivered(t *testing.T) {
	bus := NewBus()
	called := false
	bus.Subscribe(KindNavigation, func(Event) { called = true })
	bus.Publish(System{Topic: TopicBattery, Value: 80})
	if called {
		t.Error("handler for KindNavigation should not fire on KindSystem event")
	}
}

func TestBus_MultipleHandlers(t *testing.T) {
	bus := NewBus()
	count := 0
	bus.Subscribe(KindLifecycle, func(Event) { count++ })
	bus.Subscribe(KindLifecycle, func(Event) { count++ })
	bus.Subscribe(KindLifecycle, func(Event) { count++ })

	bus.Publish(Lifecycle{AppID: "test", Phase: PhaseResumed})
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

// ── Unsubscribe ───────────────────────────────────────────────────────────────

func TestBus_Unsubscribe(t *testing.T) {
	bus := NewBus()
	count := 0
	unsub := bus.Subscribe(KindSystem, func(Event) { count++ })

	bus.Publish(System{Topic: TopicNetwork, Value: true})
	if count != 1 {
		t.Fatalf("before unsub: count = %d, want 1", count)
	}

	unsub()
	bus.Publish(System{Topic: TopicNetwork, Value: false})
	if count != 1 {
		t.Errorf("after unsub: count = %d, want still 1", count)
	}
}

func TestBus_UnsubscribeOneOfMany(t *testing.T) {
	bus := NewBus()
	a, b := 0, 0
	unsub := bus.Subscribe(KindSystem, func(Event) { a++ })
	bus.Subscribe(KindSystem, func(Event) { b++ })

	unsub()
	bus.Publish(System{Topic: TopicLocale, Value: "en"})

	if a != 0 {
		t.Errorf("unsubscribed handler a called %d times, want 0", a)
	}
	if b != 1 {
		t.Errorf("remaining handler b called %d times, want 1", b)
	}
}

func TestBus_UnsubscribeTwice(t *testing.T) {
	bus := NewBus()
	unsub := bus.Subscribe(KindCustom, func(Event) {})
	unsub()
	// Second call must not panic.
	unsub()
}

// ── Event kinds ───────────────────────────────────────────────────────────────

func TestEventKinds(t *testing.T) {
	cases := []struct {
		ev   Event
		kind Kind
	}{
		{Lifecycle{AppID: "a", Phase: PhaseCreated}, KindLifecycle},
		{Navigation{Action: NavBack}, KindNavigation},
		{System{Topic: TopicDarkMode, Value: false}, KindSystem},
		{Custom{Topic: "x/y", Data: 1}, KindCustom},
	}
	for _, tc := range cases {
		if tc.ev.Kind() != tc.kind {
			t.Errorf("%T.Kind() = %v, want %v", tc.ev, tc.ev.Kind(), tc.kind)
		}
	}
}

// ── Concurrent safety ─────────────────────────────────────────────────────────

func TestBus_ConcurrentPublish(t *testing.T) {
	bus := NewBus()
	var count atomic.Int64
	bus.Subscribe(KindSystem, func(Event) { count.Add(1) })

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			bus.Publish(System{Topic: TopicBattery, Value: 100})
		}()
	}
	wg.Wait()

	if got := count.Load(); got != goroutines {
		t.Errorf("count = %d, want %d", got, goroutines)
	}
}

func TestBus_ConcurrentSubscribeUnsubscribe(t *testing.T) {
	bus := NewBus()
	var wg sync.WaitGroup
	const n = 30
	wg.Add(n * 2)

	// Concurrent subscribe + unsubscribe.
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			unsub := bus.Subscribe(KindNavigation, func(Event) {})
			unsub()
		}()
		go func() {
			defer wg.Done()
			bus.Publish(Navigation{Action: NavHome})
		}()
	}
	wg.Wait() // must not deadlock or race
}

// ── Handler re-publishes ──────────────────────────────────────────────────────

func TestBus_HandlerCanRepublish(t *testing.T) {
	bus := NewBus()
	count := 0
	bus.Subscribe(KindSystem, func(e Event) {
		count++
		if count == 1 {
			// Re-publish a different kind from within the handler — must not deadlock.
			bus.Publish(Navigation{Action: NavBack})
		}
	})
	bus.Subscribe(KindNavigation, func(Event) { count++ })

	bus.Publish(System{Topic: TopicDarkMode, Value: true})
	if count != 2 {
		t.Errorf("count = %d, want 2 (system + nav re-publish)", count)
	}
}
