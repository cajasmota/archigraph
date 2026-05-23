package ortgenai

/*
#cgo CFLAGS: -O2 -g
#include "ort_genai_wrapper.h"
*/
import "C"

import (
	"encoding/base64"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"unsafe"
)

// Images represents a collection of loaded images for multimodal processing.
type Images struct {
	imagesPtr *C.OgaImages
}

func (i *Images) destroy() {
	if i.imagesPtr != nil {
		C.DestroyOgaImages(i.imagesPtr)
	}
	i.imagesPtr = nil
}

// Destroy releases the images resources.
func (i *Images) Destroy() {
	i.destroy()
}

// MultiModalProcessor processes images and text together for multimodal models.
type multiModalProcessor struct {
	processorPtr *C.OgaMultiModalProcessor
}

func (p *multiModalProcessor) destroy() {
	if p.processorPtr != nil {
		C.DestroyOgaMultiModalProcessor(p.processorPtr)
	}
	p.processorPtr = nil
}

// Destroy releases the processor resources.
func (p *multiModalProcessor) Destroy() {
	p.destroy()
}

// NamedTensors represents a collection of named tensor inputs.
type NamedTensors struct {
	tensorsPtr *C.OgaNamedTensors
}

func (nt *NamedTensors) destroy() {
	if nt.tensorsPtr != nil {
		C.DestroyOgaNamedTensors(nt.tensorsPtr)
	}
	nt.tensorsPtr = nil
}

// Destroy releases the named tensors resources.
func (nt *NamedTensors) Destroy() {
	nt.destroy()
}

// Supports format: data:image/png;base64,<base64-encoded-data>.
func parseDataURI(dataURI string) ([]byte, error) {
	// Check if it starts with "data:"
	if !strings.HasPrefix(dataURI, "data:") {
		return nil, fmt.Errorf("invalid data URI: must start with 'data:'")
	}

	// Find the comma that separates metadata from data
	commaIdx := strings.Index(dataURI, ",")
	if commaIdx == -1 {
		return nil, fmt.Errorf("invalid data URI: missing comma separator")
	}

	// Extract metadata and check for base64
	metadata := dataURI[5:commaIdx] // skip "data:"
	if !strings.Contains(metadata, "base64") {
		return nil, fmt.Errorf("unsupported data URI encoding: only base64 is supported")
	}

	// Decode base64 data
	encodedData := dataURI[commaIdx+1:]
	decodedData, err := base64.StdEncoding.DecodeString(encodedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 data: %w", err)
	}

	return decodedData, nil
}

// LoadImage loads a single image from a file path or data URI.
func LoadImage(imagePath string) (*Images, error) {
	if !IsInitialized() {
		return nil, ErrNotInitialized
	}

	// Check if it's a data URI
	if strings.HasPrefix(imagePath, "data:") {
		imageData, err := parseDataURI(imagePath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse data URI: %w", err)
		}
		return LoadImageFromBuffer(imageData)
	}

	// Load from file path
	cPath := C.CString(imagePath)
	defer C.free(unsafe.Pointer(cPath))

	var cImages *C.OgaImages
	res := C.LoadOgaImage(cPath, &cImages)
	if err := OgaResultToError(res); err != nil {
		return nil, fmt.Errorf("LoadImage failed: %w", err)
	}
	if cImages == nil {
		return nil, errors.New("LoadImage returned nil without error")
	}

	return &Images{imagesPtr: cImages}, nil
}

// LoadImageFromBuffer loads a single image from a byte buffer.
func LoadImageFromBuffer(imageData []byte) (*Images, error) {
	if !IsInitialized() {
		return nil, ErrNotInitialized
	}

	if len(imageData) == 0 {
		return nil, errors.New("image data is empty")
	}

	// Pin the Go memory before passing to C
	var pinner runtime.Pinner
	pinner.Pin(&imageData[0])
	defer pinner.Unpin()

	// Create C array of pointers and sizes
	dataPtr := unsafe.Pointer(&imageData[0])
	dataSize := C.size_t(len(imageData))

	var cImages *C.OgaImages
	res := C.LoadOgaImagesFromBuffers(
		&dataPtr,
		&dataSize,
		1, // count = 1 for single image
		&cImages,
	)
	if err := OgaResultToError(res); err != nil {
		return nil, fmt.Errorf("LoadImageFromBuffer failed: %w", err)
	}
	if cImages == nil {
		return nil, errors.New("LoadImageFromBuffer returned nil without error")
	}

	return &Images{imagesPtr: cImages}, nil
}

