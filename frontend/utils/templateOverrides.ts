import type { MeetupPayload, TemplateOrigin } from "~/types";
import { DEFAULT_LOCATION_JITTER_METERS } from "~/utils/location";
import {
  ensureMinMeetupEndTime,
  formatTimeOfDay,
  parseTimeOfDay,
} from "~/utils/datetime";
import { isTemplateLanguageCode, type TemplateLanguageCode } from "~/utils/languages";

export type OverrideFieldId =
  | "name"
  | "language"
  | "title"
  | "details"
  | "category"
  | "coverPhotoUrl"
  | "commentsPermissions"
  | "allInvited"
  | "createdByCommunityAmbassador"
  | "placeholders"
  | "location"
  | "schedule"
  | "discord";

export function parseOriginPayload(origin?: TemplateOrigin | null): MeetupPayload {
  if (!origin?.payload) return {};
  if (typeof origin.payload === "string") {
    try {
      return JSON.parse(origin.payload) as MeetupPayload;
    } catch {
      return {};
    }
  }
  return origin.payload;
}

export type TemplateEditorSnapshot = {
  name: string;
  language: TemplateLanguageCode | string;
  title: string;
  details: string;
  category: string;
  address: string;
  coverPhotoUrl: string;
  commentsPermissions: string;
  discordChannelId: string;
  discordClubId: string;
  startTime: string;
  endTime: string;
  allInvited: boolean;
  createdByCommunityAmbassador: boolean;
  locationJitterMeters: number;
  location: { lat: number; lng: number } | null;
  placeholders: { key: string; label: string; default: string }[];
};

export function snapshotFromOrigin(
  origin: TemplateOrigin,
  fallbackLanguage: TemplateLanguageCode,
): TemplateEditorSnapshot {
  const p = parseOriginPayload(origin);
  const startTod = parseTimeOfDay(p.startTime);
  const endTod = parseTimeOfDay(p.endTime);
  const startTime = startTod ? formatTimeOfDay(startTod) : "";
  const endTime = startTod
    ? ensureMinMeetupEndTime(startTime, endTod ? formatTimeOfDay(endTod) : "")
    : endTod
      ? formatTimeOfDay(endTod)
      : "";
  const lang = isTemplateLanguageCode(origin.language || "")
    ? (origin.language as TemplateLanguageCode)
    : fallbackLanguage;
  return {
    name: origin.name || "",
    language: lang,
    title: p.name || "",
    details: p.details || "",
    category: (p.category || "").trim(),
    address: p.address || "",
    coverPhotoUrl: p.coverPhotoUrl || "",
    commentsPermissions: p.commentsPermissions || "ORGANIZERS_ONLY",
    discordChannelId: (p.discordChannelId || "").trim(),
    discordClubId: (p.discordClubId || "").trim(),
    startTime,
    endTime,
    allInvited: p.allInvited ?? true,
    createdByCommunityAmbassador: p.createdByCommunityAmbassador ?? true,
    locationJitterMeters:
      p.locationJitterMeters != null ? p.locationJitterMeters : DEFAULT_LOCATION_JITTER_METERS,
    location:
      p.latitude != null && p.longitude != null
        ? { lat: p.latitude, lng: p.longitude }
        : null,
    placeholders: (p.placeholders || []).map((ph) => ({
      key: ph.key,
      label: ph.label || "",
      default: ph.default || "",
    })),
  };
}
