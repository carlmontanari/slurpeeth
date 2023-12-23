package slurpeeth

// Config holds the yaml configuration used for slurpeeth.
type Config struct {
	// Segments is a list of Segments -- basically point-to-point connections.
	Segments []Segment `yaml:"segments"`
}

// Segment holds information for a p2p link -- that mean the interface we are attached to as well
// as the listen and send socket information.
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
