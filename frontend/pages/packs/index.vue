<script setup lang="ts">
import { sortLibraries, type LibrarySort } from "~/utils/iconLibraries";
import { downloadPacks, importPackFiles, packDownloadName } from "~/utils/packTransfer";
import { uploadIconsInBatches } from "~/utils/iconUpload";

type LibraryItem = {
  id: number;
  ownerId: string;
  ownerName?: string;
  name: string;
  description?: string;
  isDefault: boolean;
  published: boolean;
  mine: boolean;
  saved?: boolean;
  previews?: string[];
  createdAt?: string;
  updatedAt?: string;
  publishedAt?: string | null;
};

type PackStatus = "all" | "published" | "private" | "added";

type UploadDraft = {
  id: number;
  file: File;
  preview: string;
  name: string;
  category: string;
  pokemon: string[];
};

const { api, base } = useApi();
const { user, loaded, ensureAuth } = useAuth();
const toast = useToast();
const { options: categoryOptions } = useEventCategories();
const { ensure } = usePokemonList();

const libraries = ref<LibraryItem[]>([]);
const error = ref("");
const name = ref("");
const description = ref("");
const filterQ = ref("");
const filterStatus = ref<PackStatus>("all");
const sort = ref<LibrarySort>("updated_desc");
const busy = ref(false);
const busyId = ref<number | null>(null);
const importing = ref(false);
const exporting = ref(false);
const createOpen = ref(false);
const nameInput = ref<HTMLInputElement | null>(null);
const uploadTarget = ref<LibraryItem | null>(null);
const uploadOpen = ref(false);
const uploading = ref(false);
const uploadDone = ref(0);
const uploadTotal = ref(0);
const uploadError = ref("");
const fileInput = ref<HTMLInputElement | null>(null);
const drafts = ref<UploadDraft[]>([]);
let draftSeq = 0;

const filtersActive = computed(
  () => !!filterQ.value.trim() || filterStatus.value !== "all",
);

const mine = computed(() => {
  const list = libraries.value.filter((lib) => {
    const added = !!lib.saved && !lib.mine;
    if (filterStatus.value === "added") return added;
    if (!lib.mine && !added) return false;
    if (filterStatus.value === "published") return lib.mine && lib.published;
    if (filterStatus.value === "private") return lib.mine && !lib.published;
    return true;
  });
  return sortLibraries(list, sort.value);
});

onMounted(async () => {
  if (!(await ensureAuth())) return;
  void ensure();
  await load();
});

let filterTimer: ReturnType<typeof setTimeout> | null = null;
watch(filterQ, () => {
  if (filterTimer) clearTimeout(filterTimer);
  filterTimer = setTimeout(() => {
    void load();
  }, 300);
});

async function load() {
  error.value = "";
  try {
    const params = new URLSearchParams();
    if (filterQ.value.trim()) params.set("q", filterQ.value.trim());
    const qs = params.toString();
    const res = await api<{ libraries: LibraryItem[] }>(
      `/api/icon-libraries${qs ? `?${qs}` : ""}`,
    );
    libraries.value = res.libraries || [];
  } catch (e) {
    error.value = e instanceof Error ? e.message : "Failed to load packs";
  }
}

function openCreate() {
  error.value = "";
  name.value = "";
  description.value = "";
  createOpen.value = true;
  nextTick(() => nameInput.value?.focus());
}

function closeCreate() {
  if (busy.value) return;
  createOpen.value = false;
  name.value = "";
  description.value = "";
}

function onCreateKeydown(e: KeyboardEvent) {
  if (e.key !== "Escape") return;
  if (uploadOpen.value && !uploading.value) {
    closeUpload();
    return;
  }
  if (!createOpen.value || busy.value) return;
  closeCreate();
}

onMounted(() => window.addEventListener("keydown", onCreateKeydown));
onUnmounted(() => window.removeEventListener("keydown", onCreateKeydown));

