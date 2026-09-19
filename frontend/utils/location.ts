export type LatLng = { lat: number; lng: number };

const METERS_PER_DEG_LAT = 111_320;

/** Parse "lat, lng" / "lat lng" / "lat;lng" into a point, or null if invalid. */
export function parseLatLng(raw: string): LatLng | null {
  const cleaned = raw.trim().replace(/[°]/g, " ").replace(/\s+/g, " ");
  const parts = cleaned.split(/[,;\s]+/).filter(Boolean);
  if (parts.length < 2) return null;
  const lat = Number(parts[0]);
  const lng = Number(parts[1]);
  if (!Number.isFinite(lat) || !Number.isFinite(lng)) return null;
  if (lat < -90 || lat > 90 || lng < -180 || lng > 180) return null;
  return { lat, lng };
}

/**
 * Random point uniformly distributed inside a circle of maxMeters around center.
 * maxMeters <= 0 returns the center unchanged.
 */
export function randomOffsetLatLng(center: LatLng, maxMeters: number): LatLng {
  const max = Number(maxMeters);
  if (!Number.isFinite(max) || max <= 0) {
    return { lat: center.lat, lng: center.lng };
  }
  const radius = max * Math.sqrt(Math.random());
  const angle = Math.random() * 2 * Math.PI;
  const north = radius * Math.cos(angle);
  const east = radius * Math.sin(angle);
  const dLat = north / METERS_PER_DEG_LAT;
  const cosLat = Math.cos((center.lat * Math.PI) / 180);
  const metersPerDegLng = METERS_PER_DEG_LAT * Math.max(cosLat, 0.01);
  const dLng = east / metersPerDegLng;
  return {
    lat: center.lat + dLat,
    lng: center.lng + dLng,
  };
}

/** Default jitter so repeated meetups at the same template pin don't stack. */
export const DEFAULT_LOCATION_JITTER_METERS = 25;

/** Parse Campfire event location string ("[lng, lat]"). */
export function parseCampfireLocation(raw: string | undefined | null): LatLng | null {
  if (!raw?.trim()) return null;
  const cleaned = raw.trim().replace(/^\[/, "").replace(/\]$/, "");
  const parts = cleaned.split(",");
  if (parts.length !== 2) return null;
  const lng = Number(parts[0].trim());
  const lat = Number(parts[1].trim());
  if (!Number.isFinite(lat) || !Number.isFinite(lng)) return null;
  return { lat, lng };
}
