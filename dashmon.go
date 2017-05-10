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
	"runtime"
	"sort"
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
	internalID    int
	PlayList      []playItem
}

type playContext struct {
	playList   []playItem
	playItem   int64
	playMode   uint
	playRemain int64
	mux        sync.Mutex
}

var playContexte playContext

// Inc increments the counter for the given key.
func (c *playContext) SetNewPlayList(playList *[]playItem) {
	c.mux.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	defer c.mux.Unlock()
	c.playList = *playList
	c.playItem = 0
	c.playRemain = 0
}

func (c *playContext) timeElpased() bool {
	c.mux.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	defer c.mux.Unlock()
	c.playRemain--
	return c.playRemain < 1 || c.playMode != PlayModeGo
}

// Inc increments the counter for the given key.
func (c *playContext) SetPlayMode(mode uint) {
	c.mux.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	defer c.mux.Unlock()
	c.playMode = mode
}

// Return the current Play Mode (Go or Stop)
func (c *playContext) GetPlayMode() uint {
	c.mux.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	defer c.mux.Unlock()
	return c.playMode
}

// Return the current Play Item
func (c *playContext) GetPlayItem() int64 {
	c.mux.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	defer c.mux.Unlock()
	return c.playItem
}

// Value returns the current value of the counter for the given key.
func (c *playContext) GetCurrentPlayList() playItem {
	c.mux.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	defer c.mux.Unlock()
	c.playRemain = c.playList[c.playItem].Value
	return c.playList[c.playItem]
}

// Value returns the current value of the counter for the given key.
func (c *playContext) GetNextPlayList() playItem {
	c.mux.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	defer c.mux.Unlock()
	if c.playMode == PlayModeGo {
		c.playItem++
		if c.playItem < int64(len(c.playList)) {
			switch c.playList[c.playItem].Cmd {
			case PlayCmdLoop:
				c.playItem = c.playList[c.playItem].Value
				c.playRemain = c.playList[c.playItem].Value
				return c.playList[c.playItem]
			case PlayCmdURL:
				c.playRemain = c.playList[c.playItem].Value
				return c.playList[c.playItem]
			}
		}
	}
	return playItem{}
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
	io.WriteString(w, fmt.Sprintf("<div><p>%s-%s(%s) %d cpu(s)</p></div>", runtime.GOOS, runtime.GOARCH, runtime.Compiler, runtime.NumCPU()))
	playContexte.mux.Lock()
	defer playContexte.mux.Unlock()
	pItem := playContexte.playItem
	item := playContexte.playList[pItem]
	switch playContexte.playMode {
	case PlayModeGo:
		io.WriteString(w, fmt.Sprintf("<div><h2>Now playing (%d) : %s (wait:%d/%d)</h2></div>", pItem, item.Param, playContexte.playRemain, item.Value))
	case PlayModeStop:
		io.WriteString(w, fmt.Sprintf("<div><h2>Stopped on (%d) : %s</h2></div>", pItem, item.Param))
	default:
		io.WriteString(w, "<div><h2>Not playing</h2></div>")
	}

	io.WriteString(w, "<div><table><tr><th>Index</th><th>Cmd</th><th>Element</th><th>Value</th></tr>")
	for index, element := range playContexte.playList {
		io.WriteString(w, fmt.Sprintf("<tr><td>%d</td><td>%d</td><td>%s</td><td>%d</td></tr>", index, element.Cmd, element.Param, element.Value))
	}
	io.WriteString(w, "</table></div>")

	io.WriteString(w, fmt.Sprintf("<div><a href=\"%sdevices/%d/start\">Start</a></div>", cfg.DashboardSite, cfg.internalID))
	io.WriteString(w, fmt.Sprintf("<div><a href=\"%sdevices/%d/stop\">Stop</a></div>", cfg.DashboardSite, cfg.internalID))
	io.WriteString(w, fmt.Sprintf("<div><a href=\"%sdevices/%d/reload\">Reload</a></div>", cfg.DashboardSite, cfg.internalID))

	w.WriteHeader(http.StatusOK)

}

func socCheckpoint(w http.ResponseWriter, r *http.Request) {
	playContexte.SetPlayMode(PlayModeStop)
	io.WriteString(w, "Ok")
	doDial("window.location=\"https://threatmap.checkpoint.com/ThreatPortal/livemap.html\"")
	w.WriteHeader(http.StatusOK)
}

