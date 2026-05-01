# MOS — 모바일 OS 시뮬레이터 정리

> **M OS** (Mobile OS / Muang OS) — Ebitengine 기반 Go로 만든 모바일 운영체제 UX 시뮬레이터.
> 데스크톱에서 바(Bar), 플립(Flip), 폴드(Fold) 폼팩터의 화면을 모사하고, 윈도잉 서버 위에서 시스템 앱과 사용자 앱을 띄우는 구조다.

---

## 1. 전체 구조 (Top-down)

```
mos/
├── cmd/sim/main.go         # 시뮬레이터(엔트리). Ebitengine Game 루프, 디바이스/스크린 모사
├── apps/                   # 시스템 앱 + 일반 앱 (Default* 구현체)
│   ├── home.go             # 홈 화면(앱 아이콘 그리드)
│   ├── history.go          # 최근 앱(카드 캐러셀)
│   ├── statusbar.go        # 상단 상태바
│   ├── curtain.go          # 알림/빠른 설정 커튼(슬라이드 패널)
│   ├── keyboard.go         # 가상 QWERTY 키보드
│   ├── lock.go             # 잠금 화면
│   ├── wallpaper.go        # 배경
│   ├── settings.go         # 일반 앱 — 스크롤 가능한 설정 리스트
│   ├── call.go             # 일반 앱 — 통화 UI
│   └── scene_app.go        # 일반 앱 — 씬 전환 데모
├── internal/
│   ├── windowing/          # 윈도잉 서버: 앱 라이프사이클·합성·시스템 앱 슬롯
│   │   ├── windowing.go    # WindowingServer (Update/Draw 루프, 네비 의도)
│   │   ├── window.go       # Window (라이프사이클 + Tween 기반 애니메이션)
│   │   ├── app.go          # App / AppContent (앱 ID → 콘텐츠 인스턴스화)
│   │   ├── system_apps.go  # 시스템 앱 인터페이스(Home/History/...)
│   │   └── display.go      # Display (전원·해상도)
│   ├── draws/              # 렌더링 추상층 (Image/Sprite/Box/Align/Text/...)
│   ├── input/              # 키보드·마우스 입력 추상화 (Ebiten 래핑)
│   ├── tween/              # 트위닝 엔진 + 이징 함수
│   ├── times/              # playback rate 인지 시계
│   └── util/               # 공용 유틸
└── ui/                     # 위젯 (버튼/토글/슬라이더/스크롤/제스처/Box)
```

### 레이어 분리
| 레이어 | 역할 | 위치 |
|---|---|---|
| **Sim (하드웨어 모사)** | 디바이스 폼팩터, 스크린 슬롯, 베젤, 전원/모드 핫키 | `cmd/sim` |
| **Windowing (시스템)** | 활성 앱·라이프사이클·합성, 시스템 앱 호출 | `internal/windowing` |
| **System Apps** | 홈/히스토리/상태바/커튼/키보드/잠금/배경 | `apps/*` (인터페이스는 `windowing/system_apps.go`) |
| **User Apps** | Settings, Call, Scene-test, Gallery, Color | `apps/settings.go`, `apps/call.go`, … 그리고 `windowing/app.go`의 Gallery |
| **UI Toolkit** | 위젯 (Button, Toggle, Slider, ScrollBox, Gesture) | `ui/` |
| **Draws** | Sprite/Box/Align/Text 등 그리기 원시 | `internal/draws` |
| **Input/Tween/Times** | 입력 추상화, 트위닝, 시계 | `internal/{input,tween,times}` |

---

## 2. 핵심 구현 내용

### 2.1 시뮬레이터 — `cmd/sim/main.go`
- **3가지 디스플레이 모드** 정의(`displayModeBar/Flip/Fold`).
  - 각 모드별로 `displayGroup`(여러 `screenSpec`)을 갖고, 한 화면을 `primary`로 표시.
  - 갤럭시 S26, Z Flip 7, Z Fold 7 해상도를 0.24배로 스케일하여 시뮬레이터 뷰포트(1200×900)에 배치.
- **단축키로 하드웨어 시뮬레이션**:
  - `P` 전원 / `1·2·3` 모드 / `S` 활성 스크린 순환 / `B/Esc/Backspace` 뒤로 / `H` 홈 / `R` 최근앱 / `K` 키보드 / `V` 통화 / `N` 커튼 / `C` 스크린샷 / `L·F` 로그.
