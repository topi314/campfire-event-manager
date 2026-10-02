package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/topi314/campfire-event-manager/server/database"
	"github.com/topi314/campfire-event-manager/server/database/dbsqlc"
)

func templateWithOrigin(row dbsqlc.ListTemplatesByUserRow) MeetupTemplate {
	return applyOrigin(
		templateFrom(
			row.TemplateID, row.TemplateDiscordUserID, row.TemplateName, row.TemplatePayload,
			row.TemplatePublishedAt, row.TemplatePublishDescription, row.TemplateLanguage,
			row.TemplateOriginID, row.TemplateSynced, row.TemplateCreatedAt, row.TemplateUpdatedAt,
		),
		row.OriginID, row.OriginName, row.OriginPayload,
		row.OriginPublisherID, row.OriginPublisherUsername, row.OriginPublisherDisplayName, row.OriginPublisherAvatarUrl,
	)
}

func templateWithOriginGet(row dbsqlc.GetTemplateForUserRow) MeetupTemplate {
	return applyOrigin(
		templateFrom(
			row.TemplateID, row.TemplateDiscordUserID, row.TemplateName, row.TemplatePayload,
			row.TemplatePublishedAt, row.TemplatePublishDescription, row.TemplateLanguage,
			row.TemplateOriginID, row.TemplateSynced, row.TemplateCreatedAt, row.TemplateUpdatedAt,
		),
		row.OriginID, row.OriginName, row.OriginPayload,
		row.OriginPublisherID, row.OriginPublisherUsername, row.OriginPublisherDisplayName, row.OriginPublisherAvatarUrl,
	)
}

func templateFromCreate(row dbsqlc.CreateTemplateRow) MeetupTemplate {
	return templateFrom(
		row.TemplateID, row.TemplateDiscordUserID, row.TemplateName, row.TemplatePayload,
		row.TemplatePublishedAt, row.TemplatePublishDescription, row.TemplateLanguage,
		row.TemplateOriginID, row.TemplateSynced, row.TemplateCreatedAt, row.TemplateUpdatedAt,
	)
}

func templateFromUpdate(row dbsqlc.UpdateTemplateRow) MeetupTemplate {
	return templateFrom(
		row.TemplateID, row.TemplateDiscordUserID, row.TemplateName, row.TemplatePayload,
		row.TemplatePublishedAt, row.TemplatePublishDescription, row.TemplateLanguage,
		row.TemplateOriginID, row.TemplateSynced, row.TemplateCreatedAt, row.TemplateUpdatedAt,
	)
}

func templateFromPublish(row dbsqlc.PublishTemplateRow) MeetupTemplate {
	return templateFrom(
		row.TemplateID, row.TemplateDiscordUserID, row.TemplateName, row.TemplatePayload,
		row.TemplatePublishedAt, row.TemplatePublishDescription, row.TemplateLanguage,
		row.TemplateOriginID, row.TemplateSynced, row.TemplateCreatedAt, row.TemplateUpdatedAt,
	)
}

func templateFromUnpublish(row dbsqlc.UnpublishTemplateRow) MeetupTemplate {
	return templateFrom(
		row.TemplateID, row.TemplateDiscordUserID, row.TemplateName, row.TemplatePayload,
		row.TemplatePublishedAt, row.TemplatePublishDescription, row.TemplateLanguage,
		row.TemplateOriginID, row.TemplateSynced, row.TemplateCreatedAt, row.TemplateUpdatedAt,
	)
}

func templateFromByID(row dbsqlc.GetTemplateByIDRow) MeetupTemplate {
	return templateFrom(
		row.TemplateID, row.TemplateDiscordUserID, row.TemplateName, row.TemplatePayload,
		row.TemplatePublishedAt, row.TemplatePublishDescription, row.TemplateLanguage,
		row.TemplateOriginID, row.TemplateSynced, row.TemplateCreatedAt, row.TemplateUpdatedAt,
	)
}

func templateFrom(
	id int64,
	discordUserID, name string,
	payload json.RawMessage,
	publishedAt pgtype.Timestamp,
	publishDescription, language pgtype.Text,
	originID pgtype.Int8,
	synced bool,
	createdAt, updatedAt pgtype.Timestamp,
) MeetupTemplate {
	return MeetupTemplate{
		ID:                 id,
		DiscordUserID:      discordUserID,
		Name:               name,
		Payload:            payload,
		PublishedAt:        timePtr(publishedAt),
		PublishDescription: stringPtr(publishDescription),
		Language:           stringPtr(language),
		OriginID:           int64Ptr(originID),
		Synced:             synced,
		CreatedAt:          timeFrom(createdAt),
		UpdatedAt:          timeFrom(updatedAt),
	}
}

