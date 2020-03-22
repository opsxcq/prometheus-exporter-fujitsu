package main

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type Server struct {
	Handler http.Handler
}

type Exporter struct {
	APIMetrics map[string]*prometheus.Desc
	user       string
	pass       string
	base       string
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	log.Println("Initializing metrics on describe")
	for _, m := range e.APIMetrics {
		ch <- m
	}
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	fmt.Println("Calling collect")
	scrapePower(e.base, e.user, e.pass, e.APIMetrics, ch)
	scrapeTemperature(e.base, e.user, e.pass, e.APIMetrics, ch)
}

func NewServer(exporter Exporter) *Server {
	r := http.NewServeMux()

	// Register Metrics from each of the endpoints
	// This invokes the Collect method through the prometheus client libraries.
	prometheus.MustRegister(&exporter)

	r.Handle("/metrics", promhttp.Handler())
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html>
		                <head><title>Fujitsu RX300 exporter</title></head>
		                <body>
		                   <h1>Fujitsu RX300 Exporter</h1>
						   <p>For more information visit <a href=https://github.com/opsxcq>[OPSXCQ]GitHub</a></p>
		                   </body>
		                </html>
		              `))
	})

	return &Server{Handler: r}
}

func (s *Server) Start() {
	log.Fatal(http.ListenAndServe(":9900", s.Handler))
}

func get(host string, uri string, user string, pass string, postBody []byte) string {
	url := host + uri
	method := "GET"
	req, err := http.NewRequest(method, url, nil)
	//req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		log.Printf("Recieved status code '%v' auth skipped", resp.StatusCode)
	}
	digestParts := digestParts(resp)
	digestParts["uri"] = uri
	digestParts["method"] = method
	digestParts["username"] = user
	digestParts["password"] = pass
	req, err = http.NewRequest(method, url, bytes.NewBuffer(postBody))
	req.Header.Set("Authorization", getDigestAuthrization(digestParts))
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		if err != nil {
			panic(err)
		}
		log.Println("response body: ", string(body))
	}

	return string(body)
}

func digestParts(resp *http.Response) map[string]string {
	result := map[string]string{}
	if len(resp.Header["Www-Authenticate"]) > 0 {
		wantedHeaders := []string{"nonce", "realm", "qop"}
		responseHeaders := strings.Split(resp.Header["Www-Authenticate"][0], ",")
		for _, r := range responseHeaders {
			for _, w := range wantedHeaders {
				if strings.Contains(r, w) {
					result[w] = strings.Split(r, `"`)[1]
				}
			}
		}
	}
	return result
}

func getMD5(text string) string {
	hasher := md5.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}

func getCnonce() string {
	b := make([]byte, 8)
	io.ReadFull(rand.Reader, b)
	return fmt.Sprintf("%x", b)[:16]
}

func getDigestAuthrization(digestParts map[string]string) string {
	d := digestParts
	ha1 := getMD5(d["username"] + ":" + d["realm"] + ":" + d["password"])
	ha2 := getMD5(d["method"] + ":" + d["uri"])
	nonceCount := 00000001
	cnonce := getCnonce()
	response := getMD5(fmt.Sprintf("%s:%s:%v:%s:%s:%s", ha1, d["nonce"], nonceCount, cnonce, d["qop"], ha2))
	authorization := fmt.Sprintf(`Digest username="%s", realm="%s", nonce="%s", uri="%s", cnonce="%s", nc="%v", qop="%s", response="%s"`,
		d["username"], d["realm"], d["nonce"], d["uri"], cnonce, nonceCount, d["qop"], response)
	return authorization
}

func main() {
	base := os.Getenv("FUJITSU_URL")
	user := os.Getenv("FUJITSU_USER")
	pass := os.Getenv("FUJITSU_PASS")

	APIMetrics := make(map[string]*prometheus.Desc)
	APIMetrics["power"] = prometheus.NewDesc(
		prometheus.BuildFQName("fujitsu", "power", "overall"),
		"Power consumption by the whole hardware",
		[]string{"max"}, nil)

	APIMetrics["power-element"] = prometheus.NewDesc(prometheus.BuildFQName("fujitsu", "power", "element"),
		"General power consumption by hardware element",
		[]string{"element", "max"}, nil)

	APIMetrics["temperature-element"] = prometheus.NewDesc(prometheus.BuildFQName("fujitsu", "temperature", "element"),
		"Temperature of the hardware elements",
		[]string{"element", "warning", "critical"}, nil)

	exp := Exporter{
		APIMetrics: APIMetrics,
		base:       base,
		user:       user,
		pass:       pass,
	}
	NewServer(exp).Start()
}

