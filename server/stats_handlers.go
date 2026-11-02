package server

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/topi314/campfire-event-manager/server/database/dbsqlc"
)

const (
	statMeetupCreated   = "meetup_created"
	statMeetupEdited    = "meetup_edited"
	statMeetupDeleted   = "meetup_deleted"
	statTemplateCreated = "template_created"
	statTemplateEdited  = "template_edited"
	statTemplateDeleted = "template_deleted"
	statDraftCreated    = "draft_created"
	statDraftEdited     = "draft_edited"
	statDraftDeleted    = "draft_deleted"
	statDraftPublished  = "draft_published"
)

type statsResponse struct {
	MeetupsCreated     int64 `json:"meetupsCreated"`
	MeetupsEdited      int64 `json:"meetupsEdited"`
	MeetupsDeleted     int64 `json:"meetupsDeleted"`
	TemplatesCreated   int64 `json:"templatesCreated"`
	TemplatesEdited    int64 `json:"templatesEdited"`
	TemplatesDeleted   int64 `json:"templatesDeleted"`
	Templates          int64 `json:"templates"`
	TemplatesPublished int64 `json:"templatesPublished"`
	Drafts             int64 `json:"drafts"`
	DraftsCreated      int64 `json:"draftsCreated"`
	DraftsEdited       int64 `json:"draftsEdited"`
	DraftsDeleted      int64 `json:"draftsDeleted"`
	DraftsPublished    int64 `json:"draftsPublished"`
}

func (s *Server) recordStat(ctx context.Context, kind string, n int64) {
	kind = strings.TrimSpace(kind)
	if kind == "" || n <= 0 {
		return
	}
	err := s.db.IncrementStat(ctx, dbsqlc.IncrementStatParams{
		StatKind:  kind,
		StatCount: n,
	})
	if err != nil {
		slog.ErrorContext(ctx, "record stat failed", slog.String("kind", kind), slog.Any("err", err))
	}
}

func (s *Server) apiStats(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.ListStats(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "stat counts failed", slog.Any("err", err))
		writeError(w, http.StatusInternalServerError, "failed to load stats")
		return
	}
	var out statsResponse
	for _, row := range rows {
		switch row.StatKind {
		case statMeetupCreated:
			out.MeetupsCreated = row.StatCount
		case statMeetupEdited:
			out.MeetupsEdited = row.StatCount
		case statMeetupDeleted:
			out.MeetupsDeleted = row.StatCount
		case statTemplateCreated:
			out.TemplatesCreated = row.StatCount
		case statTemplateEdited:
			out.TemplatesEdited = row.StatCount
		case statTemplateDeleted:
			out.TemplatesDeleted = row.StatCount
		case statDraftCreated:
			out.DraftsCreated = row.StatCount
		case statDraftEdited:
			out.DraftsEdited = row.StatCount
		case statDraftDeleted:
			out.DraftsDeleted = row.StatCount
		case statDraftPublished:
			out.DraftsPublished = row.StatCount
		}
	}
	inventory, err := s.db.InventoryCounts(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "inventory counts failed", slog.Any("err", err))
		writeError(w, http.StatusInternalServerError, "failed to load stats")
		return
	}
	out.Templates = inventory.Templates
	out.TemplatesPublished = inventory.TemplatesPublished
	out.Drafts = inventory.Drafts
	writeJSON(w, http.StatusOK, out)
}
