<script setup lang="ts">
import { coverImageSrc } from "~/utils/covers";
import { downloadPacks, packDownloadName } from "~/utils/packTransfer";
import { uploadIconsInBatches } from "~/utils/iconUpload";

type LibraryItem = {
  id: number;
  name: string;
  description?: string;
  isDefault: boolean;
  published: boolean;
  mine: boolean;
  saved?: boolean;
  ownerId: string;
  ownerName?: string;
};

type IconItem = {
  id: number;
  filename: string;
  name: string;
  category: string;
  pokemon: string[];
  url: string;
  createdAt?: string;
};

type IconSort = "newest" | "name_asc";

type UploadDraft = {
  id: number;
  file: File;
  preview: string;
  name: string;
  category: string;
  pokemon: string[];
};

const route = useRoute();
const { api, base } = useApi();
const { apiBase } = useRuntimeConfig().public;
const { ensureAuth, user, loaded } = useAuth();
const { options: categoryOptions } = useEventCategories();
const { ensure, nameOf } = usePokemonList();
const toast = useToast();

const libraryId = computed(() => Number(route.params.id));
const library = ref<LibraryItem | null>(null);
const icons = ref<IconItem[]>([]);
const error = ref("");
const name = ref("");
const description = ref("");
const uploading = ref(false);
const uploadDone = ref(0);
const uploadTotal = ref(0);
const uploadOpen = ref(false);
const uploadError = ref("");
const fileInput = ref<HTMLInputElement | null>(null);
const drafts = ref<UploadDraft[]>([]);
let draftSeq = 0;
const filterQ = ref("");
const filterCategory = ref("");
const filterSort = ref<IconSort>("newest");
const pokemonSlug = ref("");
const saving = ref(false);
const exporting = ref(false);
const detailsOpen = ref(false);
const detailsBusy = ref(false);
const detailsError = ref("");
const nameInput = ref<HTMLInputElement | null>(null);
const editing = ref<IconItem | null>(null);
const editName = ref("");
const editCategory = ref("");
const editPokemon = ref<string[]>([]);
const editError = ref("");
const editBusy = ref(false);

const visibleIcons = computed(() => {
  const q = filterQ.value.trim().toLowerCase();
  const list = icons.value.filter((icon) => {
    if (filterCategory.value && icon.category !== filterCategory.value) return false;
    if (pokemonSlug.value && !icon.pokemon.includes(pokemonSlug.value)) return false;
    if (q) {
      const hay = [icon.name, icon.filename, icon.category, ...icon.pokemon].join(" ").toLowerCase();
      if (!hay.includes(q)) return false;
    }
    return true;
  });
  return [...list].sort((a, b) => {
    if (filterSort.value === "name_asc") {
      return (a.name || a.filename).localeCompare(b.name || b.filename) || b.id - a.id;
    }
    return (b.createdAt || "").localeCompare(a.createdAt || "") || b.id - a.id;
  });
});

const canEdit = computed(
  () => !!library.value && !!user.value && library.value.ownerId === user.value.id,
);

const badge = computed(() => {
  const lib = library.value;
  if (!lib) return "";
  if (lib.isDefault) return "Default";
  if (lib.saved && !lib.mine) return "Added";
  return lib.published ? "Published" : "Private";
});

onMounted(async () => {
  if (!(await ensureAuth())) return;
  void ensure();
  await load();
});

watch(libraryId, () => {
  void load();
});

async function load() {
  error.value = "";
  try {
    const libs = await api<{ libraries: LibraryItem[] }>("/api/icon-libraries");
    library.value = (libs.libraries || []).find((l) => l.id === libraryId.value) || null;
    if (!library.value) {
      error.value = "Pack not found";
      icons.value = [];
      return;
    }
    if (!detailsOpen.value) {
      name.value = library.value.name;
      description.value = library.value.description || "";
    }
    const res = await api<{ icons: IconItem[] }>(`/api/icon-libraries/${libraryId.value}/icons`);
    icons.value = res.icons || [];
  } catch (e) {
    error.value = e instanceof Error ? e.message : "Failed to load pack";
  }
}

function openDetails() {
  if (!canEdit.value || !library.value) return;
  name.value = library.value.name;
  description.value = library.value.description || "";
  detailsError.value = "";
  detailsOpen.value = true;
  nextTick(() => nameInput.value?.focus());
}

