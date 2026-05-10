# AOSP Legacy Windowing Handbook for mos

## 목적

이 문서는 mos의 `internal/windowing` 코드를 기준점으로 삼아 AOSP 레거시 windowing system을 학습하기 위한 handbook이다. 여기서 말하는 "레거시"는 특정 Android 버전 하나만을 뜻하기보다, Android windowing을 이해할 때 오래 유지되어 온 핵심 구조를 말한다.

목표는 다음 세 가지를 구분하는 것이다.

1. 구현하고자 하는 windowing 개념
2. AOSP가 그 개념을 구현한 방식
3. mos가 같은 개념을 표현하는 방식

mos는 실제 Android처럼 multi-process OS가 아니다. 하지만 window stack, app lifecycle, composition, input routing, focus, system UI gating, recents, split/PiP/freeform 같은 개념을 작고 읽기 쉬운 Go 코드로 압축해 둔다. 그래서 AOSP를 읽기 전에 개념 지도를 만들기에 좋다.

## 빠른 지도

```text
개념
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

## 1. Windowing Server의 역할

### 구현하고자 하는 개념

Windowing server는 화면 위에 무엇이 있고, 어떤 순서로 그려지며, 어떤 window가 입력을 받고, 언제 생성/표시/숨김/제거되는지를 결정한다.

핵심 책임은 다음과 같다.

| 책임 | 설명 |
|---|---|
| window stack | 여러 window의 z-order 관리 |
| visibility | 보이는 window와 숨겨진 window 구분 |
| lifecycle | showing, shown, hiding, destroyed 전환 |
| layout | window bounds와 mode 결정 |
| input routing | pointer/keyboard event의 대상 결정 |
| composition order | wallpaper, app, system UI layer를 순서대로 합성 |
| focus | foreground/focused window 관리 |

### AOSP 구현 방식

AOSP에서는 이 책임이 여러 시스템에 나뉘어 있다.

| 구성요소 | 역할 |
|---|---|
| `ActivityTaskManager` / `ActivityManager` | Activity launch, task stack, process/lifecycle 관리 |
| `WindowManagerService` | window state, layout, focus, visibility, app transition 관리 |
| `InputDispatcher` | focused window/input channel로 event dispatch |
| `SurfaceFlinger` | surface/layer composition |
| `SystemUI` | status bar, notification shade, keyguard, navigation 등 |
| Launcher | home/recents/task snapshot 표시 |

레거시 Android를 읽을 때 헷갈리는 이유는 "앱 하나가 화면에 보인다"는 단순한 현상이 Activity, Window, Surface, Task, Layer, InputChannel 여러 객체로 쪼개져 있기 때문이다.

### mos 구현 방식

mos는 `internal/windowing.Server`가 windowing 역할의 중심이다.

`Server`가 들고 있는 대표 상태:

| 상태 | 의미 |
|---|---|
| `windows []*Window` | app window stack |
| `focusedWindow *Window` | pointer/touch focus |
| `keyboardFocus *Window` | keyboard/IME focus |
| `multi *multiWindowState` | split/PiP/freeform state |
| `wallpaper`, `home`, `hist`, `kb`, `statusBar`, `curtain`, `lock` | system UI layers |
| `Bus *event.Bus` | lifecycle/navigation/system event broadcast |

즉, AOSP의 여러 service에 나뉜 개념이 mos에서는 `Server` 주변에 모여 있다.

```text
AOSP
  ActivityTaskManager + WindowManagerService + SurfaceFlinger + InputDispatcher

mos
  windowing.Server
