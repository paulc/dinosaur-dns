package api

import (
	"net/http"

	"github.com/paulc/dinosaur-dns/dhcp"
)

// DhcpLeases returns all active DHCP leases.

type DhcpLeasesRes struct {
	Leases []dhcp.Lease `json:"leases"`
}

func (s *ApiService) DhcpLeases(r *http.Request, req *Empty, res *DhcpLeasesRes) error {
	res.Leases = dhcp.AllLeases()
	return nil
}

// DhcpLeaseDelete removes a lease by IP address.

type DhcpLeaseDeleteReq struct {
	IP string `json:"ip"`
}

type DhcpLeaseDeleteRes struct {
	Found bool `json:"found"`
}

func (s *ApiService) DhcpLeaseDelete(r *http.Request, req *DhcpLeaseDeleteReq, res *DhcpLeaseDeleteRes) error {
	var err error
	res.Found, err = dhcp.DeleteLease(req.IP)
	return err
}

// DhcpLeaseAdd adds a static lease.

type DhcpLeaseAddReq struct {
	MAC      string `json:"mac"`
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
}

func (s *ApiService) DhcpLeaseAdd(r *http.Request, req *DhcpLeaseAddReq, res *Empty) error {
	return dhcp.AddStaticLease(req.MAC, req.IP, req.Hostname)
}
