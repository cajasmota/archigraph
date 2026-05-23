package ortgenai

/*
#cgo CFLAGS: -O2 -g
#include "ort_genai_wrapper.h"
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

var ErrEngineAPINotAvailable = errors.New("OgaEngine API not available in loaded ORT GenAI library (requires >= 0.9.1)")

// Engine provides continuous batching via the OgaEngine C API.
// Multiple goroutines may call Submit concurrently; the engine batches
// their requests for efficient inference.
type Engine struct {
	enginePtr  *C.OgaEngine
	model      *model
	tokenizer  *tokenizer
	statistics *Statistics

	mu       sync.Mutex
	requests map[*C.OgaRequest]*engineRequest

	submitCh chan *engineRequest
	stopCh   chan struct{}
	stopped  atomic.Bool
	stopOnce sync.Once
	submitMu sync.RWMutex // guards stopped check + submitCh send atomicity
	wg       sync.WaitGroup
}

// engineRequest is the internal state for one in-flight generation request.
type engineRequest struct {
	requestPtr      *C.OgaRequest
	paramsPtr       *C.OgaGeneratorParams
	tokenizerStream *tokenizerStream
	outputChan      chan SequenceDelta
	errChan         chan error
	ctx             context.Context
	firstToken      bool
	eosReached      bool
	runStart        time.Time
	tokenCount      int
}

// CreateEngine creates a new Engine from the given model path.
// The engine starts a background loop that processes submitted requests.
func CreateEngine(modelPath string) (*Engine, error) {
	if !IsInitialized() {
		return nil, ErrNotInitialized
	}
	if !IsEngineAPIAvailable() {
		return nil, ErrEngineAPINotAvailable
	}
	if modelPath == "" {
		return nil, errors.New("modelPath is empty")
	}

	cPath := C.CString(modelPath)
	defer C.free(unsafe.Pointer(cPath))

	var cModel *C.OgaModel
	res := C.CreateOgaModel(cPath, &cModel)
	if err := OgaResultToError(res); err != nil {
		return nil, fmt.Errorf("CreateOgaModel failed: %w", err)
	}
	if cModel == nil {
		return nil, errors.New("CreateOgaModel returned nil model without error")
	}

	m := &model{modelPtr: cModel}
	return newEngine(m)
}

// CreateEngineWithOptions creates a new Engine with explicit execution provider configuration.
func CreateEngineWithOptions(configDirectoryPath string, providers []string, providerOptions map[string]map[string]string) (*Engine, error) {
	if !IsInitialized() {
		return nil, ErrNotInitialized
	}
	if !IsEngineAPIAvailable() {
		return nil, ErrEngineAPINotAvailable
	}

	var cfg *C.OgaConfig
	cConfigPath := C.CString(configDirectoryPath)
	defer C.free(unsafe.Pointer(cConfigPath))
	res := C.CreateOgaConfig(cConfigPath, &cfg)
	if err := OgaResultToError(res); err != nil {
		return nil, fmt.Errorf("CreateOgaConfig failed: %w", err)
	}
	if cfg == nil {
		return nil, errors.New("CreateOgaConfig returned nil without error")
	}
	defer C.DestroyOgaConfig(cfg)

	res = C.OgaConfigClearProviders(cfg)
	if err := OgaResultToError(res); err != nil {
		return nil, fmt.Errorf("OgaConfigClearProviders failed: %w", err)
	}

	for _, providerName := range providers {
		cp := C.CString(providerName)
		res = C.OgaConfigAppendProvider(cfg, cp)
		if err := OgaResultToError(res); err != nil {
			C.free(unsafe.Pointer(cp))
			return nil, fmt.Errorf("OgaConfigAppendProvider(%s) failed: %w", providerName, err)
		}
		if opts, ok := providerOptions[providerName]; ok {
			for k, v := range opts {
				ck := C.CString(k)
				cv := C.CString(v)
				res = C.OgaConfigSetProviderOption(cfg, cp, ck, cv)
				C.free(unsafe.Pointer(ck))
				C.free(unsafe.Pointer(cv))
				if err := OgaResultToError(res); err != nil {
					C.free(unsafe.Pointer(cp))
					return nil, fmt.Errorf("OgaConfigSetProviderOption(%s,%s=%s) failed: %w", providerName, k, v, err)
				}
			}
		}
		C.free(unsafe.Pointer(cp))
	}

	var cModel *C.OgaModel
	res = C.CreateOgaModelFromConfig(cfg, &cModel)
	if err := OgaResultToError(res); err != nil {
		return nil, fmt.Errorf("CreateOgaModelFromConfig failed: %w", err)
	}
	if cModel == nil {
		return nil, errors.New("CreateOgaModelFromConfig returned nil without error")
	}

	m := &model{modelPtr: cModel}
	return newEngine(m)
}

func newEngine(m *model) (*Engine, error) {
	tok, err := newTokenizerFromModel(*m)
	if err != nil {
		m.destroy()
		return nil, fmt.Errorf("newTokenizerFromModel failed: %w", err)
	}

	var cEngine *C.OgaEngine
	res := C.CreateOgaEngine(m.modelPtr, &cEngine)
	if err := OgaResultToError(res); err != nil {
		tok.destroy()
		m.destroy()
		return nil, fmt.Errorf("CreateOgaEngine failed: %w", err)
	}
	if cEngine == nil {
		tok.destroy()
		m.destroy()
		return nil, errors.New("CreateOgaEngine returned nil without error")
	}

	e := &Engine{
		enginePtr:  cEngine,
		model:      m,
		tokenizer:  &tok,
		statistics: &Statistics{},
		requests:   make(map[*C.OgaRequest]*engineRequest),
		submitCh:   make(chan *engineRequest, 256),
		stopCh:     make(chan struct{}),
	}
	e.wg.Add(1)
	go e.runStepLoop()
	return e, nil
}

// GetStatistics returns generation performance metrics for the engine.
func (e *Engine) GetStatistics() *Statistics {
	return e.statistics
}

// Submit submits a generation request to the engine and returns channels for
// streaming output. Multiple goroutines may call Submit concurrently.
//
// Callers should pass a context with a deadline or timeout. If more than 256
// goroutines submit concurrently without deadlines, Stop may block until the
// excess callers' contexts are cancelled.
func (e *Engine) Submit(ctx context.Context, messages []Message, tools []string, opts *GenerationOptions) (<-chan SequenceDelta, <-chan error, error) {
	// Tokenize
	seqs, streams, err := e.tokenizer.tokenizeMessages([][]Message{messages}, tools)
	if err != nil {
		return nil, nil, fmt.Errorf("tokenize failed: %w", err)
	}
	if len(streams) != 1 {
		tokenizeCleanup(seqs, streams)
		return nil, nil, errors.New("expected exactly one tokenizer stream")
	}

	// Copy opts to avoid mutating the caller's struct across concurrent Submit calls.
	var localOpts GenerationOptions
	if opts != nil {
		localOpts = *opts
	}

	if localOpts.MaxLength <= 0 {
		localOpts.MaxLength = defaultMaxLength
	}
	if localOpts.BatchSize <= 0 {
		localOpts.BatchSize = 1
	}
	setDefaultGenerationOptions(&localOpts)

	paramsPtr, err := createGeneratorParams(e.model, &localOpts)
	if err != nil {
		tokenizeCleanup(seqs, streams)
		return nil, nil, fmt.Errorf("createGeneratorParams failed: %w", err)
	}

	// Create request
	var cRequest *C.OgaRequest
	res := C.CreateOgaRequest(paramsPtr, &cRequest)
	if err = OgaResultToError(res); err != nil {
		C.DestroyOgaGeneratorParams(paramsPtr)
		tokenizeCleanup(seqs, streams)
		return nil, nil, fmt.Errorf("CreateOgaRequest failed: %w", err)
	}
	if cRequest == nil {
		C.DestroyOgaGeneratorParams(paramsPtr)
		tokenizeCleanup(seqs, streams)
		return nil, nil, errors.New("CreateOgaRequest returned nil without error")
	}

	// Add tokens to request
	res = C.RequestAddTokens(cRequest, seqs.sequencesPtr)
	if err = OgaResultToError(res); err != nil {
		C.DestroyOgaRequest(cRequest)
		C.DestroyOgaGeneratorParams(paramsPtr)
		tokenizeCleanup(seqs, streams)
		return nil, nil, fmt.Errorf("RequestAddTokens failed: %w", err)
	}
	// Sequences consumed by request; prevent double-free.
	C.DestroyOgaSequences(seqs.sequencesPtr)
	seqs.sequencesPtr = nil

	outputChan := make(chan SequenceDelta, 1000)
	errChan := make(chan error, 1)

	req := &engineRequest{
		requestPtr:      cRequest,
		paramsPtr:       paramsPtr,
		tokenizerStream: streams[0],
		outputChan:      outputChan,
		errChan:         errChan,
		ctx:             ctx,
		runStart:        time.Now(),
	}

	// Hold the read-lock so that the stopped check + send is atomic with
	// respect to Stop's write-lock + final drain. This prevents a request
	// from being queued after Stop has finished draining.
	e.submitMu.RLock()
	if e.stopped.Load() {
		e.submitMu.RUnlock()
		req.destroy()
		return nil, nil, errors.New("engine is stopped")
	}
	select {
	case e.submitCh <- req:
	case <-ctx.Done():
		e.submitMu.RUnlock()
		req.destroy()
		return nil, nil, ctx.Err()
	}
	e.submitMu.RUnlock()

	return outputChan, errChan, nil
}

func (e *Engine) Generate(ctx context.Context, messages [][]Message, tools []string, opts *GenerationOptions) (<-chan SequenceDelta, <-chan error, error) {
	if len(messages) == 0 {
		return nil, nil, errors.New("no messages provided")
	}

	outputChan := make(chan SequenceDelta, 1000)
	errChan := make(chan error, len(messages))
	var wg sync.WaitGroup

	for idx, message := range messages {
		wg.Go(
			func() {
				out, errs, err := e.Submit(ctx, message, tools, opts)
				if err != nil {
					errChan <- fmt.Errorf("sequence %d: %w", idx, err)
					return
				}
				for delta := range out {
					delta.Sequence = idx
					outputChan <- delta
				}
				for err = range errs {
					if err != nil {
						errChan <- fmt.Errorf("sequence %d: %w", idx, err)
					}
				}
			})
	}

	go func() {
		wg.Wait()
		close(outputChan)
		close(errChan)
	}()

	return outputChan, errChan, nil
}

func (e *Engine) Stop() {
	e.stopOnce.Do(func() {
		e.stopped.Store(true)
		close(e.stopCh)
	})
	e.wg.Wait()
	// Acquire the write-lock so no Submit can be in-flight, then drain any
	// stragglers that were queued before the lock was acquired.
	e.submitMu.Lock()
	for {
		select {
		case req := <-e.submitCh:
			sendGenerationError(req.errChan, errors.New("engine stopped"))
			close(req.outputChan)
			close(req.errChan)
			req.destroy()
		default:
			e.submitMu.Unlock()
			return
		}
	}
}

// Destroy stops the engine and releases all resources.
// It must not be called concurrently with Submit or other Engine methods.
func (e *Engine) Destroy() {
	e.Stop()
	if e.enginePtr != nil {
		C.DestroyOgaEngine(e.enginePtr)
		e.enginePtr = nil
	}
	if e.tokenizer != nil {
		e.tokenizer.destroy()
		e.tokenizer = nil
	}
	if e.model != nil {
		e.model.destroy()
		e.model = nil
	}
}

func (e *Engine) runStepLoop() {
	defer e.wg.Done()
	// If the loop exits unexpectedly (not via Stop), mark the engine as
	// stopped so that subsequent Submit calls fail fast instead of hanging.
	defer func() {
		e.stopOnce.Do(func() {
			e.stopped.Store(true)
			close(e.stopCh)
		})
	}()

	for {
		// Drain all pending submissions first.
		drained := false
		for !drained {
			select {
			case req := <-e.submitCh:
				e.addRequest(req)
			default:
				drained = true
			}
		}

		// Check stop signal.
		select {
		case <-e.stopCh:
			e.drainOnStop()
			return
		default:
		}

		// Check if there are pending requests.
		hasPending, err := e.hasPendingRequests()
		if err != nil {
			// Fatal engine error; drain and exit.
			e.drainOnStop()
			return
		}
		if !hasPending {
			// No work — wait for a submission or stop.
			select {
			case req := <-e.submitCh:
				e.addRequest(req)
			case <-e.stopCh:
				e.drainOnStop()
				return
			}
			continue
		}

		// Run one engine step.
		var cReady *C.OgaRequest
		res := C.EngineStep(e.enginePtr, &cReady)
		if err = OgaResultToError(res); err != nil {
			// TODO: do not rely on error/recover here, but does not work if moved to hasPendingRequests
			if err.Error() == "Expected at least one request to be ready, but none were found." {
				e.checkDoneRequests()
				continue
			}
			// EngineStep failed — error and remove all active requests, but
			// keep the engine loop running. The failure is request-level (e.g.
			// model couldn't process the input) not engine-level.
			e.errorAndRemoveAllRequests(fmt.Errorf("EngineStep failed: %w", err))
			continue
		}

		if cReady != nil {
			e.processReadyRequest(cReady)
		} else {
			// EngineStep may be non-blocking; yield to avoid a tight CPU spin
			// while waiting for the next token to become ready.
			runtime.Gosched()
		}

		// Check for cancelled contexts.
		e.checkCancellations()
	}
}

func (e *Engine) addRequest(req *engineRequest) {
	res := C.EngineAddRequest(e.enginePtr, req.requestPtr)
	if err := OgaResultToError(res); err != nil {
		sendGenerationError(req.errChan, fmt.Errorf("EngineAddRequest failed: %w", err))
		close(req.outputChan)
		close(req.errChan)
		req.destroy()
		return
	}
	e.mu.Lock()
	e.requests[req.requestPtr] = req
	e.mu.Unlock()
}

func (e *Engine) processReadyRequest(cReady *C.OgaRequest) {
	e.mu.Lock()
	req, ok := e.requests[cReady]
	e.mu.Unlock()
	if !ok {
		return
	}

	// Drain all unseen tokens.
	var tokenErr error
	for {
		var hasTokens C.bool
		res := C.RequestHasUnseenTokens(cReady, &hasTokens)
		if err := OgaResultToError(res); err != nil {
			tokenErr = fmt.Errorf("RequestHasUnseenTokens failed: %w", err)
			break
		}
		if !bool(hasTokens) {
			break
		}

		var token C.int32_t
		res = C.RequestGetUnseenToken(cReady, &token)
		if err := OgaResultToError(res); err != nil {
			tokenErr = fmt.Errorf("RequestGetUnseenToken failed: %w", err)
			break
		}

		// Check for EOS.
		if slices.Contains(e.tokenizer.EOSTokenIDs, int(token)) {
			if !req.eosReached {
				select {
				case req.outputChan <- SequenceDelta{Sequence: 0, EOSReached: true}:
					req.eosReached = true
				case <-req.ctx.Done():
					// Context cancelled; stop processing. checkCancellations will clean up.
					return
				}
			}
			continue
		}

		// Suppress non-EOS tokens after EOS has been emitted.
		if req.eosReached {
			continue
		}

		decoded, decErr := req.tokenizerStream.Decode(token)
		if decErr != nil {
			tokenErr = decErr
			break
		}
		if decoded == "" {
			continue
		}

		// Leading-space normalization for first emitted token.
		if !req.firstToken {
			trimmed := strings.TrimLeft(decoded, " ")
			if trimmed == "" {
				continue
			}
			decoded = trimmed
			req.firstToken = true

			prefill := time.Since(req.runStart).Seconds()
			e.mu.Lock()
			e.statistics.CumulativePrefillSum += prefill
			e.statistics.CumulativePrefillCount++
			e.statistics.AvgPrefillSeconds = e.statistics.CumulativePrefillSum / float64(e.statistics.CumulativePrefillCount)
			e.mu.Unlock()
		}

		e.mu.Lock()
		e.statistics.CumulativeTokens++
		e.mu.Unlock()
		req.tokenCount++

		select {
		case req.outputChan <- SequenceDelta{Sequence: 0, Token: decoded}:
		case <-req.ctx.Done():
			return
		}
	}

	// On any token-drain error, finish the request immediately so channels are closed.
	if tokenErr != nil {
		sendGenerationError(req.errChan, tokenErr)
		e.finishRequest(cReady, req)
		return
	}

	// Check if request is done.
	var isDone C.bool
	res := C.RequestIsDone(cReady, &isDone)
	if err := OgaResultToError(res); err != nil {
		sendGenerationError(req.errChan, fmt.Errorf("RequestIsDone failed: %w", err))
		e.finishRequest(cReady, req)
		return
	}
	if bool(isDone) {
		e.finishRequest(cReady, req)
	}
}

func (e *Engine) finishRequest(cReady *C.OgaRequest, req *engineRequest) {
	e.mu.Lock()
	delete(e.requests, cReady)
	if req.tokenCount > 0 {
		dur := time.Since(req.runStart).Seconds()
		if dur > 0 {
			e.statistics.CumulativeTokenDurationSeconds += dur
			e.statistics.TokensPerSecond = float64(e.statistics.CumulativeTokens) / e.statistics.CumulativeTokenDurationSeconds
		}
	}
	e.mu.Unlock()

	res := C.EngineRemoveRequest(e.enginePtr, cReady)
	if err := OgaResultToError(res); err != nil {
		sendGenerationError(req.errChan, fmt.Errorf("EngineRemoveRequest failed: %w", err))
	}
	close(req.outputChan)
	close(req.errChan)
	req.destroy()
}

// errorAndRemoveAllRequests sends err to every active request's error channel,
// removes each request from the C engine, and closes their channels.
// The engine loop continues running after this call.
func (e *Engine) errorAndRemoveAllRequests(err error) {
	e.mu.Lock()
	active := make(map[*C.OgaRequest]*engineRequest, len(e.requests))
	for k, v := range e.requests {
		active[k] = v
	}
	e.requests = make(map[*C.OgaRequest]*engineRequest)
	e.mu.Unlock()

	for ptr, req := range active {
		sendGenerationError(req.errChan, err)
		C.EngineRemoveRequest(e.enginePtr, ptr)
		close(req.outputChan)
		close(req.errChan)
		req.destroy()
	}
}

func (e *Engine) checkCancellations() {
	e.mu.Lock()
	var cancelled []*C.OgaRequest
	for ptr, req := range e.requests {
		select {
		case <-req.ctx.Done():
			cancelled = append(cancelled, ptr)
		default:
		}
	}
	e.mu.Unlock()

	for _, ptr := range cancelled {
		e.mu.Lock()
		req, ok := e.requests[ptr]
		if ok {
			delete(e.requests, ptr)
		}
		e.mu.Unlock()
		if !ok {
			continue
		}

		// Send the caller's cancellation reason first (this is the error they care about).
		sendGenerationError(req.errChan, req.ctx.Err())
		C.EngineRemoveRequest(e.enginePtr, ptr)
		close(req.outputChan)
		close(req.errChan)
		req.destroy()
	}
}

func (e *Engine) checkDoneRequests() {
	e.mu.Lock()
	active := make([]*C.OgaRequest, 0, len(e.requests))
	for ptr := range e.requests {
		active = append(active, ptr)
	}
	e.mu.Unlock()

	for _, ptr := range active {
		var isDone C.bool
		res := C.RequestIsDone(ptr, &isDone)
		if err := OgaResultToError(res); err != nil {
			continue
		}
		if bool(isDone) {
			e.mu.Lock()
			req, ok := e.requests[ptr]
			e.mu.Unlock()
			if ok {
				e.finishRequest(ptr, req)
			}
		}
	}
}

func (e *Engine) hasPendingRequests() (bool, error) {
	var pending C.bool
	res := C.EngineHasPendingRequests(e.enginePtr, &pending)
	if err := OgaResultToError(res); err != nil {
		return false, err
	}
	return bool(pending), nil
}

func (e *Engine) drainOnStop() {
	// Drain submit channel.
	for {
		select {
		case req := <-e.submitCh:
			sendGenerationError(req.errChan, errors.New("engine stopped"))
			close(req.outputChan)
			close(req.errChan)
			req.destroy()
		default:
			goto done
		}
	}
done:
	// Close all active requests.
	e.mu.Lock()
	active := make(map[*C.OgaRequest]*engineRequest, len(e.requests))
	for k, v := range e.requests {
		active[k] = v
	}
	e.requests = make(map[*C.OgaRequest]*engineRequest)
	e.mu.Unlock()

	for ptr, req := range active {
		sendGenerationError(req.errChan, errors.New("engine stopped"))
		C.EngineRemoveRequest(e.enginePtr, ptr)
		close(req.outputChan)
		close(req.errChan)
		req.destroy()
	}
}

func (r *engineRequest) destroy() {
	if r.tokenizerStream != nil {
		r.tokenizerStream.destroy()
		r.tokenizerStream = nil
	}
	if r.requestPtr != nil {
		C.DestroyOgaRequest(r.requestPtr)
		r.requestPtr = nil
	}
	if r.paramsPtr != nil {
		C.DestroyOgaGeneratorParams(r.paramsPtr)
		r.paramsPtr = nil
	}
}
