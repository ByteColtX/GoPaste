package history

// SyncState summarizes how a history item reached the paired devices. It
// drives the sync section of the preview pane, where each state maps to the
// copy and the actions the user sees.
type SyncState uint8

const (
	// SyncStateNone is the zero value: the item has no sync record, as when
	// synchronization is off or the item predates delivery tracking. The
	// preview hides the sync section in this state.
	SyncStateNone SyncState = iota
	// SyncStateSynced means every target device already holds the item.
	SyncStateSynced
	// SyncStateOffline means at least one target device is offline, so the
	// item is still waiting to be delivered.
	SyncStateOffline
	// SyncStateReplaced means the item never reached any device and a newer
	// item has replaced it.
	SyncStateReplaced
)

// DeliveryState is one paired device's outcome for a history item.
type DeliveryState uint8

const (
	// DeliveryPending means the device has not received the item yet.
	DeliveryPending DeliveryState = iota
	// DeliveryReceived means the device holds the item.
	DeliveryReceived
	// DeliveryOffline means the device is offline, so delivery is waiting.
	DeliveryOffline
)

// DeviceDelivery is one target device's delivery outcome for an item. The
// device ID is the same stable ID used by the paired-device list.
type DeviceDelivery struct {
	DeviceID string
	State    DeliveryState
}
