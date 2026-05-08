package crossfile

// FileB calls into a.go's Hello function.
func Greet() string {
	return Hello()
}
