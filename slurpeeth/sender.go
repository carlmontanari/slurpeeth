package slurpeeth

import (
	"fmt"
	"log"
	"net"
	"time"
)

func senderDial(send Socket) (net.Conn, error) {
	destination := fmt.Sprintf("%s:%d", send.Address, send.Port)

	log.Printf("dial remote send destination %s\n", destination)

	var retries int

	for {
		c, err := net.Dial(TCP, destination)
		if err == nil {
			log.Printf("dial remote succeeded on %d attempt, continuing...\n", retries)

			return c, nil
		}

		retries++

		if retries > MaxSenderRetries {
			return nil, fmt.Errorf(
				"%w: maximum retries exceeeding attempting to dial destination %s",
				ErrConnectivity, destination,
			)
		}

		log.Printf(
			"dial remote send destination %q failed on attempt %d,"+
				" sleeping a bit before trying again...",
			destination, retries,
		)

		time.Sleep(time.Second)
	}
}
