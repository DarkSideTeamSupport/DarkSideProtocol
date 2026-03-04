package mux

import (
	"sync"
	"sync/atomic"
)

type Frame struct {
	StreamID uint32
	Payload  []byte
}

type Session struct {
	nextStreamID atomic.Uint32
	streams      map[uint32]chan []byte
	mu           sync.RWMutex
}

func NewSession() *Session {
	s := &Session{
		streams: make(map[uint32]chan []byte),
	}
	s.nextStreamID.Store(1)
	return s
}

func (s *Session) NewStream() (uint32, <-chan []byte) {
	id := s.nextStreamID.Add(1)
	ch := make(chan []byte, 32)
	s.mu.Lock()
	s.streams[id] = ch
	s.mu.Unlock()
	return id, ch
}

func (s *Session) Push(frame Frame) {
	s.mu.RLock()
	ch := s.streams[frame.StreamID]
	s.mu.RUnlock()
	if ch == nil {
		return
	}
	select {
	case ch <- frame.Payload:
	default:
	}
}

func (s *Session) CloseStream(id uint32) {
	s.mu.Lock()
	ch := s.streams[id]
	delete(s.streams, id)
	s.mu.Unlock()
	if ch != nil {
		close(ch)
	}
}
