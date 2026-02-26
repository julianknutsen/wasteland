package hosted

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSessionStore_CreateAndGet(t *testing.T) {
	store := NewSessionStore()
	id := store.Create("conn-123")
	if id == "" {
		t.Fatal("expected non-empty session ID")
	}

	sess, ok := store.Get(id)
	if !ok {
		t.Fatal("expected session to exist")
	}
	if sess.ConnectionID != "conn-123" {
		t.Errorf("expected conn-123, got %s", sess.ConnectionID)
	}
	if sess.ID != id {
		t.Errorf("expected ID %s, got %s", id, sess.ID)
	}
}

func TestSessionStore_GetMissing(t *testing.T) {
	store := NewSessionStore()
	_, ok := store.Get("nonexistent")
	if ok {
		t.Error("expected session not to exist")
	}
}

func TestSessionStore_Delete(t *testing.T) {
	store := NewSessionStore()
	id := store.Create("conn-123")
	store.Delete(id)

	_, ok := store.Get(id)
	if ok {
		t.Error("expected session to be deleted")
	}
}

func TestSessionStore_UniqueIDs(t *testing.T) {
	store := NewSessionStore()
	id1 := store.Create("conn-1")
	id2 := store.Create("conn-2")
	if id1 == id2 {
		t.Error("expected unique session IDs")
	}
}

func TestSignVerifySessionID(t *testing.T) {
	secret := "test-secret"
	id := "abc123"

	signed := SignSessionID(id, secret)
	if signed == id {
		t.Error("signed value should differ from plain ID")
	}

	got, ok := VerifySessionID(signed, secret)
	if !ok {
		t.Fatal("expected verification to succeed")
	}
	if got != id {
		t.Errorf("expected %s, got %s", id, got)
	}
}

func TestVerifySessionID_WrongSecret(t *testing.T) {
	signed := SignSessionID("abc123", "secret-1")
	_, ok := VerifySessionID(signed, "secret-2")
	if ok {
		t.Error("expected verification to fail with wrong secret")
	}
}

func TestVerifySessionID_Tampered(t *testing.T) {
	signed := SignSessionID("abc123", "secret")
	tampered := "tampered." + signed[len("abc123."):]
	_, ok := VerifySessionID(tampered, "secret")
	if ok {
		t.Error("expected verification to fail for tampered value")
	}
}

func TestVerifySessionID_Invalid(t *testing.T) {
	for _, val := range []string{"", "noseparator", ".leading", "trailing."} {
		_, ok := VerifySessionID(val, "secret")
		if ok {
			t.Errorf("expected verification to fail for %q", val)
		}
	}
}

func TestSetAndReadSessionCookie(t *testing.T) {
	secret := "cookie-secret"
	sessionID := "sess-42"

	w := httptest.NewRecorder()
	SetSessionCookie(w, sessionID, secret)

	// Extract the cookie from the response and put it in a request.
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != "wl_session" {
		t.Errorf("expected cookie name wl_session, got %s", cookie.Name)
	}
	if !cookie.HttpOnly {
		t.Error("expected HttpOnly cookie")
	}
	if !cookie.Secure {
		t.Error("expected Secure cookie")
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(cookie)

	got, ok := ReadSessionCookie(req, secret)
	if !ok {
		t.Fatal("expected cookie to be valid")
	}
	if got != sessionID {
		t.Errorf("expected %s, got %s", sessionID, got)
	}
}

func TestReadSessionCookie_Missing(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	_, ok := ReadSessionCookie(req, "secret")
	if ok {
		t.Error("expected no cookie")
	}
}

func TestClearSessionCookie(t *testing.T) {
	w := httptest.NewRecorder()
	ClearSessionCookie(w)
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].MaxAge != -1 {
		t.Errorf("expected MaxAge -1, got %d", cookies[0].MaxAge)
	}
}

func TestClearSessionCookie_OverwritesExisting(t *testing.T) {
	secret := "secret"
	w := httptest.NewRecorder()
	SetSessionCookie(w, "sess-1", secret)

	// Now clear it.
	w2 := httptest.NewRecorder()
	ClearSessionCookie(w2)

	// Build a request with just the cleared cookie.
	req := httptest.NewRequest("GET", "/", nil)
	for _, c := range w2.Result().Cookies() {
		req.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
	}

	_, ok := ReadSessionCookie(req, secret)
	if ok {
		t.Error("expected cleared cookie to fail verification")
	}
}
