package model

import (
	"testing"
	"time"
)

// studyMaterial builds one StudySessionMaterial. order is MaterialOrder,
// materialID is the underlying material id, studied marks StudiedAt.
func studyMaterial(order, materialID int, studied bool) StudySessionMaterial {
	sm := SessionMaterial{
		MaterialID:    materialID,
		MaterialOrder: order,
	}
	if studied {
		ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		sm.StudiedAt = &ts
	}
	return StudySessionMaterial{SessionMaterial: sm}
}

func TestStudyActiveSessionState_RecountStudied(t *testing.T) {
	t.Parallel()

	s := &StudyActiveSessionState{
		Items: []StudySessionMaterial{
			studyMaterial(1, 10, true),
			studyMaterial(2, 11, false),
			studyMaterial(3, 12, true),
		},
	}
	if got := s.RecountStudied(); got != 2 {
		t.Fatalf("RecountStudied() = %d, want 2", got)
	}
	if s.StudiedCount != 2 {
		t.Fatalf("StudiedCount = %d, want 2", s.StudiedCount)
	}
}

func TestStudyActiveSessionState_CaptureInitiallyStudied(t *testing.T) {
	t.Parallel()

	s := &StudyActiveSessionState{
		Items: []StudySessionMaterial{
			studyMaterial(1, 10, true),
			studyMaterial(2, 11, false),
			studyMaterial(3, 12, true),
		},
	}
	s.CaptureInitiallyStudied()

	if len(s.InitiallyStudiedMaterialIDs) != 2 {
		t.Fatalf("InitiallyStudiedMaterialIDs len = %d, want 2", len(s.InitiallyStudiedMaterialIDs))
	}
	if !s.InitiallyStudiedMaterialIDs[10] || !s.InitiallyStudiedMaterialIDs[12] {
		t.Fatalf("InitiallyStudiedMaterialIDs = %v, want {10,12}", s.InitiallyStudiedMaterialIDs)
	}
	if s.InitiallyStudiedMaterialIDs[11] {
		t.Fatal("material 11 was unstudied; must not be captured")
	}
}

func TestStudyActiveSessionState_ItemByOrder(t *testing.T) {
	t.Parallel()

	s := &StudyActiveSessionState{
		Items: []StudySessionMaterial{
			studyMaterial(1, 10, false),
			studyMaterial(2, 11, false),
		},
	}

	t.Run("found", func(t *testing.T) {
		item, idx, ok := s.ItemByOrder(2)
		if !ok || idx != 1 || item == nil || item.SessionMaterial.MaterialID != 11 {
			t.Fatalf("ItemByOrder(2) = %v, %d, %v", item, idx, ok)
		}
	})

	t.Run("not found", func(t *testing.T) {
		item, idx, ok := s.ItemByOrder(99)
		if ok || idx != -1 || item != nil {
			t.Fatalf("ItemByOrder(99) = %v, %d, %v; want nil,-1,false", item, idx, ok)
		}
	})
}

func TestStudyActiveSessionState_ItemAt(t *testing.T) {
	t.Parallel()

	s := &StudyActiveSessionState{
		Items: []StudySessionMaterial{
			studyMaterial(1, 10, false),
			studyMaterial(2, 11, false),
		},
	}

	if _, ok := s.ItemAt(0); !ok {
		t.Fatal("ItemAt(0) ok = false, want true")
	}
	if _, ok := s.ItemAt(-1); ok {
		t.Fatal("ItemAt(-1) ok = true, want false")
	}
	if _, ok := s.ItemAt(2); ok {
		t.Fatal("ItemAt(2) ok = true, want false")
	}
}

