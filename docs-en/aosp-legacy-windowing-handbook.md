# AOSP Legacy Windowing Handbook for mos

## Purpose

This handbook uses the `internal/windowing` code in mos as a reference point for learning AOSP legacy windowing. Here, "legacy" does not mean one exact Android version. It means the long-lived concepts that keep appearing when reading Android's windowing code.

The goal is to separate three things:

1. The windowing concept being implemented
2. How AOSP implements that concept
3. How mos represents the same concept

mos is not a real multi-process Android system. Still, it compresses window stack, app lifecycle, composition, input routing, focus, system UI gating, recents, split-screen, PiP, and freeform windows into compact Go code. That makes it useful as a conceptual map before reading AOSP.

## Quick Map

```text
Concepts
  windowing server
  app/task lifecycle
  surface and composition
  focus and input dispatch
  coordinate transform
  system UI layers
  recents and snapshots
  multi-window modes

AOSP
  ActivityTaskManager / ActivityManager
  WindowManagerService
  InputReader / InputDispatcher
  SurfaceFlinger
  SystemUI / Launcher / IME / Keyguard

mos
  windowing.Server
  Window
  anim
  multiWindowState
  system app interfaces
  app.Context / AppCommand
  event.Bus
```

## 1. Role of the Windowing Server

### Concept to implement

The windowing layer decides what is on screen, in what order it is drawn, which window receives input, and when windows are created, shown, hidden, or destroyed.

Core responsibilities:

| Responsibility | Meaning |
|---|---|
| window stack | z-order of app windows |
| visibility | which windows are visible or hidden |
| lifecycle | showing, shown, hiding, destroyed transitions |
| layout | bounds and windowing mode |
| input routing | target selection for pointer and keyboard events |
| composition order | wallpaper, apps, and system UI layers |
| focus | foreground or focused window tracking |

### AOSP approach

AOSP splits this work across several systems.

| Component | Role |
|---|---|
| `ActivityTaskManager` / `ActivityManager` | Activity launch, task stack, process and lifecycle management |
| `WindowManagerService` | window state, layout, focus, visibility, app transitions |
| `InputDispatcher` | dispatches input to focused windows and input channels |
| `SurfaceFlinger` | final surface/layer composition |
| `SystemUI` | status bar, notification shade, keyguard, navigation |
| Launcher | home screen, recents, task snapshots |

The difficult part in AOSP is that "an app is visible" is split across Activity, Window, Surface, Task, Layer, and InputChannel objects.

### mos approach

In mos, `internal/windowing.Server` is the center of the windowing role.

Representative `Server` state:

| State | Meaning |
|---|---|
| `windows []*Window` | app window stack |
| `focusedWindow *Window` | pointer/touch focus |
| `keyboardFocus *Window` | keyboard/IME focus |
| `multi *multiWindowState` | split/PiP/freeform state |
| `wallpaper`, `home`, `hist`, `kb`, `statusBar`, `curtain`, `lock` | system UI layers |
| `Bus *event.Bus` | lifecycle, navigation, and system event broadcast |

```text
AOSP
  ActivityTaskManager + WindowManagerService + SurfaceFlinger + InputDispatcher

mos
  windowing.Server
```

## 2. App Launch and Window Creation

### Concept to implement

When the user taps a home icon or a recents card, an app starts and a window appears on screen.

Required steps:

| Step | Meaning |
|---|---|
| choose app identity | decide which app opens |
| create app content | instantiate UI/content |
| create window | create the on-screen container |
| launch animation | expand from icon/card to full screen |
| lifecycle callback | call create/resume |
| stack insertion | place the window at the top of z-order |

### AOSP approach

In AOSP, Activity launch flows through `ActivityTaskManager` and `ActivityManager`, then through process startup, ActivityRecord, Task, WindowState, and Surface creation. The app process owns the View hierarchy, system_server owns WindowState, and SurfaceFlinger owns final layer composition.

Conceptual flow:

```text
Launcher tap
  -> ActivityTaskManager startActivity
  -> ActivityRecord / Task update
  -> app process Activity lifecycle
  -> Window added to WindowManager
  -> Surface created
  -> app draws buffer
  -> SurfaceFlinger composes
```

### mos approach

In mos, `Server.Launch(appID)` compresses the flow.

```text
Server.Launch(appID)
  -> choose icon position/size
  -> launchApp(...)
  -> NewWindow(...)
  -> append to ws.windows
  -> publish Lifecycle PhaseCreated
```

