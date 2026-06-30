package dhcp

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	"github.com/paulc/dinosaur-dns/config"
)

// Server manages one DHCP subnet bound to one interface.
type Server struct {
	srv      *server4.Server
	db       *leaseDB
	cfg      config.DhcpSubnetConfig
	iface    *net.Interface
	serverIP net.IP // our IP on this interface
}

// newServer creates and binds a Server (requires root for port 67).
func newServer(cfg config.DhcpSubnetConfig, pc *config.ProxyConfig) (*Server, error) {
	iface, err := net.InterfaceByName(cfg.Interface)
	if err != nil {
		return nil, fmt.Errorf("dhcp %s: %w", cfg.Interface, err)
	}

	serverIP, err := interfaceIP(iface, cfg.Subnet, cfg.SubnetMask)
	if err != nil {
		return nil, fmt.Errorf("dhcp %s: %w", cfg.Interface, err)
	}

	db, err := newLeaseDB(cfg, pc)
	if err != nil {
		return nil, err
	}

	s := &Server{cfg: cfg, iface: iface, serverIP: serverIP, db: db}

	srv, err := server4.NewServer(
		cfg.Interface,
		&net.UDPAddr{Port: dhcpv4.ServerPort},
		s.handle,
	)
	if err != nil {
		return nil, fmt.Errorf("dhcp %s: bind: %w", cfg.Interface, err)
	}
	s.srv = srv
	return s, nil
}

// Serve runs the event loop. Call in a goroutine after privilege drop.
func (s *Server) Serve() {
	if err := s.srv.Serve(); err != nil {
		// Server closed — log but don't fatal since shutdown closes the conn.
		globalLogger.Log(Event{
			Interface: s.cfg.Interface,
			MsgType:   "SERVER",
			Error:     err.Error(),
		})
	}
}

// Close shuts down the server.
func (s *Server) Close() { s.srv.Close() }

// handle is the main DHCP dispatch function.
func (s *Server) handle(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4) {
	if m.OpCode != dhcpv4.OpcodeBootRequest {
		return
	}

	mac := m.ClientHWAddr
	clientID := clientKey(m)
	hostname := clientHostname(m)

	ev := Event{
		Timestamp: time.Now(),
		Interface: s.cfg.Interface,
		MAC:       mac.String(),
		ClientID:  clientID,
		Hostname:  hostname,
		MsgType:   m.MessageType().String(),
	}

	switch m.MessageType() {
	case dhcpv4.MessageTypeDiscover:
		s.handleDiscover(conn, peer, m, clientID, mac, hostname, ev)
	case dhcpv4.MessageTypeRequest:
		s.handleRequest(conn, peer, m, clientID, mac, hostname, ev)
	case dhcpv4.MessageTypeRelease:
		s.handleRelease(m, clientID, ev)
	case dhcpv4.MessageTypeInform:
		s.handleInform(conn, peer, m, ev)
	case dhcpv4.MessageTypeDecline:
		s.handleDecline(m, ev)
	default:
		// Unsupported message type — ignore.
	}
}

func (s *Server) handleDiscover(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4, clientID string, mac net.HardwareAddr, hostname string, ev Event) {
	lease := s.db.allocate(clientID, mac, hostname)
	if lease == nil {
		ev.Result = "no-ip-available"
		globalLogger.Log(ev)
		return
	}

	ip := net.ParseIP(lease.IP).To4()

	// ARP probe — skip if not available.
	if conflict, _ := probeConflict(s.iface, ip); conflict {
		s.db.markDeclined(ip)
		ev.Result = "arp-conflict"
		globalLogger.Log(ev)
		return
	}

	reply, err := dhcpv4.NewReplyFromRequest(m,
		dhcpv4.WithServerIP(s.serverIP),
		dhcpv4.WithYourIP(ip),
		dhcpv4.WithMessageType(dhcpv4.MessageTypeOffer),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(s.serverIP)),
	)
	if err != nil {
		ev.Error = err.Error()
		globalLogger.Log(ev)
		return
	}
	s.setOptions(reply, lease.Expires)

	if err := s.send(conn, m, reply); err != nil {
		ev.Error = err.Error()
		globalLogger.Log(ev)
		return
	}
	ev.IP = ip.String()
	ev.Result = "offer"
	globalLogger.Log(ev)
}

