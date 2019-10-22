package webserver

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"net/http"

	"github.com/dedis/protobuf"
	"github.com/gorilla/mux"
	"github.com/mikanikos/Peerster/gossiper"
	"github.com/mikanikos/Peerster/helpers"
)

var g *gossiper.Gossiper
var uiPort string

func writeJSON(w http.ResponseWriter, payload interface{}) {
	bytes, err := json.Marshal(payload)
	helpers.ErrorCheck(err)
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write(bytes)
}

func sendMessage(msg string) {
	gossiperAddr, err := net.ResolveUDPAddr("udp4", helpers.BaseAddress+":"+uiPort)
	helpers.ErrorCheck(err)
	conn, err := net.DialUDP("udp", nil, gossiperAddr)
	helpers.ErrorCheck(err)
	defer conn.Close()

	packet := &helpers.Message{Text: msg}
	packetBytes, err := protobuf.Encode(packet)
	helpers.ErrorCheck(err)
	conn.Write(packetBytes)
}

func getMessageHandler(w http.ResponseWriter, r *http.Request) {
	var payload = g.GetMessages()
	writeJSON(w, payload)
}

func postMessageHandler(w http.ResponseWriter, r *http.Request) {
	bytes, err := ioutil.ReadAll(r.Body)
	helpers.ErrorCheck(err)
	message := string(bytes)
	sendMessage(message)
}

func getNodeHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, helpers.GetArrayStringFromAddresses(g.GetPeersAtomic()))
}

func postNodeHandler(w http.ResponseWriter, r *http.Request) {
	bytes, err := ioutil.ReadAll(r.Body)
	peer := string(bytes)
	peerAddr, err := net.ResolveUDPAddr("udp4", peer)
	if err == nil {
		g.AddPeer(peerAddr)
	}
}

func getIDHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, g.GetName())
}

// RunWebServer to handle get and post requests
func RunWebServer(gossiper *gossiper.Gossiper, portUI string, portGUI string) {
	g = gossiper
	uiPort = portUI

	r := mux.NewRouter()

	r.Handle("/", http.FileServer(http.Dir("./webserver")))

	r.HandleFunc("/message", getMessageHandler).Methods("GET")
	r.HandleFunc("/message", postMessageHandler).Methods("POST")
	r.HandleFunc("/node", getNodeHandler).Methods("GET")
	r.HandleFunc("/node", postNodeHandler).Methods("POST")
	r.HandleFunc("/id", getIDHandler).Methods("GET")

	if portGUI == "" {
		portGUI = portUI
	}

	log.Fatal(http.ListenAndServe(":"+portGUI, r))
}
