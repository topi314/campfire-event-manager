import type { LiveEvent } from "~/types";
import { localDateTimeDiffMs } from "~/utils/datetime";

export type MeetupSchedule = {
  category: string;
  eventTime: string;
  eventEndTime: string;
};

export type MeetupScheduleRequest = {
  timeZone?: string;
  /** Optional YYYY-MM-DD to pin the calendar day (edit flow). */
  date?: string;
  startTime?: string;
  endTime?: string;
  liveEvent?: Pick<
    LiveEvent,
    "eventName" | "startTimestamp" | "endTimestamp" | "localStartTime" | "localEndTime"
  > | null;
};

/** Server-provided live-event window. Missing bounds skip the check. */
export function isWithinLiveEventWindow(
  eventTime: string,
  eventEndTime: string,
  windowStart?: string | null,
  windowEnd?: string | null,
): boolean {
  if (!windowStart || !windowEnd) return true;
  const startOk = localDateTimeDiffMs(windowStart, eventTime);
  const endOk = localDateTimeDiffMs(eventEndTime, windowEnd);
  if (startOk == null || endOk == null) return true;
  return startOk >= 0 && endOk >= 0;
}

/** Resolve meetup wall-clock times on the server (category defaults, live-event day). */
export async function fetchMeetupSchedule(
  api: <T>(path: string, init?: RequestInit) => Promise<T>,
  req: MeetupScheduleRequest,
): Promise<MeetupSchedule> {
  return api<MeetupSchedule>("/api/campfire/meetup-schedule", {
    method: "POST",
    body: JSON.stringify({
      timeZone: req.timeZone || undefined,
      date: req.date || undefined,
      startTime: req.startTime || undefined,
      endTime: req.endTime || undefined,
      liveEvent: req.liveEvent
        ? {
            eventName: req.liveEvent.eventName,
            startTimestamp: req.liveEvent.startTimestamp,
            endTimestamp: req.liveEvent.endTimestamp,
            localStartTime: req.liveEvent.localStartTime,
            localEndTime: req.liveEvent.localEndTime,
          }
        : undefined,
    }),
  });
}
