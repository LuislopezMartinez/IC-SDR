package screens

import (
	"crypto/pbkdf2"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
	qrcode "github.com/skip2/go-qrcode"
)

type WebPanel struct {
	screen           *MainScreen
	controls         []simpleui.Element
	port             *simpleui.TextField
	password         *simpleui.TextField
	controlPassword  *simpleui.TextField
	qrMode           *simpleui.Switch
	activate         *simpleui.Button
	apply            *simpleui.Button
	feedback         string
	lanIP            string
	lastAddressCheck time.Time
	publicIP         string
	publicPending    bool
	lastPublicCheck  time.Time
	publicResults    chan publicIPResult
	qrURL            string
	qrTexture        rl.Texture2D
	qrReady          bool
}

func NewWebPanel(screen *MainScreen) *WebPanel {
	p := &WebPanel{screen: screen, publicResults: make(chan publicIPResult, 1)}
	p.port = simpleui.NewTextField("webPort", 382, toolY+94, 112, 38, "8080", 18)
	p.port.SetMaxLength(5)
	p.port.SetFilter(func(r rune) bool { return r >= '0' && r <= '9' })
	p.port.SetText(strconv.Itoa(screen.webConfig.Port))
	p.password = simpleui.NewTextField("webPassword", 516, toolY+94, 360, 38, "Nueva clave de acceso (mín. 12)", 16)
	p.password.SetMaxLength(128)
	p.password.SetPassword(true)
	p.controlPassword = simpleui.NewTextField("webControlPassword", 516, toolY+158, 360, 38, "Nueva clave de control (mín. 12)", 16)
	p.controlPassword.SetMaxLength(128)
	p.controlPassword.SetPassword(true)
	p.qrMode = simpleui.NewSwitch("webQRMode", 898, toolY+158, 404, 32, "QR LOCAL / INTERNET", screen.webConfig.QRInternet, 14)
	p.qrMode.OnChange(func(bool) { p.saveQRSettings() })
	p.activate = simpleui.NewButton("webActivate", 898, toolY+94, 148, 38, "ACTIVAR", 16)
	p.apply = simpleui.NewButton("webApply", 1057, toolY+94, 245, 38, "GUARDAR Y APLICAR", 16)
	p.activate.OnClick(func() { p.applySettings(p.screen.webServer == nil) })
	p.apply.OnClick(func() { p.applySettings(p.screen.webConfig.Enabled) })
	p.controls = []simpleui.Element{p.port, p.password, p.controlPassword, p.qrMode, p.activate, p.apply}
	p.SetVisible(false)
	return p
}

func (p *WebPanel) SetVisible(visible bool) {
	for _, control := range p.controls {
		control.SetVisible(visible)
		control.SetEnabled(visible)
	}
}

func (p *WebPanel) applySettings(enabled bool) {
	port, err := strconv.Atoi(strings.TrimSpace(p.port.Text()))
	if err != nil || port < 1024 || port > 65535 {
		p.feedback = "Puerto inválido: use 1024–65535"
		return
	}
	config := p.screen.webConfig
	config.Port, config.Enabled = port, enabled
	config.QRInternet = p.qrMode.Active()
	viewerPassword := p.password.Text()
	controllerPassword := p.controlPassword.Text()
	if viewerPassword != "" && controllerPassword != "" && viewerPassword == controllerPassword {
		p.feedback = "Use claves distintas para lectura y control"
		return
	}
	if viewerPassword != "" {
		if salt, hash, err := config.controlCredential(); err == nil {
			derived, _ := pbkdf2.Key(sha256.New, viewerPassword, salt, 60000, 32)
			if subtle.ConstantTimeCompare(derived, hash) == 1 {
				p.feedback = "Use claves distintas para lectura y control"
				return
			}
		}
	}
	if controllerPassword != "" {
		if salt, hash, err := config.credential(); err == nil {
			derived, _ := pbkdf2.Key(sha256.New, controllerPassword, salt, 60000, 32)
			if subtle.ConstantTimeCompare(derived, hash) == 1 {
				p.feedback = "Use claves distintas para lectura y control"
				return
			}
		}
	}
	if password := p.password.Text(); password != "" {
		if err := config.setPassword(password); err != nil {
			p.feedback = err.Error()
			return
		}
	}
	if password := p.controlPassword.Text(); password != "" {
		if err := config.setControlPassword(password); err != nil {
			p.feedback = err.Error()
			return
		}
	}
	if err := p.screen.applyWebConfig(config); err != nil {
		p.feedback = err.Error()
		return
	}
	p.password.SetText("")
	p.controlPassword.SetText("")
	if enabled {
		p.feedback = "Servidor activo; la contraseña anterior queda sustituida si se cambió"
	} else {
		p.feedback = "Servidor detenido y configuración guardada"
	}
}