```

## 2. 앱 실행과 Window 생성

### 구현하고자 하는 개념

사용자가 home icon이나 recents card를 누르면 앱이 실행되고 화면에 window가 나타난다.

필요한 일:

| 단계 | 설명 |
|---|---|
| app identity 결정 | 어떤 앱을 열 것인지 |
| app content 생성 | UI/content 객체 생성 |
| window 생성 | 화면에 올릴 container 생성 |
| launch animation | icon/card에서 full screen으로 확장 |
| lifecycle callback | create/resume 호출 |
| stack 추가 | z-order 위에 배치 |

### AOSP 구현 방식

AOSP에서는 Activity launch가 `ActivityTaskManager`와 `ActivityManager`를 지나 process, ActivityRecord, Task, WindowState, Surface까지 이어진다. Window는 앱 프로세스의 View hierarchy와 system_server의 WindowState, SurfaceFlinger의 layer로 나뉘어 존재한다.

개념 흐름:

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

### mos 구현 방식

mos에서는 `Server.Launch(appID)`가 이 흐름을 매우 짧게 표현한다.

```text
Server.Launch(appID)
  -> choose icon position/size
  -> launchApp(...)
  -> NewWindow(...)
  -> append to ws.windows
  -> publish Lifecycle PhaseCreated
