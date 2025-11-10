// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package navigationbar

import (
	"testing"

	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
)

func TestNewNavigationBar(t *testing.T) {
	navbar := NewNavigationBar()

	if navbar == nil {
		t.Fatal("NewNavigationBar() returned nil")
	}

	if navbar.Box == nil {
		t.Error("NavigationBar.Box should not be nil")
	}
}

func TestNavigationBar_SetAlign(t *testing.T) {
	navbar := NewNavigationBar()

	result := navbar.SetAlign(tview.AlignCenter)

	if result != navbar {
		t.Error("SetAlign() should return the same NavigationBar instance for chaining")
	}

	if navbar.align != tview.AlignCenter {
		t.Errorf("expected align to be AlignCenter, got %d", navbar.align)
	}
}

func TestNavigationBar_SetLabelColor(t *testing.T) {
	navbar := NewNavigationBar()
	testColor := tcell.ColorRed

	result := navbar.SetLabelColor(testColor)

	if result != navbar {
		t.Error("SetLabelColor() should return the same NavigationBar instance for chaining")
	}

	if navbar.labelColor != testColor {
		t.Errorf("expected labelColor to be %v, got %v", testColor, navbar.labelColor)
	}
}

func TestNavigationBar_SetLabelColorActivated(t *testing.T) {
	navbar := NewNavigationBar()
	testColor := tcell.ColorGreen

	result := navbar.SetLabelColorActivated(testColor)

	if result != navbar {
		t.Error("SetLabelColorActivated() should return the same NavigationBar instance for chaining")
	}

	if navbar.labelColorActivated != testColor {
		t.Errorf("expected labelColorActivated to be %v, got %v", testColor, navbar.labelColorActivated)
	}
}

func TestNavigationBar_SetNavBackgroundColor(t *testing.T) {
	navbar := NewNavigationBar()
	testColor := tcell.ColorBlue

	result := navbar.SetNavBackgroundColor(testColor)

	if result != navbar {
		t.Error("SetNavBackgroundColor() should return the same NavigationBar instance for chaining")
	}

	if navbar.backgroundColor != testColor {
		t.Errorf("expected backgroundColor to be %v, got %v", testColor, navbar.backgroundColor)
	}
}

func TestNavigationBar_SetNavBackgroundColorActivated(t *testing.T) {
	navbar := NewNavigationBar()
	testColor := tcell.ColorYellow

	result := navbar.SetBackgroundColorActivated(testColor)

	if result != navbar {
		t.Error("SetBackgroundColorActivated() should return the same NavigationBar instance for chaining")
	}

	if navbar.backgroundColorActivated != testColor {
		t.Errorf("expected backgroundColorActivated to be %v, got %v", testColor, navbar.backgroundColorActivated)
	}
}

func TestNavigationBar_SetUserFeedback(t *testing.T) {
	navbar := NewNavigationBar()
	feedback := "Test feedback message"
	testColor := tcell.ColorRed

	navbar.SetUserFeedback(feedback, testColor)

	if navbar.feedback == "" {
		t.Error("expected feedback to be set")
	}
	if navbar.feedbackColor != testColor {
		t.Errorf("expected feedbackColor to be %v, got %v", testColor, navbar.feedbackColor)
	}
}

func TestNavigationBar_AddButton(t *testing.T) {
	navbar := NewNavigationBar()

	result := navbar.AddButton("Test Button", nil)

	if result != navbar {
		t.Error("AddButton() should return the same NavigationBar instance for chaining")
	}

	if len(navbar.buttons) != 1 {
		t.Errorf("expected 1 button, got %d", len(navbar.buttons))
	}
}

func TestNavigationBar_AddMultipleButtons(t *testing.T) {
	navbar := NewNavigationBar()

	navbar.AddButton("Button 1", nil)
	navbar.AddButton("Button 2", nil)
	navbar.AddButton("Button 3", nil)

	if len(navbar.buttons) != 3 {
		t.Errorf("expected 3 buttons, got %d", len(navbar.buttons))
	}
}

func TestNavigationBar_SetSelectedButton(t *testing.T) {
	navbar := NewNavigationBar()
	navbar.AddButton("Button 1", nil)
	navbar.AddButton("Button 2", nil)

	result := navbar.SetSelectedButton(1)

	if result != navbar {
		t.Error("SetSelectedButton() should return the same NavigationBar instance for chaining")
	}

	if navbar.selectedButton != 1 {
		t.Errorf("expected selectedButton to be 1, got %d", navbar.selectedButton)
	}
}

func TestNavigationBar_ClearFeedback(t *testing.T) {
	navbar := NewNavigationBar()
	navbar.SetUserFeedback("Test feedback", tcell.ColorRed)

	navbar.ClearUserFeedback()

	if navbar.feedback != "" {
		t.Errorf("expected feedback to be empty after ClearUserFeedback(), got %q", navbar.feedback)
	}
}

func TestNavigationBar_GetHeight(t *testing.T) {
	navbar := NewNavigationBar()

	height := navbar.GetHeight()

	// Height should be a positive value
	if height <= 0 {
		t.Errorf("expected positive height, got %d", height)
	}
}

func TestNavigationBar_SetFinishedFunc(t *testing.T) {
	navbar := NewNavigationBar()
	called := false

	callback := func(key tcell.Key) {
		called = true
	}

	result := navbar.SetFinishedFunc(callback)

	if result != navbar {
		t.Error("SetFinishedFunc() should return the same NavigationBar instance for chaining")
	}

	// Test that callback was set
	if navbar.onFinished == nil {
		t.Error("expected onFinished callback to be set")
	}

	// Call the callback
	navbar.onFinished(tcell.KeyEnter)
	if !called {
		t.Error("expected callback to be called")
	}
}

func TestNavigationBar_SetFocusFunc(t *testing.T) {
	navbar := NewNavigationBar()
	called := false

	callback := func() {
		called = true
	}

	result := navbar.SetOnFocusFunc(callback)

	if result != navbar {
		t.Error("SetOnFocusFunc() should return the same NavigationBar instance for chaining")
	}

	if navbar.onFocus == nil {
		t.Error("expected onFocus callback to be set")
	}

	navbar.onFocus()
	if !called {
		t.Error("expected callback to be called")
	}
}

func TestNavigationBar_SetBlurFunc(t *testing.T) {
	navbar := NewNavigationBar()
	called := false

	callback := func() {
		called = true
	}

	result := navbar.SetOnBlurFunc(callback)

	if result != navbar {
		t.Error("SetOnBlurFunc() should return the same NavigationBar instance for chaining")
	}

	if navbar.onBlur == nil {
		t.Error("expected onBlur callback to be set")
	}

	navbar.onBlur()
	if !called {
		t.Error("expected callback to be called")
	}
}

func TestNavigationBar_MethodChaining(t *testing.T) {
	navbar := NewNavigationBar()

	// Test method chaining
	result := navbar.
		SetAlign(tview.AlignCenter).
		SetLabelColor(tcell.ColorRed).
		SetNavBackgroundColor(tcell.ColorBlue).
		AddButton("Test", nil)

	if result != navbar {
		t.Error("method chaining should return the same NavigationBar instance")
	}

	// Verify values were set
	if navbar.align != tview.AlignCenter {
		t.Error("align not set correctly during method chaining")
	}
	if navbar.labelColor != tcell.ColorRed {
		t.Error("labelColor not set correctly during method chaining")
	}
	if navbar.backgroundColor != tcell.ColorBlue {
		t.Error("backgroundColor not set correctly during method chaining")
	}
	if len(navbar.buttons) != 1 {
		t.Error("button not added correctly during method chaining")
	}
}
