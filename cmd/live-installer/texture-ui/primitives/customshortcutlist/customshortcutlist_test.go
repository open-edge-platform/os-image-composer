// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package customshortcutlist

import (
	"testing"

	"github.com/gdamore/tcell"
)

func TestNewList(t *testing.T) {
	list := NewList()

	if list == nil {
		t.Fatal("NewList() returned nil")
	}

	if list.Box == nil {
		t.Error("List.Box should not be nil")
	}

	// Check default values
	if !list.showSecondaryText {
		t.Error("expected showSecondaryText to be true by default")
	}

	if !list.wrapAround {
		t.Error("expected wrapAround to be true by default")
	}

	if list.currentItem != 0 {
		t.Errorf("expected currentItem to be 0, got %d", list.currentItem)
	}
}

func TestList_AddItem(t *testing.T) {
	list := NewList()

	result := list.AddItem("Item 1", "Secondary text", 'a', nil)

	if result != list {
		t.Error("AddItem() should return the same List instance for chaining")
	}

	if list.GetItemCount() != 1 {
		t.Errorf("expected 1 item, got %d", list.GetItemCount())
	}
}

func TestList_AddMultipleItems(t *testing.T) {
	list := NewList()

	list.AddItem("Item 1", "", 0, nil)
	list.AddItem("Item 2", "", 0, nil)
	list.AddItem("Item 3", "", 0, nil)

	if list.GetItemCount() != 3 {
		t.Errorf("expected 3 items, got %d", list.GetItemCount())
	}
}

func TestList_GetItemCount(t *testing.T) {
	list := NewList()

	// Empty list
	if list.GetItemCount() != 0 {
		t.Errorf("expected 0 items in empty list, got %d", list.GetItemCount())
	}

	// After adding items
	list.AddItem("Item 1", "", 0, nil)
	list.AddItem("Item 2", "", 0, nil)

	if list.GetItemCount() != 2 {
		t.Errorf("expected 2 items, got %d", list.GetItemCount())
	}
}

func TestList_GetItemText(t *testing.T) {
	list := NewList()
	list.AddItem("Main Text", "Secondary Text", 'a', nil)

	mainText, secondaryText := list.GetItemText(0)

	if mainText != "Main Text" {
		t.Errorf("expected mainText to be 'Main Text', got %q", mainText)
	}

	if secondaryText != "Secondary Text" {
		t.Errorf("expected secondaryText to be 'Secondary Text', got %q", secondaryText)
	}
}

func TestList_SetCurrentItem(t *testing.T) {
	list := NewList()
	list.AddItem("Item 1", "", 0, nil)
	list.AddItem("Item 2", "", 0, nil)
	list.AddItem("Item 3", "", 0, nil)

	result := list.SetCurrentItem(1)

	if result != list {
		t.Error("SetCurrentItem() should return the same List instance for chaining")
	}

	if list.GetCurrentItem() != 1 {
		t.Errorf("expected current item to be 1, got %d", list.GetCurrentItem())
	}
}

func TestList_SetCurrentItem_NegativeIndex(t *testing.T) {
	list := NewList()
	list.AddItem("Item 1", "", 0, nil)
	list.AddItem("Item 2", "", 0, nil)
	list.AddItem("Item 3", "", 0, nil)

	// -1 should select the last item
	list.SetCurrentItem(-1)

	if list.GetCurrentItem() != 2 {
		t.Errorf("expected current item to be 2 (last), got %d", list.GetCurrentItem())
	}
}

func TestList_SetCurrentItem_OutOfRange(t *testing.T) {
	list := NewList()
	list.AddItem("Item 1", "", 0, nil)
	list.AddItem("Item 2", "", 0, nil)

	// Index beyond range should clamp to last item
	list.SetCurrentItem(100)

	if list.GetCurrentItem() != 1 {
		t.Errorf("expected current item to be clamped to 1, got %d", list.GetCurrentItem())
	}
}

func TestList_GetCurrentItem(t *testing.T) {
	list := NewList()
	list.AddItem("Item 1", "", 0, nil)
	list.AddItem("Item 2", "", 0, nil)

	// Default should be 0
	if list.GetCurrentItem() != 0 {
		t.Errorf("expected current item to be 0, got %d", list.GetCurrentItem())
	}

	list.SetCurrentItem(1)

	if list.GetCurrentItem() != 1 {
		t.Errorf("expected current item to be 1, got %d", list.GetCurrentItem())
	}
}

func TestList_RemoveItem(t *testing.T) {
	list := NewList()
	list.AddItem("Item 1", "", 0, nil)
	list.AddItem("Item 2", "", 0, nil)
	list.AddItem("Item 3", "", 0, nil)

	result := list.RemoveItem(1)

	if result != list {
		t.Error("RemoveItem() should return the same List instance for chaining")
	}

	if list.GetItemCount() != 2 {
		t.Errorf("expected 2 items after removal, got %d", list.GetItemCount())
	}

	// Verify correct item was removed
	mainText, _ := list.GetItemText(1)
	if mainText != "Item 3" {
		t.Errorf("expected second item to be 'Item 3' after removal, got %q", mainText)
	}
}

func TestList_RemoveItem_EmptyList(t *testing.T) {
	list := NewList()

	// Should not panic on empty list
	list.RemoveItem(0)

	if list.GetItemCount() != 0 {
		t.Errorf("expected list to remain empty, got %d items", list.GetItemCount())
	}
}

