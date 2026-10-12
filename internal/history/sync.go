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
	// SyncStatePartial means some target devices hold the item but not all, so
	// the item is only partly delivered and at least one device still waits.
	SyncStatePartial
	// SyncStateOffline means no target device holds the item yet and at least
	// one is offline, so delivery is still waiting.
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

// Origin records where a history item came from. Only items captured on this
// machine are delivered to paired devices, so only they carry a sync status;
// an item received from a peer over the LAN already arrived here and shows
// none.
type Origin uint8

const (
	// OriginLocal is the zero value: the item was captured on this machine and
	// is delivered to paired devices, so the preview shows its sync status.
	OriginLocal Origin = iota
	// OriginPeer means the item arrived from a paired device over the LAN. It
	// needs no sync status, only a note of the device it came from.
	OriginPeer
)