```

`NewWindow()`는 다음을 함께 처리한다.

| 작업 | 위치 |
|---|---|
| full-screen canvas 생성 | `draws.CreateImage(screenW, screenH)` |
| app/context/proc 생성 | `newWindowProc`, `newWindowContext`, `NewApp` |
| lifecycle 초기화 | `LifecycleShowing` |
| launch animation | `anim.OpenFrom(iconPos, iconSize, fullSize, DurationOpening)` |
| app create callback | `proc.create(app.content, ctx)` |

mos에서 `Window`는 Android의 Activity, WindowState, Surface 일부 역할을 합친 작은 객체로 보면 된다.

## 3. Window 객체와 Surface

### 구현하고자 하는 개념

앱은 자기 UI를 그릴 공간이 필요하고, windowing system은 그 결과물을 화면 어딘가에 배치해야 한다.

이때 두 좌표계가 생긴다.

| 좌표계 | 의미 |
|---|---|
| app canvas 좌표 | 앱이 자기 UI를 그리는 logical surface |
| screen 좌표 | compositor가 실제 화면에 배치하는 위치 |

### AOSP 구현 방식

AOSP에서는 앱이 `ViewRootImpl`을 통해 surface buffer에 그린다. system_server의 WMS는 window bounds, visibility, z-order를 관리하고, SurfaceFlinger는 buffer layer를 합성한다.

관련 개념:

| 개념 | 설명 |
|---|---|
| `WindowState` | WMS 안의 window 상태 |
| `SurfaceControl` | surface/layer 제어 handle |
| buffer | 앱이 그린 pixel data |
| transaction | surface position, crop, alpha, layer 등의 변경 묶음 |
| SurfaceFlinger | 최종 composition |

### mos 구현 방식

mos의 `Window`는 `canvas draws.Image`를 가진다. 앱은 항상 full-screen 크기의 canvas에 그린다.

```go
type Window struct {
    canvas draws.Image
    anim   anim
    mode   Mode
    placement Placement
}
```

`Window.Draw(dst)`는 canvas를 compositor target인 `dst`에 sprite로 그린다.

| mos 요소 | AOSP 대응 |
|---|---|
| `Window.canvas` | app surface/buffer |
| `Window.Draw()` | layer composition 참여 |
| `anim.Pos()`, `anim.Size()`, `anim.Alpha()` | surface transform/alpha |
| `placement` | requested bounds |
| `mode` | windowing mode |

mos는 SurfaceFlinger 같은 별도 compositor 프로세스가 없다. `Server.Draw()`가 그 역할을 수행한다.

## 4. Composition 순서

### 구현하고자 하는 개념

화면은 여러 layer를 순서대로 합성한 결과다.

예:

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

위에 있는 layer는 아래 layer를 덮고, 경우에 따라 아래 layer의 input도 막는다.

### AOSP 구현 방식

AOSP에서는 WMS가 layer ordering과 window type을 관리하고, SurfaceFlinger가 최종 layer tree를 합성한다. SystemUI, IME, keyguard, app window는 서로 다른 window type/layer로 배치된다.

레거시 관점에서는 다음 개념을 보면 된다.

| 개념 | 의미 |
|---|---|
| application window | 앱 content |
| system window | status bar, nav bar, IME, keyguard 등 |
| z-order/layer | 어떤 window가 위에 그려지는지 |
| visibility | 보이거나 숨겨진 상태 |
| alpha/transform | animation 중 투명도와 위치/크기 |

### mos 구현 방식

mos는 `Server.Draw()`에 composition 순서가 직접 드러난다.

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

이 함수는 AOSP의 SurfaceFlinger/WMS layer ordering을 읽기 쉬운 형태로 압축한 곳이다.

특히 `curtain`은 그리기 직전에 `blurredScene(dst)`를 받아 frosted-glass backdrop을 만든다. 이것은 Android notification shade나 system UI blur/backdrop 개념과 비교해 볼 수 있다.

## 5. Lifecycle과 App Callback

### 구현하고자 하는 개념

Window는 생성되자마자 완전히 표시되는 것이 아니다. 열리는 중, 완전히 보임, 닫히는 중, 숨김, 제거됨 같은 상태가 있다. 앱 lifecycle callback도 이 흐름에 맞춰 호출되어야 한다.

### AOSP 구현 방식

AOSP에서는 Activity lifecycle과 window visibility가 연결되어 있지만 완전히 같은 것은 아니다.

관련 상태:

| 영역 | 예시 |
|---|---|
| Activity lifecycle | created, started, resumed, paused, stopped, destroyed |
| Window state | added, visible, animating, hidden, removed |
| Process state | foreground, cached, empty 등 |
| Transition | opening, closing, changing |

### mos 구현 방식

mos의 `Window` lifecycle은 훨씬 직접적이다.

```go
LifecycleInitializing
LifecycleShowing
LifecycleShown
LifecycleHiding
LifecycleHidden
LifecycleDestroying
LifecycleDestroyed
```

핵심 흐름:

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

`Server.logLifecycleChange()`는 lifecycle transition을 event bus에 publish한다.

| mos lifecycle event | 의미 |
|---|---|
| `PhaseCreated` | window/app created |
| `PhaseResumed` | window fully shown |
| `PhasePaused` | close/hide begins |
| `PhaseDestroyed` | window removed |

## 6. Launch, Close, App Transition Animation

### 구현하고자 하는 개념

앱 window는 갑자기 나타나거나 사라지지 않고, icon/card에서 확대되거나 recents card로 축소된다. transition은 현재 상태에서 끊기지 않고 이어져야 한다.

### AOSP 구현 방식

AOSP에는 app transition, starting window, remote animation, surface transaction 같은 개념이 있다. Android 버전에 따라 구현은 달라졌지만 핵심은 "window surface의 bounds/alpha를 시간에 따라 바꾸고, 앱 content 준비 전 시각 공백을 줄인다"는 점이다.

### mos 구현 방식

mos는 `internal/windowing/anim.go`의 `anim`이 window transition을 담당한다.

`Window`는 위치, 크기, alpha를 `anim`에서 읽는다.

| 동작 | mos 메서드 |
|---|---|
| icon에서 열기 | `anim.OpenFrom(...)` |
| target rect로 닫기 | `anim.CloseTo(...)` |
| multi-window bounds 변경 | `anim.Retarget(...)` |
| 즉시 위치 변경 | `anim.SnapTo(...)` |

`DismissTo()`는 열리는 중에도 닫기 animation으로 자연스럽게 바꿀 수 있다. AOSP의 interrupted transition이나 retargetable surface animation을 작게 표현한 부분이다.

## 7. Input Routing

### 구현하고자 하는 개념

사용자 입력은 화면 전체에서 발생하지만, 앱은 자기 window 좌표계의 event만 받아야 한다. notification shade나 lockscreen이 열려 있으면 아래 앱은 입력을 받으면 안 된다.

### AOSP 구현 방식

AOSP에서는 `InputReader`가 device event를 읽고, `InputDispatcher`가 focused window/input channel/touch target에 따라 event를 보낸다.

중요 개념:

| 개념 | 설명 |
|---|---|
| focused window | keyboard/input focus를 가진 window |
| touch target | pointer down이 시작된 window |
| input channel | app process로 event를 보내는 통로 |
| input window handle | dispatch에 필요한 bounds, flags, touchable region |
| pointer capture | drag 중 pointer가 window 밖으로 나가도 같은 target이 계속 받는 동작 |

### mos 구현 방식

mos는 `Server.Update()`가 input routing의 중심이다.

흐름:

```text
inputProducer.Poll()
  -> lock/curtain gating
  -> system UI update
  -> Multi-window state consumes drag gestures
  -> focus update
  -> frameForWindow()
  -> Window.UpdateApp(frame)
