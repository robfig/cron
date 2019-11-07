package cron

import (
	"sort"
	"sync"
	"time"
)

// Store is the interface which encapsulates
// a logic of interaction with scheduled jobs
type Store interface {
	// Register appends the Entry to the set of scheduled jobs
	Register(*Entry)

	// Entry returns Entry by EntryID or empty value
	// if such job doesn't registered
	Entry(EntryID) Entry

	// Update modifies the job by applying EntrySetters to the instance
	Update(EntryID, ...EntrySetter)

	// Remove removes the Entry from the set of scheduled jobs
	Remove(EntryID)

	// Snapshot returns a snapshot of the set of scheduled jobs
	Snapshot() []Entry

	// Next returns EntryID and start time of the soonest job
	Next() (EntryID, time.Time)

	// Ready returns slice of entries which are
	// ready to start at the particular time
	Ready(time.Time) []Entry
}

type InMemoryStore struct {
	mx      sync.Mutex
	entries []*Entry
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{}
}

func (s *InMemoryStore) Register(entry *Entry) {
	s.mx.Lock()
	defer s.mx.Unlock()

	s.entries = append(s.entries, entry)
}

func (s InMemoryStore) Entry(id EntryID) Entry {
	entry := s.searchEntry(id)
	if entry == nil {
		return Entry{}
	}

	return *entry
}

func (s *InMemoryStore) Update(id EntryID, setters ...EntrySetter) {
	entry := s.searchEntry(id)
	if entry == nil {
		return
	}

	for _, set := range setters {
		set(entry)
	}
}

func (s *InMemoryStore) Remove(id EntryID) {
	s.mx.Lock()
	defer s.mx.Unlock()

	for i, entry := range s.entries {
		if id == entry.ID {
			s.entries = append(s.entries[:i], s.entries[i+1:]...)
		}
	}
}

func (s InMemoryStore) Snapshot() []Entry {
	s.mx.Lock()
	defer s.mx.Unlock()

	entries := make([]Entry, 0, len(s.entries))
	for _, entry := range s.entries {
		entries = append(entries, *entry)
	}

	return entries
}

func (s *InMemoryStore) Next() (EntryID, time.Time) {
	s.mx.Lock()
	defer s.mx.Unlock()

	if len(s.entries) == 0 {
		return 0, time.Time{}
	}

	sort.Sort(byTime(s.entries))

	next := s.entries[0]
	return next.ID, next.Next
}

func (s InMemoryStore) Ready(now time.Time) []Entry {
	s.mx.Lock()
	defer s.mx.Unlock()

	var entries []Entry
	for _, entry := range s.entries {
		if entry.Next.IsZero() || entry.Next.After(now) {
			break
		}

		entries = append(entries, *entry)
	}

	return entries
}

func (s InMemoryStore) searchEntry(id EntryID) *Entry {
	s.mx.Lock()
	defer s.mx.Unlock()

	for _, entry := range s.entries {
		if id == entry.ID {
			return entry
		}
	}

	return nil
}

// byTime is a wrapper for sorting the entry array by time
// (with zero time at the end).
type byTime []*Entry

func (s byTime) Len() int      { return len(s) }
func (s byTime) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s byTime) Less(i, j int) bool {
	// Two zero times should return false.
	// Otherwise, zero is "greater" than any other time.
	// (To sort it at the end of the list.)
	if s[i].Next.IsZero() {
		return false
	}
	if s[j].Next.IsZero() {
		return true
	}
	return s[i].Next.Before(s[j].Next)
}
