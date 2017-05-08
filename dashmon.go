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
	"bytes"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/satori/go.uuid"
)

// Type de commande utilisable dans le mode PlayList
const (
	PlayCmdNone uint = iota
	PlayCmdURL
	PlayCmdLoop
)

// Type de commande utilisable dans le mode PlayList
const (
	PlayModeNone uint = iota
	PlayModeGo
	PlayModeStop
)

type playItem struct {
	Cmd   uint
	Param string
	Value int64
}

type configuration struct {
	UUID          string
	LogicalName   string
	HostName      string
	IPAddress     string
	DashboardSite string
	PlayList      []playItem
}

type playContext struct {
	playList []playItem
	playItem int64
	playMode uint
	mux      sync.Mutex
}

var playContexte playContext

// Inc increments the counter for the given key.
func (c *playContext) SetNewPlayList(playList *[]playItem) {
	c.mux.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	c.playList = *playList
	c.playItem = 0
	c.mux.Unlock()
}

// Inc increments the counter for the given key.
func (c *playContext) SetPlayMode(mode uint) {
	c.mux.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	c.playMode = mode
	c.mux.Unlock()
}

// Return the current Play Mode (Go or Stop)
func (c *playContext) GetPlayMode() uint {
	c.mux.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	defer c.mux.Unlock()
	return c.playMode
}

// Value returns the current value of the counter for the given key.
func (c *playContext) GetCurrentPlayList() string {
	c.mux.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	defer c.mux.Unlock()
	return c.playList[c.playItem].Param
}

// Value returns the current value of the counter for the given key.
func (c *playContext) GetNextPlayList() string {
	c.mux.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	defer c.mux.Unlock()
	c.playItem++
	if c.playItem < int64(len(c.playList)) {
		switch c.playList[c.playItem].Cmd {
		case PlayCmdLoop:
			c.playItem = c.playList[c.playItem].Value
			return c.playList[c.playItem].Param
		case PlayCmdURL:
			return c.playList[c.playItem].Param
		}
	}
	return ""
}

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
	io.WriteString(w, "Ok")
	doDial("window.location=\"https://threatmap.checkpoint.com/ThreatPortal/livemap.html\"")
}

func socBKFF(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Ok")
	doDial("window.location=\"http://finance.brinks.fr\"")
}

func home(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Ok")
	doDial("window.location=\"about:home\"")
}

func socWordpress(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Ok")
	doDial("window.location=\"http://frmonbcastapp01.emea.brinksgbl.com:88/\"")
}

func playlistRoutine() {
	currURL := playContexte.GetCurrentPlayList()
	playContexte.playMode = PlayModeGo
	for currURL != "" {
		if playContexte.GetPlayMode() == PlayModeGo {
			doDial(fmt.Sprintf("window.location=\"%s\"", currURL))
			fmt.Println("playing:", currURL, " then wait:", 10, "seconds")
			time.Sleep(time.Duration(10) * time.Second)
			currURL = playContexte.GetNextPlayList()
		}
	}
	playContexte.SetPlayMode(PlayModeNone)
	fmt.Println("Exiting PlayList")
}

func socPlay(w http.ResponseWriter, r *http.Request) {
	if playContexte.GetPlayMode() == PlayModeGo {
		io.WriteString(w, "PLay list ALREADY On")
		return
	}
	io.WriteString(w, "PLay list On")
	go playlistRoutine()
}

func socStop(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "PLay list Off")
	playContexte.SetPlayMode(PlayModeStop)
}

func refresh(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Ok")
	doDial("reload")
}

func dashboard(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Ok")
	u := r.URL
	// fmt.Println(u.Host)
	// fmt.Println(u.Path)
	// fmt.Println(u.String())
	// fmt.Println(u.RawQuery)
	doDial(fmt.Sprintf("window.location=\"%s%s\"", cfg.DashboardSite, u.RawQuery))
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
var cfg configuration

func getConfig(filename string, config *configuration) bool {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Println("error:", err)
		config.UUID = uuid.NewV4().String()
		config.LogicalName = getHostname()
		config.HostName = getHostname()
		config.IPAddress = getOutboundIP()
		config.DashboardSite = "http://localhost:3000/devices"
		configSt, _ := json.Marshal(config)
		fmt.Println("config:", configSt)
		ioutil.WriteFile(filename, configSt, 0644)
		return true
	}
	fmt.Println("Nous allons décoder", file)
	decoder := json.NewDecoder(file)
	err = decoder.Decode(config)
	if err != nil {
		fmt.Println("error:", err)
		return false
	}
	fmt.Println("Config décodée")
	for index, element := range config.PlayList {
		fmt.Println("Index(", index, ") - Cmd:", element.Cmd, " Param:", element.Param, " Value:", element.Value, ".")
		// index is the index where we are
		// element is the element from someSlice for where we are
	}

	config.HostName = getHostname()
	config.IPAddress = getOutboundIP()
	return true
}

func contactWebService(config *configuration) {
	values := map[string]string{"name": cfg.LogicalName, "ip": cfg.IPAddress, "uuid": cfg.UUID}
	jsonValue, _ := json.Marshal(values)
	resp, err := http.Post(cfg.DashboardSite, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		fmt.Println("response:", resp, " error:", err)
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err == nil {
		var dat map[string]interface{}
		if err := json.Unmarshal(body, &dat); err != nil {
			panic(err)
		}
		fmt.Println("body:", dat)
	}
}

func main() {
	playContexte = playContext{playItem: 0}
	if !getConfig("properties.json", &cfg) {
		return
	}

	contactWebService(&cfg)

	playContexte.SetNewPlayList(&cfg.PlayList)
	fmt.Printf("%s(%s):%s\n", cfg.HostName, cfg.IPAddress, cfg.UUID)

	// fmt.Printf("Current Item %d (%s)\n", playContexte.playItem, playContexte.GetCurrentPlayList())
	// fmt.Printf("Current Item %d (%s)\n", playContexte.playItem, playContexte.GetNextPlayList())
	// fmt.Printf("Current Item %d (%s)\n", playContexte.playItem, playContexte.GetNextPlayList())
	// fmt.Printf("Current Item %d (%s)\n", playContexte.playItem, playContexte.GetNextPlayList())
	// fmt.Printf("Current Item %d (%s)\n", playContexte.playItem, playContexte.GetNextPlayList())
	// fmt.Printf("Current Item %d (%s)\n", playContexte.playItem, playContexte.GetNextPlayList())
	// fmt.Printf("Current Item %d (%s)\n", playContexte.playItem, playContexte.GetNextPlayList())
	// fmt.Printf("Current Item %d (%s)\n", playContexte.playItem, playContexte.GetNextPlayList())
	// fmt.Printf("Current Item %d (%s)\n", playContexte.playItem, playContexte.GetNextPlayList())
	// fmt.Printf("Current Item %d (%s)\n", playContexte.playItem, playContexte.GetNextPlayList())

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
	mux["/bkff"] = socBKFF
	mux["/wordpress"] = socWordpress
	mux["/play"] = socPlay
	mux["/stop"] = socStop

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
