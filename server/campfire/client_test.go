package campfire

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientStatusContentAndTime(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantStatus int
		wantMsg    string
	}{
		{
			name:       "code 303",
			body:       `{"errors":[{"message":"Something went wrong uploading your content. Please try again.","extensions":{"code":303,"display":true}}],"data":null}`,
			wantStatus: http.StatusBadRequest,
			wantMsg:    ContentRejectedMessage,
		},
		{
			name:       "invalid time",
			body:       `{"errors":[{"message":"invalid time","path":["createActivityReminder"]}],"data":null}`,
			wantStatus: http.StatusBadRequest,
			wantMsg:    StartInPastMessage,
		},
		{
			name:       "http 401",
			body:       "Invalid token\n",
			wantStatus: http.StatusUnauthorized,
			wantMsg:    "invalid Campfire token",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(tc.body, "Invalid token") {
					w.WriteHeader(http.StatusUnauthorized)
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(srv.Close)
			c := New(Config{URL: srv.URL, MaxRetries: 1})
			err := c.Do(context.Background(), "token", "query MeQuery { me { id } }", nil, nil)
			status, msg, ok := ClientStatus(err)
			if !ok || status != tc.wantStatus || msg != tc.wantMsg {
				t.Fatalf("err=%v status=%d msg=%q ok=%v", err, status, msg, ok)
			}
		})
	}
}

func TestWritesIgnoreCallerCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		q := string(body)
		var resp string
		switch {
		case strings.Contains(q, "createActivityReminder"):
			resp = `{"data":{"createActivityReminder":{"event":{"id":"e"}}}}`
		case strings.Contains(q, "editEvent"):
			resp = `{"data":{"editEvent":{"event":{"id":"e"}}}}`
		case strings.Contains(q, "deleteEvent"):
			resp = `{"data":{"deleteEvent":{"success":true}}}`
		case strings.Contains(q, "createLink"):
			resp = `{"data":{"createLink":{"link":{"url":"https://cmpf.re/abc"}}}}`
		default:
			http.Error(w, "unexpected query", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(resp))
	}))
	t.Cleanup(srv.Close)
	c := New(Config{URL: srv.URL, MaxRetries: 1})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := c.Clubs(ctx, "token"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Clubs err=%v, reads should still honor cancel", err)
	}
	if _, err := c.CreateActivityReminder(ctx, "token", CreateActivityReminderInput{ClubID: "c", Name: "n"}); err != nil {
		t.Fatalf("CreateActivityReminder: %v", err)
	}
	if _, err := c.EditEvent(ctx, "token", EditEventInput{EventID: "e", Name: "n"}); err != nil {
		t.Fatalf("EditEvent: %v", err)
	}
	if _, err := c.DeleteEvent(ctx, "token", "e"); err != nil {
		t.Fatalf("DeleteEvent: %v", err)
	}
	if _, err := c.CreateEventLink(ctx, "token", "e"); err != nil {
		t.Fatalf("CreateEventLink: %v", err)
	}
}

func TestCode303IsNotRetried(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte(`{"errors":[{"message":"nope","extensions":{"code":303}}],"data":null}`))
	}))
	t.Cleanup(srv.Close)
	c := New(Config{URL: srv.URL, MaxRetries: 3})
	err := c.Do(context.Background(), "token", "mutation CreateActivityReminderMutation { x }", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if hits != 1 {
		t.Fatalf("hits=%d, code 303 must not be retried", hits)
	}
}
