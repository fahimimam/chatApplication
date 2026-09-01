package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestUserValidate(t *testing.T) {
	cases := []struct {
		name string
		u    User
		ok   bool
	}{
		{"ok", User{Username: "alice"}, true},
		{"empty", User{Username: ""}, false},
		{"spaces", User{Username: "alice bob"}, false},
		{"too long", User{Username: strings.Repeat("x", 65)}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.u.Validate()
			if tc.ok && err != nil {
				t.Fatalf("want ok, got %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("want error")
			}
		})
	}
}

func TestUserSafeViewHidesPassword(t *testing.T) {
	u := User{Username: "alice", PasswordHash: "secret"}
	v := u.SafeView()
	if v.PasswordHash != "" {
		t.Fatal("SafeView should drop password hash")
	}
	if v.Username != "alice" {
		t.Fatal("SafeView should preserve username")
	}
}

func TestConversationValidate(t *testing.T) {
	cases := []struct {
		name string
		c    Conversation
		ok   bool
	}{
		{"direct ok", Conversation{Kind: KindDirect}, true},
		{"group ok", Conversation{Kind: KindGroup, Title: "team"}, true},
		{"group needs title", Conversation{Kind: KindGroup}, false},
		{"bad kind", Conversation{Kind: "channel"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.c.Validate()
			if tc.ok && err != nil {
				t.Fatalf("want ok, got %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("want error")
			}
		})
	}
}

func TestMessageValidate(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		m    Message
		ok   bool
	}{
		{"ok", Message{ConversationID: "c1", SenderID: "u1", Kind: KindText, Content: "hi", CreatedAt: now}, true},
		{"no conv", Message{SenderID: "u1", Kind: KindText, Content: "hi", CreatedAt: now}, false},
		{"no sender", Message{ConversationID: "c1", Kind: KindText, Content: "hi", CreatedAt: now}, false},
		{"bad kind", Message{ConversationID: "c1", SenderID: "u1", Kind: "image", Content: "hi", CreatedAt: now}, false},
		{"empty", Message{ConversationID: "c1", SenderID: "u1", Kind: KindText, CreatedAt: now}, false},
		{"too long", Message{ConversationID: "c1", SenderID: "u1", Kind: KindText, Content: strings.Repeat("x", MaxContentLength+1), CreatedAt: now}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.m.Validate()
			if tc.ok && err != nil {
				t.Fatalf("want ok, got %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("want error")
			}
		})
	}
}

func TestValidationErrorIs(t *testing.T) {
	err := &ValidationError{Field: "x", Message: "y"}
	if !errors.Is(err, ErrValidation) {
		t.Fatal("ValidationError should satisfy errors.Is(ErrValidation)")
	}
}

func TestMessageEditedDeleted(t *testing.T) {
	now := time.Now()
	m := Message{CreatedAt: now}
	if m.Edited() || m.Deleted() {
		t.Fatal("fresh message should not be edited or deleted")
	}
	m.EditedAt = &now
	m.DeletedAt = &now
	if !m.Edited() || !m.Deleted() {
		t.Fatal("flags should reflect non-nil pointers")
	}
}