func (p *WebPanel) saveQRSettings() {
	config := p.screen.webConfig
	config.QRInternet = p.qrMode.Active()
	if err := saveWebConfig(p.screen.webConfigPath, config); err != nil {
		p.feedback = err.Error()
		return
	}
	p.screen.webConfig = config
	p.feedback = "QR actualizado; no se han cambiado los puertos"
}

func (p *WebPanel) DrawPanel() {
	config := p.screen.webConfig
	running := p.screen.webServer != nil
	p.activate.SetLabel(map[bool]string{true: "DETENER", false: "ACTIVAR"}[running])
	p.updateAddress()
	p.updatePublicAddress(running)
	title := func(text string, x, y float32, size int32, color rl.Color) {
		simpleui.DrawTextStyled(text, x, y, size, simpleui.FontSemiBold, color)
	}
	title("SERVIDOR WEB", 380, toolY+12, 21, colors.cyan)
	title("VISOR HTTP · CONTROL CON CLAVE", 575, toolY+17, 15, colors.text)
	status := "DETENIDO"
	statusColor := colors.orange
	if running {
		status, statusColor = "ACTIVO", colors.green
	}
	title("ESTADO: "+status, 380, toolY+44, 18, statusColor)
	if running {
		title(fmt.Sprintf("ESCUCHAS ACTIVAS: %d", p.screen.webServer.ListenerCount()), 585, toolY+47, 15, colors.text)
	}
	title("PUERTO", 382, toolY+74, 14, colors.text)
	title("CLAVE DE ACCESO", 516, toolY+74, 14, colors.text)
	title("CLAVE DE CONTROL REMOTO", 516, toolY+139, 14, colors.text)
	if config.ControlPasswordHash != "" {
		title("CONTROL CONFIGURADO", 898, toolY+139, 13, colors.green)
	}
	if config.PasswordHash != "" {
		title("CLAVE GUARDADA", 898, toolY+74, 13, colors.green)
	} else {
		title("SIN CLAVE", 898, toolY+74, 13, colors.orange)
	}
	url := webLANURL(p.lanIP, config.Port)
	available := running && p.lanIP != "IP-DEL-PC"
	if p.qrMode.Active() {
		url, available = "DETECTANDO IP PÚBLICA...", false
		if p.publicIP != "" {
			url, available = webLANURL(p.publicIP, config.Port), running
		}
	}
	title("IP PÚBLICA DETECTADA", 898, toolY+207, 13, colors.cyan)
	publicLabel := p.publicIP
	if publicLabel == "" {
		publicLabel = "Esperando respuesta..."
	}
	title(publicLabel, 898, toolY+224, 17, colors.text)
	mode := "LOCAL"
	if p.qrMode.Active() {
		mode = "INTERNET"
	}
	title("QR "+mode+" · ABRIR EN EL MÓVIL", 380, toolY+199, 13, colors.cyan)
	displayURL := url
	if len(displayURL) > 42 {
		displayURL = displayURL[:39] + "..."
	}
	title(displayURL, 380, toolY+216, 18, colors.text)
	message := p.feedback
	if message == "" {
		message = "HTTP sin cifrar · QR Internet requiere puerto accesible desde fuera"
	}
	title(message, 380, toolY+245, 13, colors.orange)
	p.drawQRCode(url, available)
}

