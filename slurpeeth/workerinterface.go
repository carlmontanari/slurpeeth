package slurpeeth

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"log"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

type interfaceWorker struct {
	// sender is a 10 character string that is a hash of the segment information -- meant to
	// uniquely represent this worker in a message header (so we don't send messages from this
	// worker back to itself).
	sender       string
	name         string
	details      *syscall.SockaddrLinklayer
	fd           int
	sendChan     chan *Message
	shutdownChan chan bool
}

func newInterfaceWorker(segmentName, interfaceName string) (interfaceWorker, error) {
	segmentHash := sha256.New()
	segmentHash.Write([]byte(segmentName))
	segmentHash.Write([]byte(interfaceName))

	sender := hex.EncodeToString(segmentHash.Sum(nil))[0:10]

	log.Printf(
		"interface worker for segment %q interface %q using sender id %q",
		segmentName,
		interfaceName,
		sender,
	)

	namedInterface, err := interfaceByNameOrAlias(interfaceName)
	if err != nil {
		return interfaceWorker{}, err
	}

	interfaceDetails := &syscall.SockaddrLinklayer{
		Ifindex: namedInterface.Index,
	}

	return interfaceWorker{
		sender:       sender,
		name:         interfaceName,
		details:      interfaceDetails,
		fd:           0,
		sendChan:     make(chan *Message),
		shutdownChan: make(chan bool),
	}, nil
}

func (w *Worker) shutdownInterface(idx int) {
	log.Printf(
		"interface %q for tunnel id %d received shutdown",
		w.interfaces[idx].name,
		w.segment.ID,
	)

	err := syscall.Close(w.interfaces[idx].fd)
	if err != nil {
		log.Printf(
			"ignoring error closing fd for interface %q for tunnel id %d, err: %s",
			w.interfaces[idx].name,
			w.segment.ID,
			err,
		)
	}

	w.interfaces[idx].fd = 0
}

func (w *Worker) runInterfaces() {
	for idx := range w.interfaces {
		w.runInterface(idx)
	}
}

func (w *Worker) runInterface(idx int) {
	go w.runInterfaceRead(idx)
	go w.runInterfaceWrite(idx)
}

func (w *Worker) bindInterface(idx int) error {
	log.Printf(
		"begin worker bind for interface %s for tunnel id %d, sender id %q",
		w.interfaces[idx].name, w.segment.ID, w.interfaces[idx].sender,
	)

	fd, err := syscall.Socket(
		syscall.AF_PACKET,
		syscall.SOCK_RAW,
		EthPAll,
	)
	if err != nil {
		closeErr := syscall.Close(fd)
		if closeErr != nil {
			log.Printf(
				"encountered initial error %q, and subsequent error %q attempting to"+
					" close file descriptor",
				err, closeErr,
			)
		}

		return err
	}

	err = syscall.Bind(fd, w.interfaces[idx].details)
	if err != nil {
		closeErr := syscall.Close(fd)
		if closeErr != nil {
			log.Printf(
				"encountered error %q binding to interface %s, and subsequent error %q"+
					" attempting to close file descriptor",
				err, w.interfaces[idx].name, closeErr,
			)
		}

		return err
	}

	// tell the kernel we want the packet aux data (that has vlan info)
	err = syscall.SetsockoptInt(fd, syscall.SOL_PACKET, PacketAuxData, 1)
	if err != nil {
		log.Printf(
			"encountered error setting PACKET_AUX_DATA request for socket for"+
				" interface %s, err: %s",
			w.interfaces[idx].name, err,
		)

		return err
	}

	log.Printf(
		"begin worker bind for interface %s for tunnel id %d complete!",
		w.interfaces[idx].name,
		w.segment.ID,
	)

	w.interfaces[idx].fd = fd

	return nil
}

func (w *Worker) runInterfaceRead(idx int) {
	for {
		select {
		case <-w.interfaces[idx].shutdownChan:
			w.shutdownInterface(idx)

			return
		default:
			if w.shutdownInProgress {
				// make sure we drain the shutdown channel so we dont try to close things multiple
				// times, possibly after re-setting up interfaces
				for len(w.interfaces[idx].shutdownChan) > 0 {
					<-w.interfaces[idx].shutdownChan
				}

				w.shutdownInterface(idx)

				return
			}

			data := make([]byte, ReadSize)
			auxData := make([]byte, syscall.CmsgLen(AuxReadSize))

			readN, auxReadN, _, _, err := syscall.Recvmsg(w.interfaces[idx].fd, data, auxData, 0)
			if err != nil {
				log.Printf(
					"encountered error receiving from interface %q for tunnel id %d, err: %s",
					w.interfaces[idx].name, w.segment.ID, err,
				)

				w.interfaceErrChan <- err

				return
			}

			data = data[:readN]

			// we have to get the "aux" data from the kernel for our socket -- these socket control
			// messages hold vlan tag info we may have to re-add back to the data we slurped up.
			// very useful so post about this stuff since this is all a bit of dark magic!
			// https://stackoverflow.com/questions/56653023/ \
			//	reading-vlan-field-of-a-raw-ethernet-packet-in-python
			controlMsgs, err := syscall.ParseSocketControlMessage(auxData[:auxReadN])
			if err != nil {
				log.Printf(
					"encountered error procesing socket control message(s) from interface %q"+
						" for tunnel id %d, err: %s",
					w.interfaces[idx].name, w.segment.ID, err,
				)

				w.interfaceErrChan <- err

				return
			}

			for _, controlMsg := range controlMsgs {
				if controlMsg.Header.Level == syscall.SOL_PACKET &&
					controlMsg.Header.Type == PacketAuxData {
					parsedAuxData := (*frameAuxData)(
						unsafe.Pointer(&controlMsg.Data[0]), //nolint:gosec
					)

					if parsedAuxData.vlanTCI != 0 ||
						parsedAuxData.status&unix.TP_STATUS_VLAN_VALID == 1 {
						var taggedData []byte

						vlanTag := make([]byte, VlanTagSize)

						// pack our tag stuff into the bytes
						binary.BigEndian.PutUint16(vlanTag[0:], parsedAuxData.vlanTPID)
						binary.BigEndian.PutUint16(vlanTag[2:], parsedAuxData.vlanTCI)

						taggedData = append(taggedData, data[:12]...)
						taggedData = append(taggedData, vlanTag...)
						taggedData = append(taggedData, data[12:]...)

						data = taggedData
					}
				}
			}

			msg := NewMessageFromBody(w.segment.ID, w.interfaces[idx].sender, data)

			w.destinationFanoutChan <- &msg
		}
	}
}

func (w *Worker) runInterfaceWrite(idx int) {
	for {
		select {
		case <-w.interfaces[idx].shutdownChan:
			w.shutdownInterface(idx)

			return
		case msg := <-w.interfaces[idx].sendChan:
			if w.shutdownInProgress {
				w.shutdownInterface(idx)

				return
			}

			err := syscall.Sendto(w.interfaces[idx].fd, msg.Body, 0, w.interfaces[idx].details)
			if err != nil {
				log.Printf(
					"encountered error writing message to interface %q for tunnel id %d, err: %s",
					w.interfaces[idx].name, w.segment.ID, err,
				)

				w.interfaceErrChan <- err
			}

			if w.debug {
				log.Printf(
					"wrote %d (expected to write %d) bytes to interface %q for tunnel id %d, "+
						"message came from sender %q",
					len(
						msg.Body,
					),
					msg.Header.Size,
					w.interfaces[idx].name,
					w.segment.ID,
					msg.Header.Sender,
				)
			}
		}
	}
}
