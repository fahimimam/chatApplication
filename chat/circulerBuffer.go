package chat

import "sync"

type CircularBuffer struct {
	messages []string
	size     int
	start    int
	end      int
	count    int
	mutex    sync.Mutex
}

func NewCircularBuffer(size int) *CircularBuffer {
	return &CircularBuffer{
		messages: make([]string, size),
		size:     size,
	}
}

func (cb *CircularBuffer) Add(message string) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	cb.messages[cb.end] = message
	cb.end = (cb.end + 1) % cb.size
	if cb.count == cb.size {
		cb.start = (cb.start + 1) % cb.size
	} else {
		cb.count++
	}
}

func (cb *CircularBuffer) GetAll() []string {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	result := make([]string, cb.count)
	for i := 0; i < cb.count; i++ {
		result[i] = cb.messages[(cb.start+i)%cb.size]
	}
	return result
}
