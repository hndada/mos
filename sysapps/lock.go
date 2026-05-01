package sysapps

import "time"

type LockScreen interface {
	Lock()
	Unlock()
	IsLocked() bool
}

type DefaultLockScreen struct {
	locked   bool
	lockedAt time.Time
}

func (l *DefaultLockScreen) Lock() {
	l.locked = true
	l.lockedAt = time.Now()
}

func (l *DefaultLockScreen) Unlock()       { l.locked = false }
func (l *DefaultLockScreen) IsLocked() bool { return l.locked }
