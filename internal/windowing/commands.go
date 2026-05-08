package windowing

import (
	"image/color"
	"time"

	mosapp "github.com/hndada/mos/internal/app"
)

// AppCommand is a request from app code to the windowing server, sent over the
// per-window cmdCh. The server drains the channel after each app Update and
// dispatches commands on the main goroutine.
//
// Real-OS analogue: in a multi-process mobile OS these would be Binder
// transactions (Android) or XPC messages (iOS) crossing the process
// boundary up to system_server / SpringBoard. Here the boundary is a command
// queue, which keeps the simulator lightweight.
type AppCommand any

// CmdFinish requests that the window be dismissed.
type CmdFinish struct{}

// CmdLaunch asks the OS to open another app by ID.
type CmdLaunch struct{ AppID string }

// CmdSetTitle / CmdSetAccentColor update OS-visible app metadata.
type CmdSetTitle struct{ Title string }
type CmdSetAccentColor struct{ Color color.RGBA }
type CmdSetKeepScreenOn struct{ Enabled bool }
type CmdSetPreferredOrientation struct{ Orientation mosapp.Orientation }

// CmdShowKeyboard / CmdHideKeyboard toggle the system soft keyboard.
type CmdShowKeyboard struct{}
type CmdHideKeyboard struct{}

// CmdOpenURL / CmdShareText request system-level intent/share handling.
type CmdOpenURL struct{ URL string }
type CmdShareText struct{ Text string }

// CmdPostNotice posts a notice to the curtain / notification centre.
type CmdPostNotice struct{ Notice mosapp.Notice }
type CmdShowToast struct{ Text string }
type CmdScheduleNotice struct {
	ID     string
	Notice mosapp.Notice
	At     time.Time
}
type CmdCancelNotice struct{ ID string }
type CmdSetBadge struct{ Count int }
type CmdClearBadge struct{}
type CmdSetBarStyle struct {
	Slot  string
	Style mosapp.BarStyle
}
type CmdSetSystemBarsHidden struct{ Hidden bool }

// CmdSetDarkMode changes the OS appearance setting.
type CmdSetDarkMode struct{ Enabled bool }

// CmdVibrate triggers simulated haptic feedback.
type CmdVibrate struct{ Duration time.Duration }

// CmdRequestFocus asks the windowing server to grant keyboard/IME focus to
// the sending window. The server serialises focus; the previous holder's
// ctx.HasFocus() becomes false atomically before the new holder's becomes true.
type CmdRequestFocus struct{}

// CmdReleaseFocus voluntarily surrenders keyboard/IME focus. If the sending
// window does not currently hold focus the command is a no-op.
type CmdReleaseFocus struct{}
