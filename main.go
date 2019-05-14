package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/prometheus/common/log"
)

func healthz(response http.ResponseWriter, request *http.Request) {
	fmt.Fprintln(response, "ok")
}

func main() {
	c, err := NewCollector("https://api.qrator.net/request", os.Getenv("QRATOR_CLIENT_ID"), os.Getenv("QRATOR_X_QRATOR_AUTH"))
	if err != nil {
		log.Fatalf("Can't create collector: %v", err)
	}
	prometheus.MustRegister(c)
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/healthz", healthz)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>Qrator Exporter</title></head>
			<body>
			<h1>Qrator Exporter</h1>
			<p><a href="/metrics">Metrics</a></p>
			</body>
			</html>`))
	})
	log.Infoln("Starting qrator-exporter")
	log.Fatal(http.ListenAndServe(":9502", nil))
}
