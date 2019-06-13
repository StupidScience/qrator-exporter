package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

const (
	namespace = "qrator"
)

type qratorRequest struct {
	Method string `json:"method"`
	Params string `json:"params"`
	ID     int    `json:"id"`
}

type qratorDomain struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	QratorIP string `json:"qratorIp"`
}

type qratorDomains struct {
	Domains []qratorDomain `json:"result"`
	Error   string         `json:"error"`
	ID      int            `json:"id"`
}

type qratorDomainStat struct {
	Result struct {
		Bsend        float64 `json:"bsend"`
		Brecv        float64 `json:"brecv"`
		Bout         float64 `json:"bout"`
		Psend        float64 `json:"psend"`
		Precv        float64 `json:"precv"`
		Reqspeed     float64 `json:"reqspeed"`
		Reqlonger10S int     `json:"reqlonger10s"`
		Reqlonger07S int     `json:"reqlonger07s"`
		Reqlonger05S int     `json:"reqlonger05s"`
		Reqlonger02S int     `json:"reqlonger02s"`
		Reqall       int     `json:"reqall"`
		Err50X       int     `json:"err50x"`
		Err501       int     `json:"err501"`
		Err502       int     `json:"err502"`
		Err503       int     `json:"err503"`
		Err504       int     `json:"err504"`
		Ban          int     `json:"ban"`
		BanAPI       int     `json:"ban_api"`
		BanWAF       int     `json:"ban_waf"`
		Billable     int     `json:"billable"`
	} `json:"result"`
	Error string `json:"error"`
	ID    int    `json:"id"`
}

type qratorPing struct {
	Result string `json:"result"`
	Error  string `json:"error"`
	ID     int    `json:"id"`
}

// Collector type for prometheus.Collector interface implementation
type Collector struct {
	auth         string
	clientID     int
	qratorAPIURL string

	BypassedTraffic   prometheus.GaugeVec
	IncomingTraffic   prometheus.GaugeVec
	OutgoingTraffic   prometheus.GaugeVec
	BypassedPackets   prometheus.GaugeVec
	IncomingPackets   prometheus.GaugeVec
	RequestRate       prometheus.GaugeVec
	SlowRequestsCount prometheus.GaugeVec
	RequestsCount     prometheus.GaugeVec
	ErrorsCount       prometheus.GaugeVec
	BannedIPs         prometheus.GaugeVec
	BillableTraffic   prometheus.GaugeVec

	totalScrapes             prometheus.Counter
	failedDomainScrapes      prometheus.Counter
	failedDomainStatsScrapes prometheus.Counter

	sync.Mutex
}

// Describe for prometheus.Collector interface implementation
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(c, ch)
}

