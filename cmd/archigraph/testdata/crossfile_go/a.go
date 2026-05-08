package crossfile

// FileA defines a function used by file b.go in the same package.
func Hello() string {
	return "hi"
}
