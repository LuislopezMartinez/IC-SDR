package screens

import (
	"go-zero/internal/i18n"

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
	p.password = simpleui.NewTextField("webPassword", 516, toolY+94, 360, 38, i18n.Source("text.dcfbe524ffee"), 16)
	p.password.SetMaxLength(128)
	p.password.SetPassword(true)
	p.controlPassword = simpleui.NewTextField("webControlPassword", 516, toolY+158, 360, 38, i18n.Source("text.53c802969a71"), 16)
	p.controlPassword.SetMaxLength(128)
	p.controlPassword.SetPassword(true)
	p.qrMode = simpleui.NewSwitch("webQRMode", 898, toolY+158, 404, 32, i18n.Source("text.c38f4b257383"), screen.webConfig.QRInternet, 14)
	p.qrMode.OnChange(func(bool) { p.saveQRSettings() })
	p.activate = simpleui.NewButton("webActivate", 898, toolY+94, 148, 38, i18n.Source("text.1d5e3391cad7"), 16)
	p.apply = simpleui.NewButton("webApply", 1057, toolY+94, 245, 38, i18n.Source("text.c376ed4718b8"), 16)
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
		p.feedback = i18n.Source("text.25ea0eab8675")
		return
	}
	config := p.screen.webConfig
	config.Port, config.Enabled = port, enabled
	config.QRInternet = p.qrMode.Active()
	viewerPassword := p.password.Text()
	controllerPassword := p.controlPassword.Text()
	if viewerPassword != "" && controllerPassword != "" && viewerPassword == controllerPassword {
		p.feedback = i18n.Source("text.65e667662793")
		return
	}
	if viewerPassword != "" {
		if salt, hash, err := config.controlCredential(); err == nil {
			derived, _ := pbkdf2.Key(sha256.New, viewerPassword, salt, 60000, 32)
			if subtle.ConstantTimeCompare(derived, hash) == 1 {
				p.feedback = i18n.Source("text.65e667662793")
				return
			}
		}
	}
	if controllerPassword != "" {
		if salt, hash, err := config.credential(); err == nil {
			derived, _ := pbkdf2.Key(sha256.New, controllerPassword, salt, 60000, 32)
			if subtle.ConstantTimeCompare(derived, hash) == 1 {
				p.feedback = i18n.Source("text.65e667662793")
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
		p.feedback = i18n.Source("text.6afc4ac10158")
	} else {
		p.feedback = i18n.Source("text.1be89b3a48c7")
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
	p.feedback = i18n.Source("text.4106a6f9a965")
}

func (p *WebPanel) DrawPanel() {
	config := p.screen.webConfig
	running := p.screen.webServer != nil
	p.activate.SetLabel(map[bool]string{true: i18n.Source("text.42a572b1399e"), false: i18n.Source("text.1d5e3391cad7")}[running])
	p.updateAddress()
	p.updatePublicAddress(running)
	title := func(text string, x, y float32, size int32, color rl.Color) {
		simpleui.DrawTextStyled(text, x, y, size, simpleui.FontSemiBold, color)
	}
	title(i18n.Source("text.db6a87d580b1"), 380, toolY+12, 21, colors.cyan)
	title(i18n.Source("text.87fea93e5021"), 575, toolY+17, 15, colors.text)
	status := i18n.Source("text.7dc7253c376a")
	statusColor := colors.orange
	if running {
		status, statusColor = i18n.Source("text.521ed3ec3a4a"), colors.green
	}
	title(i18n.Source("text.36319c6f0d58")+status, 380, toolY+44, 18, statusColor)
	if running {
		title(fmt.Sprintf(i18n.Source("text.476214079da5"), p.screen.webServer.ListenerCount()), 585, toolY+47, 15, colors.text)
	}
	title(i18n.Source("text.081045a9ea99"), 382, toolY+74, 14, colors.text)
	title(i18n.Source("text.189793bb736c"), 516, toolY+74, 14, colors.text)
	title(i18n.Source("text.e2414bc1a2e5"), 516, toolY+139, 14, colors.text)
	if config.ControlPasswordHash != "" {
		title(i18n.Source("text.404a99ee6c3e"), 898, toolY+139, 13, colors.green)
	}
	if config.PasswordHash != "" {
		title(i18n.Source("text.3ce1b1fa759c"), 898, toolY+74, 13, colors.green)
	} else {
		title(i18n.Source("text.51712c39dcf8"), 898, toolY+74, 13, colors.orange)
	}
	url := webLANURL(p.lanIP, config.Port)
	available := running && p.lanIP != i18n.Source("text.da399d9df54b")
	if p.qrMode.Active() {
		url, available = i18n.Source("text.d4500f7f2595"), false
		if p.publicIP != "" {
			url, available = webLANURL(p.publicIP, config.Port), running
		}
	}
	title(i18n.Source("text.3d4ab03be062"), 898, toolY+207, 13, colors.cyan)
	publicLabel := p.publicIP
	if publicLabel == "" {
		publicLabel = i18n.Source("text.723f267b551b")
	}
	title(publicLabel, 898, toolY+224, 17, colors.text)
	mode := i18n.Source("text.646c19373ac9")
	if p.qrMode.Active() {
		mode = i18n.Source("text.0c21e780658d")
	}
	title("QR "+mode+i18n.Source("text.723435970bc8"), 380, toolY+199, 13, colors.cyan)
	displayURL := url
	if len(displayURL) > 42 {
		displayURL = displayURL[:39] + "..."
	}
	title(displayURL, 380, toolY+216, 18, colors.text)
	message := p.feedback
	if message == "" {
		message = i18n.Source("text.2d9962a2aded")
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
		simpleui.DrawTextStyled(i18n.Source("text.eca62ac7f1a9"), x+13, y+73, 16, simpleui.FontSemiBold, colors.text)
		simpleui.DrawTextStyled(i18n.Source("text.c5effd919830"), x+22, y+104, 14, simpleui.FontSemiBold, colors.muted)
		return
	}
	if p.qrURL != url || !p.qrReady {
		if p.qrReady {
			rl.UnloadTexture(p.qrTexture)
			p.qrReady = false
		}
		png, err := qrcode.Encode(url, qrcode.Medium, 256)
		if err != nil {
			p.feedback = i18n.Source("text.d8cdd9685ad0")
			return
		}
		image := rl.LoadImageFromMemory(".png", png, int32(len(png)))
		p.qrTexture = rl.LoadTextureFromImage(image)
		rl.UnloadImage(image)
		if !rl.IsTextureValid(p.qrTexture) {
			p.feedback = i18n.Source("text.e58b1f8df0a5")
			return
		}
		rl.SetTextureFilter(p.qrTexture, rl.FilterPoint)
		p.qrURL, p.qrReady = url, true
	}
	rl.DrawTexturePro(p.qrTexture, rl.Rectangle{Width: float32(p.qrTexture.Width), Height: float32(p.qrTexture.Height)}, rl.Rectangle{X: x, Y: y, Width: size, Height: size}, rl.Vector2{}, 0, rl.White)
	simpleui.DrawTextStyled(i18n.Source("text.0e419ed9505f"), x+4, y+211, 13, simpleui.FontSemiBold, colors.text)
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
		return i18n.Source("text.da399d9df54b")
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
	return i18n.Source("text.da399d9df54b")
}
