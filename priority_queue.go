package intrinio

import (
	"runtime"
	"sync/atomic"
)

// priorityQueue contains one bounded queue per priority. Lower indexes have
// higher priority. The highest-priority queue is lossless once a message has
// reached it; lower-priority queues discard their oldest message when full.
type priorityQueue struct {
	queues []prioritizedMessageQueue
}

type prioritizedMessageQueue struct {
	messages   chan []byte
	dropOldest bool
	dropped    atomic.Uint64
}

func newPriorityQueue(priorityCount int, queueDepth int) *priorityQueue {
	if priorityCount < 1 {
		panic("priorityCount must be positive")
	}
	if queueDepth < 1 {
		panic("queueDepth must be positive")
	}

	queues := make([]prioritizedMessageQueue, priorityCount)
	for i := range queues {
		queues[i] = prioritizedMessageQueue{
			messages:   make(chan []byte, queueDepth),
			dropOldest: i > 0,
		}
	}
	return &priorityQueue{queues: queues}
}

func (queue *priorityQueue) enqueue(priority int, message []byte) bool {
	if priority < 0 || priority >= len(queue.queues) {
		return false
	}

	priorityQueue := &queue.queues[priority]
	for {
		select {
		case priorityQueue.messages <- message:
			return true
		default:
		}

		if !priorityQueue.dropOldest {
			return false
		}

		select {
		case <-priorityQueue.messages:
			priorityQueue.dropped.Add(1)
		default:
			// Another worker made room between the send and receive attempts.
			runtime.Gosched()
		}
	}
}

func (queue *priorityQueue) dequeue() ([]byte, bool) {
	for i := range queue.queues {
		select {
		case message := <-queue.queues[i].messages:
			return message, true
		default:
		}
	}
	return nil, false
}

func (queue *priorityQueue) depth() int {
	depth := 0
	for i := range queue.queues {
		depth += len(queue.queues[i].messages)
	}
	return depth
}

func (queue *priorityQueue) dropCount() uint64 {
	var count uint64
	for i := range queue.queues {
		count += queue.queues[i].dropped.Load()
	}
	return count
}

func (queue *priorityQueue) reset() {
	for i := range queue.queues {
	drainQueue:
		for {
			select {
			case <-queue.queues[i].messages:
			default:
				break drainQueue
			}
		}
		queue.queues[i].dropped.Store(0)
	}
}

func enqueuePriorityMessage(
	queue *priorityQueue,
	priority int,
	message []byte,
	processNext func() bool) {
	messageCopy := append([]byte(nil), message...)
	for !queue.enqueue(priority, messageCopy) {
		// The trade queue deliberately does not discard messages. Process the
		// highest-priority pending message to make room, just as the C# client
		// does when its protected trade queue is full.
		if processNext == nil || !processNext() {
			runtime.Gosched()
		}
	}
}