`NewWindow()` handles:

| Work | Location |
|---|---|
| create full-screen canvas | `draws.CreateImage(screenW, screenH)` |
| create app/context/proc | `newWindowProc`, `newWindowContext`, `NewApp` |
| initialize lifecycle | `LifecycleShowing` |
| launch animation | `anim.OpenFrom(iconPos, iconSize, fullSize, DurationOpening)` |
| create callback | `proc.create(app.content, ctx)` |

In mos, `Window` combines parts of Android's Activity, WindowState, and Surface roles into one small object.

## 3. Window Object and Surface

### Concept to implement

An app needs a place to draw UI, and the windowing layer must place the result somewhere on screen.

That creates two coordinate spaces:

| Coordinate space | Meaning |
|---|---|
| app canvas coordinates | the logical surface the app draws into |
| screen coordinates | the compositor placement on the display |

### AOSP approach

In AOSP, the app draws through `ViewRootImpl` into a surface buffer. WMS manages bounds, visibility, and z-order. SurfaceFlinger composes the buffers as layers.

Related concepts:

| Concept | Meaning |
|---|---|
| `WindowState` | window state inside WMS |
| `SurfaceControl` | handle for surface/layer control |
| buffer | pixel data drawn by the app |
| transaction | batched position, crop, alpha, and layer changes |
| SurfaceFlinger | final composition |

### mos approach

`Window` has a `canvas draws.Image`. Apps always draw into a full-screen canvas.

```go
type Window struct {
    canvas draws.Image
    anim   anim
    mode   Mode
    placement Placement
}
```

`Window.Draw(dst)` draws that canvas into the compositor target.

| mos element | AOSP analogy |
|---|---|
| `Window.canvas` | app surface/buffer |
| `Window.Draw()` | participation in layer composition |
| `anim.Pos()`, `anim.Size()`, `anim.Alpha()` | surface transform/alpha |
| `placement` | requested bounds |
| `mode` | windowing mode |

mos does not have a separate SurfaceFlinger process. `Server.Draw()` performs composition directly.

## 4. Composition Order

### Concept to implement

The screen is the result of compositing multiple layers.

Example:

```text
wallpaper
home or recents
app windows
multi-window overlays
keyboard
status bar
notification shade
lock screen
```

Higher layers draw over lower layers and may also block input to them.

### AOSP approach

AOSP uses WMS for layer ordering and window types, and SurfaceFlinger for final layer composition. SystemUI, IME, keyguard, and app windows are arranged as different window types or special layers.

Important concepts:

| Concept | Meaning |
|---|---|
| application window | app content |
| system window | status bar, navigation bar, IME, keyguard |
| z-order/layer | what appears above what |
| visibility | whether a window is shown or hidden |
| alpha/transform | opacity, position, and size during animation |

### mos approach

mos exposes composition order directly in `Server.Draw()`.

```text
wallpaper
home
recents/history
pip border
windows
split/freeform overlays
keyboard
status bar
curtain with blurred background
lock
```

This function is a compact model of AOSP's WMS/SurfaceFlinger layer ordering. The `curtain` receives `blurredScene(dst)` just before drawing, which models the idea of a notification shade or system UI backdrop.

## 5. Lifecycle and App Callbacks

### Concept to implement

A window is not fully visible the moment it is created. It moves through opening, fully shown, closing, hidden, and destroyed states. App callbacks must follow that flow.

### AOSP approach

In AOSP, Activity lifecycle and window visibility are related but not identical.

| Area | Examples |
|---|---|
| Activity lifecycle | created, started, resumed, paused, stopped, destroyed |
| Window state | added, visible, animating, hidden, removed |
| Process state | foreground, cached, empty |
| Transition | opening, closing, changing |

### mos approach

mos has direct `Window` lifecycle states.

```go
LifecycleInitializing
LifecycleShowing
LifecycleShown
LifecycleHiding
LifecycleHidden
LifecycleDestroying
LifecycleDestroyed
```

Core flow:

```text
NewWindow
  -> LifecycleShowing
  -> OnCreate
  -> open animation
  -> LifecycleShown
  -> OnResume

Dismiss
  -> LifecycleHiding
  -> OnPause
  -> close animation
  -> LifecycleHidden
  -> Destroy
  -> OnDestroy
  -> LifecycleDestroyed
```

