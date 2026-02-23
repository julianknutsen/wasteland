package main

import (
	"fmt"
	"sync"

	"github.com/steveyegge/wasteland/internal/commons"
)

// fakeWLCommonsStore is a local in-memory WLCommonsStore for cmd package tests.
type fakeWLCommonsStore struct {
	mu          sync.Mutex
	items       map[string]*commons.WantedItem
	completions map[string]*commons.CompletionRecord
	stamps      map[string]*commons.Stamp

	// Error injection fields
	InsertWantedErr     error
	ClaimWantedErr      error
	SubmitCompletionErr error
	QueryWantedErr      error
	QueryCompletionErr  error
	AcceptCompletionErr error
	UpdateWantedErr     error
	DeleteWantedErr     error
}

func newFakeWLCommonsStore() *fakeWLCommonsStore {
	return &fakeWLCommonsStore{
		items:       make(map[string]*commons.WantedItem),
		completions: make(map[string]*commons.CompletionRecord),
		stamps:      make(map[string]*commons.Stamp),
	}
}

func (f *fakeWLCommonsStore) InsertWanted(item *commons.WantedItem) error {
	if f.InsertWantedErr != nil {
		return f.InsertWantedErr
	}
	if item.ID == "" {
		return fmt.Errorf("wanted item ID cannot be empty")
	}
	if item.Title == "" {
		return fmt.Errorf("wanted item title cannot be empty")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.items[item.ID]; exists {
		return fmt.Errorf("duplicate wanted ID %q", item.ID)
	}

	stored := *item
	if stored.Status == "" {
		stored.Status = "open"
	}
	f.items[item.ID] = &stored
	return nil
}

func (f *fakeWLCommonsStore) ClaimWanted(wantedID, rigHandle string) error {
	if f.ClaimWantedErr != nil {
		return f.ClaimWantedErr
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	item, ok := f.items[wantedID]
	if !ok {
		return fmt.Errorf("wanted item %q not found", wantedID)
	}
	if item.Status != "open" {
		return fmt.Errorf("wanted item %q is not open (status: %s)", wantedID, item.Status)
	}
	item.Status = "claimed"
	item.ClaimedBy = rigHandle
	return nil
}

func (f *fakeWLCommonsStore) SubmitCompletion(completionID, wantedID, rigHandle, evidence string) error {
	if f.SubmitCompletionErr != nil {
		return f.SubmitCompletionErr
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	item, ok := f.items[wantedID]
	if !ok {
		return fmt.Errorf("wanted item %q not found", wantedID)
	}
	if item.Status != "claimed" {
		return fmt.Errorf("wanted item %q is not claimed (status: %s)", wantedID, item.Status)
	}
	if item.ClaimedBy != rigHandle {
		return fmt.Errorf("wanted item %q is not claimed by %q (claimed by %q)", wantedID, rigHandle, item.ClaimedBy)
	}
	item.Status = "in_review"

	f.completions[wantedID] = &commons.CompletionRecord{
		ID:          completionID,
		WantedID:    wantedID,
		CompletedBy: rigHandle,
		Evidence:    evidence,
	}
	return nil
}

func (f *fakeWLCommonsStore) QueryWanted(wantedID string) (*commons.WantedItem, error) {
	if f.QueryWantedErr != nil {
		return nil, f.QueryWantedErr
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	item, ok := f.items[wantedID]
	if !ok {
		return nil, fmt.Errorf("wanted item %q not found", wantedID)
	}
	cp := *item
	return &cp, nil
}

func (f *fakeWLCommonsStore) QueryCompletion(wantedID string) (*commons.CompletionRecord, error) {
	if f.QueryCompletionErr != nil {
		return nil, f.QueryCompletionErr
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	rec, ok := f.completions[wantedID]
	if !ok {
		return nil, fmt.Errorf("no completion found for wanted item %q", wantedID)
	}
	cp := *rec
	return &cp, nil
}

func (f *fakeWLCommonsStore) AcceptCompletion(wantedID, _, _ string, stamp *commons.Stamp) error {
	if f.AcceptCompletionErr != nil {
		return f.AcceptCompletionErr
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	item, ok := f.items[wantedID]
	if !ok {
		return fmt.Errorf("wanted item %q not found", wantedID)
	}
	if item.Status != "in_review" {
		return fmt.Errorf("wanted item %q is not in_review (status: %s)", wantedID, item.Status)
	}
	item.Status = "completed"

	stored := *stamp
	f.stamps[stamp.ID] = &stored
	return nil
}

func (f *fakeWLCommonsStore) UpdateWanted(wantedID string, _ /* fields */ map[string]string) error {
	if f.UpdateWantedErr != nil {
		return f.UpdateWantedErr
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	item, ok := f.items[wantedID]
	if !ok {
		return fmt.Errorf("wanted item %q not found", wantedID)
	}
	if item.Status != "open" {
		return fmt.Errorf("wanted item %q is not open (status: %s)", wantedID, item.Status)
	}
	return nil
}

func (f *fakeWLCommonsStore) DeleteWanted(wantedID string) error {
	if f.DeleteWantedErr != nil {
		return f.DeleteWantedErr
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	item, ok := f.items[wantedID]
	if !ok {
		return fmt.Errorf("wanted item %q not found", wantedID)
	}
	if item.Status != "open" {
		return fmt.Errorf("wanted item %q is not open (status: %s)", wantedID, item.Status)
	}
	item.Status = "withdrawn"
	return nil
}

// compile-time check
var _ commons.WLCommonsStore = (*fakeWLCommonsStore)(nil)