// LoadImages loads multiple images from file paths or data URIs.
func LoadImages(imagePaths []string) (*Images, error) {
	if !IsInitialized() {
		return nil, ErrNotInitialized
	}

	if len(imagePaths) == 0 {
		return nil, errors.New("no image paths provided")
	}

	// Check if any paths are data URIs - if so, we need to use buffer loading
	hasDataURI := false
	for _, path := range imagePaths {
		if strings.HasPrefix(path, "data:") {
			hasDataURI = true
			break
		}
	}

	if hasDataURI {
		// Decode all images to buffers and use buffer loading
		buffers := make([][]byte, len(imagePaths))
		for i, path := range imagePaths {
			if strings.HasPrefix(path, "data:") {
				data, err := parseDataURI(path)
				if err != nil {
					return nil, fmt.Errorf("failed to parse data URI at index %d: %w", i, err)
				}
				buffers[i] = data
			} else {
				// For file paths, we'd need to read the file
				// For now, return an error if mixing data URIs with file paths
				return nil, errors.New("cannot mix data URIs with file paths in LoadImages")
			}
		}
		return LoadImagesFromBuffers(buffers)
	}

	// All are file paths - use the C API directly
	// Create OgaStringArray
	var cStringArray *C.OgaStringArray
	res := C.CreateOgaStringArray(&cStringArray)
	if err := OgaResultToError(res); err != nil {
		return nil, fmt.Errorf("CreateOgaStringArray failed: %w", err)
	}
	defer C.DestroyOgaStringArray(cStringArray)

	// Add each path to the string array
	for _, path := range imagePaths {
		cPath := C.CString(path)
		res = C.AddStringToOgaStringArray(cStringArray, cPath)
		C.free(unsafe.Pointer(cPath))
		if err := OgaResultToError(res); err != nil {
			// cStringArray will be destroyed by defer C.DestroyOgaStringArray(cStringArray)
			return nil, fmt.Errorf("AddStringToOgaStringArray failed: %w", err)
		}
	}

	// Load images
	var cImages *C.OgaImages
	res = C.LoadOgaImages(cStringArray, &cImages)
	if err := OgaResultToError(res); err != nil {
		return nil, fmt.Errorf("LoadImages failed: %w", err)
	}
	if cImages == nil {
		return nil, errors.New("LoadImages returned nil without error")
	}

	return &Images{imagesPtr: cImages}, nil
}

// LoadImagesFromBuffers loads multiple images from byte buffers.
func LoadImagesFromBuffers(imageBuffers [][]byte) (*Images, error) {
	if !IsInitialized() {
		return nil, ErrNotInitialized
	}

	if len(imageBuffers) == 0 {
		return nil, errors.New("no image buffers provided")
	}

	// Create arrays for pointers and sizes
	dataPtrs := make([]unsafe.Pointer, len(imageBuffers))
	dataSizes := make([]C.size_t, len(imageBuffers))

	// Pin all buffer memory before passing to C
	var pinner runtime.Pinner
	defer pinner.Unpin()

	for i, buf := range imageBuffers {
		if len(buf) == 0 {
			return nil, fmt.Errorf("image buffer at index %d is empty", i)
		}
		pinner.Pin(&buf[0])
		dataPtrs[i] = unsafe.Pointer(&buf[0])
		dataSizes[i] = C.size_t(len(buf))
	}

	var cImages *C.OgaImages
	res := C.LoadOgaImagesFromBuffers(
		&dataPtrs[0],
		&dataSizes[0],
		C.size_t(len(imageBuffers)),
		&cImages,
	)
	if err := OgaResultToError(res); err != nil {
		return nil, fmt.Errorf("LoadImagesFromBuffers failed: %w", err)
	}
	if cImages == nil {
		return nil, errors.New("LoadImagesFromBuffers returned nil without error")
	}

	return &Images{imagesPtr: cImages}, nil
}

// CreateMultiModalProcessor creates a multimodal processor from a model.
func createMultiModalProcessor(model *model) (*multiModalProcessor, error) {
	if !IsInitialized() {
		return nil, ErrNotInitialized
	}

	if model == nil || model.modelPtr == nil {
		return nil, errors.New("model is nil")
	}

	var cProcessor *C.OgaMultiModalProcessor
	res := C.CreateOgaMultiModalProcessor(model.modelPtr, &cProcessor)
	if err := OgaResultToError(res); err != nil {
		return nil, fmt.Errorf("CreateMultiModalProcessor failed: %w", err)
	}
	if cProcessor == nil {
		return nil, errors.New("CreateMultiModalProcessor returned nil without error")
	}

	return &multiModalProcessor{processorPtr: cProcessor}, nil
}

// ProcessImages processes images with a prompt and returns named tensors.
func (p *multiModalProcessor) ProcessImages(prompt string, images *Images) (*NamedTensors, error) {
	if p.processorPtr == nil {
		return nil, errors.New("processor is not initialized")
	}
	if images == nil || images.imagesPtr == nil {
		return nil, errors.New("images is nil")
	}

	// TODO: support multiple prompts, somehow this gives an error on image tag mismatch
	// // Create OgaStringArray for prompts
	// var cStringArray *C.OgaStringArray
	// res := C.CreateOgaStringArray(&cStringArray)
	// if err := OgaResultToError(res); err != nil {
	// 	return nil, fmt.Errorf("CreateOgaStringArray failed: %w", err)
	// }
	// defer C.DestroyOgaStringArray(cStringArray)

	// for _, prompt := range prompts {
	// 	cPrompt := C.CString(prompt)
	// 	res = C.AddStringToOgaStringArray(cStringArray, cPrompt)
	// 	C.free(unsafe.Pointer(cPrompt))
	// 	if err := OgaResultToError(res); err != nil {
	// 		return nil, fmt.Errorf("AddStringToOgaStringArray failed: %w", err)
	// 	}
	// }
	// res = C.ProcessOgaImagesAndPrompts(p.processorPtr, cStringArray, images.imagesPtr, &cTensors)

	var cTensors *C.OgaNamedTensors
	promptC := C.CString(prompt)
	defer C.free(unsafe.Pointer(promptC))
	res := C.ProcessOgaImages(p.processorPtr, promptC, images.imagesPtr, &cTensors)
	if err := OgaResultToError(res); err != nil {
		if cTensors != nil {
			C.DestroyOgaNamedTensors(cTensors)
		}
		return nil, fmt.Errorf("ProcessImages failed: %w", err)
	}
	if cTensors == nil {
		return nil, errors.New("ProcessImages returned nil without error")
	}
	return &NamedTensors{tensorsPtr: cTensors}, nil
}