`Server.logLifecycleChange()` publishes lifecycle transitions on the event bus.

| mos lifecycle event | Meaning |
|---|---|
| `PhaseCreated` | window/app created |
| `PhaseResumed` | window fully shown |
| `PhasePaused` | close/hide begins |
| `PhaseDestroyed` | window removed |

## 6. Launch, Close, and App Transition Animation

### Concept to implement

Windows should not simply appear or disappear. They can expand from an icon/card or shrink back to a target. Transitions should remain continuous even if interrupted.

### AOSP approach

AOSP has app transitions, starting windows, remote animations, and surface transactions. Details vary by Android version, but the core idea is changing surface bounds and alpha over time while avoiding visual blank gaps.

### mos approach

`internal/windowing/anim.go` drives window transitions.

| Action | mos method |
|---|---|
| open from icon | `anim.OpenFrom(...)` |
| close to target rect | `anim.CloseTo(...)` |
| change multi-window bounds | `anim.Retarget(...)` |
| move immediately | `anim.SnapTo(...)` |

`DismissTo()` can reverse an opening transition smoothly because the transition re-bases from the current animated value.

## 7. Input Routing

### Concept to implement

Input occurs in screen space, but apps should receive only the events meant for their window, translated into app coordinates. If the notification shade or lockscreen is active, lower apps should not receive input.

### AOSP approach

`InputReader` reads device events. `InputDispatcher` sends them to the focused window, input channel, and touch target.

Important concepts:

| Concept | Meaning |
|---|---|
| focused window | receives keyboard/input focus |
| touch target | window where pointer down began |
| input channel | path to deliver events to the app process |
| input window handle | bounds, flags, and touchable region for dispatch |
| pointer capture | drag continues to same target even outside bounds |

### mos approach

`Server.Update()` is the center of input routing.

```text
inputProducer.Poll()
  -> lock/curtain gating
  -> system UI update
  -> multi-window state consumes drag gestures
  -> focus update
  -> frameForWindow()
  -> Window.UpdateApp(frame)
```

`filterEventsByRect()` captures a pointer if its `EventDown` starts inside a window.

```text
EventDown inside window
  -> captured[pointer] = true
EventMove outside window
  -> still delivered while captured
EventUp
  -> delivered, then capture removed
```

This is a compact model of AOSP touch targets and pointer capture.

## 8. Focus

### Concept to implement

The window receiving pointer input and the window receiving keyboard/IME input can be different, especially in PiP and freeform modes.

### AOSP approach

AOSP uses WMS and InputDispatcher to track focused windows, focused apps, and IME targets. The IME target can differ from the touch target.

### mos approach

mos splits focus into two fields.

| Field | Meaning |
|---|---|
| `focusedWindow` | pointer interaction target in split/freeform/PiP |
| `keyboardFocus` | window receiving keyboard/IME input |

`grantFocus()` clears the previous window's `ctx.hasFocus` and marks the new one focused.

```text
app
  -> ctx.RequestFocus()
  -> CmdRequestFocus
  -> Server.grantFocus(window)
  -> ctx.HasFocus() becomes true
```

## 9. Coordinate Transform

### Concept to implement

When a window is moved, scaled, or clipped, screen coordinates and app coordinates differ. Events must be transformed back into the app's canvas space.

### AOSP approach

In AOSP, surface bounds, crop, transform, window frame, compatibility scale, and display rotation can affect input coordinates. WMS provides input window information used by InputDispatcher.

### mos approach

The app canvas in mos is always full-screen size. Even if a window is PiP or freeform, the app draws into the full canvas, so input must be mapped back.

| Mode | Transform |
|---|---|
| fullscreen | no transform |
| split | subtract pane offset, no scale |
| PiP/freeform | scale window rect back to full-screen canvas |

Related methods:

| Method | Role |
|---|---|
| `ToCanvasFrame()` | maps a scaled rect to full canvas coordinates |
| `ToSplitCanvasFrame()` | applies split pane offset |
| `ContainsScreenPos()` | tests if screen position is inside current animated rect |

## 10. Multi-window

### Concept to implement

Windowing modes like split-screen, PiP, and freeform change bounds, focus, input routing, resize animation, and overlays.

### AOSP approach

AOSP uses task/windowing modes, root tasks, activity bounds, surface placement, organizers, and related controllers. Legacy readings often talk about split-screen stacks, pinned stacks, and freeform stacks.

### mos approach

