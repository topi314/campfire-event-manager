package server

import (
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/topi314/campfire-event-manager/server/database/dbsqlc"
)

func draftFromGet(row dbsqlc.GetDraftRow) MeetupDraftItem {
	return draftFrom(
		row.DraftID, row.DraftClubID, row.DraftDiscordUserID, row.DraftPayload,
		row.DraftCreatedAt, row.DraftUpdatedAt,
		row.CreatorID, row.CreatorUsername, row.CreatorDisplayName, row.CreatorAvatarUrl,
	)
}

func draftFromList(row dbsqlc.ListDraftsByClubIDsRow) MeetupDraftItem {
	return draftFrom(
		row.DraftID, row.DraftClubID, row.DraftDiscordUserID, row.DraftPayload,
		row.DraftCreatedAt, row.DraftUpdatedAt,
		row.CreatorID, row.CreatorUsername, row.CreatorDisplayName, row.CreatorAvatarUrl,
	)
}

func draftFrom(
	id int64,
	clubID, discordUserID string,
	payload []byte,
	createdAt, updatedAt pgtype.Timestamp,
	creatorID, creatorUsername, creatorDisplayName, creatorAvatar pgtype.Text,
) MeetupDraftItem {
	item := MeetupDraftItem{
		ID:            id,
		ClubID:        clubID,
		DiscordUserID: discordUserID,
		Payload:       payload,
		CreatedAt:     timeFrom(createdAt),
		UpdatedAt:     timeFrom(updatedAt),
	}
	if creatorID.Valid && creatorID.String != "" {
		item.Creator = &UserRef{
			ID:          creatorID.String,
			Username:    textString(creatorUsername),
			DisplayName: textString(creatorDisplayName),
			AvatarURL:   textString(creatorAvatar),
		}
	}
	return item
}
