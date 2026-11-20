package httpdelivery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/adnope/ephemeral/internal/domain"
	"github.com/adnope/ephemeral/internal/infrastructure/sqlite"
	"github.com/adnope/ephemeral/internal/middleware"
	"github.com/adnope/ephemeral/internal/migrations"
	"github.com/adnope/ephemeral/internal/usecase"
	"github.com/go-chi/chi/v5"
)

type editEventRecorder struct{ events []domain.Event }

func (b *editEventRecorder) Broadcast(event domain.Event) { b.events = append(b.events, event) }

func TestEditMessageAPI(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.OpenDB(t.TempDir()+"/edit.db", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if err := migrations.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}
	repo := sqlite.NewItemRepository(db)
	sessions := sqlite.NewSessionRepository(db)
	auth := usecase.NewAuthUseCase(sqlite.NewUserRepository(db), sessions, time.Hour)
	login, err := auth.Login(ctx, "owner", "password")
	if err != nil {
		t.Fatal(err)
	}
	broker := &editEventRecorder{}
	uc := usecase.NewItemUseCase(repo, nil, broker, nil, nil, nil, nil, nil)
	h := NewHandler(uc, nil, auth, nil, nil, HandlerSettings{ChatPageSize: 10})
	router := chi.NewRouter()
	router.Use(middleware.SessionAuth(sessions, time.Hour, false))
	router.Post("/api/message", h.Message)
	router.Patch("/api/items/{id}", h.EditMessage)
	router.Get("/api/items/{id}", h.Item)
	router.Get("/api/items", h.Items)
	request := func(method, path, body, contentType, token string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Accept", "application/json")
		r.Header.Set("Content-Type", contentType)
		if token != "" {
			r.AddCookie(&http.Cookie{Name: "session_token", Value: token})
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	decodeItem := func(w *httptest.ResponseRecorder) itemResponse {
		t.Helper()
		if w.Code != 200 {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var item itemResponse
		if err := json.Unmarshal(w.Body.Bytes(), &item); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(w.Body.String(), `"editedAtEpochMillis":`) {
			t.Fatal("edit timestamp field missing")
		}
		return item
	}
	created := decodeItem(request("POST", "/api/message", `{"text":"original"}`, "application/json", login.Token))
	if created.EditedAtEpochMillis != nil {
		t.Fatal("new text is marked edited")
	}
	path := fmt.Sprintf("/api/items/%d", created.ID)
	original := decodeItem(request("GET", path, "", "application/json", login.Token))
	broker.events = nil
	unchanged := decodeItem(request("PATCH", path, `{"text":"  original\n"}`, "application/json", login.Token))
	if unchanged.EditedAtEpochMillis != nil || len(broker.events) != 0 {
		t.Fatal("no-op marked item edited or broadcast an event")
	}
	before := time.Now().UnixMilli()
	edited := decodeItem(request("PATCH", path, `{"text":"  edited 日本語\n "}`, "application/json", login.Token))
	if edited.Text != "edited 日本語" || edited.EditedAtEpochMillis == nil || *edited.EditedAtEpochMillis < before || *edited.EditedAtEpochMillis > time.Now().UnixMilli() {
		t.Fatalf("edited item: %+v", edited)
	}
	if edited.CreatedAtEpochMillis != original.CreatedAtEpochMillis || edited.ID != original.ID {
		t.Fatal("creation timestamp or ID changed")
	}
	if len(broker.events) != 1 || broker.events[0] != (domain.Event{Type: "item:updated", ID: created.ID}) {
		t.Fatalf("events=%+v", broker.events)
	}
	unchanged = decodeItem(request("PATCH", path, `{"text":"edited 日本語"}`, "application/json", login.Token))
	if unchanged.EditedAtEpochMillis == nil || *unchanged.EditedAtEpochMillis != *edited.EditedAtEpochMillis || len(broker.events) != 1 {
		t.Fatal("no-op changed timestamp or emitted an event")
	}
	fetched := decodeItem(request("GET", path, "", "application/json", login.Token))
	if fetched.Text != edited.Text || fetched.EditedAtEpochMillis == nil || *fetched.EditedAtEpochMillis != *edited.EditedAtEpochMillis {
		t.Fatalf("fetched=%+v", fetched)
	}
	w := request("GET", "/api/items", "", "application/json", login.Token)
	var page pageResponse
	if w.Code != 200 {
		t.Fatalf("list=%d %s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].EditedAtEpochMillis == nil || *page.Items[0].EditedAtEpochMillis != *edited.EditedAtEpochMillis {
		t.Fatalf("page=%+v", page)
	}
	mediaID, err := repo.Create(ctx, &domain.Item{Type: domain.ItemTypeFile, Content: "example.txt"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, path, body, contentType, token, code string
		status                                     int
	}{
		{"no session", path, `{"text":"new"}`, "application/json", "", "unauthenticated", 401},
		{"invalid session", path, `{"text":"new"}`, "application/json", "invalid", "unauthenticated", 401},
		{"invalid id", "/api/items/nope", `{"text":"new"}`, "application/json", login.Token, "validation_error", 400},
		{"zero id", "/api/items/0", `{"text":"new"}`, "application/json", login.Token, "validation_error", 400},
		{"negative id", "/api/items/-1", `{"text":"new"}`, "application/json", login.Token, "validation_error", 400},
		{"overflow id", "/api/items/999999999999999999999", `{"text":"new"}`, "application/json", login.Token, "validation_error", 400},
		{"missing item", "/api/items/99999", `{"text":"new"}`, "application/json", login.Token, "not_found", 404},
		{"file item", fmt.Sprintf("/api/items/%d", mediaID), `{"text":"new"}`, "application/json", login.Token, "unsupported_edit", 415},
		{"form body", path, "text=new", "application/x-www-form-urlencoded", login.Token, "unsupported_media_type", 415},
		{"missing text", path, `{}`, "application/json", login.Token, "validation_error", 400},
		{"null text", path, `{"text":null}`, "application/json", login.Token, "validation_error", 400},
		{"blank text", path, `{"text":" \n\t "}`, "application/json", login.Token, "validation_error", 400},
		{"invalid type", path, `{"text":12}`, "application/json", login.Token, "validation_error", 400},
		{"malformed JSON", path, `{`, "application/json", login.Token, "validation_error", 400},
		{"unknown field", path, `{"text":"new","editedAtEpochMillis":1}`, "application/json", login.Token, "validation_error", 400},
		{"trailing JSON", path, `{"text":"new"} {}`, "application/json", login.Token, "validation_error", 400},
		{"oversized JSON", path, `{"text":"` + strings.Repeat("a", maxJSONBodyBytes) + `"}`, "application/json", login.Token, "payload_too_large", 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := request("PATCH", tc.path, tc.body, tc.contentType, tc.token)
			var failure jsonErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &failure); err != nil {
				t.Fatal(err)
			}
			if w.Code != tc.status || failure.Code != tc.code {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if len(broker.events) != 1 {
				t.Fatal("failed edit broadcast an event")
			}
		})
	}
	// Persistence failures stay server errors and must not broadcast success.
	if _, err := db.Exec(`CREATE TRIGGER fail_edit BEFORE UPDATE OF content ON items BEGIN SELECT RAISE(ABORT, 'edit failure'); END`); err != nil {
		t.Fatal(err)
	}
	w = request("PATCH", path, `{"text":"failed edit"}`, "application/json", login.Token)
	if w.Code != 500 || !strings.Contains(w.Body.String(), `"server_error"`) || len(broker.events) != 1 {
		t.Fatalf("failed edit: status=%d body=%s events=%v", w.Code, w.Body.String(), broker.events)
	}
	fetched = decodeItem(request("GET", path, "", "application/json", login.Token))
	if fetched.Text != edited.Text || fetched.EditedAtEpochMillis == nil || *fetched.EditedAtEpochMillis != *edited.EditedAtEpochMillis {
		t.Fatal("failed edit changed persisted data")
	}
}
