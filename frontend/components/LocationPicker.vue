<script setup lang="ts">
import L from "leaflet";
import markerIcon2xUrl from "leaflet/dist/images/marker-icon-2x.png";
import markerIconUrl from "leaflet/dist/images/marker-icon.png";
import markerShadowUrl from "leaflet/dist/images/marker-shadow.png";
import { DEFAULT_BASE_MAP_ID, getBaseMap, withCartoApiKey } from "~/constants/baseMaps";
import {
  DEFAULT_MAP_CENTER,
  DEFAULT_MAP_ZOOM,
  readLastMapView,
  writeLastMapView,
} from "~/constants/map";
import { parseLatLng } from "~/utils/location";

export type LatLngPoint = { lat: number; lng: number };
export type PlaceSelection = LatLngPoint & { label?: string };

type GeocodeHit = {
  label: string;
  lat: number;
  lng: number;
};

const props = withDefaults(
  defineProps<{
    modelValue: LatLngPoint | null;
    /** Optional hint when there is no pin; omitted → last view or default. */
    initialCenter?: LatLngPoint;
    baseMapId?: string;
    /** Display-only: no click/drag/locate/clear. */
    readonly?: boolean;
    /** When > 0 and a pin is set, draw the random-offset radius on the map. */
    jitterMeters?: number;
  }>(),
  {
    baseMapId: DEFAULT_BASE_MAP_ID,
    readonly: false,
    jitterMeters: 0,
  },
);

const emit = defineEmits<{
  "update:modelValue": [LatLngPoint | null];
  place: [PlaceSelection];
}>();

const { api } = useApi();
const { apiKey } = useCartoApiKey();
const mapEl = ref<HTMLElement | null>(null);
const latText = ref("");
const lngText = ref("");
const coordsError = ref("");
const searchQuery = ref("");
const searchResults = ref<GeocodeHit[]>([]);
const searchOpen = ref(false);
const searchBusy = ref(false);
const searchError = ref("");
const searchBox = ref<HTMLElement | null>(null);
let searchTimer: ReturnType<typeof setTimeout> | null = null;
let map: L.Map | null = null;
let marker: L.Marker | null = null;
let jitterCircle: L.Circle | null = null;
let baseLayer: L.TileLayer | null = null;
let locateControl: L.Control | null = null;
let clearControl: L.Control | null = null;
let clearBtnEl: HTMLAnchorElement | null = null;
let syncingCoords = false;
let persistViewTimer: ReturnType<typeof setTimeout> | null = null;
/** Skip persisting while we programmatically setView. */
let suppressViewPersist = false;

function rememberMapView() {
  if (!map || props.readonly || suppressViewPersist) return;
  const c = map.getCenter();
  writeLastMapView({ lat: c.lat, lng: c.lng, zoom: map.getZoom() });
}

function scheduleRememberMapView() {
  if (!map || props.readonly) return;
  if (persistViewTimer) clearTimeout(persistViewTimer);
  persistViewTimer = setTimeout(() => {
    persistViewTimer = null;
    rememberMapView();
  }, 250);
}

function setMapView(lat: number, lng: number, zoom?: number) {
  if (!map) return;
  suppressViewPersist = true;
  map.setView([lat, lng], zoom ?? map.getZoom());
  suppressViewPersist = false;
}

onMounted(() => {
  if (!mapEl.value) return;

  const saved = !props.readonly ? readLastMapView() : null;
  // Pin → parent center → last view → default.
  const startCenter =
    props.modelValue ||
    props.initialCenter ||
    (saved ? { lat: saved.lat, lng: saved.lng } : null) ||
    DEFAULT_MAP_CENTER;
  const startZoom = props.modelValue
    ? Math.max(saved?.zoom ?? DEFAULT_MAP_ZOOM, DEFAULT_MAP_ZOOM)
    : (saved?.zoom ?? DEFAULT_MAP_ZOOM);

  map = L.map(mapEl.value, {
    zoomControl: true,
    dragging: !props.readonly,
    scrollWheelZoom: !props.readonly,
    doubleClickZoom: !props.readonly,
    boxZoom: !props.readonly,
    keyboard: !props.readonly,
  }).setView([startCenter.lat, startCenter.lng], startZoom);
  applyBaseLayer(apiKey.value);
  if (!props.readonly) {
    locateControl = createLocateControl().addTo(map);
    clearControl = createClearControl().addTo(map);
    syncClearEnabled();
    map.on("click", (e: L.LeafletMouseEvent) => {
      // Don't pan — keeps the click from fighting map interaction / blur races.
      applyPoint({ lat: e.latlng.lat, lng: e.latlng.lng }, { pan: false });
    });
    map.on("moveend", scheduleRememberMapView);
    map.on("zoomend", scheduleRememberMapView);
    document.addEventListener("click", onDocClick);
  }

  if (props.modelValue) {
    setMarker(props.modelValue.lat, props.modelValue.lng);
    setCoordFields(props.modelValue);
  }
  syncJitterCircle();

  setTimeout(() => map?.invalidateSize(), 100);
  if (props.readonly) {
    // Modal / delayed layout: give the container time to size.
    setTimeout(() => map?.invalidateSize(), 280);
  }
});

