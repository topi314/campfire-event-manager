package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/topi314/campfire-event-manager/server/auth"
	"github.com/topi314/campfire-event-manager/server/database"
	"github.com/topi314/campfire-event-manager/server/database/dbsqlc"
)

const (
	packExportManifest  = "campfire-packs.json"
	packExportVersion   = 1
	maxExportPacks      = 40
	maxImportPacks      = 40
	maxIconsPerImported = 500
	maxPackZipBytes     = 80 << 20
)

type packExportDoc struct {
	Version int              `json:"version"`
	Packs   []packExportPack `json:"packs"`
}

type packExportPack struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Icons       []packExportIcon `json:"icons"`
}

type packExportIcon struct {
	Name     string   `json:"name,omitempty"`
	Filename string   `json:"filename,omitempty"`
	Category string   `json:"category,omitempty"`
	Pokemon  []string `json:"pokemon,omitempty"`
	File     string   `json:"file"`
}

func (s *Server) exportIconLibraries(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.GetSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid export request")
		return
	}
	if len(body.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "choose at least one pack")
		return
	}
	if len(body.IDs) > maxExportPacks {
		writeError(w, http.StatusBadRequest, "too many packs to export")
		return
	}
	doc := packExportDoc{Version: packExportVersion}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i, id := range body.IDs {
		lib, err := s.db.GetIconLibrary(r.Context(), id)
		if err != nil || !packVisible(lib, session.Session.UserID) {
			writeError(w, http.StatusNotFound, "pack not found")
			return
		}
		rows, err := s.db.ListIconsByLibrary(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to export pack")
			return
		}
		exported := packExportPack{Name: lib.LibraryName, Description: lib.LibraryDescription}
		for j, row := range rows {
			data, err := s.readIconFile(row.IconStoragePath)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to export pack")
				return
			}
			ext := path.Ext(row.IconFilename)
			if ext == "" {
				_, sniffed, ok := sniffIcon(row.IconContentType, data)
				if ok {
					ext = sniffed
				}
			}
			rel := fmt.Sprintf("packs/%d/icons/%d%s", i, j, ext)
			if err := writeZipFile(zw, rel, data); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to export pack")
				return
			}
			exported.Icons = append(exported.Icons, packExportIcon{
				Name:     row.IconName,
				Filename: row.IconFilename,
				Category: row.IconCategory.String,
				Pokemon:  nonEmpty(row.PokemonSlugs),
				File:     rel,
			})
		}
		if exported.Icons == nil {
			exported.Icons = []packExportIcon{}
		}
		doc.Packs = append(doc.Packs, exported)
	}
	manifest, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to export pack")
		return
	}
	if err := writeZipFile(zw, packExportManifest, manifest); err != nil || zw.Close() != nil {
		writeError(w, http.StatusInternalServerError, "failed to export pack")
		return
	}
	if buf.Len() > maxPackZipBytes {
		writeError(w, http.StatusBadRequest, "export is too large")
		return
	}
	name := "campfire-packs.zip"
	if len(doc.Packs) == 1 {
		name = safePackFilename(doc.Packs[0].Name) + ".zip"
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

func (s *Server) importIconLibraries(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.GetSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	if err := r.ParseMultipartForm(maxPackZipBytes); err != nil {
		writeError(w, http.StatusBadRequest, "invalid upload")
		return
	}
	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		files = r.MultipartForm.File["files"]
	}
	if len(files) == 0 {
		writeError(w, http.StatusBadRequest, "no pack file uploaded")
		return
	}
	type imported struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	var created []imported
	for _, hdr := range files {
		f, err := hdr.Open()
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to read upload")
			return
		}
		data, err := io.ReadAll(io.LimitReader(f, maxPackZipBytes+1))
		f.Close()
		if err != nil || len(data) == 0 {
			writeError(w, http.StatusBadRequest, "failed to read upload")
			return
		}
		if len(data) > maxPackZipBytes {
			writeError(w, http.StatusBadRequest, "pack file is too large")
			return
		}
		packs, blobs, err := parsePackZip(data)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		for _, pack := range packs {
			name := strings.TrimSpace(pack.Name)
			if name == "" {
				name = "Imported pack"
			}
			if len(name) > maxLibraryName {
				writeError(w, http.StatusBadRequest, "pack name is too long")
				return
			}
			description := strings.TrimSpace(pack.Description)
			if len(description) > maxLibraryDescription {
				writeError(w, http.StatusBadRequest, "description must be 500 characters or fewer")
				return
			}
			if len(pack.Icons) > maxIconsPerImported {
				writeError(w, http.StatusBadRequest, "too many icons in a pack")
				return
			}
			row, err := s.db.CreateIconLibrary(r.Context(), dbsqlc.CreateIconLibraryParams{
				LibraryDiscordUserID: session.Session.UserID,
				LibraryName:          name,
				LibraryDescription:   description,
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to import pack")
				return
			}
			var stored []string
			failed := false
			for _, icon := range pack.Icons {
				payload, ok := blobs[icon.File]
				if !ok {
					failed = true
					break
				}
				rel, err := s.storeImportedIcon(r, session.Session.UserID, row, icon, payload)
				if err != nil {
					failed = true
					break
				}
				if rel != "" {
					stored = append(stored, rel)
				}
			}
			if failed {
				s.rollbackImportedPack(r, session.Session.UserID, row.LibraryID, stored)
				writeError(w, http.StatusBadRequest, "failed to import icons for "+name)
				return
			}
			created = append(created, imported{ID: row.LibraryID, Name: row.LibraryName})
		}
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"packs":   created,
		"created": len(created),
	})
}