async function exportOne(lib: LibraryItem) {
  if (busyId.value) return;
  busyId.value = lib.id;
  error.value = "";
  try {
    await downloadPacks(base, [lib.id], packDownloadName(lib.name, 1));
    toast.success("Pack exported", lib.name);
  } catch (e) {
    error.value = e instanceof Error ? e.message : "Failed to export pack";
  } finally {
    busyId.value = null;
  }
}

async function setPublished(lib: LibraryItem, publish: boolean) {
  if (busyId.value) return;
  busyId.value = lib.id;
  error.value = "";
  try {
    const updated = await api<LibraryItem>(
      `/api/icon-libraries/${lib.id}/${publish ? "publish" : "unpublish"}`,
      { method: "POST" },
    );
    const i = libraries.value.findIndex((row) => row.id === lib.id);
    if (i >= 0) libraries.value[i] = { ...libraries.value[i], ...updated };
    toast.success(publish ? "Pack published" : "Pack unpublished", lib.name);
  } catch (e) {
    error.value = e instanceof Error ? e.message : "Failed to update publishing";
  } finally {
    busyId.value = null;
  }
}

async function removeOwned(lib: LibraryItem) {
  if (!confirm(`Delete “${lib.name}” and its icons?`)) return;
  busyId.value = lib.id;
  error.value = "";
  try {
    await api(`/api/icon-libraries/${lib.id}`, { method: "DELETE" });
    libraries.value = libraries.value.filter((row) => row.id !== lib.id);
    toast.success("Pack deleted", lib.name);
  } catch (e) {
    error.value = e instanceof Error ? e.message : "Failed to delete pack";
  } finally {
    busyId.value = null;
  }
}

async function removeSaved(lib: LibraryItem) {
  if (!confirm(`Remove “${lib.name}” from My packs? The original stays published.`)) return;
  busyId.value = lib.id;
  error.value = "";
  try {
    await api(`/api/icon-libraries/${lib.id}/save`, { method: "DELETE" });
    lib.saved = false;
    libraries.value = libraries.value.filter((row) => row.id !== lib.id || row.mine);
    toast.success("Removed from My packs", lib.name);
  } catch (e) {
    error.value = e instanceof Error ? e.message : "Failed to remove pack";
  } finally {
    busyId.value = null;
  }
}
async function exportVisible() {
  const list = mine.value;
  if (!list.length || exporting.value) return;
  exporting.value = true;
  error.value = "";
  try {
    await downloadPacks(
      base,
      list.map((lib) => lib.id),
      packDownloadName(list[0].name, list.length),
    );
    toast.success(list.length === 1 ? "Pack exported" : `Exported ${list.length} packs`);
  } catch (e) {
    error.value = e instanceof Error ? e.message : "Failed to export packs";
  } finally {
    exporting.value = false;
  }
}

async function onImport(ev: Event) {
  const input = ev.target as HTMLInputElement;
  if (!input.files?.length || importing.value) return;
  importing.value = true;
  error.value = "";
  try {
    const res = await importPackFiles(base, Array.from(input.files));
    await load();
    const n = res.created ?? res.packs?.length ?? 0;
    toast.success(n === 1 ? "Pack imported" : `${n} packs imported`);
    input.value = "";
  } catch (e) {
    error.value = e instanceof Error ? e.message : "Import failed";
  } finally {
    importing.value = false;
  }
}

function startUpload(lib: LibraryItem) {
  uploadError.value = "";
  uploadTarget.value = lib;
  fileInput.value?.click();
}

function clearDrafts() {
  for (const draft of drafts.value) URL.revokeObjectURL(draft.preview);
  drafts.value = [];
}

function closeUpload() {
  if (uploading.value) return;
  uploadOpen.value = false;
  uploadTarget.value = null;
  clearDrafts();
}

function onFiles(ev: Event) {
  const input = ev.target as HTMLInputElement;
  const files = input.files;
  if (!files?.length) {
    if (!drafts.value.length) uploadTarget.value = null;
    return;
  }
  for (const file of files) {
    const baseName = file.name.replace(/\.[^.]+$/, "") || file.name;
    drafts.value.push({
      id: ++draftSeq,
      file,
      preview: URL.createObjectURL(file),
      name: baseName.slice(0, 80),
      category: "",
      pokemon: [],
    });
  }
  input.value = "";
  uploadOpen.value = true;
}