func (s *Server) handleRequest(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4, clientID string, mac net.HardwareAddr, hostname string, ev Event) {
	// Determine request type.
	serverIDOpt := m.Options.Get(dhcpv4.OptionServerIdentifier)
	requestedIP := m.Options.Get(dhcpv4.OptionRequestedIPAddress)

	var targetIP net.IP

	if serverIDOpt != nil {
		// SELECTING: client responding to our OFFER.
		sid := net.IP(serverIDOpt)
		if !sid.Equal(s.serverIP) {
			// Not for us.
			return
		}
		if requestedIP != nil {
			targetIP = net.IP(requestedIP)
		}
	} else if !m.ClientIPAddr.Equal(net.IPv4zero) {
		// RENEWING or REBINDING.
		targetIP = m.ClientIPAddr
	} else if requestedIP != nil {
		// INIT-REBOOT.
		targetIP = net.IP(requestedIP)
	}

	if targetIP == nil {
		s.sendNAK(conn, m, "no target IP")
		ev.Result = "nak"
		globalLogger.Log(ev)
		return
	}

	var lease *Lease
	if serverIDOpt != nil {
		lease = s.db.confirm(clientID, targetIP.To4(), hostname)
	} else {
		lease = s.db.renew(clientID, targetIP.To4(), hostname)
	}

	if lease == nil {
		s.sendNAK(conn, m, "no lease")
		ev.IP = targetIP.String()
		ev.Result = "nak"
		globalLogger.Log(ev)
		return
	}

	ip := net.ParseIP(lease.IP).To4()
	reply, err := dhcpv4.NewReplyFromRequest(m,
		dhcpv4.WithServerIP(s.serverIP),
		dhcpv4.WithYourIP(ip),
		dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(s.serverIP)),
	)
	if err != nil {
		ev.Error = err.Error()
		globalLogger.Log(ev)
		return
	}
	s.setOptions(reply, lease.Expires)

	if err := s.send(conn, m, reply); err != nil {
		ev.Error = err.Error()
		globalLogger.Log(ev)
		return
	}
	ev.IP = ip.String()
	ev.Result = "ack"
	globalLogger.Log(ev)
}

func (s *Server) handleRelease(m *dhcpv4.DHCPv4, clientID string, ev Event) {
	s.db.release(clientID, m.ClientIPAddr.To4())
	ev.IP = m.ClientIPAddr.String()
	ev.Result = "released"
	globalLogger.Log(ev)
}

func (s *Server) handleInform(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4, ev Event) {
	if m.ClientIPAddr.Equal(net.IPv4zero) {
		ev.Result = "ignored-no-ciaddr"
		globalLogger.Log(ev)
		return
	}
	reply, err := dhcpv4.NewReplyFromRequest(m,
		dhcpv4.WithServerIP(s.serverIP),
		dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(s.serverIP)),
	)
	if err != nil {
		ev.Error = err.Error()
		globalLogger.Log(ev)
		return
	}
	// Set options but no lease time.
	s.setNetworkOptions(reply)

	if err := s.send(conn, m, reply); err != nil {
		ev.Error = err.Error()
	}
	ev.IP = m.ClientIPAddr.String()
	ev.Result = "ack-inform"
	globalLogger.Log(ev)
}

func (s *Server) handleDecline(m *dhcpv4.DHCPv4, ev Event) {
	if requestedIP := m.Options.Get(dhcpv4.OptionRequestedIPAddress); requestedIP != nil {
		ip := net.IP(requestedIP).To4()
		s.db.markDeclined(ip)
		ev.IP = ip.String()
	}
	ev.Result = "declined"
	globalLogger.Log(ev)
}

func (s *Server) sendNAK(conn net.PacketConn, m *dhcpv4.DHCPv4, reason string) {
	reply, err := dhcpv4.NewReplyFromRequest(m,
		dhcpv4.WithMessageType(dhcpv4.MessageTypeNak),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(s.serverIP)),
		dhcpv4.WithOption(dhcpv4.OptMessage(reason)),
	)
	if err != nil {
		return
	}
	// NAK must always be broadcast.
	conn.WriteTo(reply.ToBytes(), &net.UDPAddr{IP: net.IPv4bcast, Port: dhcpv4.ClientPort}) //nolint:errcheck
}

