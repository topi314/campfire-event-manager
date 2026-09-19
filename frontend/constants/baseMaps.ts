export interface BaseMapDefinition {
  id: string;
  label: string;
  url: string;
  attribution: string;
  maxZoom: number;
  subdomains?: string;
}

export const DEFAULT_BASE_MAP_ID = "carto-dark";

/** Same basemap set as campfire-map. */
export const BASE_MAPS: BaseMapDefinition[] = [
  {
    id: "carto-voyager",
    label: "Voyager",
    url: "https://{s}.basemaps.cartocdn.com/rastertiles/voyager/{z}/{x}/{y}{r}.png",
    attribution: "© CARTO © OpenStreetMap",
    maxZoom: 20,
    subdomains: "abcd",
  },
  {
    id: "osm",
    label: "OpenStreetMap",
    url: "https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png",
    attribution: "© OpenStreetMap",
    maxZoom: 19,
    subdomains: "abc",
  },
  {
    id: "carto-positron",
    label: "Positron (light)",
    url: "https://{s}.basemaps.cartocdn.com/light_all/{z}/{x}/{y}{r}.png",
    attribution: "© CARTO © OpenStreetMap",
    maxZoom: 20,
    subdomains: "abcd",
  },
  {
    id: "carto-dark",
    label: "Dark matter",
    url: "https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png",
    attribution: "© CARTO © OpenStreetMap",
    maxZoom: 20,
    subdomains: "abcd",
  },
  {
    id: "esri-street",
    label: "Esri street",
    url: "https://server.arcgisonline.com/ArcGIS/rest/services/World_Street_Map/MapServer/tile/{z}/{y}/{x}",
    attribution: "© Esri",
    maxZoom: 19,
  },
  {
    id: "satellite",
    label: "Satellite",
    url: "https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}",
    attribution: "© Esri",
    maxZoom: 19,
  },
];

export function getBaseMap(id: string) {
  const found = BASE_MAPS.find((m) => m.id === id);
  if (found) return found;
  return BASE_MAPS.find((m) => m.id === DEFAULT_BASE_MAP_ID) ?? BASE_MAPS[0];
}

function isCartoTileUrl(url: string) {
  return url.includes("basemaps.cartocdn.com");
}

/** Append CARTO’s required `key` query param on raster basemap URLs. */
export function withCartoApiKey(url: string, apiKey: string) {
  const key = apiKey.trim();
  if (!key || !isCartoTileUrl(url)) return url;
  const sep = url.includes("?") ? "&" : "?";
  return `${url}${sep}key=${encodeURIComponent(key)}`;
}