onBeforeUnmount(() => {
  document.removeEventListener("click", onDocClick);
  if (searchTimer) clearTimeout(searchTimer);
  if (persistViewTimer) clearTimeout(persistViewTimer);
  if (map && !props.readonly) {
    rememberMapView();
    map.off("moveend", scheduleRememberMapView);
    map.off("zoomend", scheduleRememberMapView);
  }
  locateControl?.remove();
  clearControl?.remove();
  locateControl = null;
  clearControl = null;
  clearBtnEl = null;
  map?.remove();
  map = null;
  marker = null;
  jitterCircle = null;
  baseLayer = null;
});

watch(apiKey, (key) => {
  applyBaseLayer(key);
});

watch(
  () => props.baseMapId,
  () => {
    applyBaseLayer(apiKey.value);
  },
);

watch(
  () => props.initialCenter,
  (c) => {
    if (!map || !c || props.modelValue) return;
    setMapView(c.lat, c.lng, Math.max(map.getZoom(), DEFAULT_MAP_ZOOM));
  },
  { deep: true },
);

watch(
  () => props.modelValue,
  (v) => {
    if (!map) return;
    syncClearEnabled();
    if (!v) {
      marker?.remove();
      marker = null;
      syncJitterCircle();
      if (!syncingCoords) {
        latText.value = "";
        lngText.value = "";
        coordsError.value = "";
      }
      return;
    }
    setMarker(v.lat, v.lng);
    if (!syncingCoords) {
      setMapView(v.lat, v.lng, Math.max(map.getZoom(), DEFAULT_MAP_ZOOM));
      setCoordFields(v);
      coordsError.value = "";
    }
  },
  { deep: true },
);

watch(
  () => props.jitterMeters,
  () => {
    syncJitterCircle();
  },
);

watch(searchQuery, () => {
  if (props.readonly) return;
  scheduleSearch();
});

function formatCoord(n: number) {
  return n.toFixed(6);
}

function setCoordFields(p: LatLngPoint) {
  latText.value = formatCoord(p.lat);
  lngText.value = formatCoord(p.lng);
}

function applyPoint(point: LatLngPoint, opts: { pan?: boolean; label?: string } = {}) {
  const pan = opts.pan !== false;
  setMarker(point.lat, point.lng);
  if (pan && map) {
    setMapView(point.lat, point.lng, Math.max(map.getZoom(), DEFAULT_MAP_ZOOM));
  }
  syncingCoords = true;
  setCoordFields(point);
  coordsError.value = "";
  emit("update:modelValue", point);
  if (opts.label) {
    emit("place", { ...point, label: opts.label });
  }
  syncingCoords = false;
  syncClearEnabled();
}

function applyBaseLayer(cartoKey: string) {
  if (!map) return;
  const def = getBaseMap(props.baseMapId);
  const url = withCartoApiKey(def.url, cartoKey);
  if (baseLayer) {
    map.removeLayer(baseLayer);
  }
  baseLayer = L.tileLayer(url, {
    attribution: def.attribution,
    maxZoom: def.maxZoom,
    subdomains: def.subdomains,
  }).addTo(map);
}

function setMarker(lat: number, lng: number) {
  if (!map) return;
  if (marker) {
    marker.setLatLng([lat, lng]);
    syncJitterCircle();
    return;
  }
  marker = L.marker([lat, lng], {
    draggable: !props.readonly,
    // Let map clicks win so clicking near/on the pin still repositions.
    bubblingMouseEvents: true,
    icon: locationMarkerIcon(),
  }).addTo(map);
  if (!props.readonly) {
    marker.on("drag", () => {
      syncJitterCircle();
    });
    marker.on("dragend", () => {
      const pos = marker!.getLatLng();
      applyPoint({ lat: pos.lat, lng: pos.lng }, { pan: false });
    });
  }
  syncJitterCircle();
}

