package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -fblocks
#cgo LDFLAGS: -framework Cocoa -framework UniformTypeIdentifiers
#include <stdlib.h>
#include "context.h"
*/
import "C"

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/cgo"
	"unsafe"

	"option-tab/internal/platform"
)

var fixtureCancel context.CancelFunc

//export goDiagnosticFixtureCancel
func goDiagnosticFixtureCancel() { fixtureCancel() }

func main() {
	_ = platform.NewDiagnosticExportSource()
	ctx, cancel := context.WithCancel(context.Background())
	token := cgo.NewHandle(ctx)
	defer token.Delete()
	owner := C.context_start(C.uintptr_t(token))
	if owner == nil {
		panic("owner unavailable")
	}
	defer C.context_release(owner)
	cancel()
	C.context_pump()
	if C.context_panels() != 0 || C.context_result(owner) != 2 {
		fmt.Fprintln(os.Stderr, "FAIL cancelled Go context reached panel creation before polling")
		os.Exit(1)
	}
	if len(os.Args) != 2 {
		panic("fixture directory required")
	}
	commitCtx, commitCancel := context.WithCancel(context.Background())
	defer commitCancel()
	fixtureCancel = commitCancel
	commitToken := cgo.NewHandle(commitCtx)
	defer commitToken.Delete()
	path := filepath.Join(os.Args[1], "cancelled-before-commit.json")
	nativePath := C.CString(path)
	defer C.free(unsafe.Pointer(nativePath))
	if C.context_write(C.uintptr_t(commitToken), nativePath) != 2 {
		panic("cancelled Go context admitted commit")
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		panic("cancelled commit wrote destination")
	}
	fmt.Println("PASS exact Go context cancellation before queued main, zero panel creations; no polling/native-cancel")
}
