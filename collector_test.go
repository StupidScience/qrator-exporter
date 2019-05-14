package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type jsonRequest struct {
	Method string `json:"method"`
	Params string `json:"params"`
	ID     int    `json:"id"`
}

type NewCollectorTestCase struct {
	URL           string
	ClientID      string
	Secret        string
	DomainID      int
	ExpectedError bool
}

var (
	ts          = httptest.NewServer(http.HandlerFunc(qratorTestServer))
	clientIDStr = "123"
	clientID    = 123
	secret      = "12345"
	domainID    = 321
)

var NewCollectorTestCases = []NewCollectorTestCase{
	{
		URL:      ts.URL,
		ClientID: clientIDStr,
		Secret:   secret,
		DomainID: domainID,
	},
	{
		URL:           "http://bad_host",
		ClientID:      clientIDStr,
		Secret:        secret,
		DomainID:      domainID,
		ExpectedError: true,
	},
	{
		URL:           "http://bad host",
		ClientID:      clientIDStr,
		Secret:        secret,
		DomainID:      domainID,
		ExpectedError: true,
	},
	{
		URL:           ts.URL,
		ClientID:      "bad_client_id",
		Secret:        secret,
		DomainID:      domainID,
		ExpectedError: true,
	},
	{
		URL:           ts.URL,
		ClientID:      "312",
		Secret:        secret,
		DomainID:      domainID,
		ExpectedError: true,
	},
	{
		URL:           ts.URL,
		ClientID:      clientIDStr,
		Secret:        "bad_secret",
		DomainID:      domainID,
		ExpectedError: true,
	},
	{
		URL: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"error":"bad_json","id":1`)
		})).URL,
		ClientID:      clientIDStr,
		Secret:        secret,
		DomainID:      domainID,
		ExpectedError: true,
	},
}

func qratorTestServer(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Qrator-Auth") != secret {
		fmt.Fprint(w, `{"result":null,"error":"ACLException","id":1}`)
		return
	}
	switch r.URL.Path {
	case "/client/123":
		qratorTestServerClient(w, r)
	case "/domain/321":
		qratorTestServerDomain(w, r)
	default:
		fmt.Fprint(w, `{"result":null,"error":"BadRequest","id":1}`)
	}
}

func qratorTestServerClient(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	req := jsonRequest{}
	json.NewDecoder(r.Body).Decode(&req)
	var response string
	switch req.Method {
	case "ping":
		response = `{"result":"pong","error":null,"id":1}`
	case "domains_get":
		response = `{"result":[{"id":321,"name":"www.example.com","status":"online","ip":["1.2.3.4"],"ip_json":{"balancer":"roundrobin","weights":false,"backups":true,"clusters":false,"upstreams":[{"type":"primary","ip":"1.2.3.4","weight":1,"name":""}]},"qratorIp":"1.2.3.4","isService":false,"ports":null}],"error":null,"id":1}`
	}

	fmt.Fprintf(w, response)
}

func qratorTestServerDomain(w http.ResponseWriter, r *http.Request) {
	response := `{"result":{"time":1557754384,"bsend":4111803.56044,"brecv":4203930.28571,"bout":19965551.42857,"psend":1772.3022,"precv":1898.25275,"reqspeed":151.68681,"reqlonger10s":657,"reqlonger07s":1123,"reqlonger05s":1899,"reqlonger02s":4073,"reqall":27606,"err50x":16,"err501":0,"err502":0,"err503":0,"err504":14,"ban":0,"ban_api":4,"ban_waf":0,"ban_geo":[],"billable":19},"error":null,"id":1}`
	fmt.Fprintf(w, response)
}

func TestNewCollector(t *testing.T) {
	for _, tc := range NewCollectorTestCases {
		_, err := NewCollector(tc.URL, tc.ClientID, tc.Secret)
		if err != nil && !tc.ExpectedError {
			t.Errorf("Error was not expected, got: %v", err)
		} else if tc.ExpectedError && err == nil {
			t.Error("Error was expected, got nil")
		}
	}
}

func TestDomains(t *testing.T) {
	for _, tc := range NewCollectorTestCases {
		c := Collector{
			qratorAPIURL: tc.URL,
			clientID:     clientID,
			auth:         tc.Secret,
		}
		qds, err := c.getQratorDomains()
		if err != nil && !tc.ExpectedError {
			t.Errorf("Error was not expected, got: %v", err)
			continue
		} else if err != nil && tc.ExpectedError {
			continue
		}
		if len(qds) == 0 {
			t.Error("Got no domains. Expected www.example.com")
			continue
		}
		if qds[0].Name != "www.example.com" {
			t.Errorf("Wrong domain. Expected www.example.com, got: %s", qds[0].Name)
			continue
		}
	}
}

func TestDomainStats(t *testing.T) {
	for _, tc := range NewCollectorTestCases {
		c := Collector{
			qratorAPIURL: tc.URL,
			clientID:     clientID,
			auth:         tc.Secret,
		}
		qd := qratorDomain{
			ID: tc.DomainID,
		}
		s, err := c.getQratorDomainStats(qd)
		if err != nil && !tc.ExpectedError {
			t.Errorf("Error was not expected, got: %v", err)
			continue
		} else if err != nil && tc.ExpectedError {
			continue
		}
		if s.Result.Reqlonger10S != 657 {
			t.Errorf("Expected 657 for reqlonger10s, got %d", s.Result.Reqlonger10S)
			continue
		}
	}
}
