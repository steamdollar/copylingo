package model

import (
	"testing"
	"time"
)

// studyMaterial builds one StudySessionMaterial. order is MaterialOrder,
// materialID is the underlying material id, studied marks StudiedAt.
func studyMaterial(
	order,
	materialID int,
	studied bool,
) StudySessionMaterial {
	sessionMaterial := SessionMaterial{
		MaterialID:    materialID,
		MaterialOrder: order,
	}
	if studied {
		timestamp := time.Date(
			2026,
			1,
			1,
			0,
			0,
			0,
			0,
			time.UTC,
		)
		sessionMaterial.StudiedAt = &timestamp
	}
	return StudySessionMaterial{SessionMaterial: sessionMaterial}
}

func TestStudyActiveSessionState_RecountStudied(t *testing.T) {
	t.Parallel()

	state := &StudyActiveSessionState{
		Items: []StudySessionMaterial{
			studyMaterial(
				1,
				10,
				true,
			),
			studyMaterial(
				2,
				11,
				false,
			),
			studyMaterial(
				3,
				12,
				true,
			),
		},
	}
	if got := state.RecountStudied(); got != 2 {
		t.Fatalf(
			"RecountStudied() = %d, want 2",
			got,
		)
	}
	if state.StudiedCount != 2 {
		t.Fatalf(
			"StudiedCount = %d, want 2",
			state.StudiedCount,
		)
	}
}

func TestStudyActiveSessionState_CaptureInitiallyStudied(t *testing.T) {
	t.Parallel()

	state := &StudyActiveSessionState{
		Items: []StudySessionMaterial{
			studyMaterial(
				1,
				10,
				true,
			),
			studyMaterial(
				2,
				11,
				false,
			),
			studyMaterial(
				3,
				12,
				true,
			),
		},
	}
	state.CaptureInitiallyStudied()

	if len(state.InitiallyStudiedMaterialIDs) != 2 {
		t.Fatalf(
			"InitiallyStudiedMaterialIDs len = %d, want 2",
			len(state.InitiallyStudiedMaterialIDs),
		)
	}
	if !state.InitiallyStudiedMaterialIDs[10] || !state.InitiallyStudiedMaterialIDs[12] {
		t.Fatalf(
			"InitiallyStudiedMaterialIDs = %v, want {10,12}",
			state.InitiallyStudiedMaterialIDs,
		)
	}
	if state.InitiallyStudiedMaterialIDs[11] {
		t.Fatal("material 11 was unstudied; must not be captured")
	}
}

func TestStudyActiveSessionState_ItemByOrder(t *testing.T) {
	t.Parallel()

	state := &StudyActiveSessionState{
		Items: []StudySessionMaterial{
			studyMaterial(
				1,
				10,
				false,
			),
			studyMaterial(
				2,
				11,
				false,
			),
		},
	}

	t.Run(
		"found",
		func(t *testing.T) {
			item, index, ok := state.ItemByOrder(2)
			if !ok || index != 1 || item == nil || item.SessionMaterial.MaterialID != 11 {
				t.Fatalf(
					"ItemByOrder(2) = %v, %d, %v",
					item,
					index,
					ok,
				)
			}
		},
	)

	t.Run(
		"not found",
		func(t *testing.T) {
			item, index, ok := state.ItemByOrder(99)
			if ok || index != -1 || item != nil {
				t.Fatalf(
					"ItemByOrder(99) = %v, %d, %v; want nil,-1,false",
					item,
					index,
					ok,
				)
			}
		},
	)
}

func TestStudyActiveSessionState_ItemAt(t *testing.T) {
	t.Parallel()

	state := &StudyActiveSessionState{
		Items: []StudySessionMaterial{
			studyMaterial(
				1,
				10,
				false,
			),
			studyMaterial(
				2,
				11,
				false,
			),
		},
	}

	if _, ok := state.ItemAt(0); !ok {
		t.Fatal("ItemAt(0) ok = false, want true")
	}
	if _, ok := state.ItemAt(-1); ok {
		t.Fatal("ItemAt(-1) ok = true, want false")
	}
	if _, ok := state.ItemAt(2); ok {
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
			name: "first unstudied",
			items: []StudySessionMaterial{studyMaterial(
				1,
				10,
				false,
			), studyMaterial(
				2,
				11,
				false,
			)},
			want: 0,
		},
		{
			name: "second unstudied",
			items: []StudySessionMaterial{studyMaterial(
				1,
				10,
				true,
			), studyMaterial(
				2,
				11,
				false,
			)},
			want: 1,
		},
		{
			name: "all studied returns len",
			items: []StudySessionMaterial{studyMaterial(
				1,
				10,
				true,
			), studyMaterial(
				2,
				11,
				true,
			)},
			want: 2,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(
			tt.name,
			func(t *testing.T) {
				t.Parallel()
				state := &StudyActiveSessionState{Items: tt.items}
				if got := state.NextUnstudiedIndex(); got != tt.want {
					t.Fatalf(
						"NextUnstudiedIndex() = %d, want %d",
						got,
						tt.want,
					)
				}
			},
		)
	}
}

