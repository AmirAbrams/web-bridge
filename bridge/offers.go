package bridge

import (
	"fmt"
	"strings"

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
			if offer.GetValueSize > 0 {
				linkBridge := NewLinkBridge(offer.Sender, offer.Receiver, accounts)
				pc, err := ConnectToIceServices(config)
				if err == nil && offer.GetValue != "null" {
					//fmt.Println("Offer found for", offer.Sender)
					linkBridge.Offer = strings.ReplaceAll(offer.GetValue, `""`, "")
					linkBridge.PeerConnection = pc
					linkBridges.connected = append(linkBridges.connected, linkBridge)
				} else {
					//fmt.Println("Offer found for", offer.Sender, "ConnectToIceServices failed", err)
					linkBridges.unconnected = append(linkBridges.unconnected, linkBridge)
				}
			} else {
				linkBridge := NewLinkBridge(offer.Sender, offer.Receiver, accounts)
				pc, _ := ConnectToIceServices(config)
				linkBridge.PeerConnection = pc
				linkBridges.unconnected = append(linkBridges.unconnected, linkBridge)
			}
		case <-stopchan:
			fmt.Println("GetAllOffers stopped")
			return false
		}
	}
	return true
}

// ClearOffers sets all DHT link records to null
func ClearOffers(bridges *[]Bridge) {
	fmt.Println("ClearOffers started")
	l := len(*bridges)
	clearOffers := make(chan dynamic.DHTPutReturn, l)
	for _, link := range *bridges {
		var linkBridge = NewLinkBridge(link.LinkAccount, link.MyAccount, accounts)
		dynamicd.ClearLinkRecord(linkBridge.MyAccount, linkBridge.LinkAccount, clearOffers)
	}
	for i := 0; i < l; i++ {
		offer := <-clearOffers
		fmt.Println("Offer cleared", offer)
	}
}

// PutOffers saves offers in the DHT for the link
func PutOffers(stopchan chan struct{}, bridges *[]Bridge) bool {
	fmt.Println("PutOffers started")
	l := len(*bridges)
	putOffers := make(chan dynamic.DHTPutReturn, l)
	for _, link := range *bridges {
		var linkBridge = NewLinkBridge(link.LinkAccount, link.MyAccount, accounts)
		if link.PeerConnection == nil {
			pc, err := ConnectToIceServices(config)
			if err != nil {
				fmt.Println("PutOffers error connecting tot ICE services", err)
				continue
			} else {
				link.PeerConnection = pc
			}
		}
		offer, _ := link.PeerConnection.CreateOffer(nil)
		dynamicd.PutLinkRecord(linkBridge.MyAccount, linkBridge.LinkAccount, offer.SDP, putOffers)
		link.Offer = offer.SDP
	}
	for i := 0; i < l; i++ {
		select {
		default:
			offer := <-putOffers
			fmt.Println("Offer saved", offer)
		case <-stopchan:
			fmt.Println("PutOffers stopped")
			return false
		}
	}
	return true
}
