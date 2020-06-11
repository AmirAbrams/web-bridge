package bridge

import (
	"fmt"

	util "github.com/duality-solutions/web-bridge/internal/utilities"
	"github.com/duality-solutions/web-bridge/rpc/dynamic"
)

// GetAllOffers checks the DHT for WebRTC offers from all links
func GetAllOffers(stopchan chan struct{}, links dynamic.ActiveLinks, accounts []dynamic.Account) bool {
	getOffers := make(chan dynamic.DHTGetReturn, len(links.Links))
	for _, link := range links.Links {
		var linkBridge = NewBridge(link, accounts)
		dynamicd.GetLinkRecord(linkBridge.LinkAccount, linkBridge.MyAccount, getOffers)
	}
	fmt.Println("GetAllOffers started")
	for i := 0; i < len(links.Links); i++ {
		select {
		default:
			offer := <-getOffers
			if offer.GetValueSize > 10 {
				linkBridge := NewLinkBridge(offer.Sender, offer.Receiver, accounts)
				pc, err := ConnectToIceServices(config)
				if err == nil {
					err = util.DecodeObject(offer.GetValue, &linkBridge.Offer)
					if err != nil {
						fmt.Println("GetAllOffers error with DecodeObject", linkBridge.LinkAccount, linkBridge.LinkID(), err)
						continue
					}
					linkBridge.PeerConnection = pc
					linkBridge.State = 3
					fmt.Println("Offer found for", linkBridge.LinkAccount, linkBridge.LinkID())
					linkBridges.unconnected[linkBridge.LinkID()] = &linkBridge
				} else {
					//fmt.Println("Offer found for", offer.Sender, "ConnectToIceServices failed", err)
					linkBridges.unconnected[linkBridge.LinkID()] = &linkBridge
				}
			} else {
				linkBridge := NewLinkBridge(offer.Sender, offer.Receiver, accounts)
				fmt.Println("Offer NOT found for", linkBridge.LinkAccount, linkBridge.LinkID())
				linkBridges.unconnected[linkBridge.LinkID()] = &linkBridge
			}
		case <-stopchan:
			fmt.Println("GetAllOffers stopped")
			return false
		}
	}
	return true
}

// GetOffers checks the DHT for WebRTC offers from all links
func GetOffers(stopchan chan struct{}) bool {
	fmt.Println("GetOffers started")
	l := len(linkBridges.unconnected)
	getOffers := make(chan dynamic.DHTGetReturn, l)
	for _, link := range linkBridges.unconnected {
		if link.State == 1 || link.State == 2 {
			var linkBridge = NewLinkBridge(link.LinkAccount, link.MyAccount, accounts)
			dynamicd.GetLinkRecord(linkBridge.LinkAccount, linkBridge.MyAccount, getOffers)
		} else {
			fmt.Println("GetOffers skipped", link.LinkAccount)
			l--
		}
	}
	for i := 0; i < l; i++ {
		select {
		default:
			offer := <-getOffers
			if len(offer.GetValue) > 10 {
				linkBridge := NewLinkBridge(offer.Sender, offer.Receiver, accounts)
				err := util.DecodeObject(offer.GetValue, &linkBridge.Offer)
				if err != nil {
					fmt.Println("GetOffers Error DecodeObject", linkBridge.LinkAccount, linkBridge.LinkID(), err)
					continue
				}
				link := linkBridges.unconnected[linkBridge.LinkID()]
				if link.Offer != linkBridge.Offer {
					pc, _ := ConnectToIceServices(config)
					linkBridge.PeerConnection = pc
					linkBridge.State = 3
					fmt.Println("GetOffers Offer found for", linkBridge.LinkAccount, linkBridge.LinkID())
					linkBridges.unconnected[linkBridge.LinkID()] = &linkBridge
				}
			}
		case <-stopchan:
			fmt.Println("GetOffers stopped")
			return false
		}
	}
	return true
}