- **로그 윈도우**: 우측에 `simLog`로 시스템 이벤트 타임스탬프 출력 (윈도잉 서버에 콜백을 주입).
- **모드 전환 시 상태 보존**: `applyMode`가 새 `WindowingServer`를 만들면서도 history entries / screenshots / active app 상태를 직접 마이그레이션.

### 2.2 Windowing Server — `internal/windowing/windowing.go`
- 단일 컴포지터. 매 프레임:
  1. 배경 → 홈 또는 최근앱 → 윈도우들 → 키보드 → 상태바 → 커튼 → 잠금 순으로 합성.
  2. 윈도우는 항상 `Update → Draw` 순으로 각자 라이프사이클 진행.
- **시스템 앱은 인터페이스로만 의존** (Wallpaper/Home/History/Keyboard/StatusBar/Curtain/Lock).  
  `Set*()` 주입자 패턴이라 시뮬레이터가 실제 구현체(`apps.Default*`)를 갈아 끼움.
- **네비게이션 의도(Navigation Intent) 메서드** 통일:
  - `GoHome / GoBack / GoRecents`가 우선순위 규칙(커튼 → 키보드 → 최근앱 → 활성 윈도우)으로 닫음.
  - 닫는 순간 활성 윈도우 스냅샷을 떠서 `History.AddCard`.
- **스크린샷**: `AddScreenshot`이 캔버스를 새 이미지로 복사해 보관 (Gallery 앱이 사용).

### 2.3 Window 라이프사이클 — `internal/windowing/window.go`
- 7단계 상태: `Initializing → Showing → Shown → Hiding → Hidden → Destroying → Destroyed`.
- **각 시각 속성별 독립 Tween**: `posX, posY, sizeW, sizeH, alpha`.  
  덕분에 “보여지는 도중에도 어디로든 다시 닫힐 수 있다” — `DismissTo(center, size)`로 임의 좌표/크기 타깃 지정 가능.
- 홈 아이콘 → 화면 전체로 확장하는 모션, 닫을 때 카드 위치로 축소되는 모션 모두 동일 메커니즘.
- `RestoreActiveApp`은 정적 트윈(`staticAnim`)으로 즉시 `Shown` 상태에 진입 → 모드 전환에도 활성 앱이 보존됨.

### 2.4 App 추상 — `internal/windowing/app.go`
- `AppContent` 인터페이스: `Update(cursor) / Draw(dst)`.
- 옵션 인터페이스 `closeRequester { ShouldClose() }` — 앱이 자체 종료를 요청 (예: Call의 End 버튼).
- `Gallery`만 캔버스+screenshots에 의존하므로 별도 `Prepare(screenshots)` 라이프사이클을 둠.
- 알 수 없는 ID로 들어온 앱은 색상+제목 텍스트의 placeholder 컨텐츠로 폴백.

### 2.5 시스템 앱 구현체

| 구현체 | 핵심 동작 |
|---|---|
| `DefaultHome` | 4×5 그리드. 라운드 사각 아이콘은 `vector.DrawFilledRect`+원으로 합성. 탭하면 `TappedIcon()`이 위치/크기/컬러/appID 반환. |
| `DefaultHistory` | 가로 스크롤 카드 캐러셀. `ScrollBox.ContentSize`에 `(n-1)*hStep + screenW`로 마지막 카드도 중앙 정렬. 휠/드래그/탭 분리. 포커스 카드를 마지막에 그려 위에 깔리게. 화면 종횡비를 카드에 반영(`fitAspect`). |
| `DefaultStatusBar` | 캐리어/시계/시스템 정보 텍스트만 갱신. |
| `DefaultCurtain` | y 트윈으로 위에서 슬라이드. `IsVisible`은 트윈이 끝나기 전까지 true 유지(나가는 애니메이션 보장). |
| `DefaultKeyboard` | 한 번에 키보드 캔버스를 그려두고 슬라이드 y만 트위닝 → 매 프레임 키 다시 그릴 필요 없음. |
| `DefaultLock` | locked 시 시계만 그림 (현재는 잠금 시퀀스가 시뮬레이터에 노출되어 있지 않음). |
| `DefaultWallpaper` | 단색 배경. |

