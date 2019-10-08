package webserver

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mikanikos/Peerster/gossiper"
	"github.com/mikanikos/Peerster/helpers"
)

var webserverPort = "8080"
var g *gossiper.Gossiper

func WriteJson(w http.ResponseWriter, payload interface{}) {
	bytes, err := json.Marshal(payload)
	helpers.ErrorCheck(err)
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write(bytes)
}

func GetMessageHandler(w http.ResponseWriter, r *http.Request) {
	//
}

func PostMessageHandler(w http.ResponseWriter, r *http.Request) {
	// wg := new(sync.WaitGroup)
	// bytes, err := ioutil.ReadAll(r.Body)
	// message := string(bytes)
	// command := "../client/client -UIPort " + g.UIPort + " -msg " + message
	// parts := strings.Fields(command)
	// out, err := exec.Command(parts[0], parts[1]).Output()
	// helpers.ErrorCheck(err)
	// wg.Done()
}

func GetNodeHandler(w http.ResponseWriter, r *http.Request) {
	//WriteJson(w, strings.Split(gossiper.GetPeersAtomic(), ","))
}

func PostNodeHandler(w http.ResponseWriter, r *http.Request) {
	// bytes, err := ioutil.ReadAll(r.Body)
	// peer := string(bytes)
	//g.AddPeer(peer)
}

func GetIDHandler(w http.ResponseWriter, r *http.Request) {
	//WriteJson(w, g.Name)
}

func RunWebServer(gossiper *gossiper.Gossiper) {
	g = gossiper

	r := mux.NewRouter()

	r.HandleFunc("/message", GetMessageHandler).Methods("GET")
	r.HandleFunc("/message", PostMessageHandler).Methods("POST")
	r.HandleFunc("/node", GetNodeHandler).Methods("GET")
	r.HandleFunc("/node", PostNodeHandler).Methods("POST")
	r.HandleFunc("/id", GetIDHandler).Methods("GET")

	log.Fatal(http.ListenAndServe(":"+webserverPort, r))
}