function removeDraft(id: number) {
  const draft = drafts.value.find((row) => row.id === id);
  if (draft) URL.revokeObjectURL(draft.preview);
  drafts.value = drafts.value.filter((row) => row.id !== id);
}

async function submitUpload() {
  if (!drafts.value.length || !uploadTarget.value) return;
  uploading.value = true;
  uploadError.value = "";
  uploadDone.value = 0;
  uploadTotal.value = drafts.value.length;
  const targetId = uploadTarget.value.id;
  const packName = uploadTarget.value.name;
  try {
    const { errors, uploaded } = await uploadIconsInBatches(
      (body) =>
        api<{ errors?: string[] }>(`/api/icon-libraries/${targetId}/icons`, {
          method: "POST",
          body,
        }),
      drafts.value,
      (done, total) => {
        uploadDone.value = done;
        uploadTotal.value = total;
      },
    );
    const finished = drafts.value.slice(0, uploaded);
    for (const draft of finished) URL.revokeObjectURL(draft.preview);
    drafts.value = drafts.value.slice(uploaded);
    if (errors.length) uploadError.value = errors.join(" ");
    if (uploaded) {
      toast.success(
        uploaded === 1 ? "Icon uploaded" : `${uploaded} icons uploaded`,
        packName,
      );
      await load();
    }
    if (!drafts.value.length) {
      uploadOpen.value = false;
      uploadTarget.value = null;
    }
  } catch (e) {
    uploadError.value = e instanceof Error ? e.message : "Upload failed";
  } finally {
    uploading.value = false;
  }
}