### 2.6 일반 앱
- **Settings** (`apps/settings.go`): 헤더/토글/슬라이더/내비 행을 가진 스크롤 리스트.
  - 컨텐츠는 `s.canvas`에 한 번 렌더한 뒤 `SubImage`로 보이는 영역만 잘라 화면에 블릿 → 스크롤이 매끄럽다.
  - `contentCursor`로 스크린→컨텐츠 좌표 변환, 자식 위젯이 “자기 좌표계”에서 동작.
- **Call** (`apps/call.go`): 시작 시각 기록 → 매 프레임 경과 시간 갱신. 2초 후 “Ringing” 표시. End 버튼 탭 시 `ShouldClose=true` → 윈도잉 서버가 닫음.
- **SceneTest** (`apps/scene_app.go`): 씬 3개를 좌→우로 슬라이드 전환. 트윈으로 위치 보간.
- **Gallery** (`internal/windowing/app.go`): 보관된 스크린샷의 썸네일 그리드. 화면/개수 변경 시에만 썸네일 재구축(`needsThumbRebuild`).

### 2.7 UI 위젯
- **Box**(공용): 위치 + 크기 + 정렬 9방향(`Aligns`). `In(p)`로 히트테스트.
- **GestureDetector**: 마우스 이벤트로부터 Tap / Drag / SwipeUp/Down/Left/Right 분류. 임계값(`DragThresholdPx=6`, `MinSwipePx`)을 별도로 둠.
- **TriggerButton** = Box + Gesture. SetRect로 즉시 재배치.
- **ScrollBox**: 휠+드래그 통합. `ContentSize`로 max offset 제한.
- **Toggle / Slider**: 둘 다 트윈으로 부드러운 상태 변화.

### 2.8 그리기 원시 — `internal/draws`
- `Image`는 ebiten.Image 래퍼, `Sprite = Image + Box`.
- `Box.op()`가 `Scale → Rotate → Translate(Min)`으로 Aligns에 맞는 변환 행렬을 만들어 반환.
- `Aligns`는 `(X, Y) ∈ {Start, Center, End}` 9가지. 코드 곳곳에서 `LeftTop, CenterMiddle…` 상수로 사용.

### 2.9 Tween — `internal/tween`
- Unit(begin, change, duration, easing) 시퀀스. `MaxLoop`로 반복.
- `EaseOutExponential` 한 종류만 실제 사용 — `1 - exp(-6.93·t/d)` (≈ `2^{-10·t/d}`).
- `Stop()`은 인덱스를 끝으로 보내 즉시 종료 → 정적 값 트윈 만드는 트릭(`staticAnim`)도 가능.

---

## 3. 동작 원리 (런타임)

### 3.1 1프레임 동안 일어나는 일
1. **Sim.Update** — 키 핫키 처리 → 모드/전원/네비 의도 갱신. `SetCursorOffset`으로 활성 스크린 좌상단 기준으로 좌표계 이동. 화면 하단 “네비 바 영역”에서 위로 스와이프 시 `GoHome`.
2. **WindowingServer.Update**
   - 홈이 보이는 상태면 `home.Update` → 탭된 아이콘이 있으면 `launchApp`.
   - 최근앱 화면이면 `history.Update` → 카드를 탭하면 그 위치/크기에서 윈도우 확장.
   - 커튼/키보드/상태바/잠금 갱신.
   - 모든 윈도우 `Update` → 라이프사이클 전이 시 로그.
   - `Hidden`이 된 윈도우는 다음 프레임에 `Destroyed`로 옮긴 뒤 슬라이스에서 제거.
3. **Sim.Draw** → 캔버스 클리어 → `WindowingServer.Draw` → 시뮬레이터 뷰포트의 슬롯 위에 캔버스를 합성. 그 위에 시뮬레이터 컨트롤(뒤로/홈/최근앱), 로그, 도움말 오버레이.

### 3.2 앱 실행 흐름
```
home tap → launchApp(iconPos, iconSize, color, appID)
        → NewWindow: 5개 트윈을 iconPos→화면중앙 / iconSize→화면크기로 시작
        → Showing → (size tween 끝) → Shown → app.Update가 실제 입력 받음
GoHome   → 활성 윈도우 스냅샷을 history에 저장 → DismissTo(card 위치/크기)로 축소 애니메이션
GoBack   → 우선순위(커튼 > 키보드 > 최근앱 > 활성 윈도우)로 “하나만” 닫음
ShouldClose → 앱 자신이 종료 신호 → Dismiss
```

