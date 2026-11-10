package model

import "time"

// StudyActiveSessionState is the Redis working state for an in-progress study session.
type StudyActiveSessionState struct {
	// Session is copied from the sessions table and flushed back on complete.
	Session Session `json:"session"`
	// Items are copied from session_materials + materials and mutated in Redis
	// as materials are marked studied.
	Items                       []StudySessionMaterial `json:"items"`
	CurrentIndex                int                    `json:"current_index"`
	StudiedCount                int                    `json:"studied_count"`
	InitiallyStudiedMaterialIDs map[int]bool           `json:"initially_studied_material_ids,omitempty"`
	UpdatedAt                   time.Time              `json:"updated_at"`
}

func (state *StudyActiveSessionState) RecountStudied() int {
	count := 0
	for _, item := range state.Items {
		if item.SessionMaterial.StudiedAt != nil {
			count++
		}
	}
	state.StudiedCount = count
	return count
}

func (state *StudyActiveSessionState) CaptureInitiallyStudied() {
	state.InitiallyStudiedMaterialIDs = make(map[int]bool)
	for _, item := range state.Items {
		if item.SessionMaterial.StudiedAt != nil {
			state.InitiallyStudiedMaterialIDs[item.SessionMaterial.MaterialID] = true
		}
	}
}

func (state *StudyActiveSessionState) ItemByOrder(materialOrder int) (*StudySessionMaterial, int, bool) {
	for index := range state.Items {
		if state.Items[index].SessionMaterial.MaterialOrder == materialOrder {
			return &state.Items[index], index, true
		}
	}
	return nil, -1, false
}

func (state *StudyActiveSessionState) ItemByMaterialID(materialID int) (*StudySessionMaterial, bool) {
	for index := range state.Items {
		if state.Items[index].SessionMaterial.MaterialID == materialID {
			return &state.Items[index], true
		}
	}
	return nil, false
}

func (state *StudyActiveSessionState) ItemAt(index int) (*StudySessionMaterial, bool) {
	if index < 0 || index >= len(state.Items) {
		return nil, false
	}
	return &state.Items[index], true
}

func (state *StudyActiveSessionState) NextUnstudiedIndex() int {
	for index, item := range state.Items {
		if item.SessionMaterial.StudiedAt == nil {
			return index
		}
	}
	return len(state.Items)
}

func (state *StudyActiveSessionState) MarkStudied(
	materialOrder int,
	studiedAt time.Time,
) bool {
	_, index, ok := state.ItemByOrder(materialOrder)
	if !ok || state.Items[index].SessionMaterial.StudiedAt != nil {
		return false
	}
	state.Items[index].SessionMaterial.StudiedAt = &studiedAt
	state.CurrentIndex = index
	state.UpdatedAt = studiedAt
	state.RecountStudied()
	return true
}

func (state *StudyActiveSessionState) NewlyStudiedMaterialIDs() []int {
	ids := make(
		[]int,
		0,
	)
	for _, item := range state.Items {
		if item.SessionMaterial.StudiedAt == nil {
			continue
		}
		materialID := item.SessionMaterial.MaterialID
		if state.InitiallyStudiedMaterialIDs[materialID] {
			continue
		}
		ids = append(
			ids,
			materialID,
		)
	}
	return ids
}
