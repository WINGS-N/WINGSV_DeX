package federation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// enqueue appends a receipt to the stored queue. Caller holds e.mu.
func (e *ReceiptEngine) enqueue(r Receipt) {
	queue := e.trim(e.readQueue())
	queue = append(queue, r)
	e.writeQueue(e.trim(queue))
}

// QueueSize reports how many unsent receipts are waiting.
func (e *ReceiptEngine) QueueSize() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.readQueue())
}

// Flush sends everything queued and drops what the head accepted. The window closes on the
// counters, not on a successful send: once a delta is taken the same traffic is never signed
// again, so a receipt the network refused must be kept and retried, or the node loses a
// payout over our flaky link. A retry is safe - the head rejects duplicates by nonce and
// answers success, not an error.
func (e *ReceiptEngine) Flush() {
	if !e.flushMu.TryLock() {
		return // another flush is already running
	}
	defer e.flushMu.Unlock()

	e.mu.Lock()
	queue := e.trim(e.readQueue())
	e.writeQueue(queue)
	e.mu.Unlock()
	if len(queue) == 0 {
		return
	}
	acc, ok := e.currentAccount()
	if !ok || acc.ClientID == "" {
		return // signed out: keep the queue for the next account
	}

	accepted, err := e.sendViaPanel(acc, queue)
	if err != nil {
		// The panel was unreachable by every path. The node itself is the last courier: the
		// signature is ours and it cannot forge the receipt, so handing the queue to it is safe.
		delivered := e.door.Deliver(queue)
		if delivered == receiptsNodeOff {
			// The node runs without payouts: drop the queue and stop signing for a window.
			e.mu.Lock()
			e.forget(queue)
			e.mu.Unlock()
			e.markOff()
			return
		}
		if delivered == 0 {
			return // keep the queue, retry next window
		}
	}
	// What was answered we accept whole: the head rejects duplicates itself, no point holding
	// them for a second coming.
	_ = accepted
	e.mu.Lock()
	e.forget(queue)
	e.mu.Unlock()
}

// forget removes the sent receipts from the queue by nonce. Caller holds e.mu.
func (e *ReceiptEngine) forget(sent []Receipt) {
	gone := make(map[string]struct{}, len(sent))
	for _, r := range sent {
		gone[r.Nonce] = struct{}{}
	}
	var keep []Receipt
	for _, r := range e.readQueue() {
		if _, ok := gone[r.Nonce]; !ok {
			keep = append(keep, r)
		}
	}
	e.writeQueue(keep)
}

// trim drops receipts too old for the head to take, then the oldest over the cap.
func (e *ReceiptEngine) trim(queue []Receipt) []Receipt {
	oldest := time.Now().Add(-receiptQueueMaxAge).Unix()
	var fresh []Receipt
	for _, r := range queue {
		if r.WindowEndUnix >= oldest {
			fresh = append(fresh, r)
		}
	}
	if len(fresh) > receiptQueueMax {
		fresh = fresh[len(fresh)-receiptQueueMax:]
	}
	return fresh
}

func (e *ReceiptEngine) readQueue() []Receipt {
	raw, err := os.ReadFile(filepath.Join(e.dir, "queue.json"))
	if err != nil {
		return nil
	}
	var queue []Receipt
	if err := json.Unmarshal(raw, &queue); err != nil {
		return nil // corrupt: start empty, nothing to fix here
	}
	return queue
}

func (e *ReceiptEngine) writeQueue(queue []Receipt) {
	if len(queue) == 0 {
		_ = os.Remove(filepath.Join(e.dir, "queue.json"))
		return
	}
	if raw, err := json.Marshal(queue); err == nil {
		_ = os.WriteFile(filepath.Join(e.dir, "queue.json"), raw, 0o600)
	}
}