`multiWindowState` owns the three modes.

| Mode | mos representation | AOSP analogy |
|---|---|---|
| split | `multiModeSplit`, `ModeSplit` | split-screen |
| PiP | `multiModePip`, `ModePip` | pinned/PiP window |
| freeform | `multiModeFreeform`, `ModeFloat` | freeform windowing |

### Split

`enterSplit()` selects the top two shown windows as primary and secondary and chooses vertical or horizontal split based on display shape.

| Element | Meaning |
|---|---|
| `splitFrac` | primary pane fraction |
| divider drag | `updateSplit()` consumes divider gestures |
| `applySplitPlacements()` | retargets both windows |
| `drawSplit()` | draws divider overlay |

### PiP

`enterPip()` shrinks the top window into a corner overlay.

| Element | Meaning |
|---|---|
| `pipPlacement()` | computes rect for each corner |
| drag | moves PiP within the screen |
| snap | snaps to nearest corner on release |
| event split | separates PiP events from main window events |

### Freeform

`enterFreeform()` turns shown windows into floating rects.

| Element | Meaning |
|---|---|
| title bar | drag handle |
| focus ring | visual focused-window indicator |
| stagger | offsets multiple windows |
| `updateFreeform()` | handles title-bar drag and focus changes |

## 11. System UI Layers

### Concept to implement

Mobile OS screens are not just app windows. System UI layers are always part of the scene.

| Layer | Role |
|---|---|
| wallpaper | background |
| home/launcher | app launch surface |
| recents | recent app/task switcher |
| keyboard/IME | text input |
| status bar | time/status display |
| notification shade | notifications and quick settings |
| lockscreen/keyguard | locked state |

### AOSP approach

SystemUI, Launcher, IME, and Keyguard are separate apps, processes, or services. WMS treats them as system window types or special layers.

### mos approach

`internal/windowing/system_apps.go` defines system UI layer interfaces.

| mos interface | Meaning |
|---|---|
| `Wallpaper` | background draw |
| `Home` | launcher update/draw and icon taps |
| `History` | recents card handling |
| `Keyboard` | soft keyboard show/hide/update/draw |
| `StatusBar` | status bar height/update/draw |
| `Curtain` | notification shade / quick panel |
| `Lock` | lockscreen |

`Server` updates and draws these interfaces in a fixed order.

## 12. Insets and Safe Area

### Concept to implement

Apps need to avoid areas occupied by status bars, keyboards, navigation bars, and other system UI.

### AOSP approach

Android delivers `WindowInsets` for status bars, navigation bars, IME, display cutouts, and gesture regions.

### mos approach

mos simplifies this into `Context.SafeArea()`.

`Server.SafeArea()` currently reflects:

| Source | Safe area |
|---|---|
| `statusBar.Height()` | top |
| `keyboard.Height()` | bottom |

Each frame, `Server.Update()` sets the safe area on every window context.

```text
sa := ws.SafeArea()
for each window:
  w.ctx.setSafeArea(sa)
```

## 13. Recents and Snapshots

### Concept to implement

When the user goes home or opens recents, the current app state should be represented as a card and later selectable.

### AOSP approach

AOSP uses task snapshots, recent task lists, Launcher, Quickstep, and SystemUI to provide recents.

### mos approach

`History` and `Window.HistoryEntry()` model this.

```text
GoHome or GoRecents
  -> active window HistoryEntry()
  -> History.AddCard(...)
  -> window dismisses to card rect
  -> History draws recents cards
```

`HistoryEntry()` copies the current window canvas into a snapshot. This is a small model of Android task snapshots.

## 14. Input Gating: Curtain and Lock

### Concept to implement

When notification shade or lockscreen is active, apps underneath should not receive touch input.

### AOSP approach

Keyguard, status bar shade, IME, and modal system windows use input focus, touchable regions, and window flags to block lower windows.

### mos approach

mos computes `gateBelow` in `Server.Update()`.

```text
locked := ws.IsLocked()
curtainOpen := !locked && ws.curtain != nil && ws.curtain.IsVisible()
gateBelow := locked || curtainOpen
```

When `gateBelow` is true, home, recents, and app windows do not receive normal input. Lock or curtain receives the frame events instead.

## 15. App-to-System Requests

### Concept to implement

Apps should not mutate windowing or system UI state directly. They should request work through a system API, and the system should process that request on the main loop.

### AOSP approach

