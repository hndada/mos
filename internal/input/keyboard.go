package input

import (
	"sync"
	"time"

	"github.com/hndada/mos/internal/times"
)

type KeyEventKind int

const (
	KeyEventDown KeyEventKind = iota
	KeyEventUp
)

func (k KeyEventKind) String() string {
	switch k {
	case KeyEventDown:
		return "down"
	case KeyEventUp:
		return "up"
	default:
		return "unknown"
	}
}

// KeyEvent is emitted by Keyboard's async poller whenever a tracked key changes
// state. Time is measured from the same startTime passed to Listen, so rhythm
// games can judge with input timestamps instead of frame timestamps.
type KeyEvent struct {
	Time  time.Duration
	Key   Key
	Index int
	Kind  KeyEventKind
}

// type Keystrokes struct?
type KeyboardState struct {
	Time           time.Duration // Stands for elapsed time.
	AreKeysPressed []bool
}

func (a KeyboardState) isEqual(b KeyboardState) bool {
	for k, ap := range a.AreKeysPressed {
		bp := b.AreKeysPressed[k]
		if ap != bp {
			return false
		}
	}
	return true
}

// KeyboardStateBuffer is supposed to have at least one state.
type KeyboardStateBuffer struct {
	mu  sync.Mutex
	buf []KeyboardState
	idx int
}

func NewKeyboardStateBuffer(buf []KeyboardState) *KeyboardStateBuffer {
	return &KeyboardStateBuffer{
		buf: buf,
	}
}

// Read returns the last read state and unread states before given time.
// Read is guaranteed to return at least one state.
func (kb *KeyboardStateBuffer) Read(now time.Duration) (kss []KeyboardState) {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	if len(kb.buf) == 0 {
		return nil
	}
	if kb.idx >= len(kb.buf) {
		kb.idx = len(kb.buf) - 1
	}

	kss = append(kss, kb.buf[kb.idx])
	// It is fine to pass index up to len(buf): kb.idx+1 <= len(buf).
	for _, state := range kb.buf[kb.idx+1:] {
		if state.Time > now {
			break
		}
		kss = append(kss, state)
	}
	// To make the index pointing at the last state.
	kb.idx += len(kss) - 1
	return kss
}

func (kb *KeyboardStateBuffer) Trim() {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	kb.trimLocked()
}

func (kb *KeyboardStateBuffer) trimLocked() {
	if len(kb.buf) == 0 {
		return
	}

	trimmed := make([]KeyboardState, 1, len(kb.buf))
	trimmed[0] = kb.buf[0]

	old := kb.buf[0]
	for _, now := range kb.buf[1:] {
		if old.isEqual(now) {
			continue
		}
		trimmed = append(trimmed, now)
		old = now
	}
	kb.buf = trimmed
	if kb.idx >= len(kb.buf) {
		kb.idx = len(kb.buf) - 1
	}
}

// Output trims redundant states then returns the states.
func (kb *KeyboardStateBuffer) Output() []KeyboardState {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	kb.trimLocked()
	out := make([]KeyboardState, len(kb.buf))
	copy(out, kb.buf)
	return out
}

// A primary purpose of keyboard is to provide pairs of {time, keyboard state}.
type KeyboardReader interface {
	Read(now time.Duration) []KeyboardState
}

// type KeyboardListener interface {
// 	Listen()
// 	Stop()
// }

// Keyboard should not require additional adjustment when offset has changed,
// Because Keyboard cannot seek at precise position once it starts. Same goes for music.
type Keyboard struct {
	*KeyboardStateBuffer
	fetchKeyboardState func() []bool
	keys               []Key
	startTime          time.Time
	period             time.Duration

	listenMu sync.Mutex
	stop     chan struct{}
	running  bool

	eventMu  sync.Mutex
	events   []KeyEvent
	eventIdx int
	last     []bool
}

func NewKeyboard(keys []Key) *Keyboard {
	trackedKeys := append([]Key(nil), keys...)
	kb := &Keyboard{
		KeyboardStateBuffer: &KeyboardStateBuffer{},
		fetchKeyboardState:  newFetchKeyboardState(trackedKeys),
		keys:                trackedKeys,
	}
	first := KeyboardState{-10 * time.Second, make([]bool, len(trackedKeys))}
	kb.buf = append(kb.buf, first)
	kb.SetPollingRate(defaultPollingRate)
	return kb
}

func (kb *Keyboard) SetPollingRate(rate float64) {
	if rate <= 0 {
		rate = defaultPollingRate
	}
	second := float64(time.Second) * times.PlaybackRate()
	kb.period = time.Duration(second / rate)
	if kb.period <= 0 {
		kb.period = time.Nanosecond
	}
}

// Listen starts polling keyboard state.
func (kb *Keyboard) Listen(startTime time.Time) {
	kb.listenMu.Lock()
	if kb.running {
		kb.listenMu.Unlock()
		return
	}
	kb.startTime = startTime
	kb.stop = make(chan struct{})
	stop := kb.stop
	kb.running = true
	kb.listenMu.Unlock()

	kb.resetEvents()
	go func() {
		defer func() {
			kb.listenMu.Lock()
			kb.running = false
			kb.listenMu.Unlock()
		}()
		for {
			select {
			case <-stop:
				return
			default:
				start := times.Now()
				kb.poll()
				elapsed := times.Since(start)
				// It is fine to pass negative value to time.Sleep.
				// It is fine not to update period by changing playback rate;
				// It would just cause more or less of polling.
				time.Sleep(kb.period - elapsed)
			}
		}
	}()
}

func (kb *Keyboard) poll() {
	t := times.Since(kb.startTime)
	ps := kb.fetchKeyboardState()
	state := KeyboardState{t, ps}

	kb.KeyboardStateBuffer.mu.Lock()
	kb.buf = append(kb.buf, state)
	kb.KeyboardStateBuffer.mu.Unlock()

	kb.appendEvents(state)
}

func (kb *Keyboard) Stop() {
	kb.listenMu.Lock()
	defer kb.listenMu.Unlock()
	if !kb.running {
		return
	}
	close(kb.stop)
	kb.running = false
}

// ReadEvents returns unread key transitions up to now. It is intentionally
// independent from Read so a game can consume compact events for judgement
// while still keeping state snapshots for replay/debug output.
func (kb *Keyboard) ReadEvents(now time.Duration) []KeyEvent {
	kb.eventMu.Lock()
	defer kb.eventMu.Unlock()

	if kb.eventIdx >= len(kb.events) {
		return nil
	}

	end := kb.eventIdx
	for end < len(kb.events) && kb.events[end].Time <= now {
		end++
	}
	if end == kb.eventIdx {
		return nil
	}

	out := make([]KeyEvent, end-kb.eventIdx)
	copy(out, kb.events[kb.eventIdx:end])
	kb.eventIdx = end
	return out
}

// DrainEvents returns every unread key transition regardless of timestamp.
func (kb *Keyboard) DrainEvents() []KeyEvent {
	kb.eventMu.Lock()
	defer kb.eventMu.Unlock()

	if kb.eventIdx >= len(kb.events) {
		return nil
	}
	out := make([]KeyEvent, len(kb.events)-kb.eventIdx)
	copy(out, kb.events[kb.eventIdx:])
	kb.eventIdx = len(kb.events)
	return out
}

func (kb *Keyboard) resetEvents() {
	kb.eventMu.Lock()
	defer kb.eventMu.Unlock()

	kb.events = kb.events[:0]
	kb.eventIdx = 0
	kb.last = kb.fetchKeyboardState()
}

func (kb *Keyboard) appendEvents(state KeyboardState) {
	kb.eventMu.Lock()
	defer kb.eventMu.Unlock()

	if len(kb.last) != len(state.AreKeysPressed) {
		kb.last = append([]bool(nil), state.AreKeysPressed...)
		return
	}
	for i, pressed := range state.AreKeysPressed {
		if pressed == kb.last[i] {
			continue
		}
		kind := KeyEventUp
		if pressed {
			kind = KeyEventDown
		}
		kb.events = append(kb.events, KeyEvent{
			Time:  state.Time,
			Key:   kb.keys[i],
			Index: i,
			Kind:  kind,
		})
		kb.last[i] = pressed
	}
}

// No worry of accessing with nil pointer.
// https://go.dev/play/p/B4Z1LwQC_jP
