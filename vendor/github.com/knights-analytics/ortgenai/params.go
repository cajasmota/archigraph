package ortgenai

/*
#cgo CFLAGS: -O2 -g
#include "ort_genai_wrapper.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

// createGeneratorParams creates OgaGeneratorParams for the given model and options.
// This is shared between Session and Engine.
func createGeneratorParams(m *model, opts *GenerationOptions) (*C.OgaGeneratorParams, error) {
	var cParams *C.OgaGeneratorParams
	res := C.CreateOgaGeneratorParams(m.modelPtr, &cParams)
	if err := OgaResultToError(res); err != nil {
		return nil, fmt.Errorf("CreateOgaGeneratorParams failed: %w", err)
	}
	if cParams == nil {
		return nil, errors.New("CreateOgaGeneratorParams returned nil without error")
	}

	setParam := func(name string, value float64) error {
		cName := C.CString(name)
		defer C.free(unsafe.Pointer(cName))
		res := C.GeneratorParamsSetSearchNumber(cParams, cName, C.double(value))
		return OgaResultToError(res)
	}

	if err := setParam("max_length", float64(opts.MaxLength)); err != nil {
		C.DestroyOgaGeneratorParams(cParams)
		return nil, fmt.Errorf("set max_length: %w", err)
	}
	if err := setParam("batch_size", float64(opts.BatchSize)); err != nil {
		C.DestroyOgaGeneratorParams(cParams)
		return nil, fmt.Errorf("set batch_size: %w", err)
	}
	if opts.Temperature != nil {
		if err := setParam("temperature", *opts.Temperature); err != nil {
			C.DestroyOgaGeneratorParams(cParams)
			return nil, fmt.Errorf("set temperature: %w", err)
		}
	}
	if opts.TopP != nil {
		if err := setParam("top_p", *opts.TopP); err != nil {
			C.DestroyOgaGeneratorParams(cParams)
			return nil, fmt.Errorf("set top_p: %w", err)
		}
	}
	if opts.Seed != nil {
		if err := setParam("random_seed", float64(*opts.Seed)); err != nil {
			C.DestroyOgaGeneratorParams(cParams)
			return nil, fmt.Errorf("set random_seed: %w", err)
		}
	}

	if opts.Guidance != nil {
		cGuidanceType := C.CString(string(opts.Guidance.Type))
		defer C.free(unsafe.Pointer(cGuidanceType))
		cGuidanceData := C.CString(opts.Guidance.Data)
		defer C.free(unsafe.Pointer(cGuidanceData))
		res = C.GeneratorParamsSetGuidance(cParams, cGuidanceType, cGuidanceData, C.bool(opts.Guidance.EnableFFTokens))
		if err := OgaResultToError(res); err != nil {
			C.DestroyOgaGeneratorParams(cParams)
			return nil, fmt.Errorf("GeneratorParamsSetGuidance failed: %w", err)
		}
	}

	return cParams, nil
}