Android apps use Binder calls to system services.

| Request | AOSP service |
|---|---|
| Activity launch | ActivityTaskManager |
| window update | WindowManager |
| keyboard show/hide | InputMethodManager |
| notification | NotificationManager |
| clipboard | ClipboardManager |

### mos approach

mos uses `app.Context` and `AppCommand` as the boundary.

```text
app content
  -> ctx method
  -> windowContext sends Cmd...
  -> windowProc queues command
  -> Server drains command on main goroutine
  -> Server mutates state
```

`commands.go` can be read like a small list of system-service transactions.

| Command | Meaning |
|---|---|
| `CmdFinish` | close window |
| `CmdLaunch` | launch app |
| `CmdShowKeyboard` / `CmdHideKeyboard` | IME visibility |
| `CmdPostNotice` | notification |
| `CmdSetDarkMode` | system setting |
| `CmdRequestFocus` / `CmdReleaseFocus` | keyboard focus |

## 16. Event Bus

### Concept to implement

Lifecycle, navigation, and system setting changes should be visible to other components.

### AOSP approach

AOSP uses Binder callbacks, broadcasts, lifecycle callbacks, configuration changes, and listeners.

### mos approach

mos simplifies this into `event.Bus`.

| Event kind | Meaning |
|---|---|
| `KindLifecycle` | app/window lifecycle |
| `KindNavigation` | back/home/recents |
| `KindSystem` | system setting or sensor |
| `KindCustom` | app-to-app or system custom event |

`Server` publishes lifecycle transitions and navigation actions.

## 17. mos Reading Order

### 1. `internal/windowing/server.go`

Start here. It contains the frame loop and layer order.

| Item | Why it matters |
|---|---|
| `Server` struct | state owned by the windowing server |
| `Update()` | input, lifecycle, and system UI update order |
| `Draw()` | composition order |
| `Launch()`, `GoHome()`, `GoBack()`, `GoRecents()` | task/window navigation |
| `frameForWindow()` | input routing |
| `grantFocus()` | keyboard focus |

### 2. `internal/windowing/window.go`

Read the per-window lifecycle and surface model.

| Item | Why it matters |
|---|---|
| `Window` struct | compressed Activity/Window/Surface role |
| `NewWindow()` | app launch path |
| `DismissTo()` | close transition |
| `Update()` | animation completion and resume |
| `UpdateApp()` | app update and command drain |
| `Draw()` | canvas composition |
| `ToCanvasFrame()` | input coordinate transform |

### 3. `internal/windowing/anim.go`

Read how window transitions can be retargeted.

| Item | Why it matters |
|---|---|
| open/close transition | app transition model |
| position/size/alpha | surface transform analogy |
| retarget | interrupted transition analogy |

### 4. `internal/windowing/multiwin.go`

This maps directly to Android split-screen, PiP, and freeform concepts.

| Item | Why it matters |
|---|---|
| `multiMode` | windowing mode |
| `enterSplit`, `enterPip`, `enterFreeform` | mode entry |
| `updateSplit`, `updatePip`, `updateFreeform` | gestures and routing |
| `drawSplit`, `drawPip`, `drawFreeform` | overlays |
| `cleanup` | anchor-window destruction handling |

### 5. `internal/windowing/system_apps.go`

Read the SystemUI layer interfaces.

### 6. `internal/windowing/app_context.go`

Read the boundary where apps request system/windowing work.

### 7. `internal/windowing/commands.go`

Read this like a list of small Binder-style transactions.

### 8. `internal/event/bus.go`

Read lifecycle, navigation, and system events.

## 18. AOSP Reading Order

### 1. Start with responsibility boundaries

Do not begin by memorizing classes. Start with these questions:

| Question | Where to look in AOSP |
|---|---|
| Who owns Activity/task state? | ActivityTaskManager / ActivityManager |
| Who owns window state? | WindowManagerService |
| Who dispatches input? | InputDispatcher |
| Who composes surfaces? | SurfaceFlinger |
| Where is system UI? | SystemUI, Launcher, IME, Keyguard |

### 2. Read the launch path

```text
tap launcher icon
  -> startActivity
  -> ActivityRecord/Task
  -> app process lifecycle
  -> add window
  -> create surface
  -> draw buffer
  -> compose
```

In mos, this is compressed around `Server.Launch()` and `NewWindow()`.

### 3. Read the draw/composition path