func TestList_Clear(t *testing.T) {
	list := NewList()
	list.AddItem("Item 1", "", 0, nil)
	list.AddItem("Item 2", "", 0, nil)
	list.AddItem("Item 3", "", 0, nil)

	result := list.Clear()

	if result != list {
		t.Error("Clear() should return the same List instance for chaining")
	}

	if list.GetItemCount() != 0 {
		t.Errorf("expected 0 items after Clear(), got %d", list.GetItemCount())
	}
}

func TestList_SetMainTextColor(t *testing.T) {
	list := NewList()
	testColor := tcell.ColorRed

	result := list.SetMainTextColor(testColor)

	if result != list {
		t.Error("SetMainTextColor() should return the same List instance for chaining")
	}

	if list.mainTextColor != testColor {
		t.Errorf("expected mainTextColor to be %v, got %v", testColor, list.mainTextColor)
	}
}

func TestList_SetSecondaryTextColor(t *testing.T) {
	list := NewList()
	testColor := tcell.ColorGreen

	result := list.SetSecondaryTextColor(testColor)

	if result != list {
		t.Error("SetSecondaryTextColor() should return the same List instance for chaining")
	}

	if list.secondaryTextColor != testColor {
		t.Errorf("expected secondaryTextColor to be %v, got %v", testColor, list.secondaryTextColor)
	}
}

func TestList_SetShortcutColor(t *testing.T) {
	list := NewList()
	testColor := tcell.ColorBlue

	result := list.SetShortcutColor(testColor)

	if result != list {
		t.Error("SetShortcutColor() should return the same List instance for chaining")
	}

	if list.shortcutColor != testColor {
		t.Errorf("expected shortcutColor to be %v, got %v", testColor, list.shortcutColor)
	}
}

func TestList_SetSelectedTextColor(t *testing.T) {
	list := NewList()
	testColor := tcell.ColorYellow

	result := list.SetSelectedTextColor(testColor)

	if result != list {
		t.Error("SetSelectedTextColor() should return the same List instance for chaining")
	}

	if list.selectedTextColor != testColor {
		t.Errorf("expected selectedTextColor to be %v, got %v", testColor, list.selectedTextColor)
	}
}

func TestList_SetSelectedBackgroundColor(t *testing.T) {
	list := NewList()
	testColor := tcell.ColorBlue

	result := list.SetSelectedBackgroundColor(testColor)

	if result != list {
		t.Error("SetSelectedBackgroundColor() should return the same List instance for chaining")
	}

	if list.selectedBackgroundColor != testColor {
		t.Errorf("expected selectedBackgroundColor to be %v, got %v", testColor, list.selectedBackgroundColor)
	}
}

func TestList_ShowSecondaryText(t *testing.T) {
	list := NewList()

	result := list.ShowSecondaryText(false)

	if result != list {
		t.Error("ShowSecondaryText() should return the same List instance for chaining")
	}

	if list.showSecondaryText {
		t.Error("expected showSecondaryText to be false")
	}
}

func TestList_SetSelectedFunc(t *testing.T) {
	list := NewList()
	called := false
	var capturedIndex int

	callback := func(index int, mainText, secondaryText string, shortcut rune) {
		called = true
		capturedIndex = index
	}

	result := list.SetSelectedFunc(callback)

	if result != list {
		t.Error("SetSelectedFunc() should return the same List instance for chaining")
	}

	if list.selected == nil {
		t.Error("expected selected callback to be set")
	}

	// Trigger callback
	list.selected(1, "Test", "", 'a')
	if !called {
		t.Error("expected callback to be called")
	}
	if capturedIndex != 1 {
		t.Errorf("expected captured index to be 1, got %d", capturedIndex)
	}
}

func TestList_SetChangedFunc(t *testing.T) {
	list := NewList()
	called := false

	callback := func(index int, mainText, secondaryText string, shortcut rune) {
		called = true
	}

	result := list.SetChangedFunc(callback)

	if result != list {
		t.Error("SetChangedFunc() should return the same List instance for chaining")
	}

	// Add items and change selection to trigger callback
	list.AddItem("Item 1", "", 0, nil)
	list.AddItem("Item 2", "", 0, nil)
	list.SetCurrentItem(1)

	if !called {
		t.Error("expected changed callback to be called when selection changes")
	}
}

func TestList_SetDoneFunc(t *testing.T) {
	list := NewList()
	called := false

	callback := func() {
		called = true
	}

	result := list.SetDoneFunc(callback)

	if result != list {
		t.Error("SetDoneFunc() should return the same List instance for chaining")
	}

	if list.done == nil {
		t.Error("expected done callback to be set")
	}

	list.done()
	if !called {
		t.Error("expected callback to be called")
	}
}

func TestList_MethodChaining(t *testing.T) {
	list := NewList()

	result := list.
		SetMainTextColor(tcell.ColorRed).
		SetSecondaryTextColor(tcell.ColorGreen).
		SetShortcutColor(tcell.ColorBlue).
		ShowSecondaryText(false).
		AddItem("Test", "", 0, nil)

	if result != list {
		t.Error("method chaining should return the same List instance")
	}

	// Verify values were set
	if list.mainTextColor != tcell.ColorRed {
		t.Error("mainTextColor not set correctly during method chaining")
	}
	if list.secondaryTextColor != tcell.ColorGreen {
		t.Error("secondaryTextColor not set correctly during method chaining")
	}
	if list.shortcutColor != tcell.ColorBlue {
		t.Error("shortcutColor not set correctly during method chaining")
	}
	if list.showSecondaryText {
		t.Error("showSecondaryText not set correctly during method chaining")
	}
	if list.GetItemCount() != 1 {
		t.Error("item not added correctly during method chaining")
	}
}
