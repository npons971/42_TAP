//go:build !linux

package cli

import "os"

// Other platforms retain the portable line-mode client.
func RawTerminal(file *os.File) (func(), bool, error) { return func() {}, false, nil }

func InputFile(file *os.File) (*os.File, error) { return file, nil }