// Collect for prometheus.Collector interface implementation
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.Lock()
	defer c.Unlock()

	c.totalScrapes.Inc()
	qds, err := c.getQratorDomains()
	if err != nil {
		c.failedDomainScrapes.Inc()
	}
	wg := &sync.WaitGroup{}
	for _, qd := range qds {
		wg.Add(1)
		go func(qd qratorDomain, ch chan<- prometheus.Metric, wg *sync.WaitGroup) {
			defer wg.Done()
			s, err := c.getQratorDomainStats(qd)
			if err != nil {
				c.failedDomainStatsScrapes.Inc()
				return
			}
			c.BypassedTraffic.WithLabelValues(qd.Name).Set(s.Result.Bsend)
			c.IncomingTraffic.WithLabelValues(qd.Name).Set(s.Result.Brecv)
			c.OutgoingTraffic.WithLabelValues(qd.Name).Set(s.Result.Bout)
			c.BypassedPackets.WithLabelValues(qd.Name).Set(s.Result.Psend)
			c.IncomingPackets.WithLabelValues(qd.Name).Set(s.Result.Precv)
			c.RequestRate.WithLabelValues(qd.Name).Set(s.Result.Reqspeed)
			c.SlowRequestsCount.WithLabelValues(qd.Name, "0.2").Set(float64(s.Result.Reqlonger02S))
			c.SlowRequestsCount.WithLabelValues(qd.Name, "0.5").Set(float64(s.Result.Reqlonger05S))
			c.SlowRequestsCount.WithLabelValues(qd.Name, "0.7").Set(float64(s.Result.Reqlonger07S))
			c.SlowRequestsCount.WithLabelValues(qd.Name, "1.0").Set(float64(s.Result.Reqlonger10S))
			c.RequestsCount.WithLabelValues(qd.Name).Set(float64(s.Result.Reqall))
			c.ErrorsCount.WithLabelValues(qd.Name, "50X").Set(float64(s.Result.Err50X))
			c.ErrorsCount.WithLabelValues(qd.Name, "501").Set(float64(s.Result.Err501))
			c.ErrorsCount.WithLabelValues(qd.Name, "502").Set(float64(s.Result.Err502))
			c.ErrorsCount.WithLabelValues(qd.Name, "503").Set(float64(s.Result.Err503))
			c.ErrorsCount.WithLabelValues(qd.Name, "504").Set(float64(s.Result.Err504))
			c.BannedIPs.WithLabelValues(qd.Name, "Qrator").Set(float64(s.Result.Ban))
			c.BannedIPs.WithLabelValues(qd.Name, "Qrator.API").Set(float64(s.Result.BanAPI))
			c.BannedIPs.WithLabelValues(qd.Name, "WAF").Set(float64(s.Result.BanWAF))
			c.BillableTraffic.WithLabelValues(qd.Name).Set(float64(s.Result.Billable))

			ch <- c.BypassedTraffic.WithLabelValues(qd.Name)
			ch <- c.IncomingTraffic.WithLabelValues(qd.Name)
			ch <- c.OutgoingTraffic.WithLabelValues(qd.Name)
			ch <- c.BypassedPackets.WithLabelValues(qd.Name)
			ch <- c.IncomingPackets.WithLabelValues(qd.Name)
			ch <- c.RequestRate.WithLabelValues(qd.Name)
			ch <- c.SlowRequestsCount.WithLabelValues(qd.Name, "0.2")
			ch <- c.SlowRequestsCount.WithLabelValues(qd.Name, "0.5")
			ch <- c.SlowRequestsCount.WithLabelValues(qd.Name, "0.7")
			ch <- c.SlowRequestsCount.WithLabelValues(qd.Name, "1.0")
			ch <- c.RequestsCount.WithLabelValues(qd.Name)
			ch <- c.ErrorsCount.WithLabelValues(qd.Name, "50X")
			ch <- c.ErrorsCount.WithLabelValues(qd.Name, "501")
			ch <- c.ErrorsCount.WithLabelValues(qd.Name, "502")
			ch <- c.ErrorsCount.WithLabelValues(qd.Name, "503")
			ch <- c.ErrorsCount.WithLabelValues(qd.Name, "504")
			ch <- c.BannedIPs.WithLabelValues(qd.Name, "Qrator")
			ch <- c.BannedIPs.WithLabelValues(qd.Name, "Qrator.API")
			ch <- c.BannedIPs.WithLabelValues(qd.Name, "WAF")
			ch <- c.BillableTraffic.WithLabelValues(qd.Name)
		}(qd, ch, wg)
	}

	wg.Wait()
	ch <- c.totalScrapes
	ch <- c.failedDomainScrapes
}

func (c *Collector) qratorPostRequest(methodClass string, id int, method string) (*http.Response, error) {
	reqURL := fmt.Sprintf("%s/%s/%d", c.qratorAPIURL, methodClass, id)
	reqBody := qratorRequest{
		Method: method,
		ID:     1,
	}
	b, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(b))
	if err != nil {
		log.Errorf("Cannot create new request: %v", err)
		return nil, fmt.Errorf("Cannot create new request: %v", err)
	}
	defer req.Body.Close()
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-Qrator-Auth", c.auth)

	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		log.Errorf("Cannot make new request: %v", err)
		return nil, err
	}

	return response, nil
}

