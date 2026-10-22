package intrinio

import (
	"bytes"
	"testing"
)

func TestPriorityQueueProcessesHighestPriorityFirst(t *testing.T) {
	queue := newPriorityQueue(4, 4)
	for priority := 3; priority >= 0; priority-- {
		if !queue.enqueue(priority, []byte{byte(priority)}) {
			t.Fatalf("enqueue priority %d failed", priority)
		}
	}

	for want := byte(0); want < 4; want++ {
		message, ok := queue.dequeue()
		if !ok {
			t.Fatalf("dequeue priority %d failed", want)
		}
		if !bytes.Equal(message, []byte{want}) {
			t.Fatalf("dequeue = %v, want priority %d", message, want)
		}
	}
}

func TestPriorityQueueProtectsHighestPriorityMessages(t *testing.T) {
	queue := newPriorityQueue(2, 1)
	first := []byte{1}
	if !queue.enqueue(0, first) {
		t.Fatal("first trade enqueue failed")
	}
	if queue.enqueue(0, []byte{2}) {
		t.Fatal("full trade queue unexpectedly discarded an existing trade")
	}
	if queue.dropCount() != 0 {
		t.Fatalf("drop count = %d, want 0", queue.dropCount())
	}

	message, ok := queue.dequeue()
	if !ok || !bytes.Equal(message, first) {
		t.Fatalf("dequeue = %v, %t; want %v, true", message, ok, first)
	}
}

func TestEnqueuePriorityMessageProcessesTradeToMakeRoom(t *testing.T) {
	queue := newPriorityQueue(2, 1)
	first := []byte{1}
	second := []byte{2}
	if !queue.enqueue(0, first) {
		t.Fatal("first trade enqueue failed")
	}

	var processed []byte
	enqueuePriorityMessage(queue, 0, second, func() bool {
		message, ok := queue.dequeue()
		if ok {
			processed = message
		}
		return ok
	})

	if !bytes.Equal(processed, first) {
		t.Fatalf("processed = %v, want %v", processed, first)
	}
	assertDequeuedMessage(t, queue, second)
	if queue.dropCount() != 0 {
		t.Fatalf("drop count = %d, want 0", queue.dropCount())
	}
}

func TestPriorityQueueDropsOldestLowerPriorityMessage(t *testing.T) {
	queue := newPriorityQueue(2, 2)
	for value := byte(1); value <= 3; value++ {
		if !queue.enqueue(1, []byte{value}) {
			t.Fatalf("enqueue value %d failed", value)
		}
	}
	if queue.dropCount() != 1 {
		t.Fatalf("drop count = %d, want 1", queue.dropCount())
	}

	for _, want := range []byte{2, 3} {
		message, ok := queue.dequeue()
		if !ok || !bytes.Equal(message, []byte{want}) {
			t.Fatalf("dequeue = %v, %t; want [%d], true", message, ok, want)
		}
	}
}

func TestQueueEquityMessagesSeparatesTradesAndQuotes(t *testing.T) {
	queue := newPriorityQueue(2, 2)
	quote := []byte{1, 3, 10}
	trade := []byte{0, 3, 20}
	frame := append([]byte{2}, quote...)
	frame = append(frame, trade...)

	if err := queueEquityMessages(frame, queue, nil); err != nil {
		t.Fatalf("queueEquityMessages returned %v", err)
	}

	assertDequeuedMessage(t, queue, trade)
	assertDequeuedMessage(t, queue, quote)
}

func TestQueueOptionMessagesUsesCSharpPriorityOrder(t *testing.T) {
	queue := newPriorityQueue(4, 4)
	quote := makeOptionMessage(1, OPTION_QUOTE_MSG_SIZE)
	refresh := makeOptionMessage(2, OPTION_REFRESH_MSG_SIZE)
	unusualActivity := makeOptionMessage(3, OPTION_UA_MSG_SIZE)
	trade := makeOptionMessage(0, OPTION_TRADE_MSG_SIZE)
	frame := append([]byte{4}, quote...)
	frame = append(frame, refresh...)
	frame = append(frame, unusualActivity...)
	frame = append(frame, trade...)

	if err := queueOptionMessages(frame, queue, nil); err != nil {
		t.Fatalf("queueOptionMessages returned %v", err)
	}

	assertDequeuedMessage(t, queue, trade)
	assertDequeuedMessage(t, queue, unusualActivity)
	assertDequeuedMessage(t, queue, refresh)
	assertDequeuedMessage(t, queue, quote)
}

func TestMessageRoutersRejectTruncatedFrames(t *testing.T) {
	if err := queueEquityMessages([]byte{1, 0}, newPriorityQueue(2, 1), nil); err == nil {
		t.Fatal("queueEquityMessages accepted a truncated frame")
	}

	optionFrame := append([]byte{1}, makeOptionMessage(0, OPTION_TRADE_MSG_SIZE-1)...)
	if err := queueOptionMessages(optionFrame, newPriorityQueue(4, 1), nil); err == nil {
		t.Fatal("queueOptionMessages accepted a truncated frame")
	}
}

func makeOptionMessage(messageType byte, size int) []byte {
	message := make([]byte, size)
	message[1+MAX_OPTION_SYMBOL_SIZE] = messageType
	return message
}

func assertDequeuedMessage(t *testing.T, queue *priorityQueue, want []byte) {
	t.Helper()
	message, ok := queue.dequeue()
	if !ok || !bytes.Equal(message, want) {
		t.Fatalf("dequeue = %v, %t; want %v, true", message, ok, want)
	}
}
