package slurpeeth

// Config holds the yaml configuration used for slurpeeth.
type Config struct {
	// Segments are p2p segments.
	Segments map[string]Segment `yaml:"segments"`
	// Domains are broadcast domains.
	Domains map[string]Domain `yaml:"domains"`
}

// Domain holds information about (broadcast) domain participants.
type Domain struct {
	// Participants is a mapping of DomainParticipant for the given domain.
	Participants map[string]DomainParticipant `yaml:"participants"`
}

// DomainParticipant holds the listen and send socket information for the domain participant.
type DomainParticipant struct {
	Listen Socket `yaml:"listen"`
	Send   Socket `yaml:"send"`
}

// Segment holds information for a p2p link -- that mean the interface we are attached to as well
// as the listen and send socket information.
type Segment struct {
	Interface string
	Listen    Socket `yaml:"listen"`
	Send      Socket `yaml:"send"`
}

// Socket holds information about a source or destination socket.
type Socket struct {
	// Address is the listen or target address.
	Address string `yaml:"address"`
	// Port is the listen or target port.
	Port int32 `yaml:"port"`
}
