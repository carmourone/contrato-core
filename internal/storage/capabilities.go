package storage

type Capability string

const (
	CapTx    Capability = "tx"
	CapGraph Capability = "graph"
	CapKV    Capability = "kv"
	CapDoc   Capability = "doc"
	CapCAS   Capability = "cas"
	CapTTL   Capability = "ttl"
)

type CapSet map[Capability]struct{}

func NewCapSet(caps ...Capability) CapSet {
	s := CapSet{}
	for _, c := range caps {
		s[c] = struct{}{}
	}
	return s
}

func (s CapSet) Has(c Capability) bool {
	_, ok := s[c]
	return ok
}
