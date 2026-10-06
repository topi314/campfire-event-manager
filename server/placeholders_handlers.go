package server

import (
	"net/http"
	"strings"

	"github.com/topi314/campfire-event-manager/internal/eventcategory"
	"github.com/topi314/campfire-event-manager/server/pokemon"
)

type resolvePlaceholdersBody struct {
	LiveEventName string `json:"liveEventName"`
	Category      string `json:"category"`
	Language      string `json:"language"`
}

// resolvePlaceholders returns eventPokemon* builtin values for create-flow prefills/preview.
func (s *Server) resolvePlaceholders(w http.ResponseWriter, r *http.Request) {
	var body resolvePlaceholdersBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	category := strings.TrimSpace(body.Category)
	if category == "" {
		category = eventcategory.FromName(body.LiveEventName)
	}
	values := pokemon.ResolveEventPokemon(
		body.LiveEventName,
		category,
		body.Language,
		eventcategory.All,
	)
	writeJSON(w, http.StatusOK, map[string]any{
		"category": category,
		"values":   values,
	})
}
