package tests

import (
	"encoding/hex"
	"github.com/mikanikos/Peerster/gossiper"
	"github.com/mikanikos/Peerster/helpers"
	"github.com/mikanikos/Peerster/whisper"
	"testing"
	"time"
)

func TestSimpleSym(t *testing.T) {

	// create new gossiper instance
	g1 := gossiper.NewGossiper("A", "127.0.0.1:5000", helpers.BaseAddress+":10000", "127.0.0.1:5001", 0)
	g2 := gossiper.NewGossiper("B", "127.0.0.1:5001", helpers.BaseAddress+":10001", "127.0.0.1:5002", 0)

	w1 := whisper.NewWhisper(g1)
	w2 := whisper.NewWhisper(g2)

	g1.Run()
	w1.Run()

	g2.Run()
	w2.Run()

	symKeyID1, err := w1.GenerateSymKey()
	if err != nil {
		t.Fatalf("failed when creating sym key: %s", err)
	}

	symKey, err := w1.GetSymKeyFromID(symKeyID1)
	if err != nil {
		t.Fatalf("failed when getting sym key: %s", err)
	}

	symKeyID2, err := w2.AddSymKey(hex.EncodeToString(symKey))
	if err != nil {
		t.Fatalf("failed when adding sym key: %s", err)
	}

	time.Sleep(time.Second)

	topic1 := []byte("ciao")
	topic2 := []byte("isoidsodisodiosdiosdiosdi")
	topics := make([]whisper.Topic, 0)
	topics = append(topics, whisper.ConvertBytesToTopic(topic1))
	topics = append(topics, whisper.ConvertBytesToTopic(topic2))
	crit := whisper.FilterOptions{
		SymKeyID: symKeyID1,
		MinPow: 	0.2,
		Topics:   topics,
	}

	filterHash, err := w1.NewMessageFilter(crit)
	if err != nil {
		t.Fatalf("failed when creating new filter: %s", err)
	}

	// let the status update propagate
	time.Sleep(time.Second * time.Duration(2))

	topic := []byte("ciao")
	text := "ciao andrea"
	newMessage := whisper.NewMessage{
		SymKeyID: symKeyID2,
		TTL:      60,
		Topic:    whisper.ConvertBytesToTopic(topic),
		Payload:  []byte(text),
		PowTime:  2,
	}

	_, err = w2.NewWhisperMessage(newMessage)
	if err != nil {
		t.Fatalf("failed when creating new message: %s", err)
	}

	// let the message propagate
	time.Sleep(time.Second * time.Duration(2))

	mess, err := w1.GetFilterMessages(filterHash)
	if err != nil {
		t.Fatalf("failed retrieving messages: %s", err)
	}

	if len(mess) != 1 {
		t.Fatalf("no message but expected")
	}


	for _, m := range mess {
		if string(m.Payload) != text {
			t.Logf(string(m.Payload))
			t.Fatalf("message is not expected : %s", string(m.Payload))
		}
 	}
}

func TestSimpleAsym(t *testing.T) {

	// create new gossiper instance
	g1 := gossiper.NewGossiper("A", "127.0.0.1:5000", helpers.BaseAddress+":10000", "127.0.0.1:5001", 0)
	g2 := gossiper.NewGossiper("B", "127.0.0.1:5001", helpers.BaseAddress+":10001", "127.0.0.1:5002", 0)

	w1 := whisper.NewWhisper(g1)
	w2 := whisper.NewWhisper(g2)

	g1.Run()
	w1.Run()

	g2.Run()
	w2.Run()

	pairKeyID1, err := w1.NewKeyPair()
	if err != nil {
		t.Fatalf("failed when creating key pair: %s", err)
	}

	privKey1, err := w1.GetPrivateKey(pairKeyID1)
	if err != nil {
		t.Fatalf("failed when getting priv key: %s", err)
	}

	pairKeyID2, err := w2.AddKeyPair(privKey1)
	if err != nil {
		t.Fatalf("failed when adding key pair: %s", err)
	}

	pubKey, err := w2.GetPublicKeyFromID(pairKeyID2)
	if err != nil {
		t.Fatalf("failed when getting priv key: %s", err)
	}


	time.Sleep(time.Second)

	topic1 := []byte("ciao")
	topic2 := []byte("isoidsodisodiosdiosdiosdi")
	topics := make([]whisper.Topic, 0)
	topics = append(topics, whisper.ConvertBytesToTopic(topic1))
	topics = append(topics, whisper.ConvertBytesToTopic(topic2))
	crit := whisper.FilterOptions{
		PrivateKeyID: pairKeyID1,
		MinPow: 	0.2,
		Topics:   topics,
	}

	filterHash, err := w1.NewMessageFilter(crit)
	if err != nil {
		t.Fatalf("failed when creating new filter: %s", err)
	}

	// let the status update propagate
	time.Sleep(time.Second * time.Duration(2))

	topic := []byte("ciao")
	text := "ciao andrea"
	newMessage := whisper.NewMessage{
		PublicKey: pubKey,
		TTL:      60,
		Topic:    whisper.ConvertBytesToTopic(topic),
		Payload:  []byte(text),
		PowTime:  2,
	}

	_, err = w2.NewWhisperMessage(newMessage)
	if err != nil {
		t.Fatalf("failed when creating new message: %s", err)
	}

	// let the message propagate
	time.Sleep(time.Second * time.Duration(2))

	mess, err := w1.GetFilterMessages(filterHash)
	if err != nil {
		t.Fatalf("failed retrieving messages: %s", err)
	}

	if len(mess) != 1 {
		t.Fatalf("no message but expected")
	}


	for _, m := range mess {
		if string(m.Payload) != text {
			t.Logf(string(m.Payload))
			t.Fatalf("message is not expected : %s", string(m.Payload))
		}
	}
}

