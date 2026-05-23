# hub package
Downloads HuggingFace Hub files, a port of huggingFace_hub python library to Go. 

## Introduction

A simple, straight-forward port of [github.com/huggingface/huggingface_hub](https://github.com/huggingface/huggingface_hub) library for Go.

Features supported:

- Cache system that matches HuggingFace Hub, so the same cache can be shared with Python.
- Concurrency safe: only one download when multiple workers are trying to download simultaneously the same model.
- Allow arbitrary progress function to be called (for progress bar).
- Arbitrary revision.
- Parallel download of files, max=20 by default.

TODOs:

- Add support for optional parameters.
- Authentication tokens: should be relatively easy.
- Resume downloads from interrupted connections.
- Check disk-space before starting to download.

## Example

Enumerate files from a HuggingFace repository and download all of them to a cache.

```go
	repo := hub.New(modelID).WithAuth(hfAuthToken)
	var fileNames []string
	for fileName, err := range repo.IterFileNames() {
		if err != nil { panic(err) }
		fmt.Printf("\t%s\n", fileName)
		fileNames = append(fileNames, fileName)
	}
	downloadedFiles, err := repo.DownloadFiles(fileNames...)
	if err != nil { ... }
```