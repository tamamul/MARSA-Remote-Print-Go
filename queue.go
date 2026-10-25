package main

import (
	"fmt"
	"sync"
	"time"
)

type PendingFile struct {
	ID         string
	Path       string
	FileName   string
	DetectedAt time.Time
}

type Queue struct {
	mu      sync.Mutex
	items   map[string]*PendingFile
	counter int
}

func NewQueue() *Queue {
	return &Queue{items: make(map[string]*PendingFile)}
}

func (q *Queue) Add(path, fileName string) *PendingFile {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.counter++
	item := &PendingFile{
		ID:         fmt.Sprintf("%d", q.counter),
		Path:       path,
		FileName:   fileName,
		DetectedAt: time.Now(),
	}
	q.items[item.ID] = item
	return item
}

func (q *Queue) Get(id string) (*PendingFile, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	item, ok := q.items[id]
	return item, ok
}

func (q *Queue) Remove(id string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.items, id)
}

func (q *Queue) All() []*PendingFile {
	q.mu.Lock()
	defer q.mu.Unlock()
	list := make([]*PendingFile, 0, len(q.items))
	for _, v := range q.items {
		list = append(list, v)
	}
	return list
}