func socBKFF(w http.ResponseWriter, r *http.Request) {
	playContexte.SetPlayMode(PlayModeStop)
	io.WriteString(w, "Ok")
	doDial("window.location=\"http://finance.brinks.fr\"")
	w.WriteHeader(http.StatusOK)
}

func home(w http.ResponseWriter, r *http.Request) {
	playContexte.SetPlayMode(PlayModeStop)
	io.WriteString(w, "Ok")
	doDial("window.location=\"about:home\"")
	w.WriteHeader(http.StatusOK)
}

func socWordpress(w http.ResponseWriter, r *http.Request) {
	playContexte.SetPlayMode(PlayModeStop)
	io.WriteString(w, "Ok")
	doDial("window.location=\"http://frmonbcastapp01.emea.brinksgbl.com:88/\"")
	w.WriteHeader(http.StatusOK)
}

func playlistRoutine() {
	currURL := playContexte.GetCurrentPlayList()
	playContexte.SetPlayMode(PlayModeGo)
	for currURL.Param != "" {
		if playContexte.GetPlayMode() == PlayModeGo {
			doDial(fmt.Sprintf("window.location=\"%s\"", currURL.Param))
			fmt.Println("playing:", currURL.Param)
			if currURL.Value < 1 {
				fmt.Println("wait time 0 - illimited")
				break
			}
			fmt.Println("Temps à attendre:", time.Duration(currURL.Value)*time.Second)
			for !playContexte.timeElpased() {
				time.Sleep(time.Duration(1) * time.Second)
			}
			currURL = playContexte.GetNextPlayList()
			if playContexte.GetPlayMode() != PlayModeGo {
				break
			}
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
	w.WriteHeader(http.StatusOK)

	go playlistRoutine()
}

func socStop(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "PLay list Off")
	playContexte.SetPlayMode(PlayModeStop)
	w.WriteHeader(http.StatusOK)
}

func socReload(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "New pLay list")
	playContexte.SetPlayMode(PlayModeStop)
	contactWebService()
	for index, element := range playContexte.playList {
		io.WriteString(w, fmt.Sprintf("Index(%d) - Cmd()%d) - Element(%s) - Value:%d", index, element.Cmd, element.Param, element.Value))
	}
	socPlay(w, r)
	w.WriteHeader(http.StatusOK)
}

func refresh(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Ok")
	doDial("reload")
	w.WriteHeader(http.StatusOK)
}

func dashboard(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Ok")
	u := r.URL
	// fmt.Println(u.Host)
	// fmt.Println(u.Path)
	// fmt.Println(u.String())
	// fmt.Println(u.RawQuery)
	doDial(fmt.Sprintf("window.location=\"%s%s\"", cfg.DashboardSite, u.RawQuery))
	w.WriteHeader(http.StatusOK)
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
		config.DashboardSite = "http://localhost:3100/"
		configSt, _ := json.Marshal(config)
		fmt.Println("config:", configSt)
		ioutil.WriteFile(filename, configSt, 0644)
		return true
	}
	// fmt.Println("Nous allons décoder", file)
	decoder := json.NewDecoder(file)
	err = decoder.Decode(config)
	if err != nil {
		fmt.Println("error:", err)
		return false
	}
	// fmt.Println("Config décodée")
	// index is the index where we are
	// element is the element from someSlice for where we are

	config.HostName = getHostname()
	config.IPAddress = getOutboundIP()
	return true
}

type placeJSON struct {
	Id     int
	Name   string
	Geoloc string
}

type pageJSON struct {
	Id       int
	Url      string
	Note     string
	Portrait bool
}

type playitemJSON struct {
	Id    int
	Order int
	Cmd   int
	Value int
	Page  pageJSON
}

type playlistJSON struct {
	Id        int
	Name      string
	Note      string
	Playitems []playitemJSON
}

type deviceJSON struct {
	Id       int
	Name     string
	Ip       string
	Uuid     string
	Place    placeJSON
	Playlist playlistJSON
}

// ByOrder implements sort.Interface for []playitemJSON based on
// the order field.
type ByOrder []playitemJSON

