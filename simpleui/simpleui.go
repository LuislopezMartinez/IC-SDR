package simpleui

import rl "github.com/gen2brain/raylib-go/raylib"

type runtimeConfig struct {
	width, height               int32
	initialWidth, initialHeight int32
	minWidth, minHeight         int32
	mode                        ScaleMode
	title                       string
	background, letterbox       rl.Color
	windowFlags                 uint32
	canvasFilter                rl.TextureFilterMode
	targetFPS                   int32
	maximizeKey                 int32
	started                     bool
	canvas                      *Canvas
}

var runtime = runtimeConfig{
	width: 1280, height: 720,
	minWidth: 640, minHeight: 360,
	mode:         Fit,
	title:        "SimpleUI",
	background:   rl.Color{R: 10, G: 18, B: 24, A: 255},
	letterbox:    rl.Color{R: 3, G: 7, B: 10, A: 255},
	windowFlags:  rl.FlagWindowResizable,
	canvasFilter: rl.FilterBilinear,
	targetFPS:    60,
	maximizeKey:  rl.KeyF11,
}

// SetCanvasFilter controls how the fixed logical canvas is presented when a
// window is resized. FilterPoint keeps text pixel-sharp in data-table windows.
func SetCanvasFilter(filter rl.TextureFilterMode) {
	ensureNotStarted("SetCanvasFilter")
	runtime.canvasFilter = filter
}

var activationFeedback func()
var lifecycleLogger func(string, ...any)

func SetActivationFeedback(handler func())            { activationFeedback = handler }
func SetLifecycleLogger(handler func(string, ...any)) { lifecycleLogger = handler }

func traceLifecycle(message string, args ...any) {
	if lifecycleLogger != nil {
		lifecycleLogger(message, args...)
	}
}

// PlayActivationFeedback lets code-drawn buttons use the same feedback as
// standard SimpleUI buttons.
func PlayActivationFeedback() {
	if activationFeedback != nil {
		activationFeedback()
	}
}

// SetMode configures the logical design resolution and scaling. Call it before Run.
func SetMode(width, height int32, mode ScaleMode) {
	ensureNotStarted("SetMode")
	if width <= 0 || height <= 0 {
		panic("simpleui: SetMode requires a positive width and height")
	}
	runtime.width, runtime.height, runtime.mode = width, height, mode
}

// SetInitialWindowSize sets the physical size used when the window opens while
// preserving the logical canvas configured by SetMode. This is useful for
// compact auxiliary monitors that still need a detailed, consistently laid-out
// drawing surface.
func SetInitialWindowSize(width, height int32) {
	ensureNotStarted("SetInitialWindowSize")
	if width <= 0 || height <= 0 {
		panic("simpleui: SetInitialWindowSize requires a positive width and height")
	}
	runtime.initialWidth, runtime.initialHeight = width, height
}

// SetTitle configures the host window title.
func SetTitle(title string) {
	ensureNotStarted("SetTitle")
	runtime.title = title
}

// SetMinimumSize configures the smallest allowed host window size.
func SetMinimumSize(width, height int32) {
	ensureNotStarted("SetMinimumSize")
	if width <= 0 || height <= 0 {
		panic("simpleui: SetMinimumSize requires a positive width and height")
	}
	runtime.minWidth, runtime.minHeight = width, height
}

// SetColors configures the logical canvas and letterbox colors.
func SetColors(background, letterbox rl.Color) {
	runtime.background, runtime.letterbox = background, letterbox
}

// SetTargetFPS configures the frame limiter. A value of zero disables it.
func SetTargetFPS(fps int32) {
	ensureNotStarted("SetTargetFPS")
	if fps < 0 {
		panic("simpleui: SetTargetFPS cannot be negative")
	}
	runtime.targetFPS = fps
}