func scrapePower(base, user, pass string, metrics map[string]*prometheus.Desc, ch chan<- prometheus.Metric) {
	// Url for the power consumption page
	url := "/13"
	res := get(base, url, user, pass, nil)

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(res))

	if err != nil {
		log.Println("Problems parsing response: ")
		log.Fatal(err)
	}

	total := strings.Split(doc.Find("div.form:nth-child(1) > table:nth-child(3) > tbody:nth-child(1) > tr:nth-child(2) > td:nth-child(5) > table:nth-child(1) > tbody:nth-child(1) > tr:nth-child(1) > td:nth-child(3)").Text(), " ")[0]
	current := doc.Find("div.form:nth-child(1) > table:nth-child(3) > tbody:nth-child(1) > tr:nth-child(2) > td:nth-child(5) > table:nth-child(1) > tbody:nth-child(1) > tr:nth-child(1) > td:nth-child(1)").Text()
	currentParsed, _ := strconv.ParseFloat(current, 64)

	ch <- prometheus.MustNewConstMetric(metrics["power"], prometheus.GaugeValue, currentParsed, total)

	cpu1 := parse(doc, "div.form:nth-child(3) > table:nth-child(3) > tbody:nth-child(1) > tr:nth-child(2) > td:nth-child(4)")
	ch <- prometheus.MustNewConstMetric(metrics["power-element"], prometheus.GaugeValue, cpu1, "cpu1", total)

	cpu2 := parse(doc, "div.form:nth-child(3) > table:nth-child(3) > tbody:nth-child(1) > tr:nth-child(3) > td:nth-child(4)")
	ch <- prometheus.MustNewConstMetric(metrics["power-element"], prometheus.GaugeValue, cpu2, "cpu2", total)

	systemPower := parse(doc, "div.form:nth-child(3) > table:nth-child(3) > tbody:nth-child(1) > tr:nth-child(4) > td:nth-child(4)")
	ch <- prometheus.MustNewConstMetric(metrics["power-element"], prometheus.GaugeValue, systemPower, "system", total)

	diskPower := parse(doc, "div.form:nth-child(3) > table:nth-child(3) > tbody:nth-child(1) > tr:nth-child(5) > td:nth-child(4)")
	ch <- prometheus.MustNewConstMetric(metrics["power-element"], prometheus.GaugeValue, diskPower, "disk", total)
}

func parse(doc *goquery.Document, query string) float64 {
	value := strings.Split(doc.Find(query).Text(), " ")[0]
	valueParsed, _ := strconv.ParseFloat(value, 64)
	return valueParsed
}

func scrapeTemperature(base, user, pass string, metrics map[string]*prometheus.Desc, ch chan<- prometheus.Metric) {
	// URL for the temperature page
	url := "/18"
	res := get(base, url, user, pass, nil)

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(res))

	if err != nil {
		log.Println("Problems parsing response: ")
		log.Fatal(err)
	}

	lines := doc.Find(".sensor > tbody:nth-child(1)").Children().Length() - 1
	for i := 2; i < lines; i++ {
		strIndex := strconv.FormatInt(int64(i), 10)
		name := doc.Find(".sensor > tbody:nth-child(1) > tr:nth-child(" + strIndex + ") > td:nth-child(4)").Text()
		current, _ := strconv.ParseFloat(doc.Find(".sensor > tbody:nth-child(1) > tr:nth-child("+strIndex+") > td:nth-child(5)").Text(), 64)
		warning := doc.Find(".sensor > tbody:nth-child(1) > tr:nth-child(" + strIndex + ") > td:nth-child(6)").Text()
		critical := doc.Find(".sensor > tbody:nth-child(1) > tr:nth-child(" + strIndex + ") > td:nth-child(7)").Text()

		ch <- prometheus.MustNewConstMetric(metrics["temperature-element"], prometheus.GaugeValue, current, name, warning, critical)
	}
}
