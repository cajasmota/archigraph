package ortgenai

import "errors"

/*
#cgo CFLAGS: -O2 -g
#include "ort_genai_wrapper.h"
*/
import "C"

func OgaResultToError(result *C.OgaResult) error {
	if result == nil {
		return nil
	}
	cString := C.GetOgaResultErrorString(result)
	msg := C.GoString(cString)
	C.DestroyOgaResult(result)
	return errors.New(msg)
}
