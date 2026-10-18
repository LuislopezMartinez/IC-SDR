//go:build !windows

package vlcembed

import (
	"fmt"
	"unsafe"
)

type Player struct{}

func New() *Player           { return &Player{} }
func ResolveLibrary() string { return "" }
func (p *Player) Start(unsafe.Pointer, int, int, int, int, string) error {
	return fmt.Errorf("vídeo DATV embebido no disponible en esta plataforma")
}
func (p *Player) SetBounds(int, int, int, int, bool) {}
func (p *Player) Running() bool                      { return false }
func (p *Player) Stop()                              {}
func (p *Player) Close()                             {}
