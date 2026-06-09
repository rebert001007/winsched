package main

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestAcquireNamedSingleInstanceLock(t *testing.T) {
	name := fmt.Sprintf(`Global\winsched-test-%d-%d`, os.Getpid(), time.Now().UnixNano())

	lock1, alreadyRunning, err := acquireNamedSingleInstanceLock(name)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	if alreadyRunning {
		t.Fatal("first acquire unexpectedly reported already running")
	}
	defer func() {
		if err := lock1.Close(); err != nil {
			t.Fatalf("first close failed: %v", err)
		}
	}()

	lock2, alreadyRunning, err := acquireNamedSingleInstanceLock(name)
	if err != nil {
		t.Fatalf("second acquire failed: %v", err)
	}
	if lock2 != nil {
		t.Fatal("second acquire unexpectedly returned a lock")
	}
	if !alreadyRunning {
		t.Fatal("second acquire should report already running")
	}

	if err := lock1.Close(); err != nil {
		t.Fatalf("early close failed: %v", err)
	}

	lock1 = nil

	lock3, alreadyRunning, err := acquireNamedSingleInstanceLock(name)
	if err != nil {
		t.Fatalf("third acquire failed: %v", err)
	}
	if alreadyRunning {
		t.Fatal("third acquire unexpectedly reported already running")
	}
	if lock3 == nil {
		t.Fatal("third acquire did not return a lock")
	}
	defer func() {
		if err := lock3.Close(); err != nil {
			t.Fatalf("third close failed: %v", err)
		}
	}()
}
