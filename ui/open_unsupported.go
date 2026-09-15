//go:build !windows

package ui

// Open no hace nada fuera de Windows; el servidor sigue disponible.
func Open(string) error { return nil }