```

`filterEventsByRect()`는 event가 window rect 안에서 시작하면 pointer id를 capture한다.

```text
EventDown inside window
  -> captured[pointer] = true
EventMove outside window
  -> still delivered while captured
EventUp
  -> delivered, then capture removed
```

이는 AOSP의 touch target/pointer capture를 단순화한 모델이다.

## 8. Focus

### 구현하고자 하는 개념

어떤 window가 pointer input을 받는지와 어떤 window가 keyboard/IME input을 받는지는 같을 수도 있고 다를 수도 있다. PiP나 freeform에서는 이 차이가 중요하다.

### AOSP 구현 방식

AOSP에서는 WMS와 InputDispatcher가 focused window, focused app, input method target을 관리한다. IME target은 일반 touch target과 다를 수 있다.

### mos 구현 방식

mos는 focus를 두 종류로 나눈다.

| field | 의미 |
|---|---|
| `focusedWindow` | split/freeform/PiP에서 pointer interaction 대상 |
| `keyboardFocus` | keyboard/IME event를 받을 window |

`grantFocus()`는 이전 keyboard focus window의 `ctx.hasFocus`를 false로 만들고, 새 window를 true로 만든다.

`RequestFocus()`와 `ReleaseFocus()`는 앱이 command로 focus 변경을 요청하는 경로다.

```text
app
  -> ctx.RequestFocus()
  -> CmdRequestFocus
  -> Server.grantFocus(window)
  -> ctx.HasFocus() becomes true
