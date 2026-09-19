package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

const templateColumns = `
	t.template_id,
	t.template_discord_user_id,
	t.template_name,
	t.template_payload,
	t.template_published_at,
	t.template_publish_description,
	t.template_language,
	t.template_origin_id,
	t.template_synced,
	t.template_created_at,
	t.template_updated_at
`

const templateReturning = `
	template_id,
	template_discord_user_id,
	template_name,
	template_payload,
	template_published_at,
	template_publish_description,
	template_language,
	template_origin_id,
	template_synced,
	template_created_at,
	template_updated_at
`

type SharedFilter struct {
	Query          string
	Category       string
	Language       string
	PublisherID    string
	Publisher      string // username / display name substring
	LikedByUserID  string // only templates liked by this user
	Sort           string // published_desc | updated_desc | name_asc | likes_desc | liked_desc
	ViewerUserID   string // for likedByMe
}

func (d *Database) ListTemplates(ctx context.Context, userID string) ([]MeetupTemplate, error) {
	query := `
		SELECT ` + templateColumns + `,
			o.template_id AS origin_id,
			o.template_name AS origin_name,
			o.template_payload AS origin_payload,
			ou.discord_user_id AS origin_publisher_id,
			ou.discord_user_username AS origin_publisher_username,
			ou.discord_user_display_name AS origin_publisher_display_name,
			ou.discord_user_avatar_url AS origin_publisher_avatar_url
		FROM meetup_templates t
		LEFT JOIN meetup_templates o ON o.template_id = t.template_origin_id
		LEFT JOIN discord_users ou ON ou.discord_user_id = o.template_discord_user_id
		WHERE t.template_discord_user_id = $1
		ORDER BY t.template_updated_at DESC
	`
	rows, err := d.db.QueryxContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}
	defer rows.Close()

	out := make([]MeetupTemplate, 0)
	for rows.Next() {
		t, err := scanTemplateWithOrigin(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (d *Database) GetTemplate(ctx context.Context, userID string, id int64) (*MeetupTemplate, error) {
	query := `
		SELECT ` + templateColumns + `,
			o.template_id AS origin_id,
			o.template_name AS origin_name,
			o.template_payload AS origin_payload,
			ou.discord_user_id AS origin_publisher_id,
			ou.discord_user_username AS origin_publisher_username,
			ou.discord_user_display_name AS origin_publisher_display_name,
			ou.discord_user_avatar_url AS origin_publisher_avatar_url
		FROM meetup_templates t
		LEFT JOIN meetup_templates o ON o.template_id = t.template_origin_id
		LEFT JOIN discord_users ou ON ou.discord_user_id = o.template_discord_user_id
		WHERE t.template_id = $1 AND t.template_discord_user_id = $2
	`
	rows, err := d.db.QueryxContext(ctx, query, id, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	t, err := scanTemplateWithOrigin(rows)
	if err != nil {
		return nil, err
	}
	return &t, rows.Err()
}

func (d *Database) GetTemplateByID(ctx context.Context, id int64) (*MeetupTemplate, error) {
	query := `
		SELECT ` + templateColumns + `
		FROM meetup_templates t
		WHERE t.template_id = $1
	`
	var t MeetupTemplate
	if err := d.db.GetContext(ctx, &t, query, id); err != nil {
		return nil, err
	}
	return &t, nil
}

func (d *Database) CreateTemplate(ctx context.Context, userID, name string, payload json.RawMessage) (*MeetupTemplate, error) {
	return d.CreateTemplateClone(ctx, userID, name, payload, nil)
}

func (d *Database) CreateTemplateClone(ctx context.Context, userID, name string, payload json.RawMessage, originID *int64) (*MeetupTemplate, error) {
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	now := time.Now().UTC()
	synced := originID != nil
	query := `
		INSERT INTO meetup_templates (
			template_discord_user_id, template_name, template_payload,
			template_origin_id, template_synced, template_created_at, template_updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING ` + templateReturning
	var t MeetupTemplate
	if err := d.db.GetContext(ctx, &t, query, userID, name, payload, originID, synced, now, now); err != nil {
		return nil, fmt.Errorf("failed to create template: %w", err)
	}
	return &t, nil
}

func (d *Database) UpdateTemplate(ctx context.Context, userID string, id int64, name string, payload json.RawMessage) (*MeetupTemplate, error) {
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	now := time.Now().UTC()
	// Saving always breaks sync with the origin template.
	query := `
		UPDATE meetup_templates
		SET template_name = $1,
		    template_payload = $2,
		    template_synced = FALSE,
		    template_updated_at = $5
		WHERE template_id = $3 AND template_discord_user_id = $4
		RETURNING ` + templateReturning
	var t MeetupTemplate
	if err := d.db.GetContext(ctx, &t, query, name, payload, id, userID, now); err != nil {
		return nil, err
	}
	return &t, nil
}

func (d *Database) SetTemplatePublished(ctx context.Context, userID string, id int64, published bool, description, language string) (*MeetupTemplate, error) {
	var t MeetupTemplate
	now := time.Now().UTC()
	if published {
		desc := strings.TrimSpace(description)
		var descArg any
		if desc == "" {
			descArg = nil
		} else {
			descArg = desc
		}
		lang := strings.TrimSpace(strings.ToLower(language))
		var langArg any
		if lang == "" {
			langArg = nil
		} else {
			langArg = lang
		}
		query := `
			UPDATE meetup_templates
			SET template_published_at = COALESCE(template_published_at, $3),
			    template_publish_description = $4,
			    template_language = $5,
			    template_updated_at = $3
			WHERE template_id = $1 AND template_discord_user_id = $2
			RETURNING ` + templateReturning
		if err := d.db.GetContext(ctx, &t, query, id, userID, now, descArg, langArg); err != nil {
			return nil, err
		}
		return &t, nil
	}
	query := `
		UPDATE meetup_templates
		SET template_published_at = NULL,
		    template_updated_at = $3
		WHERE template_id = $1 AND template_discord_user_id = $2
		RETURNING ` + templateReturning
	if err := d.db.GetContext(ctx, &t, query, id, userID, now); err != nil {
		return nil, err
	}
	return &t, nil
}

func (d *Database) DeleteTemplate(ctx context.Context, userID string, id int64) error {
	res, err := d.db.ExecContext(ctx, `
		DELETE FROM meetup_templates
		WHERE template_id = $1 AND template_discord_user_id = $2
	`, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete template: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("template not found")
	}
	return nil
}

func (d *Database) ListPublishedTemplates(ctx context.Context, f SharedFilter) ([]MeetupTemplate, error) {
	args := make([]any, 0, 6)
	conds := []string{"t.template_published_at IS NOT NULL"}
	fromExtra := ""

	if q := strings.TrimSpace(f.Query); q != "" {
		args = append(args, "%"+q+"%")
		conds = append(conds, fmt.Sprintf(
			"(t.template_name ILIKE $%d OR t.template_publish_description ILIKE $%d)",
			len(args), len(args),
		))
	}
	if cat := strings.TrimSpace(f.Category); cat != "" {
		args = append(args, cat)
		conds = append(conds, fmt.Sprintf("COALESCE(t.template_payload->>'category', '') = $%d", len(args)))
	}
	if lang := strings.TrimSpace(strings.ToLower(f.Language)); lang != "" {
		args = append(args, lang)
		conds = append(conds, fmt.Sprintf("COALESCE(t.template_language, '') = $%d", len(args)))
	}
	likedBy := strings.TrimSpace(f.LikedByUserID)
	if likedBy != "" {
		args = append(args, likedBy)
		fromExtra = fmt.Sprintf(
			`JOIN meetup_template_likes viewer_like
				ON viewer_like.template_id = t.template_id
				AND viewer_like.discord_user_id = $%d`,
			len(args),
		)
	} else if pub := strings.TrimSpace(f.PublisherID); pub != "" {
		args = append(args, pub)
		conds = append(conds, fmt.Sprintf("t.template_discord_user_id = $%d", len(args)))
	}
	if pubName := strings.TrimSpace(f.Publisher); pubName != "" {
		args = append(args, "%"+pubName+"%")
		conds = append(conds, fmt.Sprintf(
			`(u.discord_user_username ILIKE $%d OR u.discord_user_display_name ILIKE $%d)`,
			len(args), len(args),
		))
	}

	viewerIdx := 0
	if viewer := strings.TrimSpace(f.ViewerUserID); viewer != "" {
		args = append(args, viewer)
		viewerIdx = len(args)
	}

	order := "t.template_published_at DESC NULLS LAST, t.template_id DESC"
	switch f.Sort {
	case "updated_desc":
		order = "t.template_updated_at DESC, t.template_id DESC"
	case "name_asc":
		order = "t.template_name ASC, t.template_id ASC"
	case "likes_desc":
		order = "like_count DESC, t.template_published_at DESC NULLS LAST, t.template_id DESC"
	case "liked_desc":
		if likedBy != "" {
			order = "viewer_like.liked_at DESC, t.template_id DESC"
		}
	case "published_desc", "":
		// default
	default:
		order = "t.template_published_at DESC NULLS LAST, t.template_id DESC"
	}

	likedExpr := "FALSE"
	if viewerIdx > 0 {
		likedExpr = fmt.Sprintf(
			`EXISTS (
				SELECT 1 FROM meetup_template_likes ml
				WHERE ml.template_id = t.template_id AND ml.discord_user_id = $%d
			)`,
			viewerIdx,
		)
	}

	query := `
		SELECT ` + templateColumns + `,
			u.discord_user_id AS publisher_id,
			u.discord_user_username AS publisher_username,
			u.discord_user_display_name AS publisher_display_name,
			u.discord_user_avatar_url AS publisher_avatar_url,
			COALESCE((
				SELECT COUNT(*)::int FROM meetup_template_likes ml
				WHERE ml.template_id = t.template_id
			), 0) AS like_count,
			(` + likedExpr + `) AS liked_by_me
		FROM meetup_templates t
		JOIN discord_users u ON u.discord_user_id = t.template_discord_user_id
		` + fromExtra + `
		WHERE ` + strings.Join(conds, " AND ") + `
		ORDER BY ` + order

	rows, err := d.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list published templates: %w", err)
	}
	defer rows.Close()

	out := make([]MeetupTemplate, 0)
	ids := make([]int64, 0)
	for rows.Next() {
		t, err := scanTemplateWithPublisherAndLikes(rows)
		if err != nil {
			return nil, err
		}
		t.Payload = StripSharedFieldsFromPayload(t.Payload)
		out = append(out, t)
		ids = append(ids, t.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := d.attachTemplateLikers(ctx, out, ids); err != nil {
		return nil, err
	}
	return out, nil
}

func (d *Database) LikeTemplate(ctx context.Context, userID string, templateID int64) error {
	res, err := d.db.ExecContext(ctx, `
		INSERT INTO meetup_template_likes (template_id, discord_user_id, liked_at)
		SELECT t.template_id, $2, (now() AT TIME ZONE 'utc')
		FROM meetup_templates t
		WHERE t.template_id = $1 AND t.template_published_at IS NOT NULL
		ON CONFLICT (template_id, discord_user_id) DO NOTHING
	`, templateID, userID)
	if err != nil {
		return fmt.Errorf("failed to like template: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		// Either already liked, or template missing / not published.
		var published bool
		err := d.db.GetContext(ctx, &published, `
			SELECT template_published_at IS NOT NULL
			FROM meetup_templates
			WHERE template_id = $1
		`, templateID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return sql.ErrNoRows
			}
			return err
		}
		if !published {
			return sql.ErrNoRows
		}
	}
	return nil
}

func (d *Database) UnlikeTemplate(ctx context.Context, userID string, templateID int64) error {
	_, err := d.db.ExecContext(ctx, `
		DELETE FROM meetup_template_likes
		WHERE template_id = $1 AND discord_user_id = $2
	`, templateID, userID)
	if err != nil {
		return fmt.Errorf("failed to unlike template: %w", err)
	}
	return nil
}

func (d *Database) GetTemplateLikeState(ctx context.Context, userID string, templateID int64) (count int, likedByMe bool, likers []TemplatePublisher, err error) {
	err = d.db.GetContext(ctx, &count, `
		SELECT COUNT(*)::int FROM meetup_template_likes WHERE template_id = $1
	`, templateID)
	if err != nil {
		return 0, false, nil, err
	}
	if strings.TrimSpace(userID) != "" {
		err = d.db.GetContext(ctx, &likedByMe, `
			SELECT EXISTS (
				SELECT 1 FROM meetup_template_likes
				WHERE template_id = $1 AND discord_user_id = $2
			)
		`, templateID, userID)
		if err != nil {
			return 0, false, nil, err
		}
	}
	likers, err = d.listTemplateLikers(ctx, []int64{templateID})
	if err != nil {
		return 0, false, nil, err
	}
	return count, likedByMe, likers, nil
}

func (d *Database) attachTemplateLikers(ctx context.Context, templates []MeetupTemplate, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	likersByID, err := d.listTemplateLikersGrouped(ctx, ids)
	if err != nil {
		return err
	}
	for i := range templates {
		templates[i].Likers = likersByID[templates[i].ID]
		if templates[i].Likers == nil {
			templates[i].Likers = []TemplatePublisher{}
		}
	}
	return nil
}

func (d *Database) listTemplateLikers(ctx context.Context, ids []int64) ([]TemplatePublisher, error) {
	grouped, err := d.listTemplateLikersGrouped(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]TemplatePublisher, 0)
	for _, id := range ids {
		out = append(out, grouped[id]...)
	}
	return out, nil
}

func (d *Database) listTemplateLikersGrouped(ctx context.Context, ids []int64) (map[int64][]TemplatePublisher, error) {
	out := make(map[int64][]TemplatePublisher, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	query, args, err := sqlx.In(`
		SELECT
			l.template_id,
			u.discord_user_id,
			u.discord_user_username,
			u.discord_user_display_name,
			u.discord_user_avatar_url
		FROM meetup_template_likes l
		JOIN discord_users u ON u.discord_user_id = l.discord_user_id
		WHERE l.template_id IN (?)
		ORDER BY l.liked_at DESC, u.discord_user_username ASC
	`, ids)
	if err != nil {
		return nil, err
	}
	query = d.db.Rebind(query)
	rows, err := d.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list template likers: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var templateID int64
		var pub TemplatePublisher
		if err := rows.Scan(&templateID, &pub.ID, &pub.Username, &pub.DisplayName, &pub.AvatarURL); err != nil {
			return nil, err
		}
		out[templateID] = append(out[templateID], pub)
	}
	return out, rows.Err()
}

type scannable interface {
	Scan(dest ...any) error
}

func scanTemplateCore(row scannable, extra ...any) (MeetupTemplate, error) {
	var t MeetupTemplate
	dest := []any{
		&t.ID, &t.DiscordUserID, &t.Name, &t.Payload,
		&t.PublishedAt, &t.PublishDescription, &t.Language, &t.OriginID, &t.Synced,
		&t.CreatedAt, &t.UpdatedAt,
	}
	dest = append(dest, extra...)
	if err := row.Scan(dest...); err != nil {
		return t, err
	}
	return t, nil
}

func scanTemplateWithPublisher(row scannable) (MeetupTemplate, error) {
	var pub TemplatePublisher
	t, err := scanTemplateCore(row, &pub.ID, &pub.Username, &pub.DisplayName, &pub.AvatarURL)
	if err != nil {
		return t, err
	}
	t.Publisher = &pub
	return t, nil
}

func scanTemplateWithPublisherAndLikes(row scannable) (MeetupTemplate, error) {
	var pub TemplatePublisher
	var likeCount int
	var likedByMe bool
	t, err := scanTemplateCore(
		row,
		&pub.ID, &pub.Username, &pub.DisplayName, &pub.AvatarURL,
		&likeCount, &likedByMe,
	)
	if err != nil {
		return t, err
	}
	t.Publisher = &pub
	t.LikeCount = likeCount
	t.LikedByMe = likedByMe
	return t, nil
}

func scanTemplateWithOrigin(row scannable) (MeetupTemplate, error) {
	var originID *int64
	var originName *string
	var originPayload []byte
	var opID, opUser, opDisplay, opAvatar *string
	t, err := scanTemplateCore(
		row,
		&originID, &originName, &originPayload,
		&opID, &opUser, &opDisplay, &opAvatar,
	)
	if err != nil {
		return t, err
	}
	if originID != nil && originName != nil {
		origin := &TemplateOrigin{ID: *originID, Name: *originName}
		if opID != nil {
			origin.Publisher = &TemplatePublisher{
				ID:          *opID,
				Username:    deref(opUser),
				DisplayName: deref(opDisplay),
				AvatarURL:   deref(opAvatar),
			}
		}
		t.Origin = origin
		if t.Synced {
			t.Name = *originName
			if len(originPayload) > 0 {
				t.Payload = StripSharedFieldsFromPayload(json.RawMessage(originPayload))
			}
		}
	} else if t.Synced {
		// Origin gone — keep local snapshot and report as unsynced.
		t.Synced = false
	}
	return t, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// StripSharedFieldsFromPayload removes personal meetup defaults so Shared
// listings never redistribute location, invites, or comments settings.
func StripSharedFieldsFromPayload(payload json.RawMessage) json.RawMessage {
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
