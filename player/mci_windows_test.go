//go:build windows

package player

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Verifica la regresión que motivó el hilo dedicado: cada operación se lanza
// desde un goroutine distinto, como ocurre con peticiones HTTP separadas.
func TestMCICommandsAcrossGoroutines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "silence.wav")
	if err := os.WriteFile(path, silentWAV(time.Second), 0o600); err != nil {
		t.Fatal(err)
	}
	p := New()
	defer p.Close()

	runOnGoroutine(t, func() error { return p.Play(path) })
	time.Sleep(100 * time.Millisecond)
	runOnGoroutine(t, p.Pause)
	runOnGoroutine(t, p.Resume)
	runOnGoroutine(t, p.Stop)
}

func runOnGoroutine(t *testing.T, operation func() error) {
	t.Helper()
	result := make(chan error, 1)
	go func() { result <- operation() }()
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

func silentWAV(duration time.Duration) []byte {
	const sampleRate = 8000
	dataSize := int(duration.Seconds() * sampleRate * 2) // mono, PCM 16 bit
	data := make([]byte, 44+dataSize)
	copy(data[0:4], "RIFF")
	binary.LittleEndian.PutUint32(data[4:8], uint32(36+dataSize))
	copy(data[8:12], "WAVE")
	copy(data[12:16], "fmt ")
	binary.LittleEndian.PutUint32(data[16:20], 16)
	binary.LittleEndian.PutUint16(data[20:22], 1)
	binary.LittleEndian.PutUint16(data[22:24], 1)
	binary.LittleEndian.PutUint32(data[24:28], sampleRate)
	binary.LittleEndian.PutUint32(data[28:32], sampleRate*2)
	binary.LittleEndian.PutUint16(data[32:34], 2)
	binary.LittleEndian.PutUint16(data[34:36], 16)
	copy(data[36:40], "data")
	binary.LittleEndian.PutUint32(data[40:44], uint32(dataSize))
	return data
}
