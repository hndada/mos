# AOSP Legacy Windowing Summary

This document summarizes AOSP legacy windowing concepts through the lens of the `internal/windowing` code in mos. The goal is not to copy the AOSP architecture into mos, but to compare how recurring windowing ideas are represented in AOSP and in mos.

## Core Idea

The mos windowing code compresses a small mobile OS compositor and windowing server into one process. In AOSP, related work is split across `WindowManagerService`, `ActivityTaskManager`, `InputDispatcher`, `SurfaceFlinger`, and parts of SystemUI. In mos, the same broad responsibilities are handled by `windowing.Server`, `Window`, `multiWindowState`, `event.Bus`, and system app interfaces.

```text
AOSP legacy windowing
= Activity/Task/Window token + WindowManagerService + InputDispatcher + SurfaceFlinger

mos windowing
= Server + Window + anim + multiWindowState + system app layers + event.Bus
```

## Concept Map

| Concept to implement | AOSP approach | mos approach |
|---|---|---|
| Windowing owner | `WindowManagerService` in `system_server` tracks window state, layout, focus, and visibility. | `internal/windowing.Server` owns the window slice, focus, lifecycle, system UI layers, and update/draw order. |
| App launch and display | `ActivityTaskManager` / `ActivityManager` manage Activity launch and task stacks, while WMS attaches windows. | `Server.Launch()` creates a `NewWindow()` and appends it to the `windows` stack. |
| Window object | AOSP splits responsibilities across `WindowState`, `AppWindowToken`, `Task`, `ActivityRecord`, `SurfaceControl`, and related objects. | One `Window` holds app content, context, off-screen canvas, lifecycle, animation, placement, and input transforms. |
| Surface and composition | Apps draw buffers, and `SurfaceFlinger` composes layers. | Each `Window` draws to a full-screen canvas, and `Server.Draw()` composites wallpaper, home, recents, windows, keyboard, status bar, curtain, and lock. |
| Window lifecycle | Activity lifecycle and window visibility/animation state are spread across several services. | `LifecycleShowing`, `LifecycleShown`, `LifecycleHiding`, `LifecycleHidden`, and `LifecycleDestroyed` are directly tied to app callbacks. |
| Starting window and launch animation | Android uses starting windows, app transitions, and animation specs to avoid blank launch gaps. | `NewWindow()` uses `anim.OpenFrom()` to expand from an icon rect to full screen. |
| Input routing | `InputReader` / `InputDispatcher` route events by focused window, touch target, and pointer capture. | `Server.Update()` produces input events and routes them through `frameForWindow()`, `filterEventsByRect()`, and per-pointer capture maps. |
| Focus | WMS/InputDispatcher manage the focused app/window and input channel. | `focusedWindow` and `keyboardFocus` separate pointer focus from keyboard/IME focus. |
| Coordinate transform | Surface crop, scale, and position determine how input coordinates become window-local coordinates. | `Window.ToCanvasFrame()` and `ToSplitCanvasFrame()` convert screen-space events back into app canvas coordinates. |
| Multi-window | Android split-screen, PiP, and freeform use task/windowing modes, bounds, and surface transforms. | `multiWindowState` owns split, PiP, and freeform mode, placement, dragging, focus, and overlay drawing. |
| System UI layers | Status bar, navigation bar, notification shade, lockscreen, and IME participate as system components in the window stack. | `Wallpaper`, `Home`, `History`, `Keyboard`, `StatusBar`, `Curtain`, and `Lock` interfaces model system app layers. |
| Insets and safe area | Android delivers `WindowInsets`, IME insets, and system bar insets to apps. | `Server.SafeArea()` exposes status bar and keyboard heights through `Context.SafeArea()`. |
| Recents | Android Launcher/SystemUI display task snapshots and the recent task list. | `History` receives `Window.HistoryEntry()` snapshots and displays them as recents cards. |
| Lock/shade input gating | Keyguard or notification shade blocks input to windows underneath. | `gateBelow` in `Server.Update()` prevents lower layers from receiving input while lock or curtain is active. |
| App-to-system request | Apps use Binder calls to WMS, ActivityTaskManager, InputMethodManager, NotificationManager, and related services. | `Context` methods enqueue `AppCommand` values, and `Server` drains them on the main goroutine. |

## Recommended mos Reading Order

1. `internal/windowing/server.go`: frame loop, layer order, launch/back/home/recents, focus, input routing
2. `internal/windowing/window.go`: window lifecycle, canvas, animation, coordinate transforms
3. `internal/windowing/multiwin.go`: split/PiP/freeform bounds and dragging
4. `internal/windowing/system_apps.go`: system UI layer interfaces
5. `internal/windowing/app_context.go`: app-to-system API boundary
6. `internal/windowing/commands.go`: app command list
7. `internal/event/bus.go`: lifecycle/navigation/system event broadcast

## One-line Conclusion

The mos windowing code folds AOSP's large window/task/compositor/input architecture into a compact `Server`-centered single-process model. The learning goal is not to memorize AOSP class names, but to track where lifecycle, layer composition, focus, input routing, bounds transforms, and system UI gating happen.
