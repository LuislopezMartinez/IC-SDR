//go:build windows

package vlcembed

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wsChild        = 0x40000000
	wsVisible      = 0x10000000
	wsClipSiblings = 0x04000000
	swHide         = 0
	swShow         = 5
)

var (
	user32              = windows.NewLazySystemDLL("user32.dll")
	procCreateWindowExW = user32.NewProc("CreateWindowExW")
	procMoveWindow      = user32.NewProc("MoveWindow")
	procShowWindow      = user32.NewProc("ShowWindow")
	procDestroyWindow   = user32.NewProc("DestroyWindow")
)

type Player struct {
	mu                           sync.Mutex
	dll                          *windows.DLL
	instance, mediaPlayer, child uintptr
	playing, visible             bool
}

func New() *Player { return &Player{} }

func ResolveLibrary() string {
	for _, candidate := range []string{
		filepath.Join("DATA", "tools", "datv", "runtime", "vlc", "libvlc.dll"),
		`C:\Program Files\VideoLAN\VLC\libvlc.dll`,
		`C:\Program Files (x86)\VideoLAN\VLC\libvlc.dll`,
	} {
		if absolute, err := filepath.Abs(candidate); err == nil {
			if info, statErr := os.Stat(absolute); statErr == nil && !info.IsDir() {
				return absolute
			}
		}
	}
	return ""
}

func (p *Player) Start(parent unsafe.Pointer, x, y, width, height int, url string) error {
	p.Stop()
	p.mu.Lock()
	defer p.mu.Unlock()
	library := ResolveLibrary()
	if library == "" {
		return fmt.Errorf("libVLC no está instalado")
	}
	handle, err := windows.LoadLibraryEx(library, 0, windows.LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR|windows.LOAD_LIBRARY_SEARCH_DEFAULT_DIRS)
	if err != nil {
		return err
	}
	dll := &windows.DLL{Name: library, Handle: handle}
	find := func(name string) (*windows.Proc, error) { return dll.FindProc(name) }
	newProc, err := find("libvlc_new")
	if err != nil {
		_ = dll.Release()
		return err
	}
	_ = os.Setenv("VLC_PLUGIN_PATH", filepath.Join(filepath.Dir(library), "plugins"))
	arg0, _ := windows.BytePtrFromString("--quiet")
	argv := []*byte{arg0}
	instance, _, callErr := newProc.Call(1, uintptr(unsafe.Pointer(&argv[0])))
	if instance == 0 {
		_ = dll.Release()
		return fmt.Errorf("libVLC no pudo iniciarse: %v", callErr)
	}
	class, _ := windows.UTF16PtrFromString("STATIC")
	child, _, winErr := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(class)), 0, wsChild|wsVisible|wsClipSiblings, uintptr(x), uintptr(y), uintptr(width), uintptr(height), uintptr(parent), 0, 0, 0)
	if child == 0 {
		release, _ := find("libvlc_release")
		release.Call(instance)
		_ = dll.Release()
		return fmt.Errorf("no se pudo crear el marco de vídeo: %v", winErr)
	}
	mediaNew, _ := find("libvlc_media_new_location")
	location, _ := windows.BytePtrFromString(url)
	media, _, _ := mediaNew.Call(instance, uintptr(unsafe.Pointer(location)))
	if media == 0 {
		procDestroyWindow.Call(child)
		release, _ := find("libvlc_release")
		release.Call(instance)
		_ = dll.Release()
		return fmt.Errorf("libVLC no pudo abrir %s", url)
	}
	playerNew, _ := find("libvlc_media_player_new_from_media")
	mp, _, _ := playerNew.Call(media)
	mediaRelease, _ := find("libvlc_media_release")
	mediaRelease.Call(media)
	if mp == 0 {
		procDestroyWindow.Call(child)
		release, _ := find("libvlc_release")
		release.Call(instance)
		_ = dll.Release()
		return fmt.Errorf("libVLC no pudo crear el reproductor")
	}
	setHWND, _ := find("libvlc_media_player_set_hwnd")
	setHWND.Call(mp, child)
	play, _ := find("libvlc_media_player_play")
	result, _, _ := play.Call(mp)
	if int32(result) < 0 {
		playerRelease, _ := find("libvlc_media_player_release")
		playerRelease.Call(mp)
		procDestroyWindow.Call(child)
		release, _ := find("libvlc_release")
		release.Call(instance)
		_ = dll.Release()
		return fmt.Errorf("libVLC no pudo iniciar la reproducción")
	}
	p.dll, p.instance, p.mediaPlayer, p.child, p.playing, p.visible = dll, instance, mp, child, true, true
	return nil
}

func (p *Player) SetBounds(x, y, width, height int, visible bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.child == 0 {
		return
	}
	procMoveWindow.Call(p.child, uintptr(x), uintptr(y), uintptr(max(width, 1)), uintptr(max(height, 1)), 1)
	if visible != p.visible {
		show := uintptr(swHide)
		if visible {
			show = swShow
		}
		procShowWindow.Call(p.child, show)
		p.visible = visible
	}
}
func (p *Player) Running() bool { p.mu.Lock(); defer p.mu.Unlock(); return p.playing }
func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.dll == nil {
		return
	}
	if p.mediaPlayer != 0 {
		if proc, e := p.dll.FindProc("libvlc_media_player_stop"); e == nil {
			proc.Call(p.mediaPlayer)
		}
		if proc, e := p.dll.FindProc("libvlc_media_player_release"); e == nil {
			proc.Call(p.mediaPlayer)
		}
	}
	if p.child != 0 {
		procDestroyWindow.Call(p.child)
	}
	if p.instance != 0 {
		if proc, e := p.dll.FindProc("libvlc_release"); e == nil {
			proc.Call(p.instance)
		}
	}
	_ = p.dll.Release()
	p.dll = nil
	p.instance, p.mediaPlayer, p.child = 0, 0, 0
	p.playing, p.visible = false, false
}
func (p *Player) Close() { p.Stop() }