function syncJitterCircle() {
  if (!map) return;
  const meters = Math.max(0, Number(props.jitterMeters) || 0);
  const center = marker?.getLatLng() || null;
  if (!center || meters <= 0) {
    jitterCircle?.remove();
    jitterCircle = null;
    return;
  }
  if (jitterCircle) {
    jitterCircle.setLatLng(center);
    jitterCircle.setRadius(meters);
    return;
  }
  jitterCircle = L.circle(center, {
    radius: meters,
    interactive: false,
    bubblingMouseEvents: true,
    color: "#3d8bfd",
    weight: 2,
    opacity: 0.85,
    fillColor: "#3d8bfd",
    fillOpacity: 0.12,
  }).addTo(map);
}

function locationMarkerIcon() {
  return L.icon({
    iconUrl: markerIconUrl,
    iconRetinaUrl: markerIcon2xUrl,
    shadowUrl: markerShadowUrl,
    iconSize: [25, 41],
    iconAnchor: [12, 41],
    popupAnchor: [1, -34],
    tooltipAnchor: [16, -28],
    shadowSize: [41, 41],
  });
}

function createIconControl(
  className: string,
  title: string,
  labelHtml: string,
  onClick: () => void,
) {
  const Control = L.Control.extend({
    options: { position: "topleft" as L.ControlPosition },
    onAdd() {
      const bar = L.DomUtil.create("div", `leaflet-bar leaflet-control ${className}`);
      const btn = L.DomUtil.create("a", "", bar) as HTMLAnchorElement;
      btn.href = "#";
      btn.title = title;
      btn.setAttribute("role", "button");
      btn.setAttribute("aria-label", title);
      btn.innerHTML = labelHtml;
      L.DomEvent.disableClickPropagation(bar);
      L.DomEvent.on(btn, "click", (e) => {
        L.DomEvent.preventDefault(e);
        if (btn.classList.contains("leaflet-disabled")) return;
        onClick();
      });
      if (className.includes("location-clear")) {
        clearBtnEl = btn;
      }
      return bar;
    },
  });
  return new Control();
}

function createLocateControl() {
  return createIconControl(
    "location-locate",
    "My location",
    '<span aria-hidden="true">◎</span>',
    locateMe,
  );
}

function createClearControl() {
  return createIconControl(
    "location-clear",
    "Clear location",
    '<span aria-hidden="true">×</span>',
    clearSelection,
  );
}

function syncClearEnabled() {
  if (!clearBtnEl) return;
  const enabled = !!props.modelValue;
  clearBtnEl.classList.toggle("leaflet-disabled", !enabled);
  clearBtnEl.setAttribute("aria-disabled", enabled ? "false" : "true");
}

function clearSelection() {
  if (!props.modelValue) return;
  marker?.remove();
  marker = null;
  latText.value = "";
  lngText.value = "";
  coordsError.value = "";
  emit("update:modelValue", null);
  syncClearEnabled();
}

function locateMe() {
  if (!navigator.geolocation || !map) return;
  navigator.geolocation.getCurrentPosition(
    (pos) => {
      applyPoint({ lat: pos.coords.latitude, lng: pos.coords.longitude });
    },
    () => {
      /* ignore */
    },
  );
}

function samePoint(a: LatLngPoint | null, b: LatLngPoint) {
  if (!a) return false;
  return Math.abs(a.lat - b.lat) < 1e-9 && Math.abs(a.lng - b.lng) < 1e-9;
}

function commitCoords() {
  const latRaw = latText.value.trim();
  const lngRaw = lngText.value.trim();
  if (!latRaw && !lngRaw) {
    coordsError.value = "";
    if (props.modelValue) clearSelection();
    return;
  }
  if (!latRaw || !lngRaw) {
    coordsError.value = "Enter both latitude and longitude";
    return;
  }
  const lat = Number(latRaw);
  const lng = Number(lngRaw);
  if (!Number.isFinite(lat) || !Number.isFinite(lng) || lat < -90 || lat > 90 || lng < -180 || lng > 180) {
    coordsError.value = "Invalid coordinates";
    return;
  }
  const point = { lat, lng };
  // Skip no-op commits (e.g. blur before a map click) so setView doesn't swallow the click.
  if (samePoint(props.modelValue, point)) {
    coordsError.value = "";
    return;
  }
  applyPoint(point);
}

function onCoordBlur() {
  // Defer so a following map click can run first and win.
  setTimeout(() => commitCoords(), 0);
}

function onCoordPaste(ev: ClipboardEvent) {
  const text = ev.clipboardData?.getData("text")?.trim() ?? "";
  if (!text) return;
  const pair = parseLatLng(text);
  if (!pair) return;
  ev.preventDefault();
  applyPoint(pair);
}

