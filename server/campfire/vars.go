package campfire

type clubsVars struct {
	First int    `json:"first"`
	After string `json:"after,omitempty"`
}

type activeEventsVars struct {
	ClubID string `json:"clubId"`
	First  int    `json:"first"`
	After  string `json:"after,omitempty"`
}

type clubMembersVars struct {
	ClubID string `json:"clubId"`
	First  int    `json:"first"`
	After  string `json:"after,omitempty"`
}

type searchClubMembersVars struct {
	ClubID string `json:"clubId"`
	Search string `json:"search"`
}

type liveEventsVars struct {
	Input marketableCampfireLiveEventsInput `json:"input"`
}

type marketableCampfireLiveEventsInput struct {
	Game string `json:"game"`
}

type createActivityReminderVars struct {
	Input CreateActivityReminderInput `json:"input"`
}

// createActivityReminderMultipartVars embeds the create input and always
// emits avatarFile: null so the multipart map can bind the upload.
type createActivityReminderMultipartVars struct {
	Input createActivityReminderMultipartInput `json:"input"`
}

type createActivityReminderMultipartInput struct {
	CreateActivityReminderInput
	AvatarFile *struct{} `json:"avatarFile"`
}

type editEventVars struct {
	Input EditEventInput `json:"input"`
}

// editEventMultipartVars embeds the edit input and always emits
// avatarFile: null so the multipart map can bind the upload.
type editEventMultipartVars struct {
	Input editEventMultipartInput `json:"input"`
}

type editEventMultipartInput struct {
	EditEventInput
	AvatarFile *struct{} `json:"avatarFile"`
}

type deleteEventVars struct {
	Input DeleteEventInput `json:"input"`
}

// DeleteEventInput is the GraphQL input for deleteEvent.
type DeleteEventInput struct {
	EventID string `json:"eventId"`
}
