package main

func check(e error) {
	if e != nil {
		panic(e)
	}
}

// Assert panics if condition is false.
func Assert(condition bool, message string) {
	if !condition {
		panic("Assertion failed: " + message)
	}
}
