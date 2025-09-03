package jobs

import (
    "sync"
)

type Store interface {
    Create(job *Job) error
    Update(job *Job) error
    Get(id string) (*Job, bool)
}

type InMemoryStore struct {
    data sync.Map
}

func NewInMemoryStore() *InMemoryStore {
    return &InMemoryStore{}
}

func (s *InMemoryStore) Create(job *Job) error {
    s.data.Store(job.ID, job)
    return nil
}

func (s *InMemoryStore) Update(job *Job) error {
    s.data.Store(job.ID, job)
    return nil
}

func (s *InMemoryStore) Get(id string) (*Job, bool) {
    if v, ok := s.data.Load(id); ok {
        return v.(*Job), true
    }
    return nil, false
}


