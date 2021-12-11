package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/miekg/dns"
)

// MadnsConfig - Structure for JSON config files
type MadnsConfig struct {
	SMTPUser     string `json:",omitempty"`
	SMTPPassword string `json:",omitempty"`

	// make this "hostname:port", "smtp.gmail.com:587" for gmail+TLS
	SMTPServer string `json:",omitempty"`

	// number of seconds to aggregate responses before sending an email
	SMTPDelay int `json:",omitempty"`

	Port     int
	Handlers map[string]MadnsSubConfig
}

// MadnsSubConfig - Structure for Subdomain portion of JSON config files
type MadnsSubConfig struct {
	Redirect    string `json:",omitempty"`
	NotifyEmail string `json:",omitempty"`
	Respond     string `json:",omitempty"`
	NotifySlack string `json:",omitempty"`
}

func main() {

	var config MadnsConfig
	usage := *flag.Bool("h", false, "Show usage")
	configFile := *flag.String("c", "madns-config.json", "madns JSON Config File")
	flag.Parse()

	b, err := ioutil.ReadFile(configFile)
	if err != nil || usage {
		if err != nil {
			log.Println(err.Error())
		}
		flag.Usage()
		return
	}
	if err = json.Unmarshal(b, &config); err != nil {
		log.Fatal(err.Error())
	}

	listenString := ":" + strconv.Itoa(config.Port)

	dns.HandleFunc(".", func(w dns.ResponseWriter, req *dns.Msg) {
		handleDNS(w, req, config)
	}) // pattern-matching of HandleFunc sucks, have to do our own

	go serve("tcp", listenString)
	go serve("udp", listenString)

	sig := make(chan os.Signal)
	signal.Notify(sig)
	signal.Ignore(syscall.SIGURG)
	for {
		select {
		case s := <-sig:
			log.Fatalf("fatal: signal %s received\n", s)
		}
	}
}

func serve(net, addr string) {
	server := &dns.Server{Addr: addr, Net: net, TsigSecret: nil}
	err := server.ListenAndServe()
	if err != nil {
		log.Fatalf("Failed to setup the %s server: %v\n", net, err)
	}
}

func handleDNS(w dns.ResponseWriter, req *dns.Msg, config MadnsConfig) {

	// DETERMINE WHICH CONFIG APPLIES
	var c MadnsSubConfig
	processThis := false
	for k, v := range config.Handlers {
		if k == "." { // check default case last
			continue
		}
		reqFqdn := strings.ToLower(req.Question[0].Name)
		handlerFqdn := strings.ToLower(dns.Fqdn(k))

		if reqFqdn == handlerFqdn || strings.HasSuffix(reqFqdn, "."+handlerFqdn) {
			c = v
			processThis = true
			break
		}
	}
	if !processThis {
		cnf, ok := config.Handlers["."] // is there a default handler?
		if ok {
			c = cnf
			processThis = true
		}
	}
	if !processThis {
		log.Println("no handler for domain: ", req.Question[0].Name)
		m := new(dns.Msg)
		m.SetReply(req)
		m.SetRcode(req, dns.RcodeServerFailure)
		w.WriteMsg(m)
		return // no subsequent handling
	}

	// REDIRECT, if desired (mutually exclusive with RESPOND)
	if len(c.Redirect) > 0 {
		dnsClient := &dns.Client{Net: "udp", ReadTimeout: 4 * time.Second, WriteTimeout: 4 * time.Second, SingleInflight: true}
		if _, ok := w.RemoteAddr().(*net.TCPAddr); ok {
			dnsClient.Net = "tcp"
		}

		log.Println("redirecting using protocol: " + dnsClient.Net)

		retries := 1
	retry:
		r, _, err := dnsClient.Exchange(req, c.Redirect)
		if err == nil {
			r.Compress = true
			w.WriteMsg(r)
		} else {
			if retries > 0 {
				retries--
				log.Println("retrying...")
				goto retry
			} else {
				log.Printf("failure to forward request %q\n", err)
				m := new(dns.Msg)
				m.SetReply(req)
				m.SetRcode(req, dns.RcodeServerFailure)
				w.WriteMsg(m)
			}
		}
		// RESPOND, if desired (mutually exclusive with REDIRECT)
	} else if len(c.Respond) > 0 {
		m := new(dns.Msg)
		m.SetReply(req)
		m.SetRcode(req, dns.RcodeSuccess)

		m.Answer = make([]dns.RR, len(req.Question))
		for i := range req.Question {
			log.Println("Responding to " + req.Question[i].Name + " with " + c.Respond)

			ip := net.ParseIP(c.Respond)
			if ip == nil {
				// This is not a valid IP address, so assume it's a CNAME
				rr := new(dns.CNAME)
				rr.Hdr = dns.RR_Header{Name: req.Question[i].Name,
					Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 0}
				rr.Target = strings.TrimSuffix(c.Respond, ".") + "."
				m.Answer[i] = rr
			} else if ip.To4() == nil {
				// This is an IPv6 address, so do a AAAA record
				rr := new(dns.AAAA)
				rr.Hdr = dns.RR_Header{Name: req.Question[i].Name,
					Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 0}
				rr.AAAA = ip
				m.Answer[i] = rr
			} else {
				// This is an IPv4 address, so do an A record
				rr := new(dns.A)
				rr.Hdr = dns.RR_Header{Name: req.Question[i].Name,
					Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 0}
				rr.A = ip
				m.Answer[i] = rr
			}
		}
		w.WriteMsg(m)
	}

	body := "source: " + w.RemoteAddr().String() + "\n" +
		"proto: " + w.RemoteAddr().Network() + "\n" +
		"request:\n" + req.String() + "\n\n"

	// EMAIL NOTIFICATION, if directed
	if len(c.NotifyEmail) > 0 {
		debouncedSendEmail(c.NotifyEmail, body, config)
	}

	// Slack Notification, if directed
	if len(c.NotifySlack) > 0 {
		sendSlackMessage(c.NotifySlack, body)
	}
}