// send delivers a DHCP reply using RFC 2131 addressing rules.
func (s *Server) send(conn net.PacketConn, req *dhcpv4.DHCPv4, reply *dhcpv4.DHCPv4) error {
	var dst net.Addr
	if !req.ClientIPAddr.Equal(net.IPv4zero) {
		// Unicast to renewing client that knows its IP.
		dst = &net.UDPAddr{IP: req.ClientIPAddr, Port: dhcpv4.ClientPort}
	} else {
		// Client has no IP yet — broadcast.
		dst = &net.UDPAddr{IP: net.IPv4bcast, Port: dhcpv4.ClientPort}
	}
	_, err := conn.WriteTo(reply.ToBytes(), dst)
	return err
}

// setOptions applies all configured DHCP options including lease timing.
func (s *Server) setOptions(reply *dhcpv4.DHCPv4, expires time.Time) {
	s.setNetworkOptions(reply)

	remaining := time.Until(expires)
	leaseSeconds := uint32(remaining.Seconds())
	if leaseSeconds < 60 {
		leaseSeconds = 60
	}

	reply.Options.Update(dhcpv4.OptIPAddressLeaseTime(time.Duration(leaseSeconds) * time.Second))
	reply.Options.Update(dhcpv4.OptRenewTimeValue(time.Duration(leaseSeconds/2) * time.Second))
	reply.Options.Update(dhcpv4.OptRebindingTimeValue(time.Duration(leaseSeconds*7/8) * time.Second))
}

// setNetworkOptions applies subnet/router/DNS options (no lease time).
func (s *Server) setNetworkOptions(reply *dhcpv4.DHCPv4) {
	mask := net.IPMask(net.ParseIP(s.cfg.SubnetMask).To4())
	reply.Options.Update(dhcpv4.OptSubnetMask(mask))

	if len(s.cfg.Routers) > 0 {
		var routers []net.IP
		for _, r := range s.cfg.Routers {
			if ip := net.ParseIP(r).To4(); ip != nil {
				routers = append(routers, ip)
			}
		}
		if len(routers) > 0 {
			reply.Options.Update(dhcpv4.OptRouter(routers...))
		}
	}

	if len(s.cfg.DNS) > 0 {
		var dnsServers []net.IP
		for _, d := range s.cfg.DNS {
			if ip := net.ParseIP(d).To4(); ip != nil {
				dnsServers = append(dnsServers, ip)
			}
		}
		if len(dnsServers) > 0 {
			reply.Options.Update(dhcpv4.OptDNS(dnsServers...))
		}
	}

	if s.cfg.DomainName != "" {
		reply.Options.Update(dhcpv4.OptDomainName(s.cfg.DomainName))
	}
}

// clientKey returns the primary client identifier: option 61 (client-id) hex
// if present, otherwise the MAC address string.
func clientKey(m *dhcpv4.DHCPv4) string {
	if cid := m.Options.Get(dhcpv4.OptionClientIdentifier); len(cid) > 0 {
		return fmt.Sprintf("%x", []byte(cid))
	}
	return m.ClientHWAddr.String()
}

// clientHostname returns the hostname from option 81 (FQDN) if present,
// falling back to option 12 (Hostname). Returns the first label of a FQDN.
func clientHostname(m *dhcpv4.DHCPv4) string {
	if raw := m.Options.Get(dhcpv4.OptionFQDN); len(raw) > 3 {
		// RFC 4702: bytes 0-2 are flags/rcodes; bytes 3+ are the name.
		name := strings.TrimSuffix(string(raw[3:]), ".")
		if label := strings.SplitN(name, ".", 2)[0]; label != "" {
			return label
		}
	}
	return m.HostName()
}

// interfaceIP finds the IP address of iface that belongs to the given subnet.
func interfaceIP(iface *net.Interface, subnet, mask string) (net.IP, error) {
	subnetIP := net.ParseIP(subnet).To4()
	subnetMask := net.IPMask(net.ParseIP(mask).To4())
	if subnetIP == nil || subnetMask == nil {
		return nil, fmt.Errorf("invalid subnet/mask: %s/%s", subnet, mask)
	}
	network := &net.IPNet{IP: subnetIP.Mask(subnetMask), Mask: subnetMask}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}
	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP.To4()
		case *net.IPAddr:
			ip = v.IP.To4()
		}
		if ip != nil && network.Contains(ip) {
			return ip, nil
		}
	}
	return nil, fmt.Errorf("no address on %s in subnet %s/%s", iface.Name, subnet, mask)
}
