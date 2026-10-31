package manju

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// StyleCardStore abstracts where the 74-card style catalog is sourced from at
// runtime. The default implementation (goSourceStore) reads the baked-in
// catalog compiled into the binary, so behavior is identical to before this
// change. Operators can switch to a file-backed store (or a DB-backed store)
// via configuration without recompiling.
//
// This is the seam that makes the admin portal's "online style-card
// management" possible: the admin API writes to the active store, and the
// runtime reads from it through getStyleCardByID / StyleCatalog().
type StyleCardStore interface {
	ListStyleCards() []StyleCard
	GetStyleCard(id string) (StyleCard, bool)
	UpsertStyleCard(card StyleCard) error
	DeleteStyleCard(id string) error
}

// StyleCardStoreImpl is the active store. It defaults to goSourceStore so the
// binary behaves exactly as before; init() may replace it with a fileStore
// when MANJU_STYLE_CARDS_FILE is configured.
var StyleCardStoreImpl StyleCardStore = goSourceStore{}

// ErrStoreReadOnly is returned by write operations on the compiled (read-only)
// store. Online edits require MANJU_STYLE_CARDS_FILE to be configured.
var ErrStoreReadOnly = errors.New("style card store is read-only; set MANJU_STYLE_CARDS_FILE to enable online edits")

// goSourceStore is the read-only default backed by the compiled catalog.
type goSourceStore struct{}

func (goSourceStore) ListStyleCards() []StyleCard {
	cards := make([]StyleCard, 0, len(styleCards))
	for _, card := range styleCards {
		card.CompatibleStyleBase = append([]string(nil), card.CompatibleStyleBase...)
		cards = append(cards, card)
	}
	return cards
}

func (goSourceStore) GetStyleCard(id string) (StyleCard, bool) {
	c, ok := styleCardByID[id]
	if !ok {
		return StyleCard{}, false
	}
	c.CompatibleStyleBase = append([]string(nil), c.CompatibleStyleBase...)
	return c, true
}

// The compiled catalog is immutable, so writes are rejected unless an
// override store (file/db) is configured.
func (goSourceStore) UpsertStyleCard(StyleCard) error { return ErrStoreReadOnly }
func (goSourceStore) DeleteStyleCard(string) error    { return ErrStoreReadOnly }

// fileStore persists style cards to a JSON file, seeded from the baked-in
// catalog on first run. It enables online CRUD from the admin portal without
// recompiling the binary.
type fileStore struct {
	mu   sync.Mutex
	path string
}

type fileStoreData struct {
	Version string      `json:"version"`
	Items   []StyleCard `json:"items"`
}

// newFileStore builds a file-backed store for the given JSON path.
func newFileStore(path string) *fileStore {
	return &fileStore{path: path}
}

// read loads the JSON store. If the file does not exist, it is seeded from the
// built-in catalog and written back so the admin portal sees the full 74 cards.
func (s *fileStore) read() (fileStoreData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		seed := fileStoreData{Version: StyleCatalogVersion, Items: goSourceStore{}.ListStyleCards()}
		if werr := s.writeLocked(seed); werr != nil {
			return seed, werr
		}
		return seed, nil
	}
	if err != nil {
		return fileStoreData{}, err
	}
	var d fileStoreData
	if err := json.Unmarshal(data, &d); err != nil {
		return fileStoreData{}, err
	}
	return d, nil
}

// writeLocked persists the store atomically (temp file + rename). Caller holds s.mu.
func (s *fileStore) writeLocked(d fileStoreData) error {
	if dir := filepath.Dir(s.path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	out, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *fileStore) ListStyleCards() []StyleCard {
	d, err := s.read()
	if err != nil {
		return goSourceStore{}.ListStyleCards()
	}
	cards := make([]StyleCard, 0, len(d.Items))
	for _, c := range d.Items {
		c.CompatibleStyleBase = append([]string(nil), c.CompatibleStyleBase...)
		cards = append(cards, c)
	}
	return cards
}

func (s *fileStore) GetStyleCard(id string) (StyleCard, bool) {
	d, err := s.read()
	if err != nil {
		return goSourceStore{}.GetStyleCard(id)
	}
	for _, c := range d.Items {
		if c.ID == id {
			c.CompatibleStyleBase = append([]string(nil), c.CompatibleStyleBase...)
			return c, true
		}
	}
	return StyleCard{}, false
}

func (s *fileStore) UpsertStyleCard(card StyleCard) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.readLocked()
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		d = fileStoreData{Version: StyleCatalogVersion}
	}
	idx := -1
	for i := range d.Items {
		if d.Items[i].ID == card.ID {
			idx = i
			break
		}
	}
	if idx >= 0 {
		d.Items[idx] = card
	} else {
		d.Items = append(d.Items, card)
	}
	sort.Slice(d.Items, func(i, j int) bool { return d.Items[i].Number < d.Items[j].Number })
	return s.writeLocked(d)
}

func (s *fileStore) DeleteStyleCard(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.readLocked()
	if err != nil {
		return err
	}
	kept := d.Items[:0]
	for _, c := range d.Items {
		if c.ID != id {
			kept = append(kept, c)
		}
	}
	d.Items = kept
	return s.writeLocked(d)
}

// readLocked loads the store without acquiring the lock (caller must hold s.mu).
func (s *fileStore) readLocked() (fileStoreData, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return fileStoreData{}, err
	}
	var d fileStoreData
	if err := json.Unmarshal(data, &d); err != nil {
		return fileStoreData{}, err
	}
	return d, nil
}

// init selects the active store from the environment. When
// MANJU_STYLE_CARDS_FILE is set, the catalog is read from (and written to) that
// JSON file instead of the compiled slice. Leaving it unset keeps the binary's
// behavior identical to before.
func init() {
	if p := os.Getenv("MANJU_STYLE_CARDS_FILE"); p != "" {
		StyleCardStoreImpl = newFileStore(p)
	}
}

// Exported helpers so the hosted API layer and admin portal can drive the store
// without depending on the concrete implementation.

// ListStyleCards returns all style cards from the active store.
func ListStyleCards() []StyleCard { return StyleCardStoreImpl.ListStyleCards() }

// GetStyleCard returns a single style card by ID from the active store.
func GetStyleCard(id string) (StyleCard, bool) { return StyleCardStoreImpl.GetStyleCard(id) }

// UpsertStyleCard creates or updates a style card in the active store.
func UpsertStyleCard(card StyleCard) error { return StyleCardStoreImpl.UpsertStyleCard(card) }

// DeleteStyleCard removes a style card from the active store.
func DeleteStyleCard(id string) error { return StyleCardStoreImpl.DeleteStyleCard(id) }

// getStyleCardByID is the runtime read path used by normalizeLock and
// hydrateLockStylePrompts. It routes through the active store so online edits
// take effect immediately without recompiling.
func getStyleCardByID(id string) (StyleCard, bool) { return StyleCardStoreImpl.GetStyleCard(id) }

// StyleCardStoreConfigured reports whether an editable store (file/db) is
// active. When false the catalog is the compiled read-only source and online
// writes from the admin portal are rejected with ErrStoreReadOnly.
func StyleCardStoreConfigured() bool {
	_, readOnly := StyleCardStoreImpl.(goSourceStore)
	return !readOnly
}