```text
app draws buffer
  -> WMS/SF know layer state
  -> SurfaceFlinger composes layers
```

In mos, this corresponds to `Window.updateCanvas()` and `Server.Draw()`.

### 4. Read the input path

```text
hardware input
  -> InputReader
  -> InputDispatcher
  -> focused/touch target window
  -> app receives event
```

In mos, this corresponds to `inputProducer.Poll()` through `frameForWindow()`.

### 5. Read the multi-window path

For split/PiP/freeform, focus on bounds, surface transforms, input targets, and focus rather than only Activity objects.

`multiWindowState` gives the compact version of that map.

## 19. Comparison Cases

### Case A: Tap an app icon

#### Concept to implement

Home selects an app, a new app window opens, and lifecycle begins.

#### AOSP approach

Launcher calls `startActivity`. ActivityTaskManager prepares task/activity state. The app process creates the Activity, WMS tracks the window, and SurfaceFlinger composes the surface.

#### mos approach

```text
Home.TappedIcon()
  -> Server.launchApp(...)
  -> NewWindow(...)
  -> anim.OpenFrom(...)
  -> proc.create(...)
  -> windows append
```

### Case B: Back/Home/Recents

#### Concept to implement

Navigation actions affect the foreground window and task history.

#### AOSP approach

Back handling crosses Activity callbacks, task stack state, predictive back, and WMS transitions. Home and Recents involve Launcher/SystemUI and ActivityTaskManager.

#### mos approach

| Action | mos method |
|---|---|
| back | `Server.GoBack()` |
| home | `Server.GoHome()` |
| recents | `Server.GoRecents()` |

`HistoryEntry()` creates a snapshot, and `History.AddCard()` stores it in recents.

### Case C: Notification shade is open

#### Concept to implement

When shade is open, apps underneath should not receive input.

#### AOSP approach

SystemUI's notification shade window appears above apps, and WMS/InputDispatcher prevent touches from falling through.

#### mos approach

```text
curtainOpen = true
gateBelow = true
curtain.Update(frame with events)
app windows receive no normal events
```

### Case D: Split-screen divider drag

#### Concept to implement

Divider drag changes the bounds of two windows, and each window displays inside its pane.

#### AOSP approach

Split task bounds, divider/system UI, surface transactions, and input target updates move together.

#### mos approach

```text
updateSplit(events)
  -> consume divider drag
  -> update splitFrac
  -> applySplitPlacements(0)
  -> windows draw clipped panes
```

### Case E: PiP drag and snap

#### Concept to implement

A small overlay window can be dragged and then snapped to the nearest corner.

#### AOSP approach

Pinned/PiP task bounds and SurfaceControl transforms change, while SystemUI/PiP controllers handle drag and snap behavior.

#### mos approach

```text
updatePip(events)
  -> if pointer starts inside PiP, drag pip
  -> clamp within screen
  -> on up, nearestPipCorner
  -> anim.Retarget(corner placement)
```

## 20. Debugging Questions

When AOSP windowing code becomes hard to follow, ask:

1. Is this object about app lifecycle, window state, surface layer, or input target?
2. Does this state live in the app process, system_server, or SurfaceFlinger?
3. Is this change about layout, visibility, focus, or animation?
4. Are these coordinates screen, window, or surface coordinates?
5. Is a system UI layer blocking input to apps underneath?
6. In mos terms, would this logic belong to `Server`, `Window`, `multiWindowState`, `system_apps`, `app_context`, or `event.Bus`?

## 21. Summary

AOSP windowing is large and distributed. Activity/task management, window state, input dispatch, surface composition, and system UI live in different components.

mos folds that architecture into a compact single-process model.

| mos element | Concept handled |
|---|---|
| `Server` | windowing server, frame loop, focus, layer order |
| `Window` | app surface, lifecycle, animation, coordinate transform |
| `anim` | app/window transition |
| `multiWindowState` | split/PiP/freeform |
| `system_apps.go` interfaces | SystemUI layers |
| `app.Context` / `AppCommand` | app-to-system boundary |
| `event.Bus` | lifecycle/navigation/system notifications |

The key mapping is:

```text
AOSP
  distributed window/task/input/compositor architecture

mos
  compact Server-centered windowing model
```

The learning goal is not to memorize every AOSP class at once. Use the smaller mos structure to track how lifecycle, surface composition, input routing, focus, system UI gating, and multi-window bounds spread across the larger AOSP architecture.
