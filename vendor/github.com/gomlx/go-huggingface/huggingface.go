// Package huggingface only holds the version of the set of tools to interact with HuggingFace using GoMLX.
//
// There are 3 main sub-packages:
//
//   - hub: to download files from HuggingFace Hub, be it model files, tokenizers, data, etc.
//   - tokenizers: to create tokenizers from downloaded HuggingFace models.
//   - models: to convert model weights from different formats to GoMLX.
package huggingface

// Version of the library.
// Manually kept in sync with project releases.
var Version = "v0.0.0-dev"
