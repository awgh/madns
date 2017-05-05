package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net"
	"net/smtp"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// MadnsConfig - Structure for JSON config files
type MadnsConfig struct {
	SMTPUser     string
	SMTPPassword string
	SMTPServer   string // make this "hostname:port", "smtp.gmail.com:587" for gmail+TLS

	Port     int
	Handlers map[string]MadnsSubConfig
}

// MadnsSubConfig - Structure for Subdomain portion of JSON config files
type MadnsSubConfig struct {
	Redirect    string
	NotifyEmail string
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
		//log.Println(w.RemoteAddr().String(), fqdn, k)

		if reqFqdn == handlerFqdn || strings.HasSuffix(reqFqdn, "."+handlerFqdn) {
			//if ok, err := regexp.MatchString(".*\\."+regexp.QuoteMeta(fqdn)+"\\.", dns.Fqdn(k)); ok && err == nil {
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

	// REDIRECT, if directed
	if len(c.Redirect) > 0 {
		dnsClient := &dns.Client{Net: "udp", ReadTimeout: 4 * time.Second, WriteTimeout: 4 * time.Second, SingleInflight: true}
		if _, ok := w.RemoteAddr().(*net.TCPAddr); ok {
			dnsClient.Net = "tcp"
		}

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
	}

	// EMAIL NOTIFICATION, if directed
	if len(c.NotifyEmail) > 0 {

		body := "source: " + w.RemoteAddr().String() + "\n" +
			"proto: " + w.RemoteAddr().Network() + "\n" +
			"request:\n" + req.String() + "\n"

		smtpSend(c.NotifyEmail, body, config)
	}
}

func smtpSend(to string, body string, config MadnsConfig) {
	from := config.SMTPUser
	pass := config.SMTPPassword
	authhost, _, err := net.SplitHostPort(config.SMTPServer)
	if err != nil {
		log.Printf("SMTPServer field syntax error: %s", err)
		return
	}
	msg := "From: " + from + "\n" +
		"To: " + to + "\n" +
		"Subject: Madns Alert\n\n" +
		body

	err = smtp.SendMail(config.SMTPServer,
		smtp.PlainAuth("", from, pass, authhost),
		from, []string{to}, []byte(msg))

	if err != nil {
		log.Printf("SMTP error: %s", err)
		return
	}

	log.Print("sent email to " + to)
}