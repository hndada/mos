package windowing

import (
	"image/color"
	"reflect"
	"testing"

	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/event"
)

type lifecycleProbe struct {
	events *[]string
}

func (p *lifecycleProbe) Update(mosapp.Frame) {}
func (p *lifecycleProbe) Draw(draws.Image)    {}
func (p *lifecycleProbe) OnCreate(mosapp.Context) {
	*p.events = append(*p.events, "create")
}
func (p *lifecycleProbe) OnResume() {
	*p.events = append(*p.events, "resume")
}
func (p *lifecycleProbe) OnPause() {
	*p.events = append(*p.events, "pause")
}
func (p *lifecycleProbe) OnDestroy() {
	*p.events = append(*p.events, "destroy")
}

type resumeOnlyProbe struct {
	events *[]string
}

func (p *resumeOnlyProbe) Update(mosapp.Frame) {}
func (p *resumeOnlyProbe) Draw(draws.Image)    {}
func (p *resumeOnlyProbe) OnResume() {
	*p.events = append(*p.events, "resume")
}

func TestLifecycleDestroyPausesOnlyAfterResume(t *testing.T) {
	var events []string
	const appID = "lifecycle-probe-full"
	mosapp.Register(appID, func(mosapp.Context) mosapp.Content {
		return &lifecycleProbe{events: &events}
	})
	ws := &Server{ScreenW: 320, ScreenH: 640, Bus: event.NewBus()}

	w := NewRestoredWindow(AppState{ID: appID, Color: color.RGBA{1, 2, 3, 255}}, 320, 640, ws)
	w.Destroy()

	want := []string{"create", "resume", "pause", "destroy"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("restored lifecycle = %v, want %v", events, want)
	}
}

func TestLifecycleDismissBeforeResumeDoesNotPause(t *testing.T) {
	var events []string
	const appID = "lifecycle-probe-opening"
	mosapp.Register(appID, func(mosapp.Context) mosapp.Content {
		return &lifecycleProbe{events: &events}
	})
	ws := &Server{ScreenW: 320, ScreenH: 640, Bus: event.NewBus()}

	w := NewWindow(draws.XY{X: 160, Y: 600}, draws.XY{X: 48, Y: 48}, color.RGBA{1, 2, 3, 255}, appID, 320, 640, ws)
	w.Dismiss()
	w.Destroy()

	want := []string{"create", "destroy"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("opening lifecycle = %v, want %v", events, want)
	}
}

func TestLifecyclePhaseInterfacesAreIndependent(t *testing.T) {
	var events []string
	const appID = "lifecycle-probe-resume-only"
	mosapp.Register(appID, func(mosapp.Context) mosapp.Content {
		return &resumeOnlyProbe{events: &events}
	})
	ws := &Server{ScreenW: 320, ScreenH: 640, Bus: event.NewBus()}

	w := NewRestoredWindow(AppState{ID: appID, Color: color.RGBA{1, 2, 3, 255}}, 320, 640, ws)
	w.Destroy()

	want := []string{"resume"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("partial lifecycle = %v, want %v", events, want)
	}
}
