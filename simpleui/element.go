package simpleui

import "go-zero/internal/i18n"

import rl "github.com/gen2brain/raylib-go/raylib"

// Element is the common contract implemented by every SimpleUI control.
type Element interface {
	ID() string
	Bounds() rl.Rectangle
	SetBounds(rl.Rectangle)
	Visible() bool
	SetVisible(bool)
	Enabled() bool
	SetEnabled(bool)
	Update(Input) bool
	Draw()
}

type pointerCapturer interface {
	CapturingPointer() bool
}

type focusable interface {
	Focused() bool
	SetFocused(bool)
}

type overlayElement interface {
	OverlayOpen() bool
	UpdateOverlay(Input) bool
	DrawOverlay()
}

// BaseElement contains the state shared by all controls.
type BaseElement struct {
	id      string
	bounds  rl.Rectangle
	visible bool
	enabled bool
}

func NewBaseElement(id string, x, y, width, height float32) BaseElement {
	if id == "" {
		panic(i18n.Source("text.5d6b03c55595"))
	}
	return BaseElement{
		id:      id,
		bounds:  rl.Rectangle{X: x, Y: y, Width: width, Height: height},
		visible: true,
		enabled: true,
	}
}

func (e *BaseElement) ID() string                    { return e.id }
func (e *BaseElement) Bounds() rl.Rectangle          { return e.bounds }
func (e *BaseElement) SetBounds(bounds rl.Rectangle) { e.bounds = bounds }
func (e *BaseElement) Visible() bool                 { return e.visible }
func (e *BaseElement) SetVisible(visible bool)       { e.visible = visible }
func (e *BaseElement) Enabled() bool                 { return e.enabled }
func (e *BaseElement) SetEnabled(enabled bool)       { e.enabled = enabled }

// Input is a frame snapshot expressed in logical coordinates.
type Input struct {
	Pointer         rl.Vector2
	PointerInCanvas bool
	Pressed         bool
	Down            bool
	Released        bool
	Wheel           float32
}

func (input Input) Over(bounds rl.Rectangle) bool {
	return input.PointerInCanvas && rl.CheckCollisionPointRec(input.Pointer, bounds)
}
