package protocol

// HeartbeatMessage is sent to check connection health.
type HeartbeatMessage struct {
	Timestamp int64
}
