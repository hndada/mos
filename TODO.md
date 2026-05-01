# TODO

## 윈도잉
- [ ] **멀티 윈도** — split-screen, picture-in-picture, freeform
  - Window 타입 분리 (fullscreen / split / floating / pip)
  - 윈도우별 영역 계산기 + 드래그/리사이즈 핸들
  - 활성 입력 라우팅을 “포커스된 윈도우 한 개”로 일반화

## 입력
- [ ] **멀티터치** — 다중 포인터 추적, pinch / two-finger drag / rotate 제스처

## 시스템 표시 / 알림
- [ ] **퀵 타일 (Quick Settings)** — 커튼 안에 토글 가능한 타일 (Wi-Fi, BT, 밝기 등)
- [ ] **AOD (Always-On Display)** — 화면 OFF 상태에서 시계/알림 미리보기

## 잠금 / 사용자
- [ ] **Lock screen** — 시뮬레이터 노출, 전원 OFF→ON 시 자동 등장, 스와이프/PIN/패턴 해제
- [ ] **멀티 유저** — 사용자/게스트 프로필 전환, 프로필별 데이터 분리

## 시스템 설정
- [ ] **다크 모드 설정** — Settings 토글이 실제 컬러 토큰에 반영되도록 테마 시스템 도입
- [ ] **다국어** — locale 전환, RTL 대응
- [ ] **글꼴 스케일** — 시스템 글자 크기 → `draws.Text` 사이즈에 곱해지는 전역 배율

## 셸 경험
- [ ] **위젯 (App Widgets)** — 홈 셀에 들어가는 위젯 컨텐츠 인터페이스 + 더미 위젯(시계/날씨)
- [ ] **앱 폴더** — 홈 아이콘 묶음, 탭하면 펼쳐지는 폴더 뷰
- [ ] **화면 녹화** — 캔버스 프레임 시퀀스를 캡처해 GIF/연속 PNG로 저장
