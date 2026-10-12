package campfire

import (
	_ "embed"
)

//go:embed queries/me.graphql
var queryMe string

//go:embed queries/clubs.graphql
var queryClubs string

//go:embed queries/admin_clubs.graphql
var queryAdminClubs string

//go:embed queries/active_events.graphql
var queryActiveEvents string

//go:embed queries/active_events_lite.graphql
var queryActiveEventsLite string

//go:embed queries/live_events.graphql
var queryLiveEvents string

//go:embed queries/club_members.graphql
var queryClubMembers string

//go:embed queries/search_club_members.graphql
var querySearchClubMembers string

//go:embed queries/discord_channels.graphql
var queryDiscordChannels string

//go:embed mutations/create_activity_reminder.graphql
var mutationCreateActivityReminder string

//go:embed mutations/edit_event.graphql
var mutationEditEvent string

//go:embed mutations/delete_event.graphql
var mutationDeleteEvent string

//go:embed mutations/create_link.graphql
var mutationCreateLink string
