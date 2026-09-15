//go:build windows

// Package player implementa el puerto multimedia con APIs nativas de Windows.
package player

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"rockola/core/playback"
)

var (
	winmm             = windows.NewLazySystemDLL("winmm.dll")
	mciSendString     = winmm.NewProc("mciSendStringW")
	mciGetErrorString = winmm.NewProc("mciGetErrorStringW")
	waveOutSetVolume  = winmm.NewProc("waveOutSetVolume")
	playerSequence    atomic.Uint64
)

type mciState struct {
	alias  string
	opened bool
}

type mciResult struct {
	value any
	err   error
}

type mciRequest struct {
	operation func(*mciState) (any, error)
	result    chan mciResult
}

// MCIPlayer serializa todas las llamadas MCI en un único hilo del sistema.
// Los aliases MCI pueden dejar de resolverse si open/play/pause se ejecutan
// desde distintos hilos, algo habitual con handlers HTTP concurrentes de Go.
type MCIPlayer struct {
	mu       sync.Mutex
	requests chan mciRequest
	closed   bool
}

var _ playback.MediaPlayer = (*MCIPlayer)(nil)

func New() *MCIPlayer {
	p := &MCIPlayer{requests: make(chan mciRequest)}
	ready := make(chan struct{})
	alias := fmt.Sprintf("rockola_player_%d", playerSequence.Add(1))
	go p.run(alias, ready)
	<-ready
	return p
}

func (p *MCIPlayer) run(alias string, ready chan<- struct{}) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	state := &mciState{alias: alias}
	close(ready)
	for request := range p.requests {
		value, err := request.operation(state)
		request.result <- mciResult{value: value, err: err}
	}
}

func (p *MCIPlayer) Play(path string) error {
	_, err := p.do(func(state *mciState) (any, error) {
		if state.opened {
			_ = state.command("close " + state.alias)
			state.opened = false
		}
		if err := state.command(`open "` + path + `" alias ` + state.alias); err != nil {
			return nil, fmt.Errorf("abriendo archivo multimedia: %w", err)
		}
		state.opened = true
		if err := state.command("set " + state.alias + " time format milliseconds"); err != nil {
			state.close()
			return nil, err
		}
		if err := state.command("play " + state.alias); err != nil {
			state.close()
			return nil, fmt.Errorf("iniciando reproducción: %w", err)
		}
		return nil, nil
	})
	return err
}

func (p *MCIPlayer) Pause() error {
	_, err := p.do(func(state *mciState) (any, error) {
		return nil, state.openCommand("pause " + state.alias)
	})
	return err
}

func (p *MCIPlayer) Resume() error {
	_, err := p.do(func(state *mciState) (any, error) {
		return nil, state.openCommand("resume " + state.alias)
	})
	return err
}

func (p *MCIPlayer) Stop() error {
	_, err := p.do(func(state *mciState) (any, error) {
		if !state.opened {
			return nil, nil
		}
		if err := state.command("stop " + state.alias); err != nil {
			return nil, err
		}
		return nil, state.command("seek " + state.alias + " to start")
	})
	return err
}

func (p *MCIPlayer) SetVolume(volume float64) error {
	if volume < 0 || volume > 1 {
		return fmt.Errorf("el volumen debe estar entre 0 y 1")
	}
	_, err := p.do(func(_ *mciState) (any, error) {
		level := uint32(volume * 65535)
		stereoLevel := uintptr(level | level<<16)
		code, _, _ := waveOutSetVolume.Call(uintptr(0xffffffff), stereoLevel)
		if code != 0 {
			return nil, fmt.Errorf("error waveOutSetVolume %d", code)
		}
		return nil, nil
	})
	return err
}

func (p *MCIPlayer) IsPlaying() bool {
	value, err := p.do(func(state *mciState) (any, error) {
		if !state.opened {
			return false, nil
		}
		mode, err := state.query("status " + state.alias + " mode")
		return strings.EqualFold(mode, "playing"), err
	})
	return err == nil && value.(bool)
}

func (p *MCIPlayer) Position() time.Duration {
	return p.durationStatus("position")
}

func (p *MCIPlayer) Duration() time.Duration {
	return p.durationStatus("length")
}

// Close libera el dispositivo y termina el hilo dedicado.
func (p *MCIPlayer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	resultCh := make(chan mciResult, 1)
	p.requests <- mciRequest{
		operation: func(state *mciState) (any, error) { return nil, state.close() },
		result:    resultCh,
	}
	result := <-resultCh
	p.closed = true
	close(p.requests)
	return result.err
}

func (p *MCIPlayer) durationStatus(field string) time.Duration {
	value, err := p.do(func(state *mciState) (any, error) {
		if !state.opened {
			return time.Duration(0), nil
		}
		result, err := state.query("status " + state.alias + " " + field)
		if err != nil {
			return time.Duration(0), err
		}
		millis, err := strconv.ParseInt(strings.TrimSpace(result), 10, 64)
		if err != nil {
			return time.Duration(0), err
		}
		return time.Duration(millis) * time.Millisecond, nil
	})
	if err != nil {
		return 0
	}
	return value.(time.Duration)
}

// do mantiene el canal abierto durante toda la operación y serializa también
// Close, evitando envíos sobre un canal cerrado durante el apagado.
func (p *MCIPlayer) do(operation func(*mciState) (any, error)) (any, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil, fmt.Errorf("el reproductor está cerrado")
	}
	resultCh := make(chan mciResult, 1)
	p.requests <- mciRequest{operation: operation, result: resultCh}
	result := <-resultCh
	return result.value, result.err
}

func (state *mciState) close() error {
	if !state.opened {
		return nil
	}
	err := state.command("close " + state.alias)
	state.opened = false
	return err
}

func (state *mciState) openCommand(command string) error {
	if !state.opened {
		return fmt.Errorf("no hay un archivo multimedia abierto")
	}
	return state.command(command)
}

func (state *mciState) command(command string) error {
	_, err := sendMCI(command, 0)
	return err
}

func (state *mciState) query(command string) (string, error) {
	return sendMCI(command, 256)
}

func sendMCI(command string, resultSize int) (string, error) {
	commandPtr, err := windows.UTF16PtrFromString(command)
	if err != nil {
		return "", fmt.Errorf("comando MCI inválido: %w", err)
	}

	var result []uint16
	var resultPtr uintptr
	if resultSize > 0 {
		result = make([]uint16, resultSize)
		resultPtr = uintptr(unsafe.Pointer(&result[0]))
	}
	code, _, _ := mciSendString.Call(
		uintptr(unsafe.Pointer(commandPtr)), resultPtr, uintptr(resultSize), 0)
	if code != 0 {
		return "", mciError(uint32(code))
	}
	if resultSize == 0 {
		return "", nil
	}
	return windows.UTF16ToString(result), nil
}

func mciError(code uint32) error {
	buffer := make([]uint16, 256)
	ok, _, _ := mciGetErrorString.Call(
		uintptr(code), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if ok == 0 {
		return fmt.Errorf("error MCI %d", code)
	}
	return fmt.Errorf("error MCI %d: %s", code, windows.UTF16ToString(buffer))
}