```

## 9. Coordinate Transform

### 구현하고자 하는 개념

window가 화면에서 축소, 확대, 이동, clip되면 screen coordinate와 app coordinate가 달라진다. 입력 event는 앱이 이해하는 canvas 좌표로 변환되어야 한다.

### AOSP 구현 방식

AOSP에서는 surface bounds, crop, transform, window frame, compatibility scale, display rotation 등이 input 좌표 변환에 영향을 준다. InputDispatcher와 WMS는 input window handle에 필요한 region/bounds 정보를 전달한다.

### mos 구현 방식

mos의 앱 canvas는 항상 screen full-size다. window가 PiP/freeform으로 축소되어도 앱은 full-size canvas에 그린다. 따라서 입력 좌표를 되돌려야 한다.

| mode | 변환 방식 |
|---|---|
| fullscreen | 변환 없음 |
| split | screen offset만 빼고, scale은 하지 않음 |
| PiP/freeform | window rect에서 full-screen canvas로 scale |

관련 메서드:

| 메서드 | 역할 |
|---|---|
| `ToCanvasFrame()` | scaled window rect를 full canvas 좌표로 변환 |
| `ToSplitCanvasFrame()` | split pane offset만 반영 |
| `ContainsScreenPos()` | screen 좌표가 현재 animated rect 안인지 확인 |

이 부분은 AOSP window bounds와 input transform을 이해할 때 매우 좋은 축이다.

## 10. Multi-window

### 구현하고자 하는 개념

현대 Android에는 fullscreen 외에도 split-screen, PiP, freeform 같은 windowing mode가 있다. window bounds, focus, input routing, resize animation, overlay drawing이 모두 달라진다.

### AOSP 구현 방식

AOSP에서는 task/windowing mode, root task, activity bounds, surface placement, organizer 등이 관여한다. 레거시 관점에서는 split-screen stack, pinned stack, freeform stack 같은 개념으로 이해할 수 있다.

### mos 구현 방식

mos는 `multiWindowState`가 세 mode를 관리한다.

| mode | mos 표현 | AOSP 대응 |
|---|---|---|
| split | `multiModeSplit`, `ModeSplit` | split-screen |
| PiP | `multiModePip`, `ModePip` | pinned/PiP window |
| freeform | `multiModeFreeform`, `ModeFloat` | freeform windowing |

### Split

`enterSplit()`은 top two shown windows를 primary/secondary로 잡고, 화면 방향에 따라 vertical/horizontal split을 선택한다.

중요 포인트:

| 요소 | 설명 |
|---|---|
| `splitFrac` | primary pane 비율 |
| divider drag | `updateSplit()`이 event를 소비 |
| `applySplitPlacements()` | 두 window의 placement retarget |
| `drawSplit()` | divider overlay draw |

### PiP

`enterPip()`은 top window를 corner overlay로 줄인다.

중요 포인트:

| 요소 | 설명 |
|---|---|
| `pipPlacement()` | corner별 rect 계산 |
| drag | PiP window를 화면 안에서 이동 |
| snap | drag 끝나면 nearest corner로 snap |
| event split | PiP event와 main event를 분리 |

### Freeform

`enterFreeform()`은 shown windows를 floating rect로 만든다.

중요 포인트:

| 요소 | 설명 |
|---|---|
| title bar | drag handle 역할 |
| focused ring | focused window 시각 표시 |
| stagger | 여러 window를 살짝 어긋나게 배치 |
| `updateFreeform()` | title bar drag와 focus 변경 |

## 11. System UI Layers

### 구현하고자 하는 개념

모바일 OS의 화면은 앱 window만으로 구성되지 않는다. system UI가 항상 주변에 존재한다.

예:

| layer | 역할 |
|---|---|
| wallpaper | 배경 |
| home/launcher | 앱 시작점 |
| recents | 최근 앱/task 전환 |
| keyboard/IME | text input |
| status bar | 시간/상태 표시 |
| notification shade | 알림/quick settings |
| lockscreen/keyguard | 잠금 상태 |

### AOSP 구현 방식

AOSP에서는 SystemUI, Launcher, IME, Keyguard가 별도 앱/프로세스/service로 동작하며, WMS에서 system window type 또는 special layer로 취급된다.

### mos 구현 방식

mos는 `internal/windowing/system_apps.go`에 system UI layer interface를 정의한다.

| mos interface | 의미 |
|---|---|
| `Wallpaper` | 배경 draw |
| `Home` | launcher update/draw, icon tap |
| `History` | recents card 관리 |
| `Keyboard` | soft keyboard show/hide/update/draw |
| `StatusBar` | status bar height/update/draw |
| `Curtain` | notification shade/quick panel |
| `Lock` | lockscreen |

`Server`는 이 interface들을 알고, frame마다 정해진 순서로 update/draw한다. 실제 AOSP처럼 별도 process는 아니지만 layer 역할은 분명히 분리되어 있다.

## 12. Insets와 Safe Area

### 구현하고자 하는 개념

앱은 status bar, keyboard, navigation bar 같은 system UI가 차지하는 영역을 피해서 UI를 배치해야 한다.

### AOSP 구현 방식

Android는 `WindowInsets`로 status bar, navigation bar, IME, display cutout, system gesture inset 등을 앱에 전달한다.

### mos 구현 방식

mos는 `Context.SafeArea()`로 이 개념을 단순화한다.

`Server.SafeArea()`는 현재 다음을 반영한다.

| source | safe area |
|---|---|
| `statusBar.Height()` | top |
| `keyboard.Height()` | bottom |

`Server.Update()`는 매 frame 모든 window context에 safe area를 설정한다.

```text
sa := ws.SafeArea()
for each window:
  w.ctx.setSafeArea(sa)
```

## 13. Recents와 Snapshot

### 구현하고자 하는 개념

사용자가 home/recents로 이동하면 현재 앱의 시각 상태를 card로 남기고, 나중에 그 card를 눌러 앱처럼 다시 볼 수 있어야 한다.

### AOSP 구현 방식

AOSP는 task snapshot, recent task list, Launcher/Quickstep/SystemUI를 통해 recents 화면을 제공한다.

### mos 구현 방식

mos는 `History` interface와 `Window.HistoryEntry()`가 이 역할을 한다.

흐름:

```text
GoHome or GoRecents
  -> active window HistoryEntry()
  -> History.AddCard(...)
  -> window dismisses to card rect
  -> History draws recents cards
