// Package principal de DashMon
// Cross Compilation on Raspberry
// > set GOOS=linux
// > set GOARCH=arm
// > go build dashmon
//
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	// "net/url"
	"os"
	"strings"
	"time"

	"github.com/satori/go.uuid"
)

// Get preferred outbound ip of this machine
func getOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().String()
	idx := strings.LastIndex(localAddr, ":")

	return localAddr[0:idx]
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}
	return hostname
}

// Get preferred outbound ip of this machine
func testDTCIP() bool {
	fmt.Printf("Verif DTC @ %s", time.Now().Format("02/01/2006 15:04:05")) // 31/12/2015 08:34:12
	conn, err := net.DialTimeout("udp", "10.135.9.62:80", time.Second)
	// conn, err := net.DialTimeout("tcp", "10.135.1.1:80", time.Second)
	if err != nil {
		// fmt.Printf("Erreur %s\n", err)
		return false
	}
	defer conn.Close()

	return true
}

func hello(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Hello world!")
}

func status(w http.ResponseWriter, r *http.Request) {
	var dtcStatus string
	if !testDTCIP() {
		dtcStatus = "lien coupé"
	}
	io.WriteString(w, "<div class='statushost'>"+getHostname()+"</div><div class='statusip'>"+getOutboundIP()+"</div><div class='statusdtc'>"+dtcStatus+"</div>")
}

func socCheckpoint(w http.ResponseWriter, r *http.Request) {
	doDial("window.location=\"https://threatmap.checkpoint.com/ThreatPortal/livemap.html\"")
}

func home(w http.ResponseWriter, r *http.Request) {
	doDial("window.location=\"about:home\"")
}

func refresh(w http.ResponseWriter, r *http.Request) {
	doDial("reload")
}

func dashboard(w http.ResponseWriter, r *http.Request) {
	u := r.URL
	// fmt.Println(u.Host)
	// fmt.Println(u.Path)
	// fmt.Println(u.String())
	// fmt.Println(u.RawQuery)
	doDial("window.location=\"http://frmonbcastapp01.emea.brinksgbl.com:3030/" + u.RawQuery + "\"")
}

func doDial(cmd string) {
	// connect to this socket
	conn, err := net.Dial("tcp", remoteHostname+":"+remotePortnum)

	if err != nil {
		fmt.Printf("%s: Some error %v", cmd, err)
		return
	}

	defer conn.Close()
	fmt.Printf("Connection established between %s and localhost.\n", remoteHostname)
	fmt.Printf("Local Address : %s \n", conn.LocalAddr().String())
	fmt.Printf("Remote Address : %s \n", conn.RemoteAddr().String())

	// send to socket
	fmt.Fprintf(conn, cmd+"\n")
	// listen for reply
	message, _ := bufio.NewReader(conn).ReadString('\n')
	fmt.Print("Message from server: " + message)

}

var mux map[string]func(http.ResponseWriter, *http.Request)
var remoteHostname string
var remotePortnum string
var portNum string

type configuration struct {
	UUID        string
	LogicalName string
	HostName    string
	IPAddress   string
}

func getConfig(filename string, config *configuration) *configuration {
	file, err := os.Open(filename)
	if err != nil {
		// fmt.Println("error:", err)
		config = &configuration{
			UUID:        uuid.NewV4().String(),
			LogicalName: getHostname(),
			HostName:    getHostname(),
			IPAddress:   getOutboundIP(),
		}
		configSt, _ := json.Marshal(config)
		ioutil.WriteFile(filename, configSt, 0644)
		return config
	}
	decoder := json.NewDecoder(file)
	config = &configuration{}
	err = decoder.Decode(config)
	if err != nil {
		fmt.Println("error:", err)
		return nil
	}
	config.HostName = getHostname()
	config.IPAddress = getOutboundIP()
	return config
}

func main() {
	var config configuration
	cfg := getConfig("properties.json", &config)
	if cfg == nil {
		return
	}
	fmt.Printf("%s(%s):%s\n", cfg.HostName, cfg.IPAddress, cfg.UUID)

	portNumPtr := flag.Int("port", 8000, "Port")
	remoteHostnamePtr := flag.String("remotehost", "localhost", "Remote host")
	remotePortnumPtr := flag.Int("remoteport", 32000, "Remote port")

	flag.Parse()
	remoteHostname = *remoteHostnamePtr
	remotePortnum = fmt.Sprintf("%v", *remotePortnumPtr)

	server := http.Server{
		Addr:    fmt.Sprintf(":%v", *portNumPtr),
		Handler: &myHandler{},
	}

	mux = make(map[string]func(http.ResponseWriter, *http.Request))
	mux["/"] = hello
	//
	mux["/dashboard"] = dashboard
	mux["/status"] = status
	mux["/home"] = home
	mux["/refresh"] = refresh
	mux["/checkpoint"] = socCheckpoint

	log.Fatal(server.ListenAndServe())
}

type myHandler struct{}

func (*myHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// u := r.URL
	// fmt.Println(u.Host)
	// fmt.Println(u.Path)
	// fmt.Println(u.String())
	// fmt.Println(u.RawQuery)

	if h, ok := mux[r.URL.Path]; ok {
		h(w, r)
		return
	}

	io.WriteString(w, "My server: "+r.URL.String())
}