func (s *Server) storeImportedIcon(r *http.Request, userID string, lib dbsqlc.IconLibrary, icon packExportIcon, data []byte) (string, error) {
	if len(data) == 0 || len(data) > maxIconBytes {
		return "", fmt.Errorf("icon too large")
	}
	ct, ext, ok := sniffIcon("", data)
	if !ok {
		return "", fmt.Errorf("unsupported icon")
	}
	filename := path.Base(strings.TrimSpace(icon.Filename))
	if filename == "" || filename == "." || filename == "/" {
		filename = "icon" + ext
	}
	name := strings.TrimSpace(icon.Name)
	category := strings.TrimSpace(icon.Category)
	if len(name) > maxLibraryName || len(category) > 80 {
		return "", fmt.Errorf("name or category is too long")
	}
	slugs, err := normalizePokemonList(icon.Pokemon)
	if err != nil {
		return "", err
	}
	row, err := s.db.InsertIcon(r.Context(), dbsqlc.InsertIconParams{
		IconLibraryID:   lib.LibraryID,
		IconFilename:    filename,
		IconName:        name,
		IconContentType: ct,
		IconStoragePath: "",
		IconCategory:    database.Text(category),
	})
	if err != nil {
		return "", err
	}
	rel := path.Join("icons", userID, strconv.FormatInt(lib.LibraryID, 10), strconv.FormatInt(row.IconID, 10)+ext)
	rel = path.Clean(rel)
	if err := s.writeIconFile(rel, data); err != nil {
		_, _ = s.db.DeleteIconForOwner(r.Context(), dbsqlc.DeleteIconForOwnerParams{
			IconID:               row.IconID,
			LibraryDiscordUserID: userID,
		})
		return "", err
	}
	if err := s.db.SetIconStoragePath(r.Context(), dbsqlc.SetIconStoragePathParams{
		IconID:          row.IconID,
		IconStoragePath: rel,
	}); err != nil {
		s.removeIconFile(rel)
		_, _ = s.db.DeleteIconForOwner(r.Context(), dbsqlc.DeleteIconForOwnerParams{
			IconID:               row.IconID,
			LibraryDiscordUserID: userID,
		})
		return "", err
	}
	if err := replaceIconPokemon(r, s, row.IconID, slugs); err != nil {
		return rel, err
	}
	return rel, nil
}

func (s *Server) rollbackImportedPack(r *http.Request, userID string, libraryID int64, files []string) {
	_, _ = s.db.DeleteIconLibrary(r.Context(), dbsqlc.DeleteIconLibraryParams{
		LibraryID:            libraryID,
		LibraryDiscordUserID: userID,
	})
	for _, rel := range files {
		s.removeIconFile(rel)
	}
}

func (s *Server) readIconFile(rel string) ([]byte, error) {
	full, err := s.iconAbsPath(rel)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, err
	}
	if len(data) > maxIconBytes {
		return nil, fmt.Errorf("icon too large")
	}
	return data, nil
}

func parsePackZip(data []byte) ([]packExportPack, map[string][]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, nil, fmt.Errorf("pack file must be a zip export")
	}
	var manifest []byte
	blobs := map[string][]byte{}
	var total int
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name, err := cleanZipPath(f.Name)
		if err != nil {
			return nil, nil, err
		}
		if strings.HasPrefix(name, "__MACOSX/") {
			continue
		}
		payload, err := readZipEntry(f, maxPackZipBytes)
		if err != nil {
			return nil, nil, err
		}
		total += len(payload)
		if total > maxPackZipBytes {
			return nil, nil, fmt.Errorf("pack file is too large")
		}
		if name == packExportManifest {
			manifest = payload
			continue
		}
		blobs[name] = payload
	}
	if len(manifest) == 0 {
		return nil, nil, fmt.Errorf("pack file is missing %s", packExportManifest)
	}
	var doc packExportDoc
	if err := json.Unmarshal(manifest, &doc); err != nil {
		return nil, nil, fmt.Errorf("pack file is not valid")
	}
	if doc.Version != 0 && doc.Version != packExportVersion {
		return nil, nil, fmt.Errorf("unsupported pack file")
	}
	if len(doc.Packs) == 0 {
		return nil, nil, fmt.Errorf("pack file has no packs")
	}
	if len(doc.Packs) > maxImportPacks {
		return nil, nil, fmt.Errorf("too many packs in file")
	}
	for pi := range doc.Packs {
		if doc.Packs[pi].Icons == nil {
			doc.Packs[pi].Icons = []packExportIcon{}
		}
		for ii, icon := range doc.Packs[pi].Icons {
			file, err := cleanZipPath(icon.File)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid icon path")
			}
			doc.Packs[pi].Icons[ii].File = file
			if _, ok := blobs[file]; !ok {
				return nil, nil, fmt.Errorf("pack file is missing an icon")
			}
		}
	}
	return doc.Packs, blobs, nil
}

func cleanZipPath(name string) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	if name == "" || strings.Contains(name, "..") || strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("invalid path in pack file")
	}
	name = path.Clean(name)
	if name == "." || strings.HasPrefix(name, "/") || strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid path in pack file")
	}
	return name, nil
}

func readZipEntry(f *zip.File, limit int64) ([]byte, error) {
	if f.UncompressedSize64 > uint64(limit) {
		return nil, fmt.Errorf("pack file is too large")
	}
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to read pack file")
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read pack file")
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("pack file is too large")
	}
	return data, nil
}

func writeZipFile(zw *zip.Writer, name string, data []byte) error {
	h := &zip.FileHeader{Name: name, Method: zip.Deflate}
	w, err := zw.CreateHeader(h)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func safePackFilename(name string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "pack"
	}
	return b.String()
}
