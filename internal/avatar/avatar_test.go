package avatar_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/avatar"
	"github.com/kazuto/parlance-server/internal/avatar/avatartest"
)

// png is the start of a PNG file, enough to be recognized as one.
var png = append([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), make([]byte, 64)...)

const userID = "3f2b4c1e-6a5d-4e7f-8a9b-0c1d2e3f4a5b"

func serve(store *avatar.Store, path string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.Handle("GET /avatars/{id}", store)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

	return recorder
}

func TestSaveAndServe(t *testing.T) {
	storage := avatartest.NewMemory()
	store := avatar.New(storage)

	address, err := store.Save(context.Background(), userID, png)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if want := "/avatars/" + userID + "?v="; len(address) <= len(want) || address[:len(want)] != want {
		t.Errorf("address = %q, want %s<version>", address, want)
	}

	res := serve(store, address)
	if res.Code != http.StatusOK || res.Body.Len() != len(png) || res.Header().Get("Content-Type") != "image/png" ||
		res.Header().Get("X-Content-Type-Options") != "nosniff" || res.Header().Get("Content-Security-Policy") != "default-src 'none'" {
		t.Errorf("served %d %v, %d bytes", res.Code, res.Header(), res.Body.Len())
	}

	if err := store.Remove(context.Background(), userID); err != nil || storage.Has("avatars/"+userID) {
		t.Errorf("Remove() error = %v, stored = %v", err, storage.Has("avatars/"+userID))
	}

	if res := serve(store, "/avatars/"+userID); res.Code != http.StatusNotFound {
		t.Errorf("removed avatar: %d, want 404", res.Code)
	}
}

func TestSaveRejectsWhatIsNoImage(t *testing.T) {
	store := avatar.New(avatartest.NewMemory())

	for name, data := range map[string][]byte{
		"html":     []byte("<html><script>alert(1)</script></html>"),
		"svg":      []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"too big":  append(append([]byte{}, png...), make([]byte, avatar.MaxSize)...),
		"anything": []byte("plain text"),
	} {
		if _, err := store.Save(context.Background(), userID, data); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: error = %v, want invalid argument", name, err)
		}
	}
}

func TestServeRefusesOtherPaths(t *testing.T) {
	store := avatar.New(avatartest.NewMemory())

	for _, path := range []string{"/avatars/not-a..id", "/avatars/%2e%2e%2fsecret", "/avatars/" + userID} {
		if res := serve(store, path); res.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", path, res.Code)
		}
	}
}

func TestWithoutStorage(t *testing.T) {
	store := avatar.New(nil)

	if _, err := store.Save(context.Background(), userID, png); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("Save() error = %v, want failed precondition", err)
	}

	if err := store.Remove(context.Background(), userID); err != nil {
		t.Errorf("Remove() error = %v", err)
	}

	if res := serve(store, "/avatars/"+userID); res.Code != http.StatusNotFound {
		t.Errorf("served %d, want 404", res.Code)
	}
}