### 3.3 폼팩터 전환 시 상태 보존
- 모드를 바꾸면 active screen 해상도가 변하므로 `WindowingServer`를 통째로 새로 만든다.
- 그 전에 `HistoryEntries`, `Screenshots`, `ActiveAppState`를 뽑아내서 새 서버에 다시 주입.
- 활성 앱은 `RestoreActiveApp` → `staticAnim`으로 즉시 `Shown` 상태 윈도우로 복원 → UX상 끊김 없는 전환.

### 3.4 좌표계 처리
- 시뮬레이터: 뷰포트 절대 좌표.
- `input.SetCursorOffset(active.x, active.y)` 이후 `MouseCursorPosition`은 활성 스크린 기준 좌표를 반환 → 시스템/앱은 자기 캔버스 좌표만 알면 됨.
- Settings 같은 경우 거기서 다시 `contentCursor`로 스크롤 오프셋을 빼서 컨텐츠 좌표계로 변환.

---

## 4. 설계 원리·아이디어

1. **Composite, don't subclass** — Window가 “여러 트윈을 가진 박스” 하나로 모든 라이프사이클 모션을 표현. 새 모션이 필요해도 트윈만 추가.
2. **인터페이스 기반 시스템 앱** — `windowing` 패키지는 `apps`를 import 하지만 의존은 인터페이스로. 실제 구현은 시뮬레이터가 주입하므로 다른 시스템 앱 셋(예: 다른 런처, 다른 키보드)으로 교체 쉬움.
3. **Navigation Intent** — 단일 진입점(`GoBack/GoHome/GoRecents`)이 “현재 화면 스택에서 무엇이 가장 위인지” 판단. 호출자(시뮬레이터, 트리거 버튼, 스와이프)는 모두 동일 API만 알면 됨.
4. **모션은 “시간 기반”** — Tween + ease-out exponential을 표준으로 두고 거리에 따라 duration을 비례 조정(키보드/커튼) → 짧은 거리는 빠르게, 긴 거리는 느리게.
5. **상태와 시각 분리** — `historyState`, `Lifecycle` 같은 상태 머신이 “현재 의도”를 담고, 트윈이 “현재 시각”을 담음. `IsVisible`은 둘 다를 반영(애니메이션 도중에도 true).
6. **사전 캔버스 합성** — 키보드/설정/카드처럼 정적인 큰 배경은 1회 그려두고 위치/스크롤만 바꾸도록. 매 프레임 텍스트 재배치가 필요한 위젯도 `SubImage`로 잘라 그림.
7. **시뮬레이터=프로토콜 운영자** — Sim은 “전원, 디스플레이 종류, 활성 화면” 같은 하드웨어 책임만 진다. 그 아래 OS는 활성 화면 한 개만 가정 → 코드 단순.

---

## 5. 개선점 / 다음 단계

### 5.1 아키텍처
- **이벤트/시그널 시스템 부재**: README가 정의한 “Signal → Administrator → Receive” 흐름이 코드에 아직 없다. 현재는 `ws.Logger` 콜백으로 한 줄 로그만. `eventbus` 패키지를 만들어 라이프사이클·통화·키 이벤트를 pub/sub으로 흘리면 시스템 앱끼리 결합도가 더 떨어진다.
- **프로세스 모델 부재**: 모든 앱이 같은 Go 루틴에서 돈다. README의 “each app runs on a different process”를 흉내내려면 최소한 goroutine + 메시지 큐로 격리한 “앱 프로세스” 추상을 도입할 만하다 (지금은 메인 루프가 직접 `app.Update`를 호출).
- **Lock**이 시뮬레이터에 노출되지 않음 — 잠금/해제 이벤트와 “Splash(Starting Window)” 단계가 코드상 비어 있음. `Lifecycle`에 `Splash`를 추가하거나 별도의 시작 윈도우 표시 시간을 두면 README 정의와 맞다.
- **Display 다중 활용**: Flip/Fold에서 secondary 스크린은 단순 배경만 표시. cover 디스플레이용 미니 위젯(시계/알림)을 별도 컨텐츠로 그릴 수 있게 `Display`에 “이 디스플레이의 컨텐츠 공급자” 슬롯을 두면 멀티 스크린 시뮬레이션이 의미 있어진다.

