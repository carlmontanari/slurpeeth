package slurpeeth

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"os/exec"
)

func interfaceByNameOrAlias(interfaceNameOrAlias string) (*net.Interface, error) {
	namedInterface, err := net.InterfaceByName(interfaceNameOrAlias)
	if err == nil {
		// we found the interface by name, no need to faff about w/ checking aliases
		return namedInterface, nil
	}

	log.Printf(
		"failed looking up interface %q by name, checking for this interface by aliases...",
		interfaceNameOrAlias,
	)

	namedInterfaces, err := net.Interfaces()
	if err != nil {
		log.Printf("failed listing interfaces, error: %s", err)

		return nil, err
	}

	for _, i := range namedInterfaces {
		showCmd := exec.Command("ip", "link", "show", i.Name) //nolint: gosec

		var b []byte

		b, err = showCmd.CombinedOutput()
		if err != nil {
			log.Printf("failed executing 'ip link show', error: %s", err)

			continue
		}

		if (bytes.Contains(b, []byte("alias")) || bytes.Contains(b, []byte("altname"))) &&
			bytes.Contains(b, []byte(interfaceNameOrAlias)) {
			return &i, nil
		}
	}

	return nil, fmt.Errorf(
		"%w: could not find interface name %q as interface name or as alias/altname",
		ErrBind,
		interfaceNameOrAlias,
	)
}
