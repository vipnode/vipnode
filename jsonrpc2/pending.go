package jsonrpc2

import (
	"sort"
	"time"
)

type pendingMsg struct {
	msgChan   chan Message
	timestamp time.Time
}

type pendingItem struct {
	key       string
	timestamp time.Time
}

type pendingQueue []pendingItem

func (p pendingQueue) Len() int {
	return len(p)
}

func (p pendingQueue) Less(i, j int) bool {
	return p[i].timestamp.Before(p[j].timestamp)
}

func (p pendingQueue) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func pendingOldest(pending map[string]pendingMsg, num int) pendingQueue {
	if num > len(pending) {
		num = len(pending)
	}
	queue := make(pendingQueue, 0, len(pending))
	for key, p := range pending {
		queue = append(queue, pendingItem{
			key, p.timestamp,
		})
	}
	sort.Sort(queue)
	return queue[:num]
}