```

`HistoryEntry()`는 현재 window canvas를 snapshot으로 복사한다.

이것은 AOSP task snapshot을 매우 작게 모델링한 것이다.

## 14. Input Gating: Curtain과 Lock

### 구현하고자 하는 개념

notification shade나 lockscreen이 열려 있을 때 아래 앱이 touch event를 받으면 안 된다.

### AOSP 구현 방식

AOSP에서는 keyguard, status bar shade, IME, modal system windows가 input focus/touchable region/window flags를 통해 아래 window input을 막는다.

### mos 구현 방식

mos는 `Server.Update()`에서 `gateBelow`를 계산한다.

```text
locked := ws.IsLocked()
curtainOpen := !locked && ws.curtain != nil && ws.curtain.IsVisible()
gateBelow := locked || curtainOpen
```

`gateBelow`가 true이면 home, recents, app windows 등 아래 layer는 normal input을 받지 않는다. 대신 lock 또는 curtain이 frame events를 받는다.

이 코드는 AOSP의 input window z-order와 modal system UI를 이해하는 데 좋은 작은 모델이다.

## 15. App에서 System으로 가는 요청

### 구현하고자 하는 개념

앱은 windowing system이나 system UI 상태를 직접 수정하면 안 된다. 대신 system API로 요청하고, 시스템이 main loop에서 처리해야 한다.

### AOSP 구현 방식

AOSP 앱은 Binder를 통해 system service에 요청한다.

예:

| 요청 | AOSP service |
|---|---|
| Activity launch | ActivityTaskManager |
| window update | WindowManager |
| keyboard show/hide | InputMethodManager |
| notification | NotificationManager |
| clipboard | ClipboardManager |

### mos 구현 방식

mos는 `app.Context`와 `AppCommand`가 이 경계를 표현한다.

흐름:

```text
app content
  -> ctx method
  -> windowContext sends Cmd...
  -> windowProc queues command
  -> Server drains command on main goroutine
  -> Server mutates state
