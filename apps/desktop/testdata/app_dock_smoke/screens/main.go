package main

/*
#include <stdlib.h>
char *ot_screens_json(void);
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"os"
	"unsafe"

	"option-tab/internal/platform"
)

func main() {
	p, err := platform.New()
	if err != nil {
		panic(err)
	}
	defer p.Hotkeys().Close()
	raw := C.ot_screens_json()
	defer C.free(unsafe.Pointer(raw))
	data := C.GoString(raw)
	fmt.Println("native screen JSON:", data)
	var records []struct {
		ID   uint32 `json:"id"`
		Main bool   `json:"main"`
	}
	err = json.Unmarshal([]byte(data), &records)
	fmt.Printf("bool parse: %v\n", err)
	screens := p.Screens()
	fmt.Printf("native platform screens: %+v\n", screens)
	if err != nil || len(screens) == 0 || screens[0].ID == 0 || screens[0].Bounds.Area() == 0 {
		os.Exit(1)
	}
}
