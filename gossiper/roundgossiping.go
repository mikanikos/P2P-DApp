package gossiper

import (
	"encoding/hex"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mikanikos/Peerster/helpers"
)

func (gossiper *Gossiper) processClientBlocks() {

	for block := range gossiper.clientBlockBuffer {

		id := atomic.LoadUint32(&gossiper.seqID)
		atomic.AddUint32(&gossiper.seqID, uint32(1))

		tlcPacket := &TLCMessage{Origin: gossiper.name, ID: id, TxBlock: block, VectorClock: gossiper.getStatusToSend().Status}
		extPacket := &ExtendedGossipPacket{Packet: &GossipPacket{TLCMessage: tlcPacket}, SenderAddr: gossiper.gossiperData.Addr}

		gossiper.storeMessage(extPacket.Packet, gossiper.name, id)

		value, _ := gossiper.tlcChannels.LoadOrStore(id, make(chan *TLCAck))
		tlcChan := value.(chan *TLCAck)

		witnsesses := []string{gossiper.name}

		if hw3ex2Mode {
			printTLCMessage(extPacket.Packet.TLCMessage)
		}
		gossiper.startRumorMongering(extPacket, gossiper.name, id)

		if stubbornTimeout > 0 {
			timer := time.NewTicker(time.Duration(stubbornTimeout) * time.Second)
			for {
				select {
				case tlcAck := <-tlcChan:
					witnsesses = append(witnsesses, tlcAck.Origin)
					witnsesses = helpers.RemoveDuplicatesFromSlice(witnsesses)
					if uint64(len(witnsesses)) > gossiper.peersNumber/2 {
						timer.Stop()
						gossiper.tlcChannels.Delete(id)
						tlcPacket.Confirmed = true
						if hw3ex2Mode {
							fmt.Println("RE-BROADCAST ID " + fmt.Sprint(id) + " WITNESSES " + strings.Join(witnsesses, ","))
						}
						go gossiper.startRumorMongering(extPacket, gossiper.name, id)
						go func(b BlockPublish) {
							gossiper.uiHandler.filesIndexed <- &FileGUI{Name: b.Transaction.Name, MetaHash: hex.EncodeToString(b.Transaction.MetafileHash), Size: b.Transaction.Size}
						}(block)
						return
					}

				case <-timer.C:
					if hw3ex2Mode {
						printTLCMessage(extPacket.Packet.TLCMessage)
					}
					go gossiper.startRumorMongering(extPacket, gossiper.name, id)
				}
			}
		}
	}
}