```

`commands.go`는 mos에서 system service transaction 목록처럼 읽을 수 있다.

대표 command:

| command | 의미 |
|---|---|
| `CmdFinish` | window close |
| `CmdLaunch` | app launch |
| `CmdShowKeyboard` / `CmdHideKeyboard` | IME visibility |
| `CmdPostNotice` | notification |
| `CmdSetDarkMode` | system setting |
| `CmdRequestFocus` / `CmdReleaseFocus` | keyboard focus |

## 16. Event Bus

### 구현하고자 하는 개념

Windowing system에서 lifecycle, navigation, system setting 변화는 여러 구성요소에 알려져야 한다.

### AOSP 구현 방식

AOSP는 Binder callback, broadcast, lifecycle callback, configuration change, listener 등 여러 통로를 사용한다.

### mos 구현 방식

mos는 `event.Bus`로 단순화한다.

| event kind | 의미 |
|---|---|
| `KindLifecycle` | app/window lifecycle |
| `KindNavigation` | back/home/recents |
| `KindSystem` | system setting/sensor |
| `KindCustom` | app-to-app or system custom |

`Server`는 lifecycle 변화와 navigation action을 publish한다. 이 구조는 AOSP의 framework callback/broadcast를 작게 압축한 것이다.

## 17. mos 코드 읽기 순서

### 1. `internal/windowing/server.go`

가장 먼저 읽어야 할 파일이다. 여기에 frame loop와 layer order가 있다.

볼 것:

| 항목 | 이유 |
|---|---|
| `Server` struct | windowing server가 소유하는 상태 |
| `Update()` | input, lifecycle, system UI update 순서 |
| `Draw()` | composition order |
| `Launch()`, `GoHome()`, `GoBack()`, `GoRecents()` | task/window navigation |
| `frameForWindow()` | input routing |
| `grantFocus()` | keyboard focus |

### 2. `internal/windowing/window.go`

Window 단위의 생명주기와 surface 모델을 본다.

볼 것:

| 항목 | 이유 |
|---|---|
| `Window` struct | Activity/Window/Surface 역할의 압축 |
| `NewWindow()` | app launch path |
| `DismissTo()` | close transition |
| `Update()` | animation completion and resume |
| `UpdateApp()` | app update and command drain |
| `Draw()` | canvas composition |
| `ToCanvasFrame()` | input coordinate transform |

### 3. `internal/windowing/anim.go`

Window transition이 어떻게 retarget되는지 본다.

볼 것:

| 항목 | 이유 |
|---|---|
| open/close transition | app transition 이해 |
| position/size/alpha | surface transform 대응 |
| retarget | interrupted transition 대응 |

### 4. `internal/windowing/multiwin.go`

Android split-screen, PiP, freeform 개념과 직접 대응되는 파일이다.

볼 것:

| 항목 | 이유 |
|---|---|
| `multiMode` | windowing mode |
| `enterSplit`, `enterPip`, `enterFreeform` | mode entry |
| `updateSplit`, `updatePip`, `updateFreeform` | gesture and routing |
| `drawSplit`, `drawPip`, `drawFreeform` | overlays |
| `cleanup` | anchor window destroyed handling |

### 5. `internal/windowing/system_apps.go`

SystemUI layer의 interface를 본다.

### 6. `internal/windowing/app_context.go`

앱이 system/windowing layer에 요청하는 경계를 본다.

### 7. `internal/windowing/commands.go`

Binder transaction처럼 command 목록을 읽는다.

### 8. `internal/event/bus.go`

lifecycle/navigation/system event를 본다.

## 18. AOSP를 읽는 순서

### 1. 큰 책임 분리부터 보기

처음부터 클래스 하나를 깊게 파지 말고 다음 질문으로 시작한다.

| 질문 | AOSP에서 볼 위치 |
|---|---|
| Activity/task는 누가 관리하는가? | ActivityTaskManager/ActivityManager |
| Window state는 누가 관리하는가? | WindowManagerService |
| 입력은 누가 dispatch하는가? | InputDispatcher |
| surface는 누가 합성하는가? | SurfaceFlinger |
| system UI는 어디에 있는가? | SystemUI, Launcher, IME, Keyguard |

### 2. Launch path 읽기

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

mos에서는 이 전체가 `Server.Launch()`와 `NewWindow()` 주변에 압축되어 있다.

### 3. Draw/composition path 읽기

```text
app draws buffer
  -> WMS/SF know layer state
  -> SurfaceFlinger composes layers
```

mos에서는 `Window.updateCanvas()`와 `Server.Draw()`가 대응한다.

### 4. Input path 읽기

```text
hardware input
  -> InputReader
  -> InputDispatcher
  -> focused/touch target window
  -> app receives event
```

mos에서는 `inputProducer.Poll()`부터 `frameForWindow()`까지가 대응한다.

### 5. Multi-window path 읽기

split/PiP/freeform을 읽을 때는 "Activity"보다 "bounds, surface transform, input target, focus"를 중심으로 본다.

mos에서는 `multiWindowState`가 바로 그 지도를 제공한다.

## 19. 대표 비교 사례

### Case A: 앱 아이콘을 눌러 실행

#### 구현하고자 하는 개념

Home에서 앱을 선택하면 새 app window가 열리고 lifecycle이 시작된다.

#### AOSP 구현 방식

Launcher가 `startActivity`를 호출하고, ActivityTaskManager가 task/activity를 준비한다. 앱 process가 Activity를 만들고, WMS가 window를 관리하며, SurfaceFlinger가 surface를 합성한다.

#### mos 구현 방식

```text
Home.TappedIcon()
  -> Server.launchApp(...)
  -> NewWindow(...)
  -> anim.OpenFrom(...)
  -> proc.create(...)
  -> windows append
