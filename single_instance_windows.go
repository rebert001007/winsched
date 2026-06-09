package main

import (
	"fmt"

	"golang.org/x/sys/windows"
)

const singleInstanceMutexName = `Global\winsched-single-instance`

type singleInstanceLock struct {
	handle windows.Handle
}

func acquireSingleInstanceLock() (*singleInstanceLock, bool, error) {
	return acquireNamedSingleInstanceLock(singleInstanceMutexName)
}

func acquireNamedSingleInstanceLock(name string) (*singleInstanceLock, bool, error) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, false, fmt.Errorf("invalid mutex name %q: %w", name, err)
	}

	handle, err := windows.CreateMutex(nil, true, namePtr)
	if err != nil {
		if err == windows.ERROR_ALREADY_EXISTS {
			_ = windows.CloseHandle(handle)
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("create mutex %q: %w", name, err)
	}

	return &singleInstanceLock{handle: handle}, false, nil
}

func (l *singleInstanceLock) Close() error {
	if l == nil || l.handle == 0 {
		return nil
	}

	releaseErr := windows.ReleaseMutex(l.handle)
	closeErr := windows.CloseHandle(l.handle)
	l.handle = 0

	if releaseErr != nil {
		return fmt.Errorf("release mutex: %w", releaseErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close mutex handle: %w", closeErr)
	}
	return nil
}
