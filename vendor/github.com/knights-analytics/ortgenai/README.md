# ortgenai

Go bindings for the ONNX Runtime GenAI C API.

This package provides a thin, idiomatic Go wrapper around the ONNX Runtime GenAI shared library, exposing:

- Session and model creation from local model folders
- Tokenization and chat templating
- Batched, streaming text generation with per-token deltas
- Runtime statistics (tokens/sec, prefill timings)
- Provider selection and advanced provider options
- Multimodal input support (text + images)

Note: The current implementation loads the GenAI shared library via `dlopen`, and targets Linux (ELF `.so`).

Note: This implementation is still alpha so the API may change in future releases. You might want to rely on [hugot](https://github.com/knights-analytics/hugot) for a higher-level interface to ONNX Runtime GenAI in Go.

## Contents

- Requirements
- Installation
- Quick start
- Advanced usage
- Running tests
- Docker and containerized tests
- Troubleshooting
- License


## Requirements

- Go 1.19+
- Linux with glibc (uses `dlfcn.h` and `.so` loading)
- ONNX Runtime GenAI shared library and dependencies available at runtime:
  - `libonnxruntime-genai.so`
  - `libonnxruntime.so` (must be available in the same directory as `libonnxruntime-genai.so`)
- A local model directory compatible with ONNX Runtime GenAI (e.g., a converted `Phi-3.5` model folder)


## Installation

```bash
go get github.com/knights-analytics/ortgenai
```

At runtime, the wrapper needs to `dlopen` the ONNX Runtime GenAI shared library. You can:

1) Place `libonnxruntime-genai.so` next to your application binary (with `libonnxruntime.so` in the same folder), or
2) Call `genai.SetSharedLibraryPath("/path/to/libonnxruntime-genai.so")` before initialization.


## Quick start

```go
package main

import (
    "context"
    "fmt"
    "time"

    genai "github.com/knights-analytics/ortgenai"
)

func main() {
    // Optional: set explicit path if the .so isn't on the default loader path
    // Note: ensure libonnxruntime.so is colocated with libonnxruntime-genai.so
    genai.SetSharedLibraryPath("/usr/lib/libonnxruntime-genai.so")

    if err := genai.InitializeEnvironment(); err != nil {
        panic(fmt.Errorf("init failed: %w", err))
    }
    defer func() {
        err = genai.DestroyEnvironment()
        if err != nil {
            panic(fmt.Errorf("destroy environment: %w", err))
        }
    }()

    // Point to a local model folder compatible with ONNX Runtime GenAI
    session, err := genai.CreateSession("./models/phi3.5")
    if err != nil {
        panic(fmt.Errorf("create session: %w", err))
    }
    defer session.Destroy()

    // Prepare one or more conversations (batched)
    conv1 := []genai.Message{
        {Role: "system", Content: "You are a helpful assistant."},
        {Role: "user", Content: "What is the capital of France?"},
    }

    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
    defer cancel()

    deltas, errs, err := session.Generate(ctx, [][]genai.Message{conv1})
    if err != nil {
        panic(fmt.Errorf("generate: %w", err))
    }

    // Stream tokens as they arrive
    for {
        select {
        case d, ok := <-deltas:
            if !ok { // stream completed
                stats := session.GetStatistics()
                fmt.Printf("\nTokens/sec: %.2f\n", stats.TokensPerSecond)
                return
            }
            fmt.Print(d.Tokens) // append to your buffer
        case e := <-errs:
            if e != nil { panic(e) }
        }
    }
}
```


## Advanced usage

### Explicit shared library path

```go
genai.SetSharedLibraryPath("/opt/onnxruntime/lib/libonnxruntime-genai.so")
if err := genai.InitializeEnvironment(); err != nil { /* handle */ }
```

If not set, the code tries `libonnxruntime-genai.so` relative to the loader’s search path. Ensure `libonnxruntime.so` is colocated with the GenAI `.so`.

### Provider selection and options

If you have a JSON config path and custom providers to use (e.g., CUDA, CPU) you can create a session with advanced settings:

```go
providers := []string{"cpu"} // or e.g., []string{"cuda", "cpu"}
providerOptions := map[string]map[string]string{
    "cpu": {"intra_op_num_threads": "4"},
}

session, err := genai.CreateSessionWithOptions(
    "./path/to/config.json", // model/session config from GenAI tooling
    providers,
    providerOptions,
)
```

### Batched generation

`Session.Generate` accepts multiple conversations in one call: `[][]Message`. The returned channel carries `SequenceDelta` items, each labeled with the `Sequence` index so you can route output per-conversation.

### Statistics

After generation, inspect `session.GetStatistics()` for fields such as `TokensPerSecond`, cumulative token counts, and prefill timings.


## Running tests

Local tests require the GenAI shared libraries and a local model directory. The provided unit test expects:

- `libonnxruntime-genai.so` available (by default at `/usr/lib/libonnxruntime-genai.so` in the test; adjust via `SetSharedLibraryPath`), and
- a model directory at `./_models/phi3.5` (update the path as needed).

Run:

```bash
go test ./...
```


## Docker and containerized tests

Two Dockerfiles are provided:

- `Dockerfile` — base image with dependencies
- `test.Dockerfile` — image to run unit tests

Helper scripts are available in `scripts/`:

- `scripts/run-unit-tests-container.sh` — build the test image and run tests in a container
- `scripts/run-unit-test.sh` — run tests directly (expects environment to be prepared)

You may also use `compose-test.yaml` to orchestrate test runs.


## Troubleshooting

- error loading GenAI shared library: Ensure the path to `libonnxruntime-genai.so` is correct and readable by the process. Set it explicitly with `SetSharedLibraryPath`.
- missing `Oga...` symbols or "missing Oga..." errors: The GenAI `.so` must export required symbols (e.g., `OgaCreateModel`). Make sure versions of `libonnxruntime-genai.so` and `libonnxruntime.so` are compatible and colocated.
- segmentation fault on load: Verify that your system’s CUDA/CPU provider dependencies match the `.so` build (driver/runtime versions).
- no output / stuck: Ensure your model folder is valid for ONNX Runtime GenAI and accessible; increase timeouts during first-run warmup.


## License

This project is licensed under the terms of the MIT License. See `LICENSE` for details.
