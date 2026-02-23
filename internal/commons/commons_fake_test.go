package commons

import (
	"fmt"
	"sync"
)

// fakeWLCommonsStore is an in-memory WLCommonsStore for package tests.
type fakeWLCommonsStore struct {
	mu    sync.Mutex
	items map[string]*WantedItem

	InsertWantedErr     error
	ClaimWantedErr      error
	SubmitCompletionErr error
	QueryWantedErr      error
}

func (f *fakeWLCommonsStore) InsertWanted(item *WantedItem) error {
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

func (f *fakeWLCommonsStore) SubmitCompletion(_, wantedID, rigHandle, _ string) error {
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
	return nil
}

func (f *fakeWLCommonsStore) QueryWanted(wantedID string) (*WantedItem, error) {
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

func (f *fakeWLCommonsStore) QueryWantedDetail(wantedID string) (*WantedItem, error) {
	return f.QueryWanted(wantedID)
}

func (f *fakeWLCommonsStore) QueryCompletion(_ string) (*CompletionRecord, error) {
	return nil, fmt.Errorf("not implemented in commons fake")
}

func (f *fakeWLCommonsStore) QueryStamp(_ string) (*Stamp, error) {
	return nil, fmt.Errorf("not implemented in commons fake")
}

func (f *fakeWLCommonsStore) AcceptCompletion(_, _, _ string, _ *Stamp) error {
	return fmt.Errorf("not implemented in commons fake")
}

func (f *fakeWLCommonsStore) RejectCompletion(_, _, _ string) error {
	return fmt.Errorf("not implemented in commons fake")
}

func (f *fakeWLCommonsStore) UpdateWanted(_ string, _ *WantedUpdate) error {
	return fmt.Errorf("not implemented in commons fake")
}

func (f *fakeWLCommonsStore) DeleteWanted(_ string) error {
	return fmt.Errorf("not implemented in commons fake")
}

// compile-time check
var _ WLCommonsStore = (*fakeWLCommonsStore)(nil)
