package model

import "time"

const StudyActiveSessionStateVersion = 1

// StudyActiveSessionState is the Redis working state for an in-progress study session.
type StudyActiveSessionState struct {
	Version                     int                    `json:"version"`
	Session                     Session                `json:"session"`
	Items                       []StudySessionMaterial `json:"items"`
	CurrentIndex                int                    `json:"current_index"`
	StudiedCount                int                    `json:"studied_count"`
	InitiallyStudiedMaterialIDs map[int]bool           `json:"initially_studied_material_ids,omitempty"`
	UpdatedAt                   time.Time              `json:"updated_at"`
}

func (s *StudyActiveSessionState) RecountStudied() int {
	count := 0
	for _, item := range s.Items {
		if item.SessionMaterial.StudiedAt != nil {
			count++
		}
	}
	s.StudiedCount = count
	return count
}

func (s *StudyActiveSessionState) CaptureInitiallyStudied() {
	s.InitiallyStudiedMaterialIDs = make(map[int]bool)
	for _, item := range s.Items {
		if item.SessionMaterial.StudiedAt != nil {
			s.InitiallyStudiedMaterialIDs[item.SessionMaterial.MaterialID] = true
		}
	}
}

func (s *StudyActiveSessionState) ItemByOrder(materialOrder int) (*StudySessionMaterial, int, bool) {
	for idx := range s.Items {
		if s.Items[idx].SessionMaterial.MaterialOrder == materialOrder {
			return &s.Items[idx], idx, true
		}
	}
	return nil, -1, false
}

func (s *StudyActiveSessionState) ItemAt(idx int) (*StudySessionMaterial, bool) {
	if idx < 0 || idx >= len(s.Items) {
		return nil, false
	}
	return &s.Items[idx], true
}

func (s *StudyActiveSessionState) NextUnstudiedIndex() int {
	for idx, item := range s.Items {
		if item.SessionMaterial.StudiedAt == nil {
			return idx
		}
	}
	return len(s.Items)
}

func (s *StudyActiveSessionState) MarkStudied(materialOrder int, studiedAt time.Time) bool {
	_, idx, ok := s.ItemByOrder(materialOrder)
	if !ok || s.Items[idx].SessionMaterial.StudiedAt != nil {
		return false
	}
	s.Items[idx].SessionMaterial.StudiedAt = &studiedAt
	s.CurrentIndex = idx
	s.UpdatedAt = studiedAt
	s.RecountStudied()
	return true
}

func (s *StudyActiveSessionState) NewlyStudiedMaterialIDs() []int {
	ids := make([]int, 0)
	for _, item := range s.Items {
		if item.SessionMaterial.StudiedAt == nil {
			continue
		}
		materialID := item.SessionMaterial.MaterialID
		if s.InitiallyStudiedMaterialIDs[materialID] {
			continue
		}
		ids = append(ids, materialID)
	}
	return ids
}
