//go:build windows

package feedback

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

type Window struct {
	mu              sync.RWMutex
	view            webview2.WebView
	done            chan struct{}
	configPath      string
	preferencesPath string
	info            TechnicalInfo
}

var (
	feedbackUser32        = windows.NewLazySystemDLL("user32.dll")
	feedbackShowWindow    = feedbackUser32.NewProc("ShowWindow")
	feedbackSetForeground = feedbackUser32.NewProc("SetForegroundWindow")
)

func NewWindow(configPath string) *Window {
	return &Window{configPath: configPath, preferencesPath: UserDataPath("contact.json")}
}

func PlatformDescription() (string, string) {
	version := windows.RtlGetVersion()
	name := "Windows"
	if version.MajorVersion >= 10 && version.BuildNumber >= 22000 {
		name = "Windows 11"
	} else if version.MajorVersion >= 10 {
		name = "Windows 10"
	}
	return name + " · compilación " + fmt.Sprint(version.BuildNumber), runtime.GOARCH
}

func (window *Window) Open(info TechnicalInfo) error {
	window.mu.Lock()
	window.info = info
	if window.view != nil {
		view := window.view
		window.mu.Unlock()
		window.show(view)
		window.update(view, info)
		return nil
	}
	window.done = make(chan struct{})
	done := window.done
	window.mu.Unlock()

	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		view := webview2.NewWithOptions(webview2.WebViewOptions{
			AutoFocus: true,
			WindowOptions: webview2.WindowOptions{
				Title: "IC-SDR · Reportes y sugerencias", Width: 1050, Height: 790, Center: true,
			},
		})
		if view == nil {
			ready <- errors.New("WebView2 no pudo crear la ventana de reportes")
			close(done)
			return
		}
		if err := view.Bind("__icSDRSubmitFeedback", func(payload string) Result {
			var report Report
			if err := json.Unmarshal([]byte(payload), &report); err != nil {
				return Result{Message: "Los datos del formulario no son válidos."}
			}
			window.mu.RLock()
			current := window.info
			window.mu.RUnlock()
			report.Technical = TechnicalSummary(current, report.IncludeReceiver)
			config, err := LoadConfig(window.configPath)
			if err != nil {
				return Result{Message: "El formulario todavía no está conectado a Google Forms."}
			}
			result := Submit(config, report)
			if result.OK {
				if err := SavePreferences(window.preferencesPath, Preferences{Name: report.Name, Email: report.Email}); err != nil {
					result.Message = "El reporte se envió, pero no se pudieron guardar tus datos de contacto."
				}
			}
			return result
		}); err != nil {
			view.Destroy()
			ready <- err
			close(done)
			return
		}
		if err := view.Bind("__icSDRCloseFeedback", func() { go window.Close() }); err != nil {
			view.Destroy()
			ready <- err
			close(done)
			return
		}
		window.mu.Lock()
		window.view = view
		current := window.info
		window.mu.Unlock()
		preferences, _ := LoadPreferences(window.preferencesPath)
		view.SetHtml(feedbackHTML(current, preferences))
		ready <- nil
		view.Run()
		window.mu.Lock()
		window.view = nil
		window.mu.Unlock()
		view.Destroy()
		close(done)
	}()
	select {
	case err := <-ready:
		return err
	case <-time.After(15 * time.Second):
		return errors.New("WebView2 no respondió al abrir la ventana de reportes")
	}
}

func (window *Window) update(view webview2.WebView, info TechnicalInfo) {
	data, _ := json.Marshal(info)
	view.Dispatch(func() { view.Eval("window.updateTechnicalInfo(" + string(data) + ")") })
}

func (window *Window) show(view webview2.WebView) {
	view.Dispatch(func() {
		handle := uintptr(unsafe.Pointer(view.Window()))
		feedbackShowWindow.Call(handle, 9) // SW_RESTORE
		feedbackSetForeground.Call(handle)
	})
}

func (window *Window) Close() {
	window.mu.RLock()
	view, done := window.view, window.done
	window.mu.RUnlock()
	if view == nil {
		return
	}
	view.Terminate()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
	}
}

func feedbackHTML(info TechnicalInfo, preferences Preferences) string {
	data, _ := json.Marshal(info)
	html := strings.Replace(feedbackPage, "__INITIAL_DATA__", string(data), 1)
	contact, _ := json.Marshal(preferences)
	html = strings.Replace(html, "__CONTACT_DATA__", string(contact), 1)
	return html
}

