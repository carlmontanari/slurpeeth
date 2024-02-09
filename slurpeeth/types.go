package slurpeeth

// Config holds the yaml configuration used for slurpeeth.
type Config struct {
	// Segments is a list of Segments -- basically point-to-point connections.
	Segments []Segment `yaml:"segments"`
}

// Segment holds information about a "segment" -- that is a collection of interfaces and
// destinations. Note that *all traffic from interfaces* is sent directly to destinations -- that
// means that for local traffic it goes "out" to a TCP connection to the destination(s) listed, but
// on of those destinations can be localhost. In this way we can connect multiple interfaces locally
// -- probably mostly useful for testing as otherwise you'd just connect veths.
type Segment struct {
	// Name is an optional friendly name for the Segment.
	Name string `yaml:"name"`
	// ID is the tunnel ID that this segment is represented by -- traffic received on the
	// Config.Port by the main slurpeeth process will send messages whose header indicates they are
	// this ID to the interface and/or destination(s) in this segment.
	ID uint16 `yaml:"id"`
	// Interfaces is a listing of local interface to send traffic received on this interface to each
	// of the Destinations in the Destination field. If no interface(s) are specified it is assumed
	// that this slurpeeth instance is basically a bridge/proxy node that will just forward traffic
	// to destinations based on tunnel id.
	Interfaces []string `yaml:"interfaces"`
	// Destinations is a listing of destination to send traffic from this Segment to.
	Destinations []string `yaml:"destinations"`
}

// Bytes is a slice of bytes.
type Bytes []byte

type destinationError struct {
	idx  int
	name string
	err  error
}

type frameAuxData struct {
	status   uint32
	len      uint32
	snapLen  uint32
	mac      uint16
	net      uint16
	vlanTCI  uint16
	vlanTPID uint16
}