func TestStudyActiveSessionState_MarkStudied(t *testing.T) {
	t.Parallel()

	studiedAt := time.Date(
		2026,
		6,
		30,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	t.Run(
		"marks an unstudied material and updates counters",
		func(t *testing.T) {
			state := &StudyActiveSessionState{
				Items: []StudySessionMaterial{
					studyMaterial(
						1,
						10,
						false,
					),
					studyMaterial(
						2,
						11,
						false,
					),
				},
			}
			ok := state.MarkStudied(
				2,
				studiedAt,
			)
			if !ok {
				t.Fatal("MarkStudied(2) = false, want true")
			}
			if state.Items[1].SessionMaterial.StudiedAt == nil ||
				!state.Items[1].SessionMaterial.StudiedAt.Equal(studiedAt) {
				t.Fatalf(
					"StudiedAt = %v, want %v",
					state.Items[1].SessionMaterial.StudiedAt,
					studiedAt,
				)
			}
			if state.CurrentIndex != 1 {
				t.Fatalf(
					"CurrentIndex = %d, want 1",
					state.CurrentIndex,
				)
			}
			if !state.UpdatedAt.Equal(studiedAt) {
				t.Fatalf(
					"UpdatedAt = %v, want %v",
					state.UpdatedAt,
					studiedAt,
				)
			}
			if state.StudiedCount != 1 {
				t.Fatalf(
					"StudiedCount = %d, want 1",
					state.StudiedCount,
				)
			}
		},
	)

	t.Run(
		"already studied is a no-op returning false",
		func(t *testing.T) {
			state := &StudyActiveSessionState{
				Items: []StudySessionMaterial{studyMaterial(
					1,
					10,
					true,
				)},
			}
			if ok := state.MarkStudied(
				1,
				studiedAt,
			); ok {
				t.Fatal("MarkStudied on already-studied = true, want false")
			}
		},
	)

	t.Run(
		"unknown order returns false",
		func(t *testing.T) {
			state := &StudyActiveSessionState{
				Items: []StudySessionMaterial{studyMaterial(
					1,
					10,
					false,
				)},
			}
			if ok := state.MarkStudied(
				99,
				studiedAt,
			); ok {
				t.Fatal("MarkStudied(99) = true, want false")
			}
		},
	)
}

func TestStudyActiveSessionState_NewlyStudiedMaterialIDs(t *testing.T) {
	t.Parallel()

	t.Run(
		"excludes initially-studied and unstudied",
		func(t *testing.T) {
			state := &StudyActiveSessionState{
				Items: []StudySessionMaterial{
					studyMaterial(
						1,
						10,
						true,
					), // initially studied -> excluded
					studyMaterial(
						2,
						11,
						true,
					), // newly studied this session
					studyMaterial(
						3,
						12,
						false,
					), // still unstudied -> excluded
				},
				InitiallyStudiedMaterialIDs: map[int]bool{10: true},
			}
			ids := state.NewlyStudiedMaterialIDs()
			if len(ids) != 1 || ids[0] != 11 {
				t.Fatalf(
					"NewlyStudiedMaterialIDs() = %v, want [11]",
					ids,
				)
			}
		},
	)

	t.Run(
		"nil initial map treats all studied as newly studied",
		func(t *testing.T) {
			state := &StudyActiveSessionState{
				Items: []StudySessionMaterial{
					studyMaterial(
						1,
						10,
						true,
					),
					studyMaterial(
						2,
						11,
						false,
					),
				},
			}
			ids := state.NewlyStudiedMaterialIDs()
			if len(ids) != 1 || ids[0] != 10 {
				t.Fatalf(
					"NewlyStudiedMaterialIDs() = %v, want [10]",
					ids,
				)
			}
		},
	)

	t.Run(
		"none newly studied returns empty non-nil slice",
		func(t *testing.T) {
			state := &StudyActiveSessionState{
				Items: []StudySessionMaterial{studyMaterial(
					1,
					10,
					true,
				)},
				InitiallyStudiedMaterialIDs: map[int]bool{10: true},
			}
			ids := state.NewlyStudiedMaterialIDs()
			if ids == nil {
				t.Fatal("NewlyStudiedMaterialIDs() = nil, want empty slice")
			}
			if len(ids) != 0 {
				t.Fatalf(
					"NewlyStudiedMaterialIDs() = %v, want empty",
					ids,
				)
			}
		},
	)
}