function closeDetails() {
  if (detailsBusy.value) return;
  detailsOpen.value = false;
}

async function saveDetails() {
  if (!canEdit.value || !library.value || detailsBusy.value) return;
  const trimmedName = name.value.trim();
  const trimmedDesc = description.value.trim();
  if (!library.value.isDefault && (!trimmedName || trimmedName.length > 80)) {
    detailsError.value = "Pack name is required (max 80 characters)";
    return;
  }
  if (trimmedDesc.length > 500) {
    detailsError.value = "Description must be 500 characters or fewer";
    return;
  }
  const body: { name?: string; description: string } = { description: trimmedDesc };
  if (!library.value.isDefault) body.name = trimmedName;
  const unchanged =
    trimmedDesc === (library.value.description || "") &&
    (library.value.isDefault || trimmedName === library.value.name);
  if (unchanged) {
    detailsOpen.value = false;
    return;
  }
  detailsBusy.value = true;
  detailsError.value = "";
  try {
    library.value = await api<LibraryItem>(`/api/icon-libraries/${libraryId.value}`, {
      method: "PATCH",
      body: JSON.stringify(body),
    });
    name.value = library.value.name;
    description.value = library.value.description || "";
    detailsOpen.value = false;
    toast.success("Pack updated", library.value.name);
  } catch (e) {
    detailsError.value = e instanceof Error ? e.message : "Failed to update pack";
  } finally {
    detailsBusy.value = false;
  }
}

async function setPublished(publish: boolean) {
  error.value = "";
  try {
    library.value = await api<LibraryItem>(
      `/api/icon-libraries/${libraryId.value}/${publish ? "publish" : "unpublish"}`,
      { method: "POST" },
    );
    toast.success(publish ? "Pack published" : "Pack unpublished", library.value?.name);
  } catch (e) {
    error.value = e instanceof Error ? e.message : "Failed to update publishing";
  }
}

async function exportPack() {
  if (!library.value || exporting.value) return;
  exporting.value = true;
  error.value = "";
  try {
    await downloadPacks(base, [library.value.id], packDownloadName(library.value.name, 1));
    toast.success("Pack exported", library.value.name);
  } catch (e) {
    error.value = e instanceof Error ? e.message : "Failed to export pack";
  } finally {
    exporting.value = false;
  }
}

async function addToMine() {
  if (!library.value || library.value.mine) return;
  saving.value = true;
  error.value = "";
  try {
    const updated = await api<LibraryItem>(`/api/icon-libraries/${libraryId.value}/save`, {
      method: "POST",
    });
    library.value = { ...library.value, ...updated, saved: true };
    toast.success("Added to My packs", library.value.name);
  } catch (e) {
    error.value = e instanceof Error ? e.message : "Failed to add pack";
  } finally {
    saving.value = false;
  }
}

async function removeFromMine() {
  if (!library.value) return;
  if (!confirm(`Remove “${library.value.name}” from My packs? The original stays published.`)) return;
  saving.value = true;
  error.value = "";
  try {
    await api(`/api/icon-libraries/${libraryId.value}/save`, { method: "DELETE" });
    library.value.saved = false;
    toast.success("Removed from My packs", library.value.name);
  } catch (e) {
    error.value = e instanceof Error ? e.message : "Failed to remove pack";
  } finally {
    saving.value = false;
  }
}

async function removeLibrary() {
  if (!confirm("Delete this pack and its icons?")) return;
  try {
    await api(`/api/icon-libraries/${libraryId.value}`, { method: "DELETE" });
    await navigateTo("/packs");
  } catch (e) {
    error.value = e instanceof Error ? e.message : "Failed to delete pack";
  }
}

function openUpload() {
  uploadError.value = "";
  fileInput.value?.click();
}

function clearDrafts() {
  for (const draft of drafts.value) URL.revokeObjectURL(draft.preview);
  drafts.value = [];
}

function closeUpload() {
  if (uploading.value) return;
  uploadOpen.value = false;
  clearDrafts();
}

function openEdit(icon: IconItem) {
  editing.value = icon;
  editName.value = icon.name;
  editCategory.value = icon.category;
  editPokemon.value = [...icon.pokemon];
  editError.value = "";
}

function closeEdit() {
  if (editBusy.value) return;
  editing.value = null;
}

function onKeydown(e: KeyboardEvent) {
  if (e.key !== "Escape") return;
  if (detailsOpen.value && !detailsBusy.value) {
    closeDetails();
    return;
  }
  if (uploadOpen.value && !uploading.value) closeUpload();
  else if (editing.value && !editBusy.value) closeEdit();
}

onMounted(() => window.addEventListener("keydown", onKeydown));
onUnmounted(() => window.removeEventListener("keydown", onKeydown));

function onFiles(ev: Event) {
  const input = ev.target as HTMLInputElement;
  const files = input.files;
  if (!files?.length) return;
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
  if (!drafts.value.length) return;
  uploading.value = true;
  uploadError.value = "";
  uploadDone.value = 0;
  uploadTotal.value = drafts.value.length;
  try {
    const { errors, uploaded } = await uploadIconsInBatches(
      (body) =>
        api<{ errors?: string[] }>(`/api/icon-libraries/${libraryId.value}/icons`, {
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
    if (uploaded) await load();
    if (!drafts.value.length) uploadOpen.value = false;
  } catch (e) {
    uploadError.value = e instanceof Error ? e.message : "Upload failed";
  } finally {
    uploading.value = false;
  }
}

async function saveEdit() {
  if (!editing.value) return;
  editBusy.value = true;
  editError.value = "";
  try {
    const updated = await api<IconItem>(`/api/icons/${editing.value.id}`, {
      method: "PATCH",
      body: JSON.stringify({
        name: editName.value,
        category: editCategory.value,
        pokemon: editPokemon.value,
      }),
    });
    const i = icons.value.findIndex((row) => row.id === editing.value?.id);
    if (i >= 0) icons.value[i] = { ...icons.value[i], ...updated };
    editing.value = null;
    toast.success("Icon saved");
  } catch (e) {
    editError.value = e instanceof Error ? e.message : "Failed to save icon";
  } finally {
    editBusy.value = false;
  }
}

async function deleteIcon(icon: IconItem) {
  if (!confirm(`Delete “${icon.name || icon.filename}”?`)) return;
  error.value = "";
  try {
    await api(`/api/icons/${icon.id}`, { method: "DELETE" });
    icons.value = icons.value.filter((row) => row.id !== icon.id);
  } catch (e) {
    error.value = e instanceof Error ? e.message : "Failed to delete icon";
  }
}

function src(url: string) {
  return coverImageSrc(url, apiBase as string);
}
</script>

<template>
  <AppShell>
    <div v-if="!loaded" class="panel">
      <p class="muted">Loading…</p>
    </div>
    <div v-else-if="library" class="panel">
      <p class="crumbs muted">
        <NuxtLink to="/packs">My packs</NuxtLink>
        <template v-if="library.published">
          · <NuxtLink to="/packs/browse">Browse packs</NuxtLink>
        </template>
      </p>

      <div class="page-head">
        <div class="title-block">
          <div class="title-row">
            <h2>{{ library.name }}</h2>
            <span class="badge">{{ badge }}</span>
            <button
              v-if="canEdit"
              type="button"
              class="icon-btn"
              title="Edit pack"
              aria-label="Edit pack"
              @click="openDetails"
            >
              <Icon name="edit" />
            </button>
          </div>
          <p v-if="library.isDefault" class="hint muted">This default pack stays private.</p>
          <p v-else-if="!library.mine" class="hint muted">
            Read-only<span v-if="library.ownerName"> · {{ library.ownerName }}</span>
          </p>
          <p v-else-if="library.published" class="hint muted">Published for other people to add.</p>
          <p v-else class="hint muted">Private until you publish it.</p>
          <p v-if="library.description" class="pack-description">{{ library.description }}</p>
        </div>
        <div class="page-actions">
          <button
            v-if="!library.mine && library.published && !library.saved"
            type="button"
            class="primary"
            :disabled="saving"
            @click="addToMine"
          >
            <Icon name="add" />
            Add to my packs
          </button>
          <button
            v-else-if="!library.mine && library.saved"
            type="button"
            class="btn"
            :disabled="saving"
            @click="removeFromMine"
          >
            Remove from my packs
          </button>
          <button
            v-if="canEdit && !library.isDefault && !library.published"
            type="button"
            class="btn"
            @click="setPublished(true)"
          >
            <Icon name="publish" />
            Publish
          </button>
          <button
            v-else-if="canEdit && !library.isDefault && library.published"
            type="button"
            class="btn"
            @click="setPublished(false)"
          >
            <Icon name="unpublish" />
            Unpublish
          </button>
          <button type="button" class="btn" :disabled="exporting" @click="exportPack">
            <Icon name="export" />
            {{ exporting ? "Exporting…" : "Export" }}
          </button>
          <button
            v-if="canEdit && !library.isDefault"
            type="button"
            class="btn danger"
            @click="removeLibrary"
          >
            <Icon name="trash" />
            Delete
          </button>
          <button v-if="canEdit" type="button" class="primary" @click="openUpload">
            <Icon name="import" />
            Upload
          </button>
          <input
            v-if="canEdit"
            ref="fileInput"
            type="file"
            accept="image/jpeg,image/png,image/webp"
            multiple
            hidden
            :disabled="uploading"
            @change="onFiles"
          />
        </div>
      </div>

      <p v-if="error" class="error">{{ error }}</p>

      <div v-if="icons.length" class="filters">
        <div class="field">
          <label for="lib-icons-q">Search</label>
          <input id="lib-icons-q" v-model="filterQ" type="search" placeholder="Name…" />
        </div>
        <div class="field">
          <label for="lib-icons-cat">Category</label>
          <select id="lib-icons-cat" v-model="filterCategory">
            <option value="">Any</option>
            <option v-for="opt in categoryOptions" :key="opt" :value="opt">{{ opt }}</option>
          </select>
        </div>
        <div class="field">
          <label for="lib-icons-poke">Pokémon</label>
          <PokemonSelect id="lib-icons-poke" v-model="pokemonSlug" placeholder="Any" />
        </div>
        <div class="field">
          <label for="lib-icons-sort">Sort</label>
          <select id="lib-icons-sort" v-model="filterSort">
            <option value="newest">Newest</option>
            <option value="name_asc">Name A–Z</option>
          </select>
        </div>
      </div>

      <div v-if="visibleIcons.length" class="icon-grid">
        <article v-for="icon in visibleIcons" :key="icon.id" class="icon-card">
          <img :src="src(icon.url)" :alt="icon.name || icon.filename" />
          <div class="icon-body">
            <div class="icon-title-row">
              <strong>{{ icon.name || icon.filename }}</strong>
              <div v-if="canEdit" class="card-actions">
                <button type="button" class="icon-btn" title="Edit" aria-label="Edit" @click="openEdit(icon)">
                  <Icon name="edit" />
                </button>
                <button
                  type="button"
                  class="icon-btn icon-btn-danger"
                  title="Delete"
                  aria-label="Delete"
                  @click="deleteIcon(icon)"
                >
                  <Icon name="trash" />
                </button>
              </div>
            </div>
            <div v-if="icon.category || icon.pokemon.length" class="tags">
              <span v-if="icon.category" class="tag">{{ icon.category }}</span>
              <span v-for="slug in icon.pokemon.slice(0, 3)" :key="slug" class="tag">{{ nameOf(slug) }}</span>
              <span v-if="icon.pokemon.length > 3" class="tag">+{{ icon.pokemon.length - 3 }}</span>
            </div>
          </div>
        </article>
      </div>
      <p v-else-if="icons.length" class="muted">No icons match these filters.</p>
      <p v-else class="muted empty-pack">No icons in this pack yet.</p>

      <div v-if="uploadOpen" class="modal-backdrop" @click.self="closeUpload">
        <form
          class="modal upload-modal"
          role="dialog"
          aria-modal="true"
          aria-labelledby="upload-title"
          @submit.prevent="submitUpload"
        >
          <div class="upload-head">
            <div>
              <h2 id="upload-title">Upload icons</h2>
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

      <div v-if="detailsOpen" class="modal-backdrop" @click.self="closeDetails">
        <form class="modal edit-modal" role="dialog" aria-modal="true" aria-labelledby="edit-pack-title" @submit.prevent="saveDetails">
          <h2 id="edit-pack-title">Edit pack</h2>
          <div class="field">
            <label for="pack-name">Name</label>
            <input
              id="pack-name"
              ref="nameInput"
              v-model="name"
              type="text"
              maxlength="80"
              required
              :disabled="library.isDefault || detailsBusy"
            />
          </div>
          <p v-if="library.isDefault" class="hint muted">The default pack name stays as it is.</p>
          <div class="field">
            <label for="pack-description">Description</label>
            <textarea
              id="pack-description"
              v-model="description"
              maxlength="500"
              rows="4"
              placeholder="What this pack is for"
              :disabled="detailsBusy"
            />
          </div>
          <p v-if="detailsError" class="error">{{ detailsError }}</p>
          <div class="modal-actions">
            <button type="button" :disabled="detailsBusy" @click="closeDetails">Cancel</button>
            <button type="submit" class="primary" :disabled="detailsBusy">
              {{ detailsBusy ? "Saving…" : "Save" }}
            </button>
          </div>
        </form>
      </div>

      <div v-if="editing" class="modal-backdrop" @click.self="closeEdit">
        <form class="modal edit-modal" role="dialog" aria-modal="true" aria-labelledby="edit-icon-title" @submit.prevent="saveEdit">
          <h2 id="edit-icon-title">Edit icon</h2>
          <img class="edit-preview" :src="src(editing.url)" alt="" />
          <IconMetaFields
            v-model:name="editName"
            v-model:category="editCategory"
            v-model:pokemon="editPokemon"
            :categories="categoryOptions"
          />
          <p v-if="editError" class="error">{{ editError }}</p>
          <div class="modal-actions">
            <button type="button" :disabled="editBusy" @click="closeEdit">Cancel</button>
            <button type="submit" class="primary" :disabled="editBusy">
              {{ editBusy ? "Saving…" : "Save" }}
            </button>
          </div>
        </form>
      </div>
    </div>
    <div v-else-if="user" class="panel">
      <p v-if="error" class="error">{{ error }}</p>
      <p v-else class="muted">Loading…</p>
    </div>
  </AppShell>
</template>

<style scoped>
.crumbs {
  margin: 0 0 0.75rem;
}
.page-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 1rem;
  flex-wrap: wrap;
  margin-bottom: 1rem;
}
.title-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.45rem 0.6rem;
}
.page-head h2 {
  margin: 0;
  font-size: 1.15rem;
}
.hint {
  margin: 0.3rem 0 0;
}
.pack-description {
  margin: 0.55rem 0 0;
  max-width: 40rem;
  white-space: pre-wrap;
}
.badge {
  font-size: 0.7rem;
  font-weight: 600;
  padding: 0.12rem 0.45rem;
  border-radius: 999px;
  border: 1px solid var(--border);
  color: var(--muted);
}
.page-actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
  align-items: center;
}
.edit-modal textarea {
  width: 100%;
  min-height: 6rem;
  resize: vertical;
}
.filters {
  display: grid;
  grid-template-columns: minmax(8rem, 1.3fr) minmax(7rem, 0.9fr) minmax(8rem, 1fr) minmax(7rem, 0.8fr);
  gap: 0.65rem 0.75rem;
  margin: 0 0 1rem;
  align-items: end;
}
.filters .field {
  margin-bottom: 0;
}
.icon-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 0.75rem;
}
.icon-card {
  display: flex;
  flex-direction: column;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  overflow: hidden;
}
.icon-card img {
  width: 100%;
  aspect-ratio: 1;
  object-fit: cover;
  background: var(--bg-elevated);
  display: block;
}
.icon-body {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  padding: 0.65rem 0.7rem 0.75rem;
}
.icon-title-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.35rem;
}
.icon-title-row strong {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.card-actions {
  display: flex;
  gap: 0.2rem;
  flex: none;
}
.tags {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
}
.tag {
  font-size: 0.7rem;
  font-weight: 600;
  padding: 0.1rem 0.4rem;
  border-radius: 999px;
  border: 1px solid var(--border);
  color: var(--muted);
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.empty-pack {
  margin: 0.25rem 0 0;
}
.edit-modal {
  width: min(28rem, 100%);
}
.edit-preview {
  width: 96px;
  height: 96px;
  object-fit: cover;
  border-radius: 6px;
  background: var(--bg);
  display: block;
  margin-bottom: 0.85rem;
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
@media (max-width: 800px) {
  .filters {
    grid-template-columns: 1fr 1fr;
  }
}
@media (max-width: 560px) {
  .filters {
    grid-template-columns: 1fr;
  }
}
</style>
