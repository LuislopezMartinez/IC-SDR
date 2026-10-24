package simpleui

import (
	"testing"
	"unicode"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestTextFieldUsesRunePositions(t *testing.T) {
	field := NewTextField("name", 0, 0, 200, 40, "", 16)
	field.SetText("España Ω")
	if field.Text() != "España Ω" {
		t.Fatalf("got %q", field.Text())
	}
	if field.cursor != 8 {
		t.Fatalf("cursor=%d, want 8 rune positions", field.cursor)
	}
}

func TestTextFieldEnforcesMaximumLength(t *testing.T) {
	field := NewTextField("name", 0, 0, 200, 40, "", 16)
	field.SetMaxLength(5)
	field.SetText("123456789")
	if field.Text() != "12345" {
		t.Fatalf("got %q, want %q", field.Text(), "12345")
	}
}

func TestTextFieldFilter(t *testing.T) {
	field := NewTextField("number", 0, 0, 200, 40, "", 16)
	field.SetFilter(unicode.IsDigit)
	field.SetText("14.261 MHz")
	if field.Text() != "14261" {
		t.Fatalf("got %q, want digits only", field.Text())
	}
}

func TestTextFieldReplacesSelection(t *testing.T) {
	field := NewTextField("name", 0, 0, 200, 40, "", 16)
	field.SetText("abcdef")
	field.anchor, field.cursor = 2, 5
	field.replaceSelection([]rune("XY"), false)
	if field.Text() != "abXYf" {
		t.Fatalf("got %q, want %q", field.Text(), "abXYf")
	}
}

func TestDisabledTextFieldCannotReceiveFocus(t *testing.T) {
	field := NewTextField("name", 0, 0, 200, 40, "", 16)
	field.SetEnabled(false)
	field.SetFocused(true)
	if field.Focused() {
		t.Fatal("disabled text field received focus")
	}
}

func TestTextFieldCustomBorderCanBeSetAndCleared(t *testing.T) {
	field := NewTextField("name", 0, 0, 200, 40, "", 16)
	field.SetBorderColor(rl.Red)
	if !field.customBorder || field.borderColor != rl.Red {
		t.Fatalf("custom border was not stored: %+v", field.borderColor)
	}
	field.ClearBorderColor()
	if field.customBorder {
		t.Fatal("custom border was not cleared")
	}
}