function mapViewbox(): string {
  if (!map) return "";
  const b = map.getBounds();
  // Nominatim viewbox: left,top,right,bottom (west,north,east,south)
  return [b.getWest(), b.getNorth(), b.getEast(), b.getSouth()].join(",");
}

function scheduleSearch() {
  if (searchTimer) clearTimeout(searchTimer);
  const q = searchQuery.value.trim();
  if (q.length < 2) {
    searchResults.value = [];
    searchOpen.value = false;
    searchError.value = "";
    searchBusy.value = false;
    return;
  }
  const asCoords = parseLatLng(q);
  if (asCoords) {
    searchResults.value = [
      {
        label: `${formatCoord(asCoords.lat)}, ${formatCoord(asCoords.lng)}`,
        lat: asCoords.lat,
        lng: asCoords.lng,
      },
    ];
    searchOpen.value = true;
    searchError.value = "";
    return;
  }
  searchBusy.value = true;
  searchTimer = setTimeout(() => {
    searchTimer = null;
    void runSearch(q);
  }, 350);
}

async function runSearch(q: string) {
  searchError.value = "";
  try {
    const params = new URLSearchParams({ q, limit: "6" });
    const viewbox = mapViewbox();
    if (viewbox) params.set("viewbox", viewbox);
    const hits = await api<GeocodeHit[]>(`/api/geocode?${params.toString()}`);
    // Ignore stale responses if the user kept typing.
    if (searchQuery.value.trim() !== q) return;
    searchResults.value = hits || [];
    searchOpen.value = true;
    if (!searchResults.value.length) {
      searchError.value = "No places found";
    }
  } catch (e: any) {
    if (searchQuery.value.trim() !== q) return;
    searchResults.value = [];
    searchOpen.value = false;
    searchError.value = e.message || "Search failed";
  } finally {
    if (searchQuery.value.trim() === q) searchBusy.value = false;
  }
}

function pickResult(hit: GeocodeHit) {
  searchQuery.value = hit.label;
  searchResults.value = [];
  searchOpen.value = false;
  searchError.value = "";
  applyPoint({ lat: hit.lat, lng: hit.lng }, { label: hit.label });
}

function onSearchKeydown(ev: KeyboardEvent) {
  if (ev.key === "Escape") {
    searchOpen.value = false;
    return;
  }
  if (ev.key === "Enter") {
    ev.preventDefault();
    if (searchResults.value.length) {
      pickResult(searchResults.value[0]);
      return;
    }
    const q = searchQuery.value.trim();
    const asCoords = parseLatLng(q);
    if (asCoords) {
      applyPoint(asCoords);
      searchOpen.value = false;
    }
  }
}

function onDocClick(ev: MouseEvent) {
  if (!searchBox.value) return;
  if (searchBox.value.contains(ev.target as Node)) return;
  searchOpen.value = false;
}
</script>

<template>
  <div class="location-picker" :class="{ readonly }">
    <div v-if="!readonly" ref="searchBox" class="location-search">
      <label for="location-search">Search place</label>
      <div class="search-input-wrap">
        <input
          id="location-search"
          v-model="searchQuery"
          type="search"
          autocomplete="off"
          spellcheck="false"
          placeholder="Park, address, city…"
          :aria-busy="searchBusy"
          @keydown="onSearchKeydown"
          @focus="searchOpen = searchResults.length > 0"
        />
        <span
          v-if="searchBusy"
          class="search-spinner"
          role="status"
          aria-label="Searching"
        >
          <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <circle
              cx="12"
              cy="12"
              r="9"
              stroke="currentColor"
              stroke-width="2.5"
              stroke-linecap="round"
              stroke-dasharray="40 60"
            />
          </svg>
        </span>
      </div>
      <ul v-if="searchOpen && searchResults.length" class="search-results" role="listbox">
        <li v-for="(hit, i) in searchResults" :key="`${hit.lat},${hit.lng},${i}`">
          <button type="button" role="option" @click="pickResult(hit)">
            {{ hit.label }}
          </button>
        </li>
      </ul>
      <p
        v-if="searchError && !searchBusy"
        class="search-status error"
        aria-live="polite"
      >
        {{ searchError }}
      </p>
    </div>
    <div ref="mapEl" class="location-map" />
    <div v-if="!readonly" class="location-meta">
      <div class="coords-fields">
        <div class="coords-field">
          <label for="location-lat">Lat</label>
          <input
            id="location-lat"
            v-model="latText"
            type="text"
            inputmode="decimal"
            autocomplete="off"
            spellcheck="false"
            placeholder="50.110000"
            title="Latitude of the meetup pin (−90 to 90)"
            @keydown.enter.prevent="commitCoords"
            @blur="onCoordBlur"
            @paste="onCoordPaste"
          />
        </div>
        <div class="coords-field">
          <label for="location-lng">Lng</label>
          <input
            id="location-lng"
            v-model="lngText"
            type="text"
            inputmode="decimal"
            autocomplete="off"
            spellcheck="false"
            placeholder="8.680000"
            title="Longitude of the meetup pin (−180 to 180)"
            @keydown.enter.prevent="commitCoords"
            @blur="onCoordBlur"
            @paste="onCoordPaste"
          />
        </div>
      </div>
      <slot name="meta" />
    </div>
    <p v-if="!readonly" class="hint muted">
      Search for a place, click the map, or drag the marker. You can also type or paste coordinates.
    </p>
    <p v-if="coordsError" class="error coords-error">{{ coordsError }}</p>
  </div>
