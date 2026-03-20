package render

import (
	"strings"
	"testing"
)

func TestSettingsStartEdit(t *testing.T) {
	s := newSettingsModel("code", []string{"/code"}, 20)
	s.startEdit()

	f := s.fields[s.cursor]
	if !f.editing {
		t.Error("expected editing = true after startEdit")
	}
	if f.editBuf != "code" {
		t.Errorf("editBuf = %q, want %q (copied from value)", f.editBuf, "code")
	}
}

func TestSettingsCancelEdit(t *testing.T) {
	s := newSettingsModel("code", []string{"/code"}, 20)
	s.startEdit()
	s.typeChar("x")
	s.cancelEdit()

	f := s.fields[s.cursor]
	if f.editing {
		t.Error("expected editing = false after cancelEdit")
	}
	if f.editBuf != "" {
		t.Errorf("editBuf = %q, want empty after cancelEdit", f.editBuf)
	}
	if f.value != "code" {
		t.Errorf("value = %q, want %q (unchanged after cancel)", f.value, "code")
	}
}

func TestSettingsConfirmEdit(t *testing.T) {
	s := newSettingsModel("code", []string{"/code"}, 20)
	s.startEdit()

	// Clear and type new value
	s.fields[s.cursor].editBuf = "vim"
	s.confirmEdit()

	f := s.fields[s.cursor]
	if f.editing {
		t.Error("expected editing = false after confirmEdit")
	}
	if f.value != "vim" {
		t.Errorf("value = %q, want %q after confirmEdit", f.value, "vim")
	}
	if f.editBuf != "" {
		t.Errorf("editBuf = %q, want empty after confirmEdit", f.editBuf)
	}
}

func TestSettingsTypeChar(t *testing.T) {
	s := newSettingsModel("", nil, 20)
	s.startEdit()
	s.typeChar("a")
	s.typeChar("b")
	s.typeChar("c")

	if s.fields[0].editBuf != "abc" {
		t.Errorf("editBuf = %q, want %q", s.fields[0].editBuf, "abc")
	}
}

func TestSettingsTypeCharIgnoredWhenNotEditing(t *testing.T) {
	s := newSettingsModel("code", nil, 20)
	s.typeChar("x") // not editing, should be ignored

	if s.fields[0].editBuf != "" {
		t.Errorf("editBuf = %q, want empty (typeChar should be ignored when not editing)", s.fields[0].editBuf)
	}
}

func TestSettingsBackspace(t *testing.T) {
	s := newSettingsModel("", nil, 20)
	s.startEdit()
	s.typeChar("a")
	s.typeChar("b")
	s.typeChar("c")
	s.backspace()

	if s.fields[0].editBuf != "ab" {
		t.Errorf("editBuf = %q, want %q after backspace", s.fields[0].editBuf, "ab")
	}
}

func TestSettingsBackspaceOnEmpty(t *testing.T) {
	s := newSettingsModel("", nil, 20)
	s.startEdit()
	s.backspace() // should not panic on empty string

	if s.fields[0].editBuf != "" {
		t.Errorf("editBuf = %q, want empty after backspace on empty", s.fields[0].editBuf)
	}
}

func TestSettingsBackspaceUnicode(t *testing.T) {
	s := newSettingsModel("", nil, 20)
	s.startEdit()
	s.fields[0].editBuf = "café"
	s.backspace()

	if s.fields[0].editBuf != "caf" {
		t.Errorf("editBuf = %q, want %q after unicode backspace", s.fields[0].editBuf, "caf")
	}
}

func TestSettingsIsEditing(t *testing.T) {
	s := newSettingsModel("code", nil, 20)
	if s.isEditing() {
		t.Error("isEditing should be false initially")
	}
	s.startEdit()
	if !s.isEditing() {
		t.Error("isEditing should be true after startEdit")
	}
	s.cancelEdit()
	if s.isEditing() {
		t.Error("isEditing should be false after cancelEdit")
	}
}

func TestSettingsEditFlow(t *testing.T) {
	s := newSettingsModel("code", []string{"/a", "/b"}, 20)

	// Navigate to "Base dirs"
	s.moveDown()
	if s.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", s.cursor)
	}

	// Start editing, type new value, confirm
	s.startEdit()
	s.fields[s.cursor].editBuf = "/code, /projects"
	s.confirmEdit()

	if s.fields[1].value != "/code, /projects" {
		t.Errorf("value = %q, want %q", s.fields[1].value, "/code, /projects")
	}
}

func TestSettingsViewEditingState(t *testing.T) {
	s := newSettingsModel("code", []string{"/code"}, 20)
	// Non-editing: should show "edit" help
	view := s.view()
	if !strings.Contains(view, "edit") {
		t.Error("non-editing view should contain 'edit' help text")
	}

	// Editing: should show "save" and "cancel" help
	s.startEdit()
	view = s.view()
	if !strings.Contains(view, "save") {
		t.Error("editing view should contain 'save' help text")
	}
	if !strings.Contains(view, "cancel") {
		t.Error("editing view should contain 'cancel' help text")
	}
}

func TestSettingsViewEmptyValue(t *testing.T) {
	s := newSettingsModel("", nil, 20)
	view := s.view()
	if !strings.Contains(view, "(not set)") {
		t.Error("empty value should show '(not set)' placeholder")
	}
}

func TestSettingsViewContainsLabels(t *testing.T) {
	s := newSettingsModel("code", []string{"/code"}, 20)
	view := s.view()

	for _, label := range []string{"Editor", "Base dirs", "Max items"} {
		if !strings.Contains(view, label) {
			t.Errorf("view should contain label %q", label)
		}
	}
}
