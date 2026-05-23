package ortgenai

/*
#cgo LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdlib.h>
#include "ort_genai_wrapper.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// genAiLibraryHandle holds the dlopen handle for the GenAI shared library once loaded.
var genAiLibraryHandle unsafe.Pointer

func platformCleanup() error {
	engineAPIAvailable = false
	if genAiLibraryHandle == nil {
		return nil
	}
	if returnCode := C.dlclose(genAiLibraryHandle); returnCode != 0 {
		return fmt.Errorf("error closing GenAI shared library: %d", int(returnCode))
	}
	genAiLibraryHandle = nil
	return nil
}

func createSym(handle unsafe.Pointer, name string) unsafe.Pointer {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	C.dlerror() // clear previous error per POSIX
	sym := C.dlsym(handle, cName)
	return sym
}

// InitializeGenAiLibrary loads the ONNX Runtime GenAI shared library specified by
// onnxGenaiSharedLibraryPath (or a default) so its exported symbols become available.
// The assumption is that libonnxruntime.so is available in the same folder where the libonnxruntime-genai.so is located.
func InitializeGenAiLibrary() error {
	if genAiLibraryHandle != nil {
		return fmt.Errorf("GenAI library already initialized")
	}
	libPath := onnxGenaiSharedLibraryPath
	if libPath == "" {
		libPath = "libonnxruntime-genai.so"
	}
	cName := C.CString(libPath)
	defer C.free(unsafe.Pointer(cName))
	handle := C.dlopen(cName, C.RTLD_LAZY)
	if handle == nil {
		msg := C.GoString(C.dlerror())
		return fmt.Errorf("error loading GenAI shared library %q: %v", libPath, msg)
	}

	symCreate := createSym(handle, "OgaCreateModel")
	if symCreate == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaCreateModel")
	}

	symErr := createSym(handle, "OgaResultGetError")
	if symErr == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaResultGetError")
	}
	symDestroyRes := createSym(handle, "OgaDestroyResult")
	if symDestroyRes == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaDestroyResult")
	}
	symDestroyModel := createSym(handle, "OgaDestroyModel")
	if symDestroyModel == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaDestroyModel")
	}
	symCreateTokenizer := createSym(handle, "OgaCreateTokenizer")
	if symCreateTokenizer == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaCreateTokenizer")
	}
	symDestroyTokenizer := createSym(handle, "OgaDestroyTokenizer")
	if symDestroyTokenizer == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaDestroyTokenizer")
	}
	symCreateTokenizerStream := createSym(handle, "OgaCreateTokenizerStream")
	if symCreateTokenizerStream == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaCreateTokenizerStream")
	}
	symDestroyTokenizerStream := createSym(handle, "OgaDestroyTokenizerStream")
	if symDestroyTokenizerStream == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaDestroyTokenizerStream")
	}
	symApplyChatTemplate := createSym(handle, "OgaTokenizerApplyChatTemplate")
	if symApplyChatTemplate == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaTokenizerApplyChatTemplate")
	}
	symDestroyString := createSym(handle, "OgaDestroyString")
	if symDestroyString == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaDestroyString")
	}
	symCreateSequence := createSym(handle, "OgaCreateSequences")
	if symCreateSequence == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaCreateSequence")
	}
	symDestroySequence := createSym(handle, "OgaDestroySequences")
	if symDestroySequence == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaDestroySequences")
	}
	symTokenizerEncode := createSym(handle, "OgaTokenizerEncode")
	if symTokenizerEncode == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaTokenizerEncode")
	}
	symCreateGenerator := createSym(handle, "OgaCreateGenerator")
	if symCreateGenerator == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaCreateGenerator")
	}
	symDestroyGenerator := createSym(handle, "OgaDestroyGenerator")
	if symDestroyGenerator == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaDestroyGenerator")
	}
	symCreateGeneratorParams := createSym(handle, "OgaCreateGeneratorParams")
	if symCreateGeneratorParams == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaCreateGeneratorParams")
	}
	symDestroyGeneratorParams := createSym(handle, "OgaDestroyGeneratorParams")
	if symDestroyGeneratorParams == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaDestroyGeneratorParams")
	}
	symGeneratorParamsSetSearchNumber := createSym(handle, "OgaGeneratorParamsSetSearchNumber")
	if symGeneratorParamsSetSearchNumber == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaGeneratorParamsSetSearchNumber")
	}
	symGeneratorAppendTokenSequences := createSym(handle, "OgaGenerator_AppendTokenSequences")
	if symGeneratorAppendTokenSequences == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaGeneratorAppendTokenSequences")
	}
	symGeneratorSetInputs := createSym(handle, "OgaGenerator_SetInputs")
	if symGeneratorSetInputs == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaGeneratorSetInputs")
	}
	symGeneratorGenerateNextToken := createSym(handle, "OgaGenerator_GenerateNextToken")
	if symGeneratorGenerateNextToken == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaGeneratorGenerateNextToken")
	}
	symGeneratorGetSequenceCount := createSym(handle, "OgaGenerator_GetSequenceCount")
	if symGeneratorGetSequenceCount == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaGeneratorGetSequenceCount")
	}
	symGeneratorGetSequenceData := createSym(handle, "OgaGenerator_GetSequenceData")
	if symGeneratorGetSequenceData == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaGeneratorGetSequenceData")
	}
	symTokenizerStreamDecode := createSym(handle, "OgaTokenizerStreamDecode")
	if symTokenizerStreamDecode == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaTokenizerStreamDecode")
	}
	symIsDone := createSym(handle, "OgaGenerator_IsDone")
	if symIsDone == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaGenerator_IsDone")
	}

	// EOS token ids
	symTokenizerGetEosTokenIDs := createSym(handle, "OgaTokenizerGetEosTokenIds")
	if symTokenizerGetEosTokenIDs == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaTokenizerGetEosTokenIds")
	}

	// Config-related symbols
	symCreateConfig := createSym(handle, "OgaCreateConfig")
	if symCreateConfig == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaCreateConfig")
	}
	symConfigClearProviders := createSym(handle, "OgaConfigClearProviders")
	if symConfigClearProviders == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaConfigClearProviders")
	}
	symConfigAppendProvider := createSym(handle, "OgaConfigAppendProvider")
	if symConfigAppendProvider == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaConfigAppendProvider")
	}
	symConfigSetProviderOption := createSym(handle, "OgaConfigSetProviderOption")
	if symConfigSetProviderOption == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaConfigSetProviderOption")
	}
	symCreateModelFromConfig := createSym(handle, "OgaCreateModelFromConfig")
	if symCreateModelFromConfig == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaCreateModelFromConfig")
	}
	symDestroyConfig := createSym(handle, "OgaDestroyConfig")
	if symDestroyConfig == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaDestroyConfig")
	}

	// Multimodal symbols
	symLoadImage := createSym(handle, "OgaLoadImage")
	if symLoadImage == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaLoadImage")
	}
	symLoadImages := createSym(handle, "OgaLoadImages")
	if symLoadImages == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaLoadImages")
	}
	symLoadImagesFromBuffers := createSym(handle, "OgaLoadImagesFromBuffers")
	if symLoadImagesFromBuffers == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaLoadImagesFromBuffers")
	}
	symDestroyImages := createSym(handle, "OgaDestroyImages")
	if symDestroyImages == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaDestroyImages")
	}
	symCreateMultiModalProcessor := createSym(handle, "OgaCreateMultiModalProcessor")
	if symCreateMultiModalProcessor == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaCreateMultiModalProcessor")
	}
	symDestroyMultiModalProcessor := createSym(handle, "OgaDestroyMultiModalProcessor")
	if symDestroyMultiModalProcessor == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaDestroyMultiModalProcessor")
	}
	symProcessorProcessImages := createSym(handle, "OgaProcessorProcessImages")
	if symProcessorProcessImages == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaProcessorProcessImages")
	}
	symDestroyNamedTensors := createSym(handle, "OgaDestroyNamedTensors")
	if symDestroyNamedTensors == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaDestroyNamedTensors")
	}
	symCreateStringArray := createSym(handle, "OgaCreateStringArray")
	if symCreateStringArray == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaCreateStringArray")
	}
	symDestroyStringArray := createSym(handle, "OgaDestroyStringArray")
	if symDestroyStringArray == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaDestroyStringArray")
	}
	symStringArrayAddString := createSym(handle, "OgaStringArrayAddString")
	if symStringArrayAddString == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaStringArrayAddString")
	}
	symProcessorProcessImagesAndPrompts := createSym(handle, "OgaProcessorProcessImagesAndPrompts")
	if symProcessorProcessImagesAndPrompts == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaProcessorProcessImagesAndPrompts")
	}
	symGeneratorParamsSetGuidance := createSym(handle, "OgaGeneratorParamsSetGuidance")
	if symGeneratorParamsSetGuidance == nil {
		C.dlclose(handle)
		return fmt.Errorf("missing OgaGeneratorParamsSetGuidance")
	}

	if rc := C.SetGenAiApi(symCreate, symErr, symDestroyRes, symDestroyModel, symCreateTokenizer, symDestroyTokenizer,
		symCreateTokenizerStream, symDestroyTokenizerStream, symApplyChatTemplate, symDestroyString, symCreateSequence, symDestroySequence,
		symTokenizerEncode, symCreateGenerator, symDestroyGenerator, symCreateGeneratorParams, symDestroyGeneratorParams,
		symGeneratorParamsSetSearchNumber, symGeneratorAppendTokenSequences, symGeneratorSetInputs, symGeneratorGenerateNextToken, symGeneratorGetSequenceCount,
		symGeneratorGetSequenceData, symTokenizerStreamDecode, symIsDone, symTokenizerGetEosTokenIDs, symCreateConfig, symConfigClearProviders, symConfigAppendProvider, symConfigSetProviderOption, symCreateModelFromConfig, symDestroyConfig,
		symLoadImage, symLoadImages, symLoadImagesFromBuffers, symDestroyImages, symCreateMultiModalProcessor, symDestroyMultiModalProcessor, symProcessorProcessImages, symDestroyNamedTensors, symCreateStringArray, symDestroyStringArray, symStringArrayAddString,
		symProcessorProcessImagesAndPrompts, symGeneratorParamsSetGuidance); rc != 0 {
		C.dlclose(handle)
		return fmt.Errorf("SetGenAiApi failed with code %d", int(rc))
	}

	// Engine/Request API (optional — available in ORT GenAI >= 0.9.1)
	initEngineAPI(handle)

	genAiLibraryHandle = handle
	return nil
}

// engineAPIAvailable is true if the OgaEngine continuous batching symbols were found.
var engineAPIAvailable bool

// IsEngineAPIAvailable reports whether the loaded ORT GenAI library supports the Engine API.
func IsEngineAPIAvailable() bool {
	return engineAPIAvailable
}

func initEngineAPI(handle unsafe.Pointer) {
	symCreateEngine := createSym(handle, "OgaCreateEngine")
	symDestroyEngine := createSym(handle, "OgaDestroyEngine")
	symEngineStep := createSym(handle, "OgaEngineStep")
	symEngineHasPendingRequests := createSym(handle, "OgaEngineHasPendingRequests")
	symEngineAddRequest := createSym(handle, "OgaEngineAddRequest")
	symEngineRemoveRequest := createSym(handle, "OgaEngineRemoveRequest")
	symCreateRequest := createSym(handle, "OgaCreateRequest")
	symDestroyRequest := createSym(handle, "OgaDestroyRequest")
	symRequestAddTokens := createSym(handle, "OgaRequestAddTokens")
	symRequestSetOpaqueData := createSym(handle, "OgaRequestSetOpaqueData")
	symRequestGetOpaqueData := createSym(handle, "OgaRequestGetOpaqueData")
	symRequestHasUnseenTokens := createSym(handle, "OgaRequestHasUnseenTokens")
	symRequestGetUnseenToken := createSym(handle, "OgaRequestGetUnseenToken")
	symRequestIsDone := createSym(handle, "OgaRequestIsDone")

	if symCreateEngine == nil || symDestroyEngine == nil ||
		symEngineStep == nil || symEngineHasPendingRequests == nil ||
		symEngineAddRequest == nil || symEngineRemoveRequest == nil ||
		symCreateRequest == nil || symDestroyRequest == nil ||
		symRequestAddTokens == nil || symRequestSetOpaqueData == nil ||
		symRequestGetOpaqueData == nil || symRequestHasUnseenTokens == nil ||
		symRequestGetUnseenToken == nil || symRequestIsDone == nil {
		return
	}

	rc := C.SetGenAiEngineApi(
		symCreateEngine, symDestroyEngine,
		symEngineStep, symEngineHasPendingRequests,
		symEngineAddRequest, symEngineRemoveRequest,
		symCreateRequest, symDestroyRequest,
		symRequestAddTokens, symRequestSetOpaqueData,
		symRequestGetOpaqueData, symRequestHasUnseenTokens,
		symRequestGetUnseenToken, symRequestIsDone)
	if rc != 0 {
		return
	}
	engineAPIAvailable = true
}