```

### Case B: Back/Home/Recents

#### 구현하고자 하는 개념

navigation action은 현재 foreground window와 task history에 영향을 준다.

#### AOSP 구현 방식

Back handling은 Activity callback, task stack, predictive back, WMS transition 등 여러 층에 걸친다. Home/Recents는 Launcher/SystemUI와 ActivityTaskManager가 관여한다.

#### mos 구현 방식

| action | mos method |
|---|---|
| back | `Server.GoBack()` |
| home | `Server.GoHome()` |
| recents | `Server.GoRecents()` |

`HistoryEntry()`로 snapshot을 만들고, `History.AddCard()`로 recents에 보관한다.

### Case C: Notification shade가 열린 상태

#### 구현하고자 하는 개념

shade가 열리면 아래 앱은 입력을 받지 않고, shade가 input target이 된다.

#### AOSP 구현 방식

SystemUI의 notification shade window가 위에 올라오고, WMS/InputDispatcher가 아래 window로 touch가 내려가지 않게 한다.

#### mos 구현 방식

```text
curtainOpen = true
gateBelow = true
curtain.Update(frame with events)
app windows receive no normal events
```

### Case D: Split-screen divider drag

#### 구현하고자 하는 개념

두 window의 bounds를 divider drag로 조절하고, 각 window는 자기 pane에 맞게 표시된다.

#### AOSP 구현 방식

split task bounds, divider/system UI, surface transaction, input target update가 함께 움직인다.

#### mos 구현 방식

```text
updateSplit(events)
  -> consume divider drag
  -> update splitFrac
  -> applySplitPlacements(0)
  -> windows draw clipped panes
```

### Case E: PiP drag and snap

#### 구현하고자 하는 개념

작은 overlay window를 드래그하고, 손을 떼면 가장 가까운 corner로 붙인다.

#### AOSP 구현 방식

Pinned/PiP task bounds와 SurfaceControl transform이 바뀌고, SystemUI/PiP controller가 drag/snap을 관리한다.

#### mos 구현 방식

```text
updatePip(events)
  -> if pointer starts inside PiP, drag pip
  -> clamp within screen
  -> on up, nearestPipCorner
  -> anim.Retarget(corner placement)
```

## 20. 판단 기준

AOSP windowing 코드를 읽다가 길을 잃으면 다음 질문을 던진다.

1. 지금 보고 있는 객체는 app lifecycle, window state, surface layer, input target 중 무엇인가?
2. 이 상태는 app process에 있는가, system_server에 있는가, SurfaceFlinger에 있는가?
3. 이 변화는 layout 변화인가, visibility 변화인가, focus 변화인가, animation 변화인가?
4. 입력 좌표는 screen 기준인가, window 기준인가, surface 기준인가?
5. system UI layer가 아래 앱 input을 막고 있는가?
6. mos라면 이 로직은 `Server`, `Window`, `multiWindowState`, `system_apps`, `app_context`, `event.Bus` 중 어디에 들어가는가?

## 21. 요약

AOSP windowing은 크고 분산되어 있다. Activity/task 관리, window state, input dispatch, surface composition, system UI가 서로 다른 component에 있다.

mos는 이 구조를 작은 단일 프로세스 모델로 접는다.

| mos 요소 | 담당하는 개념 |
|---|---|
| `Server` | windowing server, frame loop, focus, layer order |
| `Window` | app surface, lifecycle, animation, coordinate transform |
| `anim` | app/window transition |
| `multiWindowState` | split/PiP/freeform |
| `system_apps.go` interfaces | SystemUI layers |
| `app.Context` / `AppCommand` | app-to-system boundary |
| `event.Bus` | lifecycle/navigation/system notifications |

가장 중요한 대응은 이것이다.

```text
AOSP
  distributed window/task/input/compositor architecture

mos
  compact Server-centered windowing model
```

따라서 학습 목표는 AOSP의 모든 클래스를 한 번에 외우는 것이 아니라, mos의 작은 구조를 기준으로 window lifecycle, surface composition, input routing, focus, system UI gating, multi-window bounds가 AOSP에서 어느 component로 흩어지는지 추적하는 것이다.



