package windowing

import mosapp "github.com/hndada/mos/internal/app"

// AppCommand is a request from an app's goroutine to the windowing server,
// sent over the per-window cmdCh. The server drains the channel after each
// goroutine ack and dispatches commands on the main goroutine.
//
// Real-OS analogue: in a multi-process mobile OS these would be Binder
// transactions (Android) or XPC messages (iOS) crossing the process
// boundary up to system_server / SpringBoard. Here the boundary is a
// goroutine boundary; the protocol shape is the same.
type AppCommand interface{ appCommand() }

// CmdFinish requests that the window be dismissed.
type CmdFinish struct{}

// CmdLaunch asks the OS to open another app by ID.
type CmdLaunch struct{ AppID string }

// CmdShowKeyboard / CmdHideKeyboard toggle the system soft keyboard.
type CmdShowKeyboard struct{}
type CmdHideKeyboard struct{}

// CmdPostNotice posts a notice to the curtain / notification centre.
type CmdPostNotice struct{ Notice mosapp.Notice }

func (CmdFinish) appCommand()       {}
func (CmdLaunch) appCommand()       {}
func (CmdShowKeyboard) appCommand() {}
func (CmdHideKeyboard) appCommand() {}
func (CmdPostNotice) appCommand()   {}