func (s *Server) listPublishedTemplates(ctx context.Context, f SharedFilter) ([]MeetupTemplate, error) {
	sort := strings.TrimSpace(f.Sort)
	switch sort {
	case "updated_desc", "name_asc", "likes_desc", "liked_desc", "published_desc":
	default:
		sort = "published_desc"
	}

	rows, err := s.db.ListPublishedTemplates(ctx, dbsqlc.ListPublishedTemplatesParams{
		ViewerID:    database.Text(f.ViewerUserID),
		Query:       database.TextLike(f.Query),
		Category:    database.Text(f.Category),
		Language:    database.Text(strings.ToLower(strings.TrimSpace(f.Language))),
		PublisherID: database.Text(f.PublisherID),
		Publisher:   database.TextLike(f.Publisher),
		LikedBy:     database.Text(f.LikedByUserID),
		Sort:        sort,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list published templates: %w", err)
	}

	out := make([]MeetupTemplate, 0, len(rows))
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		t := MeetupTemplate{
			ID:                 row.TemplateID,
			DiscordUserID:      row.TemplateDiscordUserID,
			Name:               row.TemplateName,
			Payload:            stripSharedFieldsFromPayload(row.TemplatePayload),
			PublishedAt:        timePtr(row.TemplatePublishedAt),
			PublishDescription: stringPtr(row.TemplatePublishDescription),
			Language:           stringPtr(row.TemplateLanguage),
			OriginID:           int64Ptr(row.TemplateOriginID),
			Synced:             row.TemplateSynced,
			CreatedAt:          timeFrom(row.TemplateCreatedAt),
			UpdatedAt:          timeFrom(row.TemplateUpdatedAt),
			LikeCount:          int(row.LikeCount),
			LikedByMe:          row.LikedByMe,
			Publisher: &UserRef{
				ID:          row.PublisherID,
				Username:    row.PublisherUsername,
				DisplayName: row.PublisherDisplayName,
				AvatarURL:   row.PublisherAvatarUrl,
			},
		}
		out = append(out, t)
		ids = append(ids, t.ID)
	}
	if err := s.attachTemplateLikers(ctx, out, ids); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Server) likeTemplateRecord(ctx context.Context, userID string, templateID int64) error {
	n, err := s.db.LikeTemplate(ctx, dbsqlc.LikeTemplateParams{
		UserID:     userID,
		TemplateID: templateID,
	})
	if err != nil {
		return fmt.Errorf("failed to like template: %w", err)
	}
	if n == 0 {
		published, err := s.db.IsTemplatePublished(ctx, templateID)
		if err != nil {
			return database.MapError(err)
		}
		if !published {
			return sql.ErrNoRows
		}
	}
	return nil
}

func (s *Server) templateLikeState(ctx context.Context, userID string, templateID int64) (count int, likedByMe bool, likers []UserRef, err error) {
	c, err := s.db.CountTemplateLikes(ctx, templateID)
	if err != nil {
		return 0, false, nil, err
	}
	count = int(c)
	if strings.TrimSpace(userID) != "" {
		likedByMe, err = s.db.TemplateLikedByUser(ctx, dbsqlc.TemplateLikedByUserParams{
			TemplateID:    templateID,
			DiscordUserID: userID,
		})
		if err != nil {
			return 0, false, nil, err
		}
	}
	grouped, err := s.listTemplateLikersGrouped(ctx, []int64{templateID})
	if err != nil {
		return 0, false, nil, err
	}
	likers = grouped[templateID]
	if likers == nil {
		likers = []UserRef{}
	}
	return count, likedByMe, likers, nil
}

func (s *Server) attachTemplateLikers(ctx context.Context, templates []MeetupTemplate, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	likersByID, err := s.listTemplateLikersGrouped(ctx, ids)
	if err != nil {
		return err
	}
	for i := range templates {
		templates[i].Likers = likersByID[templates[i].ID]
		if templates[i].Likers == nil {
			templates[i].Likers = []UserRef{}
		}
	}
	return nil
}

func (s *Server) listTemplateLikersGrouped(ctx context.Context, ids []int64) (map[int64][]UserRef, error) {
	out := make(map[int64][]UserRef, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.db.ListTemplateLikers(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to list template likers: %w", err)
	}
	for _, row := range rows {
		out[row.TemplateID] = append(out[row.TemplateID], UserRef{
			ID:          row.DiscordUserID,
			Username:    row.DiscordUserUsername,
			DisplayName: row.DiscordUserDisplayName,
			AvatarURL:   row.DiscordUserAvatarUrl,
		})
	}
	return out, nil
}

func applyOrigin(
	t MeetupTemplate,
	originID pgtype.Int8,
	originName pgtype.Text,
	originPayload []byte,
	opID, opUser, opDisplay, opAvatar pgtype.Text,
) MeetupTemplate {
	if originID.Valid && originName.Valid {
		origin := &TemplateOrigin{ID: originID.Int64, Name: originName.String}
		if opID.Valid {
			origin.Publisher = &UserRef{
				ID:          opID.String,
				Username:    textString(opUser),
				DisplayName: textString(opDisplay),
				AvatarURL:   textString(opAvatar),
			}
		}
		t.Origin = origin
		if t.Synced {
			t.Name = originName.String
			if len(originPayload) > 0 {
				t.Payload = stripSharedFieldsFromPayload(json.RawMessage(originPayload))
			}
		}
	} else if t.Synced {
		t.Synced = false
	}
	return t
}

// stripSharedFieldsFromPayload removes personal meetup defaults so Shared
// listings never redistribute location, invites, or comments settings.
func stripSharedFieldsFromPayload(payload json.RawMessage) json.RawMessage {
	if len(payload) == 0 {
		return payload
	}
	var m map[string]any
	if err := json.Unmarshal(payload, &m); err != nil {
		return payload
	}
	changed := false
	for _, key := range []string{
		"latitude",
		"longitude",
		"address",
		"placeId",
		"locationJitterMeters",
		"commentsPermissions",
		"allInvited",
		"inviteeIds",
	} {
		if _, ok := m[key]; ok {
			delete(m, key)
			changed = true
		}
	}
	if !changed {
		return payload
	}
	out, err := json.Marshal(m)
	if err != nil {
		return payload
	}
	return out
}

func timeFrom(ts pgtype.Timestamp) time.Time {
	if !ts.Valid {
		return time.Time{}
	}
	return ts.Time
}

func timePtr(ts pgtype.Timestamp) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}

func stringPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

func int64Ptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	i := v.Int64
	return &i
}

func textString(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}