// ClearOffers sets all DHT link records to null
func ClearOffers() {
	fmt.Println("ClearOffers started")
	l := len(linkBridges.unconnected)
	clearOffers := make(chan dynamic.DHTPutReturn, l)
	for _, link := range linkBridges.unconnected {
		var linkBridge = NewLinkBridge(link.LinkAccount, link.MyAccount, accounts)
		dynamicd.ClearLinkRecord(linkBridge.MyAccount, linkBridge.LinkAccount, clearOffers)
	}
	for _, link := range linkBridges.connected {
		var linkBridge = NewLinkBridge(link.LinkAccount, link.MyAccount, accounts)
		dynamicd.ClearLinkRecord(linkBridge.MyAccount, linkBridge.LinkAccount, clearOffers)
	}
	for i := 0; i < l; i++ {
		offer := <-clearOffers
		fmt.Println("Offer cleared", offer)
	}
}

// PutOffers saves offers in the DHT for the link
func PutOffers(stopchan chan struct{}) bool {
	fmt.Println("PutOffers started")
	l := len(linkBridges.unconnected)
	putOffers := make(chan dynamic.DHTPutReturn, l)
	for _, link := range linkBridges.unconnected {
		if link.State == 1 {
			fmt.Println("PutOffers for", link.LinkParticipants())
			var linkBridge = NewLinkBridge(link.LinkAccount, link.MyAccount, accounts)
			if link.PeerConnection == nil {
				pc, err := ConnectToIceServices(config)
				if err != nil {
					fmt.Println("PutOffers error connecting tot ICE services", err)
					continue
				} else {
					link.PeerConnection = pc
					dataChannel, err := link.PeerConnection.CreateDataChannel(link.LinkParticipants(), nil)
					if err != nil {
						fmt.Println("PutOffers error creating dataChannel for", link.LinkAccount, link.LinkID())
						continue
					}
					link.DataChannel = dataChannel
				}
			}
			link.Offer, _ = link.PeerConnection.CreateOffer(nil)
			encoded, err := util.EncodeObject(link.Offer)
			if err != nil {
				fmt.Println("PutOffers error EncodeObject", err)
			}
			dynamicd.PutLinkRecord(linkBridge.MyAccount, linkBridge.LinkAccount, encoded, putOffers)
			link.State = 2
		} else {
			l--
		}
	}
	for i := 0; i < l; i++ {
		select {
		default:
			offer := <-putOffers
			fmt.Println("PutOffers Offer saved", offer)
		case <-stopchan:
			fmt.Println("PutOffers stopped")
			return false
		}
	}
	return true
}

// DisconnectedLinks reinitializes the WebRTC link bridge struct
func DisconnectedLinks(stopchan chan struct{}) bool {
	l := len(linkBridges.unconnected)
	putOffers := make(chan dynamic.DHTPutReturn, l)
	for _, link := range linkBridges.unconnected {
		if link.State == 0 {
			fmt.Println("DisconnectedLinks for", link.LinkParticipants())
			var linkBridge = NewLinkBridge(link.LinkAccount, link.MyAccount, accounts)
			pc, err := ConnectToIceServices(config)
			if err != nil {
				fmt.Println("DisconnectedLinks error connecting tot ICE services", err)
				continue
			} else {
				linkBridge.PeerConnection = pc
				dataChannel, err := linkBridge.PeerConnection.CreateDataChannel(link.LinkParticipants(), nil)
				if err != nil {
					fmt.Println("DisconnectedLinks error creating dataChannel for", link.LinkAccount, link.LinkID())
					continue
				}
				linkBridge.DataChannel = dataChannel
			}
			linkBridge.Offer, _ = linkBridge.PeerConnection.CreateOffer(nil)
			linkBridge.Answer = link.Answer
			encoded, err := util.EncodeObject(linkBridge.Offer)
			if err != nil {
				fmt.Println("DisconnectedLinks error EncodeObject", err)
			}
			dynamicd.PutLinkRecord(linkBridge.MyAccount, linkBridge.LinkAccount, encoded, putOffers)
			linkBridge.State = 2
			linkBridges.unconnected[linkBridge.LinkID()] = &linkBridge
		} else {
			l--
		}
	}
	for i := 0; i < l; i++ {
		select {
		default:
			offer := <-putOffers
			fmt.Println("DisconnectedLinks Offer saved", offer)
		case <-stopchan:
			fmt.Println("DisconnectedLinks stopped")
			return false
		}
	}
	return true
}