</template>

<style scoped>
.location-search {
  position: relative;
  margin-bottom: 0.55rem;
}
.location-search label {
  margin-bottom: 0.25rem;
}
.search-input-wrap {
  position: relative;
}
.search-input-wrap input {
  width: 100%;
  padding-right: 2.35rem;
}
.search-spinner {
  position: absolute;
  right: 0.65rem;
  top: 50%;
  width: 1rem;
  height: 1rem;
  transform: translateY(-50%);
  color: var(--muted, #888);
  pointer-events: none;
}
.search-spinner svg {
  display: block;
  width: 100%;
  height: 100%;
  animation: search-spin 0.75s linear infinite;
}
@keyframes search-spin {
  to {
    transform: rotate(360deg);
  }
}
.search-results {
  position: absolute;
  z-index: 1000;
  left: 0;
  right: 0;
  top: calc(100% + 0.2rem);
  margin: 0;
  padding: 0.25rem;
  list-style: none;
  max-height: 14rem;
  overflow: auto;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--bg-elevated);
  box-shadow: 0 8px 24px color-mix(in srgb, #000 18%, transparent);
}
.search-results button {
  display: block;
  width: 100%;
  text-align: left;
  border: none;
  background: transparent;
  color: var(--text);
  padding: 0.45rem 0.55rem;
  border-radius: calc(var(--radius) - 2px);
  font: inherit;
  cursor: pointer;
  line-height: 1.35;
}
.search-results button:hover,
.search-results button:focus-visible {
  background: color-mix(in srgb, var(--accent) 14%, var(--bg-input));
  outline: none;
}
.search-status {
  position: absolute;
  z-index: 999;
  left: 0;
  right: 0;
  top: calc(100% + 0.2rem);
  margin: 0;
  padding: 0.45rem 0.55rem;
  font-size: 0.85rem;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--bg-elevated);
  box-shadow: 0 8px 24px color-mix(in srgb, #000 18%, transparent);
}
.location-map {
  height: 320px;
  width: 100%;
  border-radius: var(--radius);
  border: 1px solid var(--border);
  z-index: 0;
}
.location-picker.readonly .location-map {
  height: 180px;
  border: none;
  border-radius: 0;
}
.location-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 0.75rem 1rem;
  margin-top: 0.5rem;
}
.coords-fields {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.5rem 0.75rem;
  flex: 1 1 16rem;
  min-width: min(100%, 16rem);
}
.coords-field {
  min-width: 0;
}
.coords-field label {
  margin-bottom: 0.25rem;
}
.coords-field input {
  width: 100%;
  font-variant-numeric: tabular-nums;
}
.hint {
  margin: 0.35rem 0 0;
  font-size: 0.85rem;
}
.coords-error {
  margin: 0.35rem 0 0;
}
:deep(.leaflet-bar) {
  border: 1px solid var(--border);
  border-radius: var(--radius);
  overflow: hidden;
  box-shadow: none;
}
:deep(.leaflet-bar a) {
  background: var(--bg-elevated);
  color: var(--text);
  border-bottom-color: var(--border);
  width: 2rem;
  height: 2rem;
  line-height: 2rem;
  font-size: 1.15rem;
  text-align: center;
  text-decoration: none;
}
:deep(.location-locate a span),
:deep(.location-clear a span) {
  display: block;
  line-height: 2rem;
  font-size: 1.05rem;
}
:deep(.location-clear a span) {
  font-size: 1.25rem;
}
:deep(.leaflet-bar a:hover),
:deep(.leaflet-bar a:focus) {
  background: var(--bg-input);
  color: var(--accent-hover);
  border-bottom-color: var(--border);
}
:deep(.leaflet-bar a.leaflet-disabled) {
  background: var(--bg-elevated);
  color: var(--muted);
  cursor: default;
}
</style>
