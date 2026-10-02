import type { LiveEvent } from "~/types";

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
