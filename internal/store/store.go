// Package store keeps the live container inventory resolved from wslc list.
package store

import (
	"sort"
	"strings"
	"sync"

	"wozzle/internal/wslc"
)

// Store is a concurrency-safe container inventory.
type Store struct {
	mu      sync.RWMutex
	byID    map[string]wslc.Container
	order   []string
	nameIdx map[string]string
}

// New creates an empty store.
func New() *Store {
	return &Store{byID: map[string]wslc.Container{}, nameIdx: map[string]string{}}
}

// Replace swaps the whole inventory atomically.
func (s *Store) Replace(list []wslc.Container) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID = map[string]wslc.Container{}
	s.nameIdx = map[string]string{}
	s.order = nil
	for _, c := range list {
		if c.ID == "" || c.Name == "" {
			continue
		}
		if _, dup := s.byID[c.ID]; dup {
			continue
		}
		s.byID[c.ID] = c
		s.nameIdx[c.Name] = c.ID
		s.order = append(s.order, c.ID)
	}
}

// All returns containers sorted running-first then by name.
func (s *Store) All() []wslc.Container {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]wslc.Container, 0, len(s.order))
	for _, id := range s.order {
		out = append(out, s.byID[id])
	}
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := out[i].State == "running", out[j].State == "running"
		if ri != rj {
			return ri
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Count returns (running, total).
func (s *Store) Count() (running, total int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.byID {
		total++
		if c.State == "running" {
			running++
		}
	}
	return
}

// Resolve accepts an exact ID, exact name, or a unique-enough prefix of either.
func (s *Store) Resolve(idOrName string) *wslc.Container {
	if idOrName == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if c, ok := s.byID[idOrName]; ok {
		cp := c
		return &cp
	}
	if id, ok := s.nameIdx[idOrName]; ok {
		cp := s.byID[id]
		return &cp
	}
	for id := range s.byID {
		if strings.HasPrefix(id, idOrName) {
			cp := s.byID[id]
			return &cp
		}
	}
	for name, id := range s.nameIdx {
		if strings.HasPrefix(name, idOrName) {
			cp := s.byID[id]
			return &cp
		}
	}
	return nil
}
