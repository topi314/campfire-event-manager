import type { CreateMeetupInput, DraftMeetupPayload, MeetupDraftItem } from "~/types";
import {
  formatLocalDateTime,
  formatTimeOfDay,
  parseLocalDateTime,
  toISO,
} from "~/utils/datetime";
import { randomOffsetLatLng } from "~/utils/location";
import {
  applyPlaceholders,
  resolveBuiltinPlaceholderValues,
} from "~/utils/placeholders";

export function parseDraftPayload(item: MeetupDraftItem): DraftMeetupPayload {
  const p = (
    typeof item.payload === "string" ? JSON.parse(item.payload) : item.payload
  ) as DraftMeetupPayload;
  return {
    ...p,
    createdByCommunityAmbassador: p.createdByCommunityAmbassador ?? true,
  };
}

/**
 * Preview-only: fill {{tokens}} in the browser so list/Preview match what the
 * server will resolve when posting. Create/edit/post send unresolved text and
 * let the API apply placeholders.
 */
export function resolveDraftTextFields(
  item: DraftMeetupPayload,
  timeZone?: string,
): Pick<DraftMeetupPayload, "name" | "details" | "address" | "coverPhotoUrl"> {
  const start = parseLocalDateTime(item.eventTime);
  const end = parseLocalDateTime(item.eventEndTime);
  const builtins = resolveBuiltinPlaceholderValues({
    clubName: item.clubName,
    liveEventName: item.liveEventName,
    category: item.category || "",
    date: start
      ? { year: start.year, month: start.month, day: start.day }
      : undefined,
    startTime: start ? formatTimeOfDay(start) : "",
    endTime: end ? formatTimeOfDay(end) : "",
    timeZone: timeZone || "",
    title: item.name || "",
    address: item.address || "",
    latitude: item.location?.lat ?? null,
    longitude: item.location?.lng ?? null,
  });
  const values = {
    ...builtins,
    ...(item.placeholderValues || {}),
  };
  return {
    name: applyPlaceholders(item.name, values),
    details: applyPlaceholders(item.details, values),
    address: applyPlaceholders(item.address, values),
    coverPhotoUrl: applyPlaceholders(item.coverPhotoUrl, values),
  };
}

/** Resolved display fields for a saved draft row (list / progress labels). */
export function resolveDraftDisplay(
  item: MeetupDraftItem,
  timeZone?: string,
): Pick<DraftMeetupPayload, "name" | "details" | "address" | "coverPhotoUrl"> {
  return resolveDraftTextFields(parseDraftPayload(item), timeZone);
}

/** Build create-meetup API body with unresolved {{tokens}} for server-side fill. */
export function buildMeetupBodyFromDraft(
  item: DraftMeetupPayload,
  timeZone?: string,
): CreateMeetupInput {
  const jitter = Math.max(0, Number(item.locationJitterMeters) || 0);
  const jittered = randomOffsetLatLng(item.location, jitter);
  return {
    clubId: item.clubId,
    name: item.name.trim(),
    details: item.details,
    eventTime: toISO(item.eventTime, timeZone),
    eventEndTime: item.eventEndTime ? toISO(item.eventEndTime, timeZone) : undefined,
    latitude: jittered.lat,
    longitude: jittered.lng,
    address: item.address || undefined,
    coverPhotoUrl: item.coverPhotoUrl || undefined,
    commentsPermissions: item.commentsPermissions || undefined,
    campfireLiveEventId: item.liveEventId || undefined,
    allInvited: item.allInvited,
    createdByCommunityAmbassador: item.createdByCommunityAmbassador ?? true,
    clubName: item.clubName || undefined,
    liveEventName: item.liveEventName || undefined,
    category: item.category || undefined,
    timeZone: timeZone || undefined,
    ...(item.placeholderValues && Object.keys(item.placeholderValues).length
      ? { placeholderValues: item.placeholderValues }
      : {}),
  };
}

export function formatDraftWhen(local: string) {
  return formatLocalDateTime(local);
}