const feedbackPage = `<!doctype html>
<html lang="es"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<style>
:root{color-scheme:dark;--bg:#081523;--panel:#0c1b2b;--line:#2d4661;--text:#f4f7fb;--muted:#91a6bf;--cyan:#27baff;--pink:#ec09d3;--orange:#ff7a22}
*{box-sizing:border-box}body{margin:0;background:radial-gradient(circle at 85% 5%,#12304b 0,transparent 32%),linear-gradient(145deg,#091827,#07131f);font-family:"Segoe UI",sans-serif;color:var(--text);min-height:100vh}
.app{max-width:1000px;margin:auto;padding:28px 34px 24px}.hero{display:flex;align-items:center;gap:25px;margin-bottom:24px}.mark{position:relative;width:112px;height:92px;flex:none}.bubble{position:absolute;inset:3px 16px 13px 2px;border-radius:26px;background:linear-gradient(145deg,#ff8a1f 5%,#ff385c 50%,#d700f5);box-shadow:0 0 30px #ff285c44}.bubble:after{content:"";position:absolute;left:15px;bottom:-14px;border:18px solid transparent;border-top-color:#d700f5;transform:rotate(18deg)}.bugbox{position:absolute;left:29px;top:-4px;width:72px;height:66px;border:7px solid #ffd04e;border-radius:22px;display:grid;place-items:center;color:#fff}.bugbox svg{width:38px;height:38px;fill:none;stroke:#fff;stroke-width:2.3;stroke-linecap:round;stroke-linejoin:round}.spark{position:absolute;right:0;top:15px;color:#ff9c31;font-size:31px;font-weight:900}.title h1{margin:0;font-size:38px;letter-spacing:1px}.title h1 span{background:linear-gradient(90deg,#26baff,#20d5fa);background-clip:text;color:transparent}.title p{font-size:25px;color:#b6c7df;margin:4px 0 0}
.grid{display:grid;grid-template-columns:1fr 1fr;gap:15px 22px}label{display:block;font-weight:650;font-size:15px;margin:0 0 7px}.field{width:100%;height:47px;border:1px solid #45617e;border-radius:7px;background:#102033;color:#e9f1fb;padding:0 15px;font-size:16px;outline:none}.field:focus,textarea:focus{border-color:var(--cyan);box-shadow:0 0 0 2px #27baff22}.wide{grid-column:1/-1}textarea.field{height:112px;padding:14px;resize:vertical;font-family:inherit}.counter{float:right;color:var(--muted);font-size:12px;margin-top:-24px;margin-right:11px;position:relative}.tech{margin-top:18px;border:1px solid #304861;border-radius:8px;overflow:hidden;background:#091827aa}.tech h2{font-size:16px;margin:0;padding:11px 17px;border-bottom:1px solid #2b4158}.rows{padding:5px 18px 8px}.row{display:grid;grid-template-columns:26px 30px 1fr;align-items:center;min-height:39px;border-bottom:1px solid #294057;color:#c6d5e7}.row:last-child{border:none}.check{width:20px;height:20px;border-radius:50%;background:var(--cyan);display:grid;place-items:center;color:#082038;font-weight:900}.ico{font-size:18px;color:#9eb5d0}.context{display:flex;align-items:center;gap:10px;margin:17px 0;color:#d8e5f4}.context input{appearance:none;width:25px;height:25px;border:1px solid #4b6a8b;border-radius:5px;background:#102033}.context input:checked{background:linear-gradient(135deg,#39c9ff,#087bea);position:relative}.context input:checked:after{content:"✓";position:absolute;inset:0;display:grid;place-items:center;font-weight:900}.footer{display:flex;align-items:center;gap:12px}.privacy{color:#7f94ad;font-size:14px;flex:1}.actions{display:flex;gap:12px}.btn{height:52px;padding:0 26px;border-radius:8px;border:1px solid #526d89;background:#13243a;color:#e3edf8;font-weight:700;font-size:15px;cursor:pointer}.primary{min-width:205px;border:none;background:linear-gradient(100deg,var(--orange),#ff385b 52%,var(--pink));color:#fff;box-shadow:0 8px 24px #e8176e33}.btn:hover{filter:brightness(1.12)}.btn:disabled{opacity:.55;cursor:wait}.status{min-height:22px;margin-top:10px;text-align:right;font-size:14px}.status.ok{color:#42df9a}.status.error{color:#ff8a8a}
@media(max-width:760px){.app{padding:20px}.grid{grid-template-columns:1fr}.wide{grid-column:auto}.title h1{font-size:29px}.title p{font-size:20px}.mark{transform:scale(.85);margin-left:-8px}.footer{align-items:stretch;flex-direction:column}.actions{justify-content:flex-end}}
</style></head><body><main class="app">
<section class="hero"><div class="mark"><div class="bubble"></div><div class="bugbox"><svg viewBox="0 0 32 32" aria-hidden="true"><path d="M11 10h10v10a5 5 0 0 1-10 0V10Z"/><path d="M13 10V8a3 3 0 0 1 6 0v2M8 13h3m10 0h3M7 18h4m10 0h4M9 24l3-2m11 2-3-2M9 7l3 3m11-3-3 3M16 10v15"/></svg></div><div class="spark">›</div></div><div class="title"><h1>REPORTES Y <span>SUGERENCIAS</span></h1><p>Ayúdanos a mejorar IC-SDR</p></div></section>
<form id="form"><div class="grid"><div><label for="type">Tipo de mensaje</label><select class="field" id="type"><option>Problema técnico</option><option>Sugerencia</option><option>Pregunta</option><option>Otro</option></select></div><div><label for="name">Nombre o indicativo (opcional)</label><input class="field" id="name" maxlength="80" placeholder="Ej. EA4ABC"></div><div></div><div><label for="email">Correo de contacto (opcional)</label><input class="field" id="email" type="email" maxlength="160" placeholder="Ej. tu.correo@ejemplo.es"></div><div class="wide"><label for="comment">Cuéntanos qué ha ocurrido</label><textarea class="field" id="comment" maxlength="2000" required placeholder="Describe el problema o tu sugerencia con el mayor detalle posible..."></textarea><span class="counter"><span id="count">0</span>/2000</span></div></div>
<section class="tech"><h2>Información técnica incluida</h2><div class="rows"><div class="row"><span class="check">✓</span><span class="ico">⚙</span><span id="version"></span></div><div class="row"><span class="check">✓</span><span class="ico">⊞</span><span id="windows"></span></div><div class="row"><span class="check">✓</span><span class="ico">▣</span><span id="sdr"></span></div><div class="row"><span class="check">✓</span><span class="ico">◷</span><span id="date"></span></div></div></section>
<label class="context"><input id="context" type="checkbox">Incluir contexto actual del receptor (modo, banda y frecuencia)</label>
<div class="footer"><span class="privacy">🔒 Tus datos no se enviarán hasta que pulses Enviar</span><div class="actions"><button class="btn" type="button" onclick="window.__icSDRCloseFeedback()">CANCELAR</button><button class="btn primary" id="submit" type="submit">➤ &nbsp; ENVIAR REPORTE</button></div></div><div class="status" id="status"></div></form></main>
<script>
const initial=__INITIAL_DATA__,contact=__CONTACT_DATA__;const $=id=>document.getElementById(id);$('name').value=contact.Name||'';$('email').value=contact.Email||'';window.updateTechnicalInfo=i=>{$('version').textContent='IC-SDR '+(i.Version||'desarrollo');$('windows').textContent=(i.Windows||'Windows')+' · '+(i.Architecture||'x64');$('sdr').textContent=(i.SDR||'No detectado')+' · '+(i.SDRStatus||'Desconectado');$('date').textContent=(i.LocalTime||'')+(i.TimeZone?' · '+i.TimeZone:'')};updateTechnicalInfo(initial);$('comment').oninput=()=>{$('count').textContent=$('comment').value.length};$('form').onsubmit=async e=>{e.preventDefault();const button=$('submit'),status=$('status');button.disabled=true;status.className='status';status.textContent='Enviando…';try{const result=await window.__icSDRSubmitFeedback(JSON.stringify({type:$('type').value,name:$('name').value,email:$('email').value,comment:$('comment').value,includeReceiver:$('context').checked}));status.className='status '+(result.ok?'ok':'error');status.textContent=result.message;if(result.ok){$('comment').value='';$('count').textContent='0'}}catch(_){status.className='status error';status.textContent='No se pudo enviar el reporte.'}finally{button.disabled=false}};
</script></body></html>`
