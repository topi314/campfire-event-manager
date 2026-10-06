export type DiscordUser = {
  id: string;
  username: string;
  displayName: string;
  avatarUrl: string;
  admin: boolean;
  /** Saved IANA timezone. Null until this account has chosen one. */
  timeZone?: string | null;
};

export type MeetupTemplate = {
  id: number;
  discordUserId: string;
  name: string;
  payload: MeetupPayload;
  publishedAt?: string | null;
  publishDescription?: string | null;
  /** ISO 639-1 language code for the template (e.g. en, de). */
  language?: string | null;
  originTemplateId?: number | null;
  /** Clone still follows the origin until you edit it. */
  synced?: boolean;
  createdAt: string;
  updatedAt: string;
  publisher?: TemplatePublisher;
  origin?: TemplateOrigin;
  likeCount?: number;
  likedByMe?: boolean;
  likers?: TemplatePublisher[];
};

export type TemplatePublisher = {
  id: string;
  username: string;
  displayName: string;
  avatarUrl: string;
};

export type TemplateOrigin = {
  id: number;
  name: string;
  publisher?: TemplatePublisher;
};

export type TemplatePlaceholder = {
  key: string;
  label?: string;
  default?: string;
};

export type MeetupPayload = {
  name?: string;
  details?: string;
  latitude?: number;
  longitude?: number;
  /**
   * Max random offset (meters) applied when creating a meetup from this pin.
   * 0 = exact coordinates. Default ~25m when omitted on create.
   */
  locationJitterMeters?: number;
  address?: string;
  placeId?: string;
  coverPhotoUrl?: string;
  commentsPermissions?: string;
  /** Optional live-event category (campfire-tools style). Used to filter/auto-select templates. */
  category?: string;
  /** Preferred template start time-of-day (HH:mm). */
  startTime?: string;
  /** Preferred template end time-of-day (HH:mm). */
  endTime?: string;
  /** Legacy full datetime; only the time-of-day is used when creating. */
  eventTime?: string;
  eventEndTime?: string;
  campfireLiveEventId?: string;
  allInvited?: boolean;
  /** When true, meetup is marked as created by a Community Ambassador. */
  createdByCommunityAmbassador?: boolean;
  inviteeIds?: string[];
  /** Optional labels/defaults for {{key}} tokens used in string fields. */
  placeholders?: TemplatePlaceholder[];
};

export type CampfireMe = {
  id: string;
  username: string;
  displayName: string;
  avatarUrl: string;
  badges: { badgeType: string; alias: string }[];
};

export type Club = {
  id: string;
  name: string;
  game: string;
  avatarUrl: string;
  createdByCommunityAmbassador: boolean;
  amIAdmin: boolean;
  onlyAllowAdminsToCreateEvents: boolean;
  members: { totalCount: number };
};

export type LiveEvent = {
  id: string;
  eventName: string;
  startTimestamp: string;
  endTimestamp: string;
  localStartTime?: string;
  localEndTime?: string;
  modalHeadingImageUrl?: string;
  location?: { latitude: number; longitude: number };
};

export type ClubMember = {
  id: string;
  username: string;
  displayName: string;
  avatarUrl: string;
};

export type CreateMeetupInput = {
  clubId: string;
  name: string;
  details?: string;
  eventTime: string;
  eventEndTime?: string;
  latitude: number;
  longitude: number;
  address?: string;
  placeId?: string;
  coverPhotoUrl?: string;
  commentsPermissions?: string;
  campfireLiveEventId?: string;
  allInvited?: boolean;
  createdByCommunityAmbassador?: boolean;
  inviteeIds?: string[];
  /** Context for server-side {{placeholder}} resolution. */
  clubName?: string;
  liveEventName?: string;
  category?: string;
  timeZone?: string;
  /** Template language for eventPokemon translation / CP unit. */
  language?: string;
  placeholderValues?: Record<string, string>;
};

export type EditMeetupInput = {
  name: string;
  details?: string;
  eventTime: string;
  eventEndTime?: string;
  latitude: number;
  longitude: number;
  address?: string;
  placeId?: string;
  coverPhotoUrl?: string;
  commentsPermissions?: string;
  campfireLiveEventId?: string;
  allInvited?: boolean;
  createdByCommunityAmbassador?: boolean;
  hasEventPhotoChanged?: boolean;
  /** Context for server-side {{placeholder}} resolution. */
  clubName?: string;
  liveEventName?: string;
  category?: string;
  timeZone?: string;
  language?: string;
  placeholderValues?: Record<string, string>;
};

export type ClubEvent = {
  id: string;
  name: string;
  address?: string;
  location?: string;
  coverPhotoUrl?: string;
  details?: string;
  eventTime: string;
  eventEndTime?: string;
  commentsPermissions?: string;
  allInvited?: boolean;
  createdByCommunityAmbassador?: boolean;
  campfireLiveEventId?: string;
  clubId: string;
  creator?: {
    id: string;
    username: string;
    displayName: string;
  };
};

export type DraftMeetupPayload = {
  templateName: string;
  /** Template used when the draft was saved; restored on edit when still available. */
  templateId?: number;
  clubId: string;
  clubName: string;
  liveEventId: string;
  liveEventName: string;
  /** Optional live-event category for {{category}} when posting. */
  category?: string;
  name: string;
  details: string;
  eventTime: string;
  eventEndTime: string;
  address: string;
  coverPhotoUrl: string;
  commentsPermissions: string;
  allInvited: boolean;
  createdByCommunityAmbassador: boolean;
  locationJitterMeters: number;
  location: { lat: number; lng: number };
  /**
   * Custom placeholder fills collected on create. Applied only when posting;
   * title/details/address keep {{tokens}} until then.
   */
  placeholderValues?: Record<string, string>;
};

export type DraftCreator = {
  id: string;
  username: string;
  displayName: string;
  avatarUrl: string;
};

export type MeetupDraftItem = {
  id: number;
  clubId: string;
  discordUserId: string;
  payload: DraftMeetupPayload;
  createdAt: string;
  updatedAt: string;
  creator?: DraftCreator;
};
