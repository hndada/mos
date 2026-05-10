# AOSP Legacy Windowing Summary

이 문서는 mos의 `internal/windowing` 코드를 기준으로 AOSP 레거시 windowing system을 학습하기 위한 요약본이다. 목표는 AOSP 구조를 mos에 그대로 도입하는 것이 아니라, windowing에서 반복되는 핵심 개념이 AOSP와 mos에서 각각 어떻게 표현되는지 비교하는 것이다.

## 핵심 요약

mos의 windowing 코드는 작은 모바일 OS의 compositor와 windowing server를 한 프로세스 안에 압축해 둔 구조다. AOSP로 대응시키면 `WindowManagerService`, `ActivityTaskManager`, `InputDispatcher`, `SurfaceFlinger`, system UI 일부가 나뉘어 맡는 일을 `windowing.Server`, `Window`, `multiWindowState`, `event.Bus`, system app interface가 함께 처리한다.

```text
AOSP legacy windowing
= Activity/Task/Window token + WindowManagerService + InputDispatcher + SurfaceFlinger

mos windowing
= Server + Window + anim + multiWindowState + system app layers + event.Bus
```

## 개념 대응표

| 구현하고자 하는 개념 | AOSP 구현 방식 | mos 구현 방식 |
|---|---|---|
| Windowing 주체 | `system_server` 안의 `WindowManagerService`가 window state, layout, focus, visibility를 관리한다. | `internal/windowing.Server`가 window slice, focus, lifecycle, system UI layer, draw/update 순서를 소유한다. |
| 앱 실행과 화면 표시 | `ActivityTaskManager`/`ActivityManager`가 Activity launch와 task stack을 관리하고, WMS가 window를 붙인다. | `Server.Launch()`가 `NewWindow()`를 만들고 `windows` stack에 추가한다. |
| Window 객체 | AOSP는 `WindowState`, `AppWindowToken`, `Task`, `ActivityRecord`, `SurfaceControl` 등으로 역할이 쪼개져 있다. | `Window` 하나가 app content, context, off-screen canvas, lifecycle, animation, placement, input transform을 함께 가진다. |
| Surface/composition | 앱은 buffer를 그리고, `SurfaceFlinger`가 layer를 합성한다. | 각 `Window`가 full-screen canvas에 그리고, `Server.Draw()`가 wallpaper, home, recents, windows, keyboard, status bar, curtain, lock 순서로 합성한다. |
| Window lifecycle | Activity lifecycle과 window visibility/animation state가 여러 service에 걸쳐 관리된다. | `LifecycleShowing`, `LifecycleShown`, `LifecycleHiding`, `LifecycleHidden`, `LifecycleDestroyed`와 app callback이 직접 연결된다. |
| Starting window / launch animation | Android는 starting window, app transition, animation spec을 통해 launch 시각 공백을 줄인다. | `NewWindow()`가 icon rect에서 full-screen으로 열리는 `anim.OpenFrom()`을 사용한다. |
| Input routing | `InputReader`/`InputDispatcher`가 focused window, touch target, pointer capture에 따라 event를 라우팅한다. | `Server.Update()`가 input events를 만들고 `frameForWindow()`, `filterEventsByRect()`, pointer capture map으로 라우팅한다. |
| Focus | WMS/InputDispatcher가 focused app/window와 input channel을 관리한다. | `focusedWindow`와 `keyboardFocus`를 분리해서 pointer focus와 keyboard/IME focus를 관리한다. |
| Coordinate transform | Surface crop/scale/position에 따라 input 좌표가 window local 좌표로 변환된다. | `Window.ToCanvasFrame()`과 `ToSplitCanvasFrame()`이 screen-space event를 app canvas 좌표로 변환한다. |
| Multi-window | Android split-screen, PiP, freeform은 task/windowing mode, bounds, surface transform으로 관리된다. | `multiWindowState`가 split, PiP, freeform mode와 placement, drag, focus, overlay drawing을 관리한다. |
| System UI layer | Status bar, navigation bar, notification shade, lockscreen, IME가 별도 system component로 window stack에 참여한다. | `Wallpaper`, `Home`, `History`, `Keyboard`, `StatusBar`, `Curtain`, `Lock` interface가 system app layer 역할을 한다. |
| Insets / safe area | Android는 `WindowInsets`, IME insets, system bar insets를 앱에 전달한다. | `Server.SafeArea()`가 status bar와 keyboard 높이를 `Context.SafeArea()`로 전달한다. |
| Recents | Android Launcher/SystemUI가 task snapshot과 recent task list를 표시한다. | `History`가 `Window.HistoryEntry()` snapshot을 받아 recents carousel처럼 표시한다. |
| Lock/shade input gating | keyguard나 notification shade가 보이면 아래 window input을 막는다. | `Server.Update()`의 `gateBelow`가 lock/curtain 상태일 때 아래 layer로 input이 내려가지 않게 한다. |
| App to system request | Binder call로 WMS/ActivityTaskManager/InputMethodManager/NotificationManager 등에 요청한다. | `Context` method가 `AppCommand`를 command channel에 넣고, `Server`가 main goroutine에서 drain한다. |

## mos 코드 읽기 순서

1. `internal/windowing/server.go`: frame loop, layer order, launch/back/home/recents, focus, input routing
2. `internal/windowing/window.go`: window lifecycle, canvas, animation, coordinate transform
3. `internal/windowing/multiwin.go`: split/PiP/freeform bounds와 drag 처리
4. `internal/windowing/system_apps.go`: system UI layer interface
5. `internal/windowing/app_context.go`: app에서 system으로 가는 API boundary
6. `internal/windowing/commands.go`: app command 목록
7. `internal/event/bus.go`: lifecycle/navigation/system event broadcast

## 한 줄 결론

mos의 windowing 코드는 AOSP의 거대한 window/task/compositor/input 구조를 `Server` 중심의 작은 단일 프로세스 모델로 접은 것이다. 그래서 학습 목표는 AOSP 클래스를 암기하는 것이 아니라, window lifecycle, layer composition, focus, input routing, bounds transform, system UI gating이 어디서 어떻게 일어나는지 대응시키는 것이다.