### 5.2 Windowing
- **Z-order/Window stack 명시화**: 지금은 `windows []*Window`의 슬라이스 순서가 곧 Z-order인데 “foreground/background” API가 없다. 스택 push/pop과 “bring to front”를 추상화하면 멀티 윈도우 앱(설정 안의 하위 화면 등)이 가능해진다.
- **History 중복 제거 정책**: `AddCard`가 같은 `(AppID, Color)`만 dedup → 같은 앱이지만 상태가 다른 인스턴스가 들어오면 덮어쓴다. `AppState` 전체로 비교하거나 명시적 thumbnail 갱신만 하는 모드를 둘 것.
- **윈도우 입력 hit test**: 현재 활성 윈도우 한 개에 대해서만 cursor를 그대로 넘김. 윈도우 변환(축소 애니 중)에는 입력이 어색할 수 있어 `Shown` 상태일 때만 입력을 라우팅하는 가드(이미 일부 있음)를 더 명시화.

### 5.3 입력/제스처
- **터치/멀티터치 모델**: Ebiten의 touch ID를 활용한 멀티터치, 핀치 등은 미지원. `GestureDetector`가 단일 포인터 가정.
- **키 이벤트 큐**: 시뮬레이터에서 단축키를 직접 if 체인으로 받고 있어 추가가 늘면 비대해짐. 키→커맨드 매핑 테이블로 빼면 도움말 문자열도 자동 생성 가능.

### 5.4 렌더링/성능
- **이미지 풀링**: 매 트윈/이벤트마다 `draws.CreateImage`가 새 ebiten.Image를 만든다. 윈도우 캔버스는 라이프사이클 단위로 재사용하는데, 히스토리 카드/제어판 배경 등도 캐싱 가능.
- **dirty flag 미활용**: `Display`에 “Dirty flags drive repaints only when state changes” 주석은 있는데 실제 구현은 없음. 정적 시스템 앱(상태바·홈)이 시계가 변할 때만 다시 그리도록 변경.
- **텍스트 레이아웃**: `draws.Text`가 매번 face·shaping을 재계산하는지 검토. 자주 갱신되는 시계/타이머 텍스트는 별도 캐시.

### 5.5 코드 품질
- `internal/draws/_filler.go`, `_flexbox.go`, `_length.go`처럼 비활성(prefix `_`) 파일이 남아 있음 → 의도적 보류라면 README에 표시하거나 `// +build ignore`로 명시.
- `apps/lock.go`는 폰트 크기/배경 등이 거의 비어있어 다른 시스템 앱 수준의 디자인으로 채울 여지.
- **테스트 부재**: 트윈 종료 조건, ScrollBox max offset, Window 라이프사이클 전이 같은 순수 로직은 단위 테스트가 잘 붙는 후보.
- **로그 추상화**: `WindowingServer.log`가 string 한 줄을 받는데, 구조화 로그(이벤트 타입+속성)로 만들면 시뮬레이터 로그 패널에서 필터링/색상화 가능.

### 5.6 UX 작은 것들
- 홈에서 다른 앱을 탭했을 때 이미 윈도우가 떠 있으면 무시(`hasVisibleWindow`) → 빠르게 두 번 탭하면 묵음. 의도라면 OK, 아니라면 큐잉 또는 시각 피드백.
- 히스토리 카드 “스와이프 업으로 닫기” 미지원 (탭으로 열기만 가능). 모바일 OS 클로닝이라면 필요한 동작.
- 커튼이 열린 상태에서 윈도우 입력이 그대로 들어감 → 커튼 보일 때 하부 입력을 차단해야 자연스럽다.
- 가상 키보드가 텍스트 필드에 연결되어 있지 않음 (시각적 데모만). `KeyEventBus`를 통해 활성 앱이 이벤트를 받게 연결.

---

## 6. 한 문장 요약

> **MOS는 “윈도잉 서버 + 시스템 앱 인터페이스 + 트윈으로 표현되는 라이프사이클” 세 축으로 모바일 OS UX를 시뮬레이션한다. 새 폼팩터·새 시스템 앱·새 모션은 각각 독립 축에서 추가할 수 있는 게 핵심 강점이고, 이벤트/프로세스 모델과 상태 보존·재사용 정책이 다음 성장 포인트다.**

