package service

import (
	"context"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

type contentRepoStub struct {
	existing map[string]bool
	saved    []model.Content
}

func (r *contentRepoStub) ExistsByURL(_ context.Context, url string) (bool, error) {
	return r.existing[url], nil
}

func (r *contentRepoStub) Create(_ context.Context, content *model.Content) error {
	r.saved = append(r.saved, *content)
	return nil
}

func TestContentServiceSaveSkipsDuplicates(t *testing.T) {
	repo := &contentRepoStub{existing: map[string]bool{"https://existing.com": true}}
	contents := []model.Content{
		{SourceURL: "https://new1.com"},
		{SourceURL: "https://existing.com"},
		{SourceURL: "https://new2.com"},
	}

	result, err := NewContentService(repo).Save(context.Background(), contents)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if result.Saved != 2 || result.Duplicates != 1 || len(repo.saved) != 2 {
		t.Fatalf("result = %+v, saved = %d; want 2 new and 1 duplicate", result, len(repo.saved))
	}
}
