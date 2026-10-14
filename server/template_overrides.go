package server

import (
	"encoding/json"
	"slices"
	"strings"
)

// Override field / group ids returned in overriddenFields and accepted on update.
const (
	overrideName         = "name"
	overrideLanguage     = "language"
	overrideTitle        = "title" // meetup title (payload.name)
	overrideDetails      = "details"
	overrideCategory     = "category"
	overrideCover        = "coverPhotoUrl"
	overrideComments     = "commentsPermissions"
	overrideInvited      = "allInvited"
	overrideCA           = "createdByCommunityAmbassador"
	overridePlaceholders = "placeholders"
	overrideLocation     = "location"
	overrideSchedule     = "schedule"
	overrideDiscord      = "discord"
)

var locationPayloadKeys = []string{
	"latitude", "longitude", "address", "placeId", "locationJitterMeters",
}

var schedulePayloadKeys = []string{
	"startTime", "endTime", "eventTime", "eventEndTime",
}

var discordPayloadKeys = []string{
	"discordChannelId", "discordClubId",
}

var singlePayloadOverrideKeys = []string{
	overrideDetails,
	overrideCategory,
	overrideCover,
	overrideComments,
	overrideInvited,
	overrideCA,
	overridePlaceholders,
	"inviteeIds",
	"campfireLiveEventId",
}

func isEmptyOverrides(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return true
	}
	s := strings.TrimSpace(string(raw))
	return s == "" || s == "{}" || s == "null"
}

func parseOverrideMap(raw json.RawMessage) map[string]any {
	if isEmptyOverrides(raw) {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return map[string]any{}
	}
	return m
}

func parsePayloadMap(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return map[string]any{}
	}
	return m
}

func marshalPayloadMap(m map[string]any) json.RawMessage {
	if m == nil {
		return json.RawMessage(`{}`)
	}
	out, err := json.Marshal(m)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return out
}

func overrideHasAnyKey(overrides map[string]any, keys []string) bool {
	for _, k := range keys {
		if _, ok := overrides[k]; ok {
			return true
		}
	}
	return false
}

// overriddenFieldIDs maps stored override keys to UI field/group ids.
func overriddenFieldIDs(overrides map[string]any) []string {
	if len(overrides) == 0 {
		return []string{}
	}
	var out []string
	if _, ok := overrides[overrideName]; ok {
		out = append(out, overrideName)
	}
	if _, ok := overrides[overrideLanguage]; ok {
		out = append(out, overrideLanguage)
	}
	if _, ok := overrides[overrideTitle]; ok {
		out = append(out, overrideTitle)
	}
	if overrideHasAnyKey(overrides, locationPayloadKeys) {
		out = append(out, overrideLocation)
	}
	if overrideHasAnyKey(overrides, schedulePayloadKeys) {
		out = append(out, overrideSchedule)
	}
	if overrideHasAnyKey(overrides, discordPayloadKeys) {
		out = append(out, overrideDiscord)
	}
	for _, k := range singlePayloadOverrideKeys {
		if _, ok := overrides[k]; ok && !slices.Contains(out, k) {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}

// buildOverridesFromFields builds a sparse override map from the submitted values
// and the list of overridden field/group ids.
func buildOverridesFromFields(
	fields []string,
	name string,
	language string,
	payload json.RawMessage,
) (json.RawMessage, error) {
	pm := parsePayloadMap(payload)
	out := map[string]any{}
	seen := map[string]bool{}
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		switch f {
		case overrideName:
			out[overrideName] = name
		case overrideLanguage:
			out[overrideLanguage] = language
		case overrideTitle:
			if v, ok := pm["name"]; ok {
				out[overrideTitle] = v
			} else {
				out[overrideTitle] = nil
			}
		case overrideLocation:
			for _, k := range locationPayloadKeys {
				if v, ok := pm[k]; ok {
					out[k] = v
				} else {
					out[k] = nil
				}
			}
		case overrideSchedule:
			for _, k := range schedulePayloadKeys {
				if v, ok := pm[k]; ok {
					out[k] = v
				} else {
					out[k] = nil
				}
			}
		case overrideDiscord:
			for _, k := range discordPayloadKeys {
				if v, ok := pm[k]; ok {
					out[k] = v
				} else {
					out[k] = nil
				}
			}
		default:
			if v, ok := pm[f]; ok {
				out[f] = v
			} else {
				out[f] = nil
			}
		}
	}
	if len(out) == 0 {
		return json.RawMessage(`{}`), nil
	}
	return json.Marshal(out)
}

type mergedTemplate struct {
	Name             string
	Language         *string
	Payload          json.RawMessage
	OverriddenFields []string
	Synced           bool
}

func mergeLinkedTemplate(
	localName string,
	localLanguage *string,
	localPayload json.RawMessage,
	overridesRaw json.RawMessage,
	originName string,
	originLanguage *string,
	originPayload json.RawMessage,
	originPresent bool,
) mergedTemplate {
	overrides := parseOverrideMap(overridesRaw)
	fields := overriddenFieldIDs(overrides)

	if !originPresent {
		return mergedTemplate{
			Name:             localName,
			Language:         localLanguage,
			Payload:          localPayload,
			OverriddenFields: fields,
			Synced:           false,
		}
	}

	name := originName
	if v, ok := overrides[overrideName]; ok {
		if s, ok := v.(string); ok {
			name = s
		}
	}

	lang := originLanguage
	if v, ok := overrides[overrideLanguage]; ok {
		if s, ok := v.(string); ok {
			copied := s
			lang = &copied
		} else if v == nil {
			lang = nil
		}
	}

	pm := parsePayloadMap(originPayload)
	for k, v := range overrides {
		if k == overrideName || k == overrideLanguage {
			continue
		}
		if k == overrideTitle {
			if v == nil {
				delete(pm, "name")
			} else {
				pm["name"] = v
			}
			continue
		}
		if v == nil {
			delete(pm, k)
		} else {
			pm[k] = v
		}
	}

	return mergedTemplate{
		Name:             name,
		Language:         lang,
		Payload:          marshalPayloadMap(pm),
		OverriddenFields: fields,
		Synced:           len(overrides) == 0,
	}
}

func materializeEffective(m mergedTemplate) (name string, language string, payload json.RawMessage) {
	name = m.Name
	if m.Language != nil {
		language = *m.Language
	}
	payload = m.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	return name, language, payload
}

func allOverrideFieldIDs() []string {
	return []string{
		overrideName,
		overrideLanguage,
		overrideTitle,
		overrideDetails,
		overrideCategory,
		overrideCover,
		overrideComments,
		overrideInvited,
		overrideCA,
		overridePlaceholders,
		overrideLocation,
		overrideSchedule,
		overrideDiscord,
		"inviteeIds",
		"campfireLiveEventId",
	}
}
