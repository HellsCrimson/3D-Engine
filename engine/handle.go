package engine

// Handle identifies an entity for as long as that entity lives, and stops
// identifying it the moment it dies.
//
// Index addresses a slot in the world's table; Generation counts how many times
// that slot has been reused. A handle to a despawned entity keeps the old
// generation, so when the slot is handed to a new entity the stale handle no
// longer resolves — instead of silently addressing whatever moved in. That was
// the failure mode of the old sequential ids, which restarted from zero on
// every scene load.
//
// The zero Handle is never issued (generations start at 1), so it doubles as
// the nil handle.
type Handle struct {
	Index      uint32
	Generation uint32
}

// NoHandle is the invalid handle.
var NoHandle = Handle{}

func (h Handle) IsZero() bool {
	return h == NoHandle
}

// Encode packs the handle into the single integer the RPC surface carries.
func (h Handle) Encode() uint64 {
	return uint64(h.Generation)<<32 | uint64(h.Index)
}

// DecodeHandle is the inverse of Encode.
func DecodeHandle(v uint64) Handle {
	return Handle{
		Index:      uint32(v),
		Generation: uint32(v >> 32),
	}
}

func (h Handle) String() string {
	if h.IsZero() {
		return "Handle(none)"
	}
	return "Handle(" + itoa(h.Index) + "v" + itoa(h.Generation) + ")"
}

func itoa(v uint32) string {
	if v == 0 {
		return "0"
	}

	var buf [10]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
