# TODO Implementation

## `internal/input/keyboard.go`
Renamed `KeyboardState.KeysPressed` → `AreKeysPressed` (boolean predicate convention).

## `cmd/sim/main.go`
`Simulator` now implements `ebiten.Game` (`Update`, `Draw`, `Layout`) and has a `main()` entry point so the package compiles and runs.

## `ui/box.go`
Added drag-and-drop state to `Box`:
- `BeginDrag(cursor XY)` — records cursor offset relative to box origin.
- `UpdateDrag(cursor XY)` — repositions box to follow cursor.
- `EndDrag()` / `IsDragging() bool` — lifecycle helpers.
- `GhostSprite(src Image) draws.Sprite` — returns a half-alpha sprite at the drag position for the compositor to paint as the ghost window.

## `sysapps/wallpaper.go`
`Wallpaper` interface + `DefaultWallpaper` (wraps a `draws.Sprite`, delegates `Draw`).

## `sysapps/lock.go`
`LockScreen` interface + `DefaultLockScreen` with `Lock` / `Unlock` / `IsLocked`.

## `sysapps/statusbar.go`
`StatusBar` interface + `DefaultStatusBar` with an injectable clock and a `Draw` stub (renders clock, battery, signal).

## `internal/fw/windowing.go`
Added method bodies to `ShowKeyboard`, `HideKeyboard`, `showSplash` (were bare declarations — invalid Go). Changed pointer fields to interface fields to match the `sysapps` interface types.
