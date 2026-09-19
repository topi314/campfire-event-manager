package campfire

import (
	_ "embed"
)

//go:embed queries/me.graphql
var queryMe string

//go:embed queries/clubs.graphql
var queryClubs string

//go:embed queries/create_poi_meetup.graphql
var mutationCreatePoiMeetup string

//go:embed queries/edit_event.graphql
var mutationEditEvent string

//go:embed queries/delete_event.graphql
var mutationDeleteEvent string

//go:embed queries/active_events.graphql
var queryActiveEvents string

//go:embed queries/live_events.graphql
var queryLiveEvents string

//go:embed queries/map_objects.graphql
var queryMapObjects string

//go:embed queries/club_members.graphql
var queryClubMembers string

//go:embed queries/search_club_members.graphql
var querySearchClubMembers string

//go:embed queries/upload_file.graphql
var mutationUploadFile string

//go:embed queries/process_staging_image.graphql
var mutationProcessStagingImage string