async function createLibrary() {
  const trimmed = name.value.trim();
  if (!trimmed) return;
  busy.value = true;
  error.value = "";
  try {
    const created = await api<LibraryItem>("/api/icon-libraries", {
      method: "POST",
      body: JSON.stringify({ name: trimmed, description: description.value.trim() }),
    });
    name.value = "";
    description.value = "";
    createOpen.value = false;
    await navigateTo(`/packs/${created.id}`);
  } catch (e) {
    error.value = e instanceof Error ? e.message : "Failed to create pack";
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <AppShell>
    <div v-if="!loaded" class="panel">
      <p class="muted">Loading…</p>
    </div>
    <div v-else-if="user" class="panel">
      <div class="page-head">
        <div>
          <h2>My packs</h2>
          <p class="muted">
            Your packs, plus published packs you add from
            <NuxtLink to="/packs/browse">Browse packs</NuxtLink>.
            Added packs stay read-only and follow the original.
          </p>
        </div>
        <div class="page-actions">
          <label class="btn">
            <Icon name="import" />
            {{ importing ? "Importing…" : "Import" }}
            <input
              type="file"
              accept=".zip,application/zip"
              multiple
              hidden
              :disabled="importing"
              @change="onImport"
            />
          </label>
          <button
            type="button"
            class="btn"
            :disabled="!mine.length || exporting"
            title="Export the packs shown as a zip"
            @click="exportVisible"
          >
            <Icon name="export" />
            {{ exporting ? "Exporting…" : "Export" }}{{ filtersActive && mine.length ? ` (${mine.length})` : "" }}
          </button>
          <button type="button" class="primary" @click="openCreate">
            <Icon name="plus" />
            New pack
          </button>
        </div>
      </div>

      <div
        v-if="createOpen"
        class="modal-backdrop"
        @click.self="closeCreate"
      >
        <form class="modal" role="dialog" aria-modal="true" aria-labelledby="new-library-title" @submit.prevent="createLibrary">
          <h2 id="new-library-title">New pack</h2>
          <p class="muted">Extra packs can be published. Your default pack stays private.</p>
          <div class="field">
            <label for="new-library-name">Name</label>
            <input
              id="new-library-name"
              ref="nameInput"
              v-model="name"
              type="text"
              maxlength="80"
              required
              placeholder="Pack name"
            />
          </div>
          <div class="field">
            <label for="new-library-description">Description</label>
            <textarea
              id="new-library-description"
              v-model="description"
              maxlength="500"
              rows="3"
              placeholder="Optional"
            />
          </div>
          <p v-if="error" class="error">{{ error }}</p>
          <div class="modal-actions">
            <button type="button" :disabled="busy" @click="closeCreate">Cancel</button>
            <button type="submit" class="primary" :disabled="busy || !name.trim()">
              {{ busy ? "Creating…" : "Create" }}
            </button>
          </div>
        </form>
      </div>

      <p v-if="error && !createOpen" class="error">{{ error }}</p>

      <div class="filters">
        <div class="field" style="margin-bottom: 0">
          <label for="icon-q">Search</label>
          <input id="icon-q" v-model="filterQ" type="search" placeholder="Pack name or description…" />
        </div>
        <div class="field" style="margin-bottom: 0">
          <label for="icon-status">Status</label>
          <select id="icon-status" v-model="filterStatus">
            <option value="all">All</option>
            <option value="published">Published</option>
            <option value="private">Private</option>
            <option value="added">Added</option>
          </select>
        </div>
        <div class="field" style="margin-bottom: 0">
          <label for="icon-sort">Sort</label>
          <select id="icon-sort" v-model="sort">
            <option value="updated_desc">Recently updated</option>
            <option value="created_desc">Newest created</option>
            <option value="published_desc">Newest published</option>
            <option value="name_asc">Name A–Z</option>
          </select>
        </div>
      </div>

      <div v-if="mine.length" class="lib-grid">
        <IconLibraryCard
          v-for="lib in mine"
          :key="lib.id"
          :name="lib.name"
          :description="lib.description"
          :detail="lib.mine ? '' : lib.ownerName"
          :to="`/packs/${lib.id}`"
          :previews="lib.previews"
          :badge="lib.isDefault ? 'Default' : lib.saved && !lib.mine ? 'Added' : lib.published ? 'Published' : 'Private'"
        >
          <template #action>
            <button
              v-if="lib.mine"
              type="button"
              class="icon-btn icon-btn-accent"
              title="Edit"
              aria-label="Edit"
              @click="navigateTo(`/packs/${lib.id}`)"
            >
              <Icon name="edit" />
            </button>
            <button
              v-if="lib.mine"
              type="button"
              class="icon-btn"
              title="Upload"
              aria-label="Upload"
              @click="startUpload(lib)"
            >
              <Icon name="import" />
            </button>
            <button
              v-if="lib.mine && !lib.isDefault && !lib.published"
              type="button"
              class="icon-btn"
              title="Publish"
              aria-label="Publish"
              :disabled="busyId === lib.id"
              @click="setPublished(lib, true)"
            >
              <Icon name="publish" />
            </button>
            <button
              v-if="lib.mine && !lib.isDefault && lib.published"
              type="button"
              class="icon-btn"
              title="Unpublish"
              aria-label="Unpublish"
              :disabled="busyId === lib.id"
              @click="setPublished(lib, false)"
            >
              <Icon name="unpublish" />
            </button>
            <button
              type="button"
              class="icon-btn"
              title="Export"
              aria-label="Export"
              :disabled="busyId === lib.id"
              @click="exportOne(lib)"
            >
              <Icon name="export" />
            </button>
            <button
              v-if="lib.saved && !lib.mine"
              type="button"
              class="icon-btn icon-btn-danger"
              title="Remove from my packs"
              aria-label="Remove from my packs"
              :disabled="busyId === lib.id"
              @click="removeSaved(lib)"
            >
              <Icon name="trash" />
            </button>
            <button
              v-if="lib.mine && !lib.isDefault"
              type="button"
              class="icon-btn icon-btn-danger"
              title="Delete"
              aria-label="Delete"
              :disabled="busyId === lib.id"
              @click="removeOwned(lib)"
            >
              <Icon name="trash" />
            </button>
          </template>
        </IconLibraryCard>
      </div>
      <p v-else class="muted">No packs match.</p>

      <input
        ref="fileInput"
        type="file"
        accept="image/jpeg,image/png,image/webp"
        multiple
        hidden
        :disabled="uploading"
        @change="onFiles"
      />
      <div v-if="uploadOpen && uploadTarget" class="modal-backdrop" @click.self="closeUpload">
        <form
          class="modal upload-modal"
          role="dialog"
          aria-modal="true"
          aria-labelledby="upload-title"
          @submit.prevent="submitUpload"
        >
          <div class="upload-head">
            <div>
              <h2 id="upload-title">Upload to {{ uploadTarget.name }}</h2>
              <p class="muted">
                Name each image, then set a category and Pokémon. JPEG, PNG, or WebP, up to 8MB.
              </p>
            </div>
            <button type="button" class="btn" :disabled="uploading" @click="fileInput?.click()">
              Add images
            </button>
          </div>
          <ul v-if="drafts.length" class="drafts">
            <li v-for="draft in drafts" :key="draft.id">
              <img :src="draft.preview" alt="" />
              <IconMetaFields
                v-model:name="draft.name"
                v-model:category="draft.category"
                v-model:pokemon="draft.pokemon"
                compact
                :categories="categoryOptions"
              />
              <button
                type="button"
                class="icon-btn"
                title="Remove"
                aria-label="Remove"
                :disabled="uploading"
                @click="removeDraft(draft.id)"
              >
                <Icon name="trash" />
              </button>
            </li>
          </ul>
          <p v-else class="upload-empty muted">No images selected yet.</p>
          <p v-if="uploadError" class="error upload-error">{{ uploadError }}</p>
          <div class="modal-actions">
            <button type="button" :disabled="uploading" @click="closeUpload">Cancel</button>
            <button type="submit" class="primary" :disabled="uploading || !drafts.length">
              {{
                uploading
                  ? `Uploading ${uploadDone} of ${uploadTotal}`
                  : `Upload${drafts.length ? ` ${drafts.length}` : ""}`
              }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </AppShell>
</template>

<style scoped>
.page-head {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
  align-items: flex-start;
  margin-bottom: 1rem;
}
.page-head h2 {
  margin: 0 0 0.35rem;
  font-size: 1.15rem;
}
.page-head .muted {
  margin: 0;
}
.page-actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
  align-items: center;
}
.filters {
  display: grid;
  grid-template-columns: minmax(10rem, 1.4fr) minmax(8rem, 0.8fr) minmax(10rem, 1fr);
  gap: 0.65rem 0.75rem;
  margin: 0 0 1rem;
  align-items: end;
}
@media (max-width: 900px) {
  .filters {
    grid-template-columns: 1fr 1fr;
  }
}
@media (max-width: 560px) {
  .filters {
    grid-template-columns: 1fr;
  }
}
.lib-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 0.75rem;
}
.upload-modal {
  width: min(820px, 100%);
  max-height: min(88vh, 46rem);
  display: flex;
  flex-direction: column;
  padding: 0;
  overflow: hidden;
}
.upload-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 1rem;
  padding: 1.1rem 1.2rem 0.9rem;
  border-bottom: 1px solid var(--border);
}
.upload-head h2 {
  margin: 0 0 0.3rem;
  font-size: 1.15rem;
}
.upload-head .muted {
  margin: 0;
  font-size: 0.85rem;
}
.upload-head .btn {
  flex: none;
}
.drafts {
  list-style: none;
  margin: 0;
  padding: 0.85rem 1.2rem;
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  overflow: auto;
}
.drafts li {
  display: grid;
  grid-template-columns: 84px minmax(0, 1fr) auto;
  gap: 0.85rem;
  align-items: start;
  padding: 0.7rem;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--bg);
}
.drafts img {
  width: 84px;
  height: 84px;
  object-fit: cover;
  border-radius: 6px;
  background: var(--bg-elevated);
}
.upload-empty,
.upload-error {
  margin: 0.85rem 1.2rem;
}
.upload-modal .modal-actions {
  margin-top: 0;
  padding: 0.85rem 1.2rem;
  border-top: 1px solid var(--border);
}
</style>