func (c *Collector) getQratorDomainStats(qd qratorDomain) (qratorDomainStat, error) {
	r, err := c.qratorPostRequest("domain", qd.ID, "statistics_get")
	if err != nil {
		log.Errorf("Got an error on domain stats request: %v", err)
		return qratorDomainStat{}, fmt.Errorf("Got an error on domain stats request: %v", err)
	}
	defer r.Body.Close()

	s := qratorDomainStat{}
	err = json.NewDecoder(r.Body).Decode(&s)
	if err != nil {
		log.Errorf("Got an error on domain stats parsing: %v", err)
		return qratorDomainStat{}, fmt.Errorf("Got an error on domain stats parsing: %v", err)
	}

	if s.Error != "" {
		log.Errorf("Got error in domain stats response: %v", s.Error)
		return qratorDomainStat{}, fmt.Errorf("Got error in domain stats response: %v", s.Error)
	}

	return s, nil
}

func (c *Collector) getQratorDomains() ([]qratorDomain, error) {
	r, err := c.qratorPostRequest("client", c.clientID, "domains_get")
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	qds := qratorDomains{}

	err = json.NewDecoder(r.Body).Decode(&qds)
	if err != nil {
		log.Errorf("Can't decode received json: %v", err)
		return nil, err
	}
	if qds.Error != "" {
		log.Errorf("Wrong request: %s", qds.Error)
		return nil, fmt.Errorf("Wrong request: %s", qds.Error)
	}

	return qds.Domains, nil
}

func (c *Collector) qratorCheck() error {
	r, err := c.qratorPostRequest("client", c.clientID, "ping")
	if err != nil {
		return err
	}
	defer r.Body.Close()
	ping := qratorPing{}
	err = json.NewDecoder(r.Body).Decode(&ping)
	if err != nil {
		return fmt.Errorf("Got error while decoding json. %v", err)
	}
	if ping.Error != "" {
		return fmt.Errorf("Got error in response: %s", ping.Error)
	}

	return nil
}

// NewCollector create new collector struct
func NewCollector(url, clientID, auth string) (*Collector, error) {
	c := Collector{}

	var err error
	c.clientID, err = strconv.Atoi(clientID)
	if err != nil {
		return nil, fmt.Errorf("Expected digits only in client id, got: \"%s\". %v", clientID, err)
	}

	c.auth = auth
	c.qratorAPIURL = url
	err = c.qratorCheck()
	if err != nil {
		return nil, err
	}

	c.totalScrapes = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "exporter_scrapes_total",
		Help:      "Count of total scrapes",
	})

	c.failedDomainScrapes = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "exporter_failed_domain_scrapes_total",
		Help:      "Count of failed domains scrapes",
	})

	c.failedDomainStatsScrapes = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "exporter_failed_domain_stats_scrapes_total",
		Help:      "Count of failed stats scrapes",
	})

	c.BypassedTraffic = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "bypassed_traffic",
			Help:      "Bypassed traffic (bps)",
		},
		[]string{
			"domain",
		},
	)

	c.IncomingTraffic = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "incoming_traffic",
			Help:      "Incoming traffic (bps)",
		},
		[]string{
			"domain",
		},
	)

	c.OutgoingTraffic = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "outgoing_traffic",
			Help:      "Outgoing traffic (bps)",
		},
		[]string{
			"domain",
		},
	)

	c.BypassedPackets = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "bypassed_packets",
			Help:      "Bypassed packets (pps)",
		},
		[]string{
			"domain",
		},
	)

	c.IncomingPackets = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "incoming_packets",
			Help:      "Incoming packets (pps)",
		},
		[]string{
			"domain",
		},
	)

	c.RequestRate = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "request_rate",
			Help:      "Request rate (rps)",
		},
		[]string{
			"domain",
		},
	)

	c.SlowRequestsCount = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "slow_requests_count",
			Help:      "Slow request count by treshold",
		},
		[]string{
			"domain",
			"treshold_seconds",
		},
	)

	c.RequestsCount = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "requests_count_total",
			Help:      "Requests count",
		},
		[]string{
			"domain",
		},
	)

	c.ErrorsCount = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "errors_count",
			Help:      "Errors count by code",
		},
		[]string{
			"domain",
			"code",
		},
	)

	c.BannedIPs = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "banned_ip_addresses_count",
			Help:      "Number of IPs banned by Qrator",
		},
		[]string{
			"domain",
			"source",
		},
	)

	c.BillableTraffic = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "billable_traffic",
			Help:      "Billable traffic (Mbps)",
		},
		[]string{
			"domain",
		},
	)
	return &c, nil
}