func (p *WebPanel) Close() {
	if p.qrReady {
		rl.UnloadTexture(p.qrTexture)
		p.qrReady = false
	}
}

func (p *WebPanel) updateAddress() {
	if p.lanIP == "" || time.Since(p.lastAddressCheck) > 3*time.Second {
		p.lanIP = localLANAddress()
		p.lastAddressCheck = time.Now()
	}
}

func (p *WebPanel) updatePublicAddress(running bool) {
	select {
	case result := <-p.publicResults:
		p.publicPending = false
		p.publicIP = result.ip
	default:
	}
	if !running || !p.qrMode.Active() || p.publicPending {
		return
	}
	interval := 10 * time.Minute
	if p.publicIP == "" {
		interval = 30 * time.Second
	}
	if !p.lastPublicCheck.IsZero() && time.Since(p.lastPublicCheck) < interval {
		return
	}
	p.publicPending = true
	p.lastPublicCheck = time.Now()
	results := p.publicResults
	go func() {
		ip, _ := lookupPublicIPv4()
		select {
		case results <- publicIPResult{ip: ip}:
		default:
		}
	}()
}

func webLANURL(ip string, port int) string { return fmt.Sprintf("http://%s:%d/", ip, port) }

func (p *WebPanel) drawQRCode(url string, available bool) {
	const x, y, size = float32(1366), float32(645), float32(206)
	if !available {
		if p.qrReady {
			rl.UnloadTexture(p.qrTexture)
			p.qrReady = false
			p.qrURL = ""
		}
		rl.DrawRectangleRounded(rl.Rectangle{X: x, Y: y, Width: size, Height: size}, .04, 6, colors.panelAlt)
		simpleui.DrawTextStyled("QR NO DISPONIBLE", x+13, y+73, 16, simpleui.FontSemiBold, colors.text)
		simpleui.DrawTextStyled("REVISE DIRECCIÓN", x+22, y+104, 14, simpleui.FontSemiBold, colors.muted)
		return
	}
	if p.qrURL != url || !p.qrReady {
		if p.qrReady {
			rl.UnloadTexture(p.qrTexture)
			p.qrReady = false
		}
		png, err := qrcode.Encode(url, qrcode.Medium, 256)
		if err != nil {
			p.feedback = "No se pudo generar el QR"
			return
		}
		image := rl.LoadImageFromMemory(".png", png, int32(len(png)))
		p.qrTexture = rl.LoadTextureFromImage(image)
		rl.UnloadImage(image)
		if !rl.IsTextureValid(p.qrTexture) {
			p.feedback = "No se pudo mostrar el QR"
			return
		}
		rl.SetTextureFilter(p.qrTexture, rl.FilterPoint)
		p.qrURL, p.qrReady = url, true
	}
	rl.DrawTexturePro(p.qrTexture, rl.Rectangle{Width: float32(p.qrTexture.Width), Height: float32(p.qrTexture.Height)}, rl.Rectangle{X: x, Y: y, Width: size, Height: size}, rl.Vector2{}, 0, rl.White)
	simpleui.DrawTextStyled("ESCANEAR PARA ABRIR", x+4, y+211, 13, simpleui.FontSemiBold, colors.text)
}

func localLANAddress() string {
	if connection, err := net.Dial("udp", "192.0.2.1:80"); err == nil {
		ip := connection.LocalAddr().(*net.UDPAddr).IP
		_ = connection.Close()
		if ip.To4() != nil && ip.IsPrivate() {
			return ip.String()
		}
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return "IP-DEL-PC"
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err == nil && ip.To4() != nil && ip.IsPrivate() {
				return ip.String()
			}
		}
	}
	return "IP-DEL-PC"
}
