//go:build !windows

package player

import (
	"errors"
	"time"
)

var errUnsupported = errors.New("el reproductor multimedia solo está disponible en Windows")

type MCIPlayer struct{}

func New() *MCIPlayer                        { return &MCIPlayer{} }
func (p *MCIPlayer) Play(string) error       { return errUnsupported }
func (p *MCIPlayer) Pause() error            { return errUnsupported }
func (p *MCIPlayer) Resume() error           { return errUnsupported }
func (p *MCIPlayer) Stop() error             { return errUnsupported }
func (p *MCIPlayer) SetVolume(float64) error { return errUnsupported }
func (p *MCIPlayer) IsPlaying() bool         { return false }
func (p *MCIPlayer) Position() time.Duration { return 0 }
func (p *MCIPlayer) Duration() time.Duration { return 0 }
func (p *MCIPlayer) Close() error            { return nil }
