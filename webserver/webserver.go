package webserver

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mikanikos/Peerster/client/clientsender"
	"github.com/mikanikos/Peerster/gossiper"
	"github.com/mikanikos/Peerster/helpers"
)

// Webserver struct
type Webserver struct {
	Gossiper *gossiper.Gossiper
	Client   *clientsender.Client
}

// NewWebserver for gui
func NewWebserver(uiPort string, gossiper *gossiper.Gossiper) *Webserver {
	return &Webserver{
		Gossiper: gossiper,
		Client:   clientsender.NewClient(uiPort),
	}
}

// Run webserver to handle get and post requests
func (webserver *Webserver) Run(portGUI string) {

	r := mux.NewRouter()
	r.Handle("/", http.FileServer(http.Dir("./webserver")))

	r.HandleFunc("/message", webserver.getMessageHandler).Methods("GET")
	r.HandleFunc("/message", webserver.postMessageHandler).Methods("POST")
	r.HandleFunc("/node", webserver.getNodeHandler).Methods("GET")
	r.HandleFunc("/node", webserver.postNodeHandler).Methods("POST")
	r.HandleFunc("/id", webserver.getIDHandler).Methods("GET")
	r.HandleFunc("/origin", webserver.getOriginHandler).Methods("GET")
	//r.HandleFunc("/private", postPrivateHandler).Methods("POST")

	// if portGUI == "" {
	// 	portGUI = portUI
	// }

	log.Fatal(http.ListenAndServe(":"+portGUI, r))
}

func writeJSON(w http.ResponseWriter, payload interface{}) {
	bytes, err := json.Marshal(payload)
	helpers.ErrorCheck(err)
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write(bytes)
}

// func (webserver *Webserver) sendMessage(msg string, dest string) {
// 	gossiperAddr, err := net.ResolveUDPAddr("udp4", helpers.BaseAddress+":"+uiPort)
// 	helpers.ErrorCheck(err)
// 	conn, err := net.DialUDP("udp", nil, gossiperAddr)
// 	helpers.ErrorCheck(err)
// 	defer conn.Close()

// 	packet := &helpers.Message{Text: msg, Destination: &dest}
// 	packetBytes, err := protobuf.Encode(packet)
// 	helpers.ErrorCheck(err)
// 	conn.Write(packetBytes)
// }

func (webserver *Webserver) getMessageHandler(w http.ResponseWriter, r *http.Request) {
	var payload = webserver.Gossiper.GetMessages()
	writeJSON(w, payload)
}

func (webserver *Webserver) postMessageHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	helpers.ErrorCheck(err)

	message := r.PostForm.Get("text")
	destination := r.PostForm.Get("destination")
	file := r.PostForm.Get("file")
	request := r.PostForm.Get("request")

	webserver.Client.SendMessage(message, &destination, &file, &request)
}

func (webserver *Webserver) getNodeHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, helpers.GetArrayStringFromAddresses(webserver.Gossiper.GetPeersAtomic()))
}

func (webserver *Webserver) postNodeHandler(w http.ResponseWriter, r *http.Request) {
	bytes, err := ioutil.ReadAll(r.Body)
	peer := string(bytes)
	peerAddr, err := net.ResolveUDPAddr("udp4", peer)
	if err == nil {
		webserver.Gossiper.AddPeer(peerAddr)
	}
}

func (webserver *Webserver) getIDHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, webserver.Gossiper.GetName())
}

func (webserver *Webserver) getOriginHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, webserver.Gossiper.GetOriginsFromRoutingTable())
}
