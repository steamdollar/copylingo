package model

import "testing"

func TestLevelProgressPercentRoundsDown(t *testing.T) {
	cases := []struct {
		name     string
		progress LevelProgress
		want     int
	}{
		{name: "no materials", progress: LevelProgress{}, want: 0},
		{name: "almost done", progress: LevelProgress{TotalMaterials: 1000, StudiedMaterials: 999}, want: 99},
		{name: "done", progress: LevelProgress{TotalMaterials: 800, StudiedMaterials: 800}, want: 100},
	}
	for _, testCase := range cases {
		if got := testCase.progress.Percent(); got != testCase.want {
			t.Errorf(
				"%s: Percent() = %d, want %d",
				testCase.name,
				got,
				testCase.want,
			)
		}
	}
}
