/** Büsingpark, Offenbach am Main — same default as campfire-map */
export const DEFAULT_MAP_CENTER = { lat: 50.10056, lng: 8.76167 };
export const DEFAULT_MAP_ZOOM = 16;

const LAST_MAP_VIEW_KEY = "campfire-event-manager.last-map-view";

export type MapView = {
  lat: number;
  lng: number;
  zoom: number;
};

function isFiniteNumber(n: unknown): n is number {
  return typeof n === "number" && Number.isFinite(n);
}

/** Last pan/zoom the user left a location picker on (browser-local). */
export function readLastMapView(): MapView | null {
  if (!import.meta.client) return null;
  try {
    const raw = localStorage.getItem(LAST_MAP_VIEW_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as Partial<MapView>;
    if (!isFiniteNumber(parsed.lat) || !isFiniteNumber(parsed.lng)) return null;
    if (parsed.lat < -90 || parsed.lat > 90 || parsed.lng < -180 || parsed.lng > 180) {
      return null;
    }
    const zoom = isFiniteNumber(parsed.zoom) ? parsed.zoom : DEFAULT_MAP_ZOOM;
    return { lat: parsed.lat, lng: parsed.lng, zoom: Math.min(22, Math.max(1, zoom)) };
  } catch {
    return null;
  }
}

export function writeLastMapView(view: MapView) {
  if (!import.meta.client) return;
  try {
    localStorage.setItem(
      LAST_MAP_VIEW_KEY,
      JSON.stringify({
        lat: view.lat,
        lng: view.lng,
        zoom: view.zoom,
      }),
    );
  } catch {
    /* private mode */
  }
}