//func TestGossip(t *testing.T) {
//
//	// create new gossiper instance
//	g1 := gossiper.NewGossiper("A", "127.0.0.1:5000", "127.0.0.1:10000", "127.0.0.1:5002", 0)
//	g3 := gossiper.NewGossiper("C", "127.0.0.1:5002", "127.0.0.1:10002", "", 0)
//	g2 := gossiper.NewGossiper("B", "127.0.0.1:5001", "127.0.0.1:10001", "", 0)
//
//	//w1 := whisper.NewWhisper(g1)
//	//w2 := whisper.NewWhisper(g2)
//	//w3 := whisper.NewWhisper(g3)
//
//	g1.Run()
//	//w1.Run()
//
//	g2.Run()
//	//w2.Run()
//
//	g3.Run()
//	//w3.Run()
//
//	packet1 := g1.CreateRumorMessage("ciao")
//	go g1.StartRumorMongering(packet1, packet1.Packet.Rumor.Origin, packet1.Packet.Rumor.ID)
//
//	packet2 := g2.CreateRumorMessage("ciao")
//	go g2.StartRumorMongering(packet2, packet2.Packet.Rumor.Origin, packet2.Packet.Rumor.ID)
//
//	packet3 := g3.CreateRumorMessage("ciao")
//	go g3.StartRumorMongering(packet3, packet3.Packet.Rumor.Origin, packet3.Packet.Rumor.ID)
//
//	time.Sleep(time.Duration(20) * time.Second)
//
//}

func Test3NodesAsym(t *testing.T) {

	// create new gossiper instance
	g1 := gossiper.NewGossiper("A", "127.0.0.1:5000", "127.0.0.1:10000", "127.0.0.1:5002", 0)
	g3 := gossiper.NewGossiper("C", "127.0.0.1:5002", "127.0.0.1:10002", "", 0)
	g2 := gossiper.NewGossiper("B", "127.0.0.1:5001", "127.0.0.1:10001", "", 0)

	w1 := whisper.NewWhisper(g1)
	w2 := whisper.NewWhisper(g2)
	w3 := whisper.NewWhisper(g3)

	g1.Run()
	w1.Run()

	g2.Run()
	w2.Run()

	g3.Run()
	w3.Run()

	pairKeyID1, err := w1.NewKeyPair()
	if err != nil {
		t.Fatalf("failed when creating key pair: %s", err)
	}

	privKey1, err := w1.GetPrivateKey(pairKeyID1)
	if err != nil {
		t.Fatalf("failed when getting priv key: %s", err)
	}

	pairKeyID2, err := w2.AddKeyPair(privKey1)
	if err != nil {
		t.Fatalf("failed when adding key pair: %s", err)
	}

	pubKey, err := w2.GetPublicKeyFromID(pairKeyID2)
	if err != nil {
		t.Fatalf("failed when getting priv key: %s", err)
	}


	time.Sleep(time.Second)

	topic1 := []byte("ciao")
	topic2 := []byte("isoidsodisodiosdiosdiosdi")
	topics := make([]whisper.Topic, 0)
	topics = append(topics, whisper.ConvertBytesToTopic(topic1))
	topics = append(topics, whisper.ConvertBytesToTopic(topic2))
	crit := whisper.FilterOptions{
		PrivateKeyID: pairKeyID1,
		MinPow: 	0.2,
		Topics:   topics,
	}

	filterHash, err := w1.NewMessageFilter(crit)
	if err != nil {
		t.Fatalf("failed when creating new filter: %s", err)
	}

	// let the status update propagate
	time.Sleep(time.Second * time.Duration(2))

	topic := []byte("ciao")
	text := "ciao andrea"
	newMessage := whisper.NewMessage{
		PublicKey: pubKey,
		TTL:      60,
		Topic:    whisper.ConvertBytesToTopic(topic),
		Payload:  []byte(text),
		PowTime:  2,
	}

	_, err = w2.NewWhisperMessage(newMessage)
	if err != nil {
		t.Fatalf("failed when creating new message: %s", err)
	}

	// let the message propagate
	time.Sleep(time.Second * time.Duration(2))

	mess, err := w1.GetFilterMessages(filterHash)
	if err != nil {
		t.Fatalf("failed retrieving messages: %s", err)
	}

	if len(mess) != 1 {
		t.Fatalf("no message but expected")
	}


	for _, m := range mess {
		if string(m.Payload) != text {
			t.Logf(string(m.Payload))
			t.Fatalf("message is not expected : %s", string(m.Payload))
		}
	}
}