---

## 7. Android 주요 기능 vs 현재 시뮬레이터

> 범례 — ✅ 있음, 🟡 부분/시각만, ❌ 없음

### 7.1 앱 / 프로세스 모델
| Android 개념 | 설명 | 현재 |
|---|---|---|
| 프로세스 격리 | 앱마다 독립 프로세스(zygote fork). 메모리 보호, OOM Killer | ❌ 모든 앱이 한 메인 루프 안. README가 정의했지만 미구현 |
| Activity 라이프사이클 | onCreate/Start/Resume/Pause/Stop/Destroy | 🟡 `Lifecycle` 7단계는 있으나 앱 코드 콜백으로 전달되지 않음 (`Showing/Shown/Hiding/Hidden`만 외부 효과) |
| Task / Back stack | 작업(Task)별 액티비티 스택, 동일 앱 다중 인스턴스 | ❌ 윈도우는 단일 평면 슬라이스. 같은 앱 재실행 시 dedup만 |
| Intent | 명시/암시 인텐트, share target, deep link | ❌ 앱 ID 문자열로 직접 launch만 |
| Service | 바운드/스타티드 서비스, foreground service | ❌ |
| BroadcastReceiver | 시스템 이벤트 구독 | ❌ — README의 “Signal → Administrator → Receive”가 이 자리 |
| ContentProvider | 앱 간 데이터 공유 URI | ❌ |
| WorkManager / JobScheduler / AlarmManager | 지연·예약·조건부 작업 | ❌ |
| Doze / App Standby | 백그라운드 제약, 절전 | ❌ |
| Splash Screen API (Android 12+) | 시작 윈도우, 아이콘 + 브랜드 | ❌ README 정의만, 없음 |
| Cold/Warm/Hot Start | 시작 단계 구분 | ❌ |

### 7.2 윈도우 시스템
| Android 개념 | 설명 | 현재 |
|---|---|---|
| WindowManager Z-order | 시스템/앱/플로팅 윈도우 타입별 z | 🟡 슬라이스 순서가 곧 z. 명시적 타입 없음 |
| Dialog / Popup / Toast | 모달/비모달 보조 윈도우 | ❌ |
| 멀티 윈도우 / Split-screen / PiP | 동시 두 앱, 작은 영상창 | ❌ 활성 앱은 항상 단일 풀스크린 |
| Soft Input Mode | 키보드가 뜰 때 컨텐츠 resize/pan | ❌ 키보드는 단순 오버레이 |
| Insets / Cutouts | 상태바·내비바·노치·홀펀치 영역 | 🟡 status bar 높이 24px만 상수, API 없음 |
| Edge-to-edge / Immersive | 시스템바 숨김 모드 | ❌ |
| Live Wallpaper | 동적 배경 서비스 | ❌ 단색 배경만 |
| Adaptive Icons | 마스크/포어그라운드/배경 분리 | ❌ 라운드 사각만 |
| Recents 액션 | clear all, app pin, screenshot | 🟡 카드 탭만 |
| 화면 회전 / Configuration change | 회전 후 컨텐츠 재배치, state 보존 | ❌ |
| 스크린 캐스트 / 미러링 | 외부 디스플레이 출력 | ❌ |

### 7.3 입력 / IME
| Android 개념 | 설명 | 현재 |
|---|---|---|
| 멀티터치 / 제스처 | pinch, rotate, two-finger drag | ❌ 단일 포인터 (마우스) |
| InputMethodService (IME) | 텍스트 필드 ↔ 키보드 연결, IPC | ❌ 가상 키보드는 시각만, 입력 라우팅 없음 |
| IME 타입 | 숫자/이메일/URL/패스워드 키패드 | ❌ |
| 다국어 / 자동완성 / 필기 | | ❌ |
| 하드웨어 키 처리 | 볼륨, 전원 길게/짧게, 도움말 키 | 🟡 전원/모드/네비만 키 매핑 |
| 햅틱 | 터치/롱프레스 진동 | ❌ |
| 접근성 — TalkBack | 화면 리더 | ❌ |
| 스타일러스 | 압력, hover, 펜 버튼 | ❌ |
| 제스처 내비게이션 | 좌우 가장자리 스와이프 = 뒤로 | 🟡 하단 swipe-up = home만 |

