//go:build windows

package qrz

import (
	"encoding/json"
	"errors"
	"runtime"
	"sync"
	"time"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

type webViewSession struct {
	mu   sync.RWMutex
	view webview2.WebView
	done chan struct{}
}

var (
	user32Window = windows.NewLazySystemDLL("user32.dll")
	showWindow   = user32Window.NewProc("ShowWindow")
)

func startBrowser(profileDir string, snapshots chan pageSnapshot) (browserSession, error) {
	session := &webViewSession{done: make(chan struct{})}
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		view := webview2.NewWithOptions(webview2.WebViewOptions{
			DataPath:  profileDir,
			AutoFocus: true,
			WindowOptions: webview2.WindowOptions{
				Title: "IC-SDR · Acceso a QRZ", Width: 1120, Height: 760, Center: true,
			},
		})
		if view == nil {
			ready <- errors.New("WebView2 no pudo crear la ventana de acceso a QRZ")
			close(session.done)
			return
		}
		if err := view.Bind("__icSDRQRZSnapshot", func(payload string) error {
			var snapshot pageSnapshot
			if json.Unmarshal([]byte(payload), &snapshot) == nil {
				select {
				case snapshots <- snapshot:
				default:
					select {
					case <-snapshots:
					default:
					}
					snapshots <- snapshot
				}
			}
			return nil
		}); err != nil {
			view.Destroy()
			ready <- err
			close(session.done)
			return
		}
		view.Init(snapshotScript)
		session.mu.Lock()
		session.view = view
		session.mu.Unlock()
		ready <- nil
		view.Run()
		session.mu.Lock()
		session.view = nil
		session.mu.Unlock()
		view.Destroy()
		close(session.done)
	}()
	select {
	case err := <-ready:
		if err != nil {
			return nil, err
		}
		return session, nil
	case <-time.After(15 * time.Second):
		return nil, errors.New("WebView2 no respondió al abrir QRZ")
	}
}

const snapshotScript = `
(() => {
  if (window.__icSDRQRZTimer) return;
  document.addEventListener('click', event => {
    const link=event.target&&event.target.closest&&event.target.closest('a[href]');
    if (!link) return;
    try {
      const target=new URL(link.href, location.href);
      if (target.protocol!=='https:' || (target.hostname!=='qrz.com' && !target.hostname.endsWith('.qrz.com'))) event.preventDefault();
    } catch (_) { event.preventDefault(); }
  }, true);
  window.__icSDRQRZTimer = setInterval(() => {
    if (typeof window.__icSDRQRZSnapshot !== 'function') return;
    const visible = e => { const r=e.getBoundingClientRect(),s=getComputedStyle(e); return r.width>0&&r.height>0&&s.display!=='none'&&s.visibility!=='hidden'; };
    const imgs=[...document.images].filter(visible).map(i=>{const r=i.getBoundingClientRect();return {src:i.currentSrc||i.src,x:r.x,y:r.y,w:r.width,h:r.height}}).filter(i=>i.w>60&&i.h>60&&i.y>70&&i.y<560&&i.w/i.h<1.7&&!/logo|banner|advert|pixel/i.test(i.src)).sort((a,b)=>(b.x+b.w)-(a.x+a.w));
    const links=[...document.querySelectorAll('a[href]')];
    const accountLink=links.some(a=>{const r=a.getBoundingClientRect(),t=(a.textContent||'').trim();return r.y<130&&/^[A-Z0-9/]{3,16}$/.test(t)&&/\/db\//i.test(a.href||'');});
    const authenticated=accountLink||links.some(a=>/logout|logoff|signout/i.test((a.href||'')+' '+(a.textContent||'')));
    const flagSources=[...document.images].map(i=>i.currentSrc||i.src||'').filter(src=>/flags-iso/i.test(src));
    const flagMatch=flagSources.map(src=>src.match(/flags-iso\/(?:flat\/\d+\/)?([a-z]{2})\.(?:png|gif|svg)/i)).find(Boolean);
    const data={url:location.href,title:document.title,text:(document.body&&document.body.innerText||'').slice(0,120000),imageURL:imgs[0]?.src||'',countryCode:(flagMatch&&flagMatch[1]||'').toUpperCase(),authenticated};
    window.__icSDRQRZSnapshot(JSON.stringify(data)).catch(()=>{});
  }, 900);
})();`

func (s *webViewSession) Navigate(pageURL string) error {
	s.mu.RLock()
	view := s.view
	s.mu.RUnlock()
	if view == nil {
		return errors.New("la ventana QRZ está cerrada")
	}
	view.Dispatch(func() { view.Navigate(pageURL) })
	return nil
}

func (s *webViewSession) Alive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.view != nil
}

func (s *webViewSession) Hide() { s.setWindowVisible(false) }
func (s *webViewSession) Show() { s.setWindowVisible(true) }

func (s *webViewSession) setWindowVisible(visible bool) {
	s.mu.RLock()
	view := s.view
	s.mu.RUnlock()
	if view == nil {
		return
	}
	view.Dispatch(func() {
		const swHide, swShow = uintptr(0), uintptr(5)
		command := swHide
		if visible {
			command = swShow
		}
		_, _, _ = showWindow.Call(uintptr(unsafe.Pointer(view.Window())), command)
	})
}

func (s *webViewSession) Close() {
	s.mu.RLock()
	view := s.view
	s.mu.RUnlock()
	if view != nil {
		view.Terminate()
	}
	select {
	case <-s.done:
	case <-time.After(3 * time.Second):
	}
}