func (a ByOrder) Len() int           { return len(a) }
func (a ByOrder) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByOrder) Less(i, j int) bool { return a[i].Order < a[j].Order }

func enrolDevice() *deviceJSON {
	var dev deviceJSON
	values := map[string]string{"name": cfg.LogicalName, "ip": cfg.IPAddress, "uuid": cfg.UUID}
	jsonValue, _ := json.Marshal(values)
	resp, err := http.Post(fmt.Sprintf("%sdevices", cfg.DashboardSite), "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		fmt.Println("response:", resp, " error:", err)
		return nil
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("error:", err)
		panic(err)
	}
	if err := json.Unmarshal(body, &dev); err != nil {
		fmt.Println("body:", string(body))
		panic(err)
	}
	return &dev
}

func getPlaylist(id int) *playlistJSON {
	var dev playlistJSON
	resp, err := http.Get(fmt.Sprintf("%splaylists/%d.json", cfg.DashboardSite, id))
	if err != nil {
		fmt.Println("response:", resp, " error:", err)
		return nil
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err == nil {
		if err := json.Unmarshal(body, &dev); err != nil {
			fmt.Println("body:", string(body))
			panic(err)
		}
		return &dev
	}
	return nil
}

func contactWebService() {
	dev := enrolDevice()
	if dev != nil {
		cfg.internalID = dev.Id
		play := getPlaylist(dev.Playlist.Id)
		// fmt.Println("body:", play)
		sort.Sort(ByOrder(play.Playitems))
		// fmt.Println("body:", play)
		pL := make([]playItem, len(play.Playitems))
		for i, p := range play.Playitems {
			pL[i].Cmd = uint(p.Cmd)
			pL[i].Param = p.Page.Url
			pL[i].Value = int64(p.Value)
		}
		playContexte.SetNewPlayList(&pL)
	}
}

func main() {
	playContexte = playContext{}
	if !getConfig("properties.json", &cfg) {
		return
	}

	portNumPtr := flag.Int("port", 8000, "Port")
	remoteHostnamePtr := flag.String("remotehost", "localhost", "Remote host")
	remotePortnumPtr := flag.Int("remoteport", 32000, "Remote port")
	noAutomaticEnrolmentPtr := flag.Bool("noautomatic", false, fmt.Sprintf("No automatic enrolment on %s", cfg.DashboardSite))

	flag.Parse()

	if !*noAutomaticEnrolmentPtr {
		fmt.Printf("Enrolment on %s\n", cfg.DashboardSite)
		contactWebService()
	}
	if *noAutomaticEnrolmentPtr {
		playContexte.SetNewPlayList(&cfg.PlayList)
	}

	for index, element := range playContexte.playList {
		fmt.Println("Index(", index, ") - Cmd:", element.Cmd, " Param:", element.Param, " Value:", element.Value, ".")
	}

	// fmt.Printf("Current Item %d (%s)\n", playContexte.playItem, playContexte.GetCurrentPlayList().Param)
	// fmt.Printf("Current Item %d (%s)\n", playContexte.playItem, playContexte.GetNextPlayList().Param)
	// fmt.Printf("Current Item %d (%s)\n", playContexte.playItem, playContexte.GetNextPlayList().Param)
	// fmt.Printf("Current Item %d (%s)\n", playContexte.playItem, playContexte.GetNextPlayList().Param)
	// fmt.Printf("Current Item %d (%s)\n", playContexte.playItem, playContexte.GetNextPlayList().Param)
	// fmt.Printf("Current Item %d (%s)\n", playContexte.playItem, playContexte.GetNextPlayList().Param)
	// fmt.Printf("Current Item %d (%s)\n", playContexte.playItem, playContexte.GetNextPlayList().Param)
	// fmt.Printf("Current Item %d (%s)\n", playContexte.playItem, playContexte.GetNextPlayList().Param)
	// fmt.Printf("Current Item %d (%s)\n", playContexte.playItem, playContexte.GetNextPlayList().Param)
	// fmt.Printf("Current Item %d (%s)\n", playContexte.playItem, playContexte.GetNextPlayList().Param)

	fmt.Printf("%s(%s):%s\n", cfg.HostName, cfg.IPAddress, cfg.UUID)

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
	mux["/reload"] = socReload

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