### 7.4 알림 / 시스템 표시
| Android 개념 | 설명 | 현재 |
|---|---|---|
| NotificationChannel | 채널별 중요도/사운드/뱃지 | ❌ |
| Heads-up 알림 | 상단 떠있는 카드 | ❌ |
| Quick Settings tiles | 커튼의 토글 타일, 길게=상세 | 🟡 정적 텍스트만 |
| 상태바 시스템 아이콘 | 신호/배터리/Wi-Fi 실시간 | 🟡 정적 텍스트 “5G Wi-Fi 87%” |
| 잠금화면 알림 | 알림 미리보기 | ❌ |
| 알림 액션 / Reply / Snooze | | ❌ |
| Always-On Display | | ❌ |
| 토스트 / 스낵바 | | ❌ |

### 7.5 보안 / 잠금
| Android 개념 | 설명 | 현재 |
|---|---|---|
| Lock screen | PIN / 패턴 / 패스워드 / 생체 | 🟡 `DefaultLock` 시계만, 시뮬레이터에 노출조차 안 됨 |
| 생체 인증 (BiometricPrompt) | 지문, 얼굴 | ❌ |
| 런타임 권한 | 카메라/위치/마이크 등 dangerous 권한 grant | ❌ |
| App Sandbox / SELinux | UID 격리, MAC | ❌ |
| Keystore / EncryptedSharedPreferences | 키 보관 | ❌ |
| Verified Boot / Play Protect | | ❌ |
| Screen pinning / Kiosk | 한 앱 고정 | ❌ |
| 안전 모드 / 게스트 / 멀티 유저 | | ❌ |
| Work profile | 업무용 격리 프로필 | ❌ |

### 7.6 폴더블 / 멀티 디스플레이
| Android 개념 | 설명 | 현재 |
|---|---|---|
| 디스플레이 모드 (Bar/Flip/Fold) | 폼팩터 별 화면 배치 | ✅ 시뮬레이터가 직접 모사 |
| Posture 감지 | FLAT / HALF_OPEN / FULL — 힌지 각도 센서 | ❌ posture 개념 없음, 사용자가 키로 모드 전환 |
| App Continuity | cover → main 펼치면 같은 앱 이어서 | 🟡 모드 전환 시 active app 상태만 직렬화 후 복원, “이어 보기” 애니메이션 없음 |
| Flex 모드 | 반접힘 시 위/아래 패널로 UI 분할 | ❌ |
| 디스플레이마다 다른 컨텐츠 | cover에 시계/알림 위젯 | ❌ secondary는 검은 배경뿐 |
| Span across folds | 양쪽 화면을 가로지르는 앱 | ❌ |
| Hinge angle API | | ❌ |

### 7.7 미디어 / 센서 / 연결
| Android 개념 | 설명 | 현재 |
|---|---|---|
| 센서 (가속도/자이로/조도/근접) | 화면 자동 회전, 자동 밝기 | ❌ |
| 오디오 포커스 / 볼륨 스트림 | 미디어/알림/통화 분리 | ❌ |
| MediaSession / Now Playing | 잠금화면/커튼 컨트롤 | ❌ |
| TextToSpeech / SpeechRecognizer | | ❌ |
| 카메라2 / CameraX | preview, capture | ❌ |
| 위치 (GPS/Fused) | | ❌ |
| 텔레포니 / SMS / RIL | 통화 상태 머신, 자동 응답 | 🟡 “전화 받기 시뮬” 키 V만, 상태 머신/이력 없음 |
| Wi-Fi / Bluetooth / NFC 매니저 | | ❌ |
| USB / 충전 / 전원 이벤트 | | ❌ |

### 7.8 데이터 / 설정 영속화
| Android 개념 | 설명 | 현재 |
|---|---|---|
| SharedPreferences / DataStore | KV 영속화 | ❌ Settings 토글/슬라이더 모두 메모리에만 |
| SQLite / Room | RDBMS | ❌ |
| Storage Access Framework | 사용자 파일 선택 | ❌ |
| Scoped Storage / MediaStore | 미디어 인덱싱 | ❌ — Gallery는 인메모리 스크린샷만 |
| Backup & Restore | 자동 백업, 디바이스 이전 | ❌ |

