package simpleui

import "go-zero/internal/i18n"

import rl "github.com/gen2brain/raylib-go/raylib"

// Manager owns controls and dispatches input from front to back.
type Manager struct {
	elements []Element
	byID     map[string]Element
	captured Element
	focused  Element
	// pointerBlocked remains true for the complete frame in which an overlay
	// handled input, including the click that closes it. This prevents canvas
	// controls drawn below a popup from seeing the same physical event.
	pointerBlocked bool
}

func NewManager() *Manager {
	return &Manager{byID: make(map[string]Element)}
}

var defaultManager = NewManager()

// Add registers a control in the default manager.
func Add(element Element) {
	defaultManager.Add(element)
}

func (m *Manager) Add(element Element) {
	if element == nil {
		panic(i18n.Source("text.ed4fad023ba1"))
	}
	if _, exists := m.byID[element.ID()]; exists {
		panic(i18n.Source("text.bbba2a0d8209") + element.ID())
	}
	m.elements = append(m.elements, element)
	m.byID[element.ID()] = element
}

// Get returns a control from the default manager by ID.
func Get(id string) Element {
	return defaultManager.Get(id)
}

func (m *Manager) Get(id string) Element {
	return m.byID[id]
}

func (m *Manager) Update(input Input) {
	m.pointerBlocked = false
	if overlay := m.topOpenOverlay(); overlay != nil {
		m.pointerBlocked = true
		consumed := overlay.UpdateOverlay(input)
		m.rememberFocus(overlay.(Element))
		if consumed || overlay.OverlayOpen() {
			return
		}
	}

	if input.Pressed {
		hit := m.topElementAt(input)
		if m.focused != nil && hit != m.focused {
			m.focused.(focusable).SetFocused(false)
			m.focused = nil
		}
	}

	if m.captured != nil {
		m.captured.Update(input)
		m.rememberFocus(m.captured)
		if capture, ok := m.captured.(pointerCapturer); !ok || !capture.CapturingPointer() {
			m.captured = nil
		}
		return
	}

	available := input
	for index := len(m.elements) - 1; index >= 0; index-- {
		element := m.elements[index]
		if element.Visible() && element.Enabled() {
			consumed := element.Update(available)
			m.rememberFocus(element)
			if capture, ok := element.(pointerCapturer); ok && capture.CapturingPointer() {
				m.captured = element
				return
			}
			if consumed {
				available.PointerInCanvas = false
				available.Pressed = false
				available.Down = false
				available.Released = false
				available.Wheel = 0
			}
		}
	}
}

// PointerInputBlocked reports whether an overlay is open or consumed pointer
// input during the current frame. Non-Element canvas controls use it to avoid
// click-through and wheel-through beneath dropdowns and popups.
func (m *Manager) PointerInputBlocked() bool {
	return m.pointerBlocked || m.topOpenOverlay() != nil
}

// PointerInputBlocked is the default manager counterpart used by screens.
func PointerInputBlocked() bool { return defaultManager.PointerInputBlocked() }

func (m *Manager) topElementAt(input Input) Element {
	for index := len(m.elements) - 1; index >= 0; index-- {
		element := m.elements[index]
		if element.Visible() && element.Enabled() && input.Over(element.Bounds()) {
			return element
		}
	}
	return nil
}

func (m *Manager) rememberFocus(element Element) {
	control, ok := element.(focusable)
	if !ok || !control.Focused() {
		return
	}
	if m.focused != nil && m.focused != element {
		m.focused.(focusable).SetFocused(false)
	}
	m.focused = element
}

func (m *Manager) Draw() {
	for _, element := range m.elements {
		if element.Visible() {
			element.Draw()
		}
	}
	for _, element := range m.elements {
		if overlay, ok := element.(overlayElement); ok && element.Visible() && overlay.OverlayOpen() {
			overlay.DrawOverlay()
		}
	}
}

func (m *Manager) topOpenOverlay() overlayElement {
	for index := len(m.elements) - 1; index >= 0; index-- {
		if overlay, ok := m.elements[index].(overlayElement); ok && m.elements[index].Visible() && overlay.OverlayOpen() {
			return overlay
		}
	}
	return nil
}

func currentInput() Input {
	return Input{
		Pointer:         MousePosition(),
		PointerInCanvas: MouseInViewport(),
		Pressed:         rl.IsMouseButtonPressed(rl.MouseButtonLeft),
		Down:            rl.IsMouseButtonDown(rl.MouseButtonLeft),
		Released:        rl.IsMouseButtonReleased(rl.MouseButtonLeft),
		Wheel:           rl.GetMouseWheelMove(),
	}
}
