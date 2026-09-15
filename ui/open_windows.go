//go:build windows

package ui

import "os/exec"

// Open inicia la interfaz en el navegador predeterminado de Windows.
func Open(url string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}