### 7.9 시스템 설정 / 테마
| Android 개념 | 설명 | 현재 |
|---|---|---|
| Settings provider | 시스템 전역 설정 | 🟡 UI만, 값이 어디에도 반영되지 않음 |
| 다크모드 / Material You | 동적 컬러, 테마 토큰 | 🟡 토글은 있으나 실제 색 변경 없음 |
| 글꼴 크기 / 디스플레이 배율 | 접근성 스케일링 | ❌ |
| 다국어 | 시스템 locale, RTL | ❌ |
| 접근성 (스위치, 자막, 색반전) | | ❌ |
| 개발자 옵션 (애니 스케일 등) | | ❌ |

### 7.10 사용자 / 셸 경험
| Android 개념 | 설명 | 현재 |
|---|---|---|
| 위젯 (App Widgets) | 홈에 띄우는 미니 UI | ❌ 홈은 그리드 아이콘만 |
| 앱 서랍 | 전체 앱 리스트 | ❌ 4×5 그리드만, 첫 4칸만 실제 앱 |
| 폴더 / 그룹 / 검색 | | ❌ |
| Google Discover / 좌측 패널 | | ❌ |
| 스크린샷 후 편집 / 공유 시트 | | 🟡 캡처만 됨, 이후 흐름 없음 |
| 화면 녹화 | | ❌ |

### 7.11 “있는데 형식적”인 것 (실제 동작은 미완)
- **Curtain**: 텍스트 배너만, 토글 타일 미작동.
- **Status bar**: 시계만 실시간, 나머지는 정적 문자열.
- **Settings**: 그리기·인터랙션은 되지만 결과를 어디서도 읽지 않음.
- **Lock**: 인스턴스만 있고 시뮬레이터에서 호출 경로 없음.
- **Keyboard**: 키 비주얼/슬라이드만, 키 입력 이벤트 미발행.
- **Gallery**: 시뮬레이터 메모리 스크린샷만, 디스크 파일·미디어스토어 없음.
- **History 카드 dedup**: `(AppID, Color)` 기준 — 같은 앱·다른 상태가 들어오면 덮어씀.
- **Call**: 받기/끊기만, 보류·스피커·DTMF·통화 이력 없음.

### 7.12 우선 순위 제안 (구현 난이도 vs 임팩트)

1. **이벤트 버스 + 알림 시스템** — `Curtain`을 진짜 알림 패널로. 통화/시스템 이벤트가 알림으로 등록되면 README의 “Signal/Administrator” 모델도 채워짐.  
2. **IME ↔ 텍스트 필드 연결** — 키보드 키 누름이 활성 앱의 포커스된 입력 위젯으로 라우팅. Settings에 텍스트 필드 1개만 추가해도 데모 가능.  
3. **Posture 시뮬레이션** — Flip의 “half-open”을 키로 한 단계 추가, 앱이 `posture` 콜백을 받게 → Flex UI 데모 1종(예: Camera 상단 미러).  
4. **Splash + Activity 라이프사이클 콜백** — Window의 라이프사이클 전이를 앱 콘텐츠에 콜백으로 노출. Cold start 시 splash 윈도우를 1프레임 뒤 본 캔버스로 cross-fade.  
5. **런타임 권한 다이얼로그** — Call/Camera 같은 앱이 처음 launch될 때 모달 다이얼로그. 윈도우 위에 떠야 하므로 진짜 dialog 윈도우 타입을 추가하는 계기.  
6. **다크모드 / Material You 토큰** — 색상 상수를 테마 토큰으로 빼고 Settings의 토글이 실제로 적용되게.  
7. **잠금화면 + 생체/PIN 모의** — `DefaultLock`을 시뮬레이터에 노출 + 전원 OFF→ON 시 잠금 등장, 위로 스와이프해서 해제. 그 위에 알림 미리보기.  
8. **App Widgets / 홈 폴더** — 홈 셀에 위젯 컨텐츠 인터페이스를 넣어 `Clock`, `Weather` 더미 위젯.  
9. **Persistent Storage** — `internal/util/fs.go`를 `os.UserConfigDir`로 연결해 Settings 값 디스크 저장.  
10. **테스트 + 로그 구조화** — 라이프사이클/스크롤 한계/제스처 분류 단위 테스트, 로그를 `{type, app, payload}` 구조로.