func TestStudyActiveSessionState_NextUnstudiedIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		items []StudySessionMaterial
		want  int
	}{
		{
			name:  "first unstudied",
			items: []StudySessionMaterial{studyMaterial(1, 10, false), studyMaterial(2, 11, false)},
			want:  0,
		},
		{
			name:  "second unstudied",
			items: []StudySessionMaterial{studyMaterial(1, 10, true), studyMaterial(2, 11, false)},
			want:  1,
		},
		{
			name:  "all studied returns len",
			items: []StudySessionMaterial{studyMaterial(1, 10, true), studyMaterial(2, 11, true)},
			want:  2,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &StudyActiveSessionState{Items: tt.items}
			if got := s.NextUnstudiedIndex(); got != tt.want {
				t.Fatalf("NextUnstudiedIndex() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestStudyActiveSessionState_MarkStudied(t *testing.T) {
	t.Parallel()

	studiedAt := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)

	t.Run("marks an unstudied material and updates counters", func(t *testing.T) {
		s := &StudyActiveSessionState{
			Items: []StudySessionMaterial{
				studyMaterial(1, 10, false),
				studyMaterial(2, 11, false),
			},
		}
		ok := s.MarkStudied(2, studiedAt)
		if !ok {
			t.Fatal("MarkStudied(2) = false, want true")
		}
		if s.Items[1].SessionMaterial.StudiedAt == nil || !s.Items[1].SessionMaterial.StudiedAt.Equal(studiedAt) {
			t.Fatalf("StudiedAt = %v, want %v", s.Items[1].SessionMaterial.StudiedAt, studiedAt)
		}
		if s.CurrentIndex != 1 {
			t.Fatalf("CurrentIndex = %d, want 1", s.CurrentIndex)
		}
		if !s.UpdatedAt.Equal(studiedAt) {
			t.Fatalf("UpdatedAt = %v, want %v", s.UpdatedAt, studiedAt)
		}
		if s.StudiedCount != 1 {
			t.Fatalf("StudiedCount = %d, want 1", s.StudiedCount)
		}
	})

	t.Run("already studied is a no-op returning false", func(t *testing.T) {
		s := &StudyActiveSessionState{
			Items: []StudySessionMaterial{studyMaterial(1, 10, true)},
		}
		if ok := s.MarkStudied(1, studiedAt); ok {
			t.Fatal("MarkStudied on already-studied = true, want false")
		}
	})

	t.Run("unknown order returns false", func(t *testing.T) {
		s := &StudyActiveSessionState{
			Items: []StudySessionMaterial{studyMaterial(1, 10, false)},
		}
		if ok := s.MarkStudied(99, studiedAt); ok {
			t.Fatal("MarkStudied(99) = true, want false")
		}
	})
}

func TestStudyActiveSessionState_NewlyStudiedMaterialIDs(t *testing.T) {
	t.Parallel()

	t.Run("excludes initially-studied and unstudied", func(t *testing.T) {
		s := &StudyActiveSessionState{
			Items: []StudySessionMaterial{
				studyMaterial(1, 10, true),  // initially studied -> excluded
				studyMaterial(2, 11, true),  // newly studied this session
				studyMaterial(3, 12, false), // still unstudied -> excluded
			},
			InitiallyStudiedMaterialIDs: map[int]bool{10: true},
		}
		ids := s.NewlyStudiedMaterialIDs()
		if len(ids) != 1 || ids[0] != 11 {
			t.Fatalf("NewlyStudiedMaterialIDs() = %v, want [11]", ids)
		}
	})

	t.Run("nil initial map treats all studied as newly studied", func(t *testing.T) {
		s := &StudyActiveSessionState{
			Items: []StudySessionMaterial{
				studyMaterial(1, 10, true),
				studyMaterial(2, 11, false),
			},
		}
		ids := s.NewlyStudiedMaterialIDs()
		if len(ids) != 1 || ids[0] != 10 {
			t.Fatalf("NewlyStudiedMaterialIDs() = %v, want [10]", ids)
		}
	})

	t.Run("none newly studied returns empty non-nil slice", func(t *testing.T) {
		s := &StudyActiveSessionState{
			Items:                       []StudySessionMaterial{studyMaterial(1, 10, true)},
			InitiallyStudiedMaterialIDs: map[int]bool{10: true},
		}
		ids := s.NewlyStudiedMaterialIDs()
		if ids == nil {
			t.Fatal("NewlyStudiedMaterialIDs() = nil, want empty slice")
		}
		if len(ids) != 0 {
			t.Fatalf("NewlyStudiedMaterialIDs() = %v, want empty", ids)
		}
	})
}
