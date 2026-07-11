package model

import (
	"fmt"
	"sort"
	"time"
)

type Buffer struct {
	Name      string
	Data      string
	CreatedAt time.Time
	Order     int64
}

func (s *Server) SetBuffer(name, data string, appendData bool) Buffer {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Buffers == nil {
		s.Buffers = make(map[string]*Buffer)
	}
	if name == "" {
		for {
			name = fmt.Sprintf("buffer%d", s.NextBufferID)
			s.NextBufferID++
			if _, exists := s.Buffers[name]; !exists {
				break
			}
		}
	}
	buffer := s.Buffers[name]
	if buffer == nil {
		buffer = &Buffer{Name: name, CreatedAt: time.Now()}
		s.Buffers[name] = buffer
	}
	if appendData {
		buffer.Data += data
	} else {
		buffer.Data = data
	}
	s.NextBufferOrder++
	buffer.Order = s.NextBufferOrder
	return *buffer
}

func (s *Server) ShowBuffer(name string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	buffer := s.bufferLocked(name)
	if buffer == nil {
		return "", noBufferError(name)
	}
	return buffer.Data, nil
}

func (s *Server) DeleteBuffer(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	buffer := s.bufferLocked(name)
	if buffer == nil {
		return noBufferError(name)
	}
	delete(s.Buffers, buffer.Name)
	return nil
}

func (s *Server) RenameBuffer(name string, newName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if newName == "" {
		return fmt.Errorf("empty buffer name")
	}
	buffer := s.bufferLocked(name)
	if buffer == nil {
		return noBufferError(name)
	}
	if _, exists := s.Buffers[newName]; exists {
		return fmt.Errorf("buffer already exists: %s", newName)
	}
	delete(s.Buffers, buffer.Name)
	buffer.Name = newName
	s.Buffers[newName] = buffer
	return nil
}

func (s *Server) ListBuffers() []Buffer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	buffers := make([]Buffer, 0, len(s.Buffers))
	for _, buffer := range s.Buffers {
		buffers = append(buffers, *buffer)
	}
	sort.Slice(buffers, func(i, j int) bool {
		if buffers[i].Order == buffers[j].Order {
			return buffers[i].Name < buffers[j].Name
		}
		return buffers[i].Order > buffers[j].Order
	})
	return buffers
}

func (s *Server) bufferLocked(name string) *Buffer {
	if len(s.Buffers) == 0 {
		return nil
	}
	if name != "" {
		return s.Buffers[name]
	}
	var selected *Buffer
	for _, buffer := range s.Buffers {
		if selected == nil || buffer.Order > selected.Order {
			selected = buffer
		}
	}
	return selected
}

func noBufferError(name string) error {
	if name == "" {
		return fmt.Errorf("no buffers")
	}
	return fmt.Errorf("no buffer %s", name)
}
