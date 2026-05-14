package sse

// Event represents an SSE message sent to connected clients.
type Event struct {
	Type string // e.g., "item:new", "item:updated"
	ID   int64  // item ID for targeted re-renders
}
