[![GoDev](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/gomlx/exceptions?tab=doc)

# exceptions

Simple exceptions support for Go, using `panic()` and `recover()`.

Go's `panic(any)` will unwind the stack one frame at a time until there is a matching call to `recover()`.
This library calls the parameter passed to `panic` an _exception_, and provides simplified methods to _catch_ those exceptions. 

## Examples

### Catching any exception with `Try`

An exception in Go is what goes into the `panic()` call. The returned value (`exception` in the example) is of type `any`.

```go
import . "github.com/gomlx/exceptions"

...
	exception := Try(func() { panic("cookies") })
	if exception != nil {
		fmt.Printf("Caught: %v\n", exception)
	}
```

It will print: `Caught: cookies`.

### Throwing an exception with `Panicf`

We keep the Go word term for it, `panic`: one can simply use `panic` as usual,
or use the provided `Panicf`, which converts the given text into an error+stack:

```go
import . "github.com/gomlx/exceptions"

func MyRatio(myThing, total float64) float64 {
	if total == 0 {
		Panicf("MyRatio(%f, %f) has no things (0) to calculate a ratio from", myThing, total)
	}
	return myThing/total
}
```

### Catching Typed Exceptions with `TryCatch`

Go `panic()` supports any value to be passed -- the value is what we interpret as an exception.

The generic `TryCatch[E any]` API allows a function to be called and an exception of the given type `E` to be caught.
Any other exception types are not handled and continues to propagate up the stack as usual.

```go
import . "github.com/gomlx/exceptions"

	var x ResultType
	err := TryCatch[error](func() { x = DoSomething() })
	if err != nil {
		// Handle error ...
	}
```
