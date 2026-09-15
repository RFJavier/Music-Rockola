// Package playback define el contrato multimedia independiente de plataforma.
package playback

import "time"

// MediaPlayer es el puerto que utilizará el Core. Ningún tipo de Windows se
// filtra por esta interfaz.
type MediaPlayer interface {
	Play(path string) error
	Pause() error
	Resume() error
	Stop() error
	SetVolume(volume float64) error
	IsPlaying() bool
	Position() time.Duration
	Duration() time.Duration
}
