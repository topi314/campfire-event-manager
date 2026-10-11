package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type importEntry struct {
	ID       int64
	Name     string
	Payload  json.RawMessage
	Language string
}

type importEntryJSON struct {
	ID       *int64          `json:"id"`
	Name     string          `json:"name"`
	Language string          `json:"language"`
	Payload  json.RawMessage `json:"payload"`
}

// parseImportEntries reads one uploaded JSON file into one or more template entries.
// Supports:
//   - { "id"?, "name", "payload", "language"? }
//   - { "templates": [ ... ] }
//   - [ { "id"?, "name", "payload", "language"? }, ... ]
//   - bare meetup payload object (name from filename)
func parseImportEntries(filename string, data []byte) ([]importEntry, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || !json.Valid(data) {
		return nil, fmt.Errorf("invalid json")
	}
	fallback := strings.TrimSuffix(filename, ".json")
	fallback = strings.TrimSpace(fallback)
	if fallback == "" {
		fallback = "Imported template"
	}

	var bundle struct {
		Templates []json.RawMessage `json:"templates"`
	}
	if err := json.Unmarshal(data, &bundle); err == nil && len(bundle.Templates) > 0 {
		return entriesFromRawList(bundle.Templates, fallback)
	}

	if data[0] == '[' {
		var items []json.RawMessage
		if err := json.Unmarshal(data, &items); err != nil {
			return nil, fmt.Errorf("invalid template array")
		}
		if len(items) == 0 {
			return nil, fmt.Errorf("empty template array")
		}
		return entriesFromRawList(items, fallback)
	}

	entry, err := entryFromRaw(data, fallback)
	if err != nil {
		return nil, err
	}
	return []importEntry{entry}, nil
}

func entriesFromRawList(items []json.RawMessage, fallback string) ([]importEntry, error) {
	out := make([]importEntry, 0, len(items))
	for i, raw := range items {
		name := fallback
		if len(items) > 1 {
			name = fmt.Sprintf("%s (%d)", fallback, i+1)
		}
		entry, err := entryFromRaw(raw, name)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, nil
}

func entryFromRaw(raw json.RawMessage, fallbackName string) (importEntry, error) {
	var item importEntryJSON
	if err := json.Unmarshal(raw, &item); err != nil {
		return importEntry{}, fmt.Errorf("invalid template object")
	}
	id := int64(0)
	if item.ID != nil && *item.ID > 0 {
		id = *item.ID
	}
	payload := bytes.TrimSpace(item.Payload)
	if len(payload) > 0 && json.Valid(payload) {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = fallbackName
		}
		return importEntry{
			ID:       id,
			Name:     name,
			Payload:  json.RawMessage(payload),
			Language: strings.TrimSpace(item.Language),
		}, nil
	}
	// Bare meetup payload — template name comes from the filename, not meetup title.
	if !json.Valid(raw) {
		return importEntry{}, fmt.Errorf("invalid template payload")
	}
	return importEntry{
		Name:    fallbackName,
		Payload: json.RawMessage(bytes.TrimSpace(raw)),
	}, nil
}
