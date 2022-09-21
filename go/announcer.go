package main

import (
	"fmt"
	"net"

	"github.com/brutella/dnssd"
)

func getNetInterfaces() []net.Interface {
	if interfaceList, interfaceListErr := net.Interfaces(); interfaceListErr == nil {
		return interfaceList
	}
	return []net.Interface{}
}

func getInterfaceIPs(iface net.Interface) []net.IP {
	if addrs, addrs_err := iface.Addrs(); addrs_err == nil {
		result := make([]net.IP, 0)
		for _, addr := range addrs {
			if ip, _, err := net.ParseCIDR(addr.String()); err == nil {
				result = append(result, ip)
			}
		}
		return result
	}
	return nil
}

func configureAnnouncer(ips []net.IP, hostName string, port int) (dnssd.Responder, dnssd.ServiceHandle, error) {
	// We only want to advertise on the interfaces that go with the addresses!
	interfaces := make([]string, 0)

	// Let's check against each of the interfaces. If (one of) the address(es) on that interface matches
	// one that we are announcing, then we want to send it out that address!
InterfacesLoop:
	for _, iface := range getNetInterfaces() {
		// Go through all the IPs associated with each interface.
		for _, ifaceIp := range getInterfaceIPs(iface) {
			// Go through all the IPs that we are broadcasting on.
			for _, ip := range ips {
				// If there is an intersection, then add the interface to the list we will advertise on
				// and then continue!

				if ifaceIp.Equal(ip) {
					interfaces = append(interfaces, iface.Name)
					continue InterfacesLoop
				}
			}
		}
	}
	cfg := dnssd.Config{
		Name:   fmt.Sprintf("RPM Test Server(%d)", port),
		Type:   "_nq._tcp",
		Domain: "local",
		Host:   hostName,
		IPs:    ips,
		Ifaces: interfaces,
		Port:   port,
	}

	dns_service, dns_service_err := dnssd.NewService(cfg)
	if dns_service_err != nil {
		return nil, nil, dns_service_err
	}
	dns_responder, dns_responder_err := dnssd.NewResponder()
	if dns_responder_err != nil {
		return nil, nil, dns_responder_err
	}
	dns_service_handle, dns_service_handle_err := dns_responder.Add(dns_service)
	if dns_service_handle_err != nil {
		return nil, nil, dns_service_handle_err
	}

	return dns_responder, dns_service_handle, nil
}