// Run owns the window and calls draw once per frame in logical coordinates.
func Run(draw func()) {
	if draw == nil {
		panic("simpleui: Run requires a draw function")
	}
	ensureNotStarted("Run")
	runtime.started = true

	traceLifecycle("Raylib: configuring window flags")
	rl.SetConfigFlags(runtime.windowFlags)
	traceLifecycle("Raylib: calling InitWindow (%dx%d)", runtime.width, runtime.height)
	windowWidth, windowHeight := runtime.width, runtime.height
	if runtime.initialWidth > 0 && runtime.initialHeight > 0 {
		windowWidth, windowHeight = runtime.initialWidth, runtime.initialHeight
	}
	rl.InitWindow(windowWidth, windowHeight, runtime.title)
	traceLifecycle("Raylib: InitWindow finished · monitor=%d · screen=%dx%d", rl.GetCurrentMonitor(), rl.GetScreenWidth(), rl.GetScreenHeight())
	placeWindowOnPrimaryMonitor(windowWidth, windowHeight)
	rl.SetWindowMinSize(int(runtime.minWidth), int(runtime.minHeight))
	if runtime.targetFPS > 0 {
		rl.SetTargetFPS(runtime.targetFPS)
	}
	traceLifecycle("Raylib: creating primary render texture")
	runtime.canvas = NewCanvas(runtime.width, runtime.height, runtime.mode)
	traceLifecycle("Raylib: render texture created · id=%d", runtime.canvas.target.Texture.ID)

	defer func() {
		unloadFonts()
		runtime.canvas.Unload()
		runtime.canvas = nil
		rl.CloseWindow()
		runtime.started = false
	}()

	traceLifecycle("Raylib: entering interface loop")
	for !rl.WindowShouldClose() {
		handleWindowShortcuts()
		runtime.canvas.Begin(runtime.background)
		defaultManager.Update(currentInput())
		draw()
		defaultManager.Draw()
		runtime.canvas.End(runtime.letterbox)
	}
}

func placeWindowOnPrimaryMonitor(wantedWidth, wantedHeight int32) {
	const primaryMonitor = 0
	monitorCount := rl.GetMonitorCount()
	monitorWidth := rl.GetMonitorWidth(primaryMonitor)
	monitorHeight := rl.GetMonitorHeight(primaryMonitor)
	monitorPosition := rl.GetMonitorPosition(primaryMonitor)
	width, height, x, y := safeWindowBounds(
		int(wantedWidth), int(wantedHeight), monitorWidth, monitorHeight,
		int(monitorPosition.X), int(monitorPosition.Y),
	)
	traceLifecycle("Raylib: monitors=%d · primary=%dx%d at (%d,%d)", monitorCount, monitorWidth, monitorHeight, int(monitorPosition.X), int(monitorPosition.Y))
	rl.SetWindowMonitor(primaryMonitor)
	rl.SetWindowSize(width, height)
	rl.SetWindowPosition(x, y)
	traceLifecycle("Raylib: window placed on primary monitor · position=(%d,%d) · size=%dx%d", x, y, width, height)
}

func safeWindowBounds(wantedWidth, wantedHeight, monitorWidth, monitorHeight, monitorX, monitorY int) (width, height, x, y int) {
	const horizontalMargin = 48
	const verticalMargin = 80
	width = min(wantedWidth, max(640, monitorWidth-horizontalMargin))
	height = min(wantedHeight, max(480, monitorHeight-verticalMargin))
	width = min(width, monitorWidth)
	height = min(height, monitorHeight)
	x = monitorX + max(0, (monitorWidth-width)/2)
	y = monitorY + max(0, (monitorHeight-height)/2)
	return
}

// MousePosition returns the pointer in logical design coordinates.
func MousePosition() rl.Vector2 {
	if runtime.canvas == nil {
		return rl.Vector2{}
	}
	return runtime.canvas.Viewport.MousePosition()
}

// MouseInViewport reports whether the pointer is over the logical canvas.
func MouseInViewport() bool {
	return runtime.canvas != nil && runtime.canvas.Viewport.ContainsScreenPoint(rl.GetMousePosition())
}

// Scale returns the current logical-to-window scale factors.
func Scale() rl.Vector2 {
	if runtime.canvas == nil {
		return rl.Vector2{X: 1, Y: 1}
	}
	return runtime.canvas.Viewport.Scale()
}

func handleWindowShortcuts() {
	if runtime.maximizeKey != 0 && rl.IsKeyPressed(runtime.maximizeKey) {
		if rl.IsWindowMaximized() {
			rl.RestoreWindow()
		} else {
			rl.MaximizeWindow()
		}
	}
}

func ensureNotStarted(operation string) {
	if runtime.started {
		panic("simpleui: " + operation + " must be called before Run")
	}
}
