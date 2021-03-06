package bridge

import (
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
	util.Info.Println("GetAllOffers started")
	for i := 0; i < len(links.Links); i++ {
		select {
		default:
			offer := <-getOffers
			linkBridge := NewLinkBridge(offer.Sender, offer.Receiver, accounts)
			linkBridge.Get = offer.DHTGetJSON
			linkBridge.SessionID = i
			if offer.DHTGetJSON.NullRecord == "true" {
				util.Info.Println("GetAllOffers null offer found for", offer.Sender, offer.Receiver)
				linkBridges.unconnected[linkBridge.LinkID()] = &linkBridge
				continue
			}
			if offer.GetValueSize > MinimumOfferValueLength && offer.Minutes() <= OfferExpireMinutes {
				pc, err := ConnectToIceServices(config)
				if err == nil {
					err = util.DecodeObject(offer.GetValue, &linkBridge.Offer)
					if err != nil {
						util.Info.Println("GetAllOffers error with DecodeObject", linkBridge.LinkAccount, linkBridge.LinkID(), err)
						linkBridges.unconnected[linkBridge.LinkID()] = &linkBridge
						continue
					}
					linkBridge.PeerConnection = pc
					linkBridge.State = StateSendAnswer
					util.Info.Println("Offer found for", linkBridge.LinkAccount, linkBridge.LinkID())
					linkBridges.unconnected[linkBridge.LinkID()] = &linkBridge
				} else {
					linkBridges.unconnected[linkBridge.LinkID()] = &linkBridge
				}
			} else if offer.Minutes() > OfferExpireMinutes && offer.GetValueSize > MinimumOfferValueLength {
				util.Info.Println("Stale Offer found for", linkBridge.LinkAccount, linkBridge.LinkID(), "minutes", offer.Minutes())
				linkBridges.unconnected[linkBridge.LinkID()] = &linkBridge
			} else {
				util.Info.Println("Offer NOT found for", linkBridge.LinkAccount, linkBridge.LinkID())
				linkBridges.unconnected[linkBridge.LinkID()] = &linkBridge
			}
		case <-stopchan:
			util.Info.Println("GetAllOffers stopped")
			return false
		}
	}
	return true
}

// GetOffers checks the DHT for WebRTC offers from all links
func GetOffers(stopchan chan struct{}) bool {
	util.Info.Println("GetOffers started")
	l := len(linkBridges.unconnected)
	getOffers := make(chan dynamic.DHTGetReturn, l)
	for _, link := range linkBridges.unconnected {
		if link.State == StateNew || link.State == StateWaitForAnswer {
			var linkBridge = NewLinkBridge(link.LinkAccount, link.MyAccount, accounts)
			dynamicd.GetLinkRecord(linkBridge.LinkAccount, linkBridge.MyAccount, getOffers)
			util.Info.Println("GetOffer for", link.LinkAccount)
		} else {
			util.Info.Println("GetOffers skipped", link.LinkAccount)
			l--
		}
	}
	for i := 0; i < l; i++ {
		select {
		default:
			offer := <-getOffers
			linkBridge := NewLinkBridge(offer.Sender, offer.Receiver, accounts)
			link := linkBridges.unconnected[linkBridge.LinkID()]
			if link.Get.GetValue != offer.GetValue {
				if offer.GetSeq > link.Get.GetSeq {
					link.Get = offer.DHTGetJSON
					if link.Get.NullRecord == "true" {
						util.Info.Println("GetOffers null", offer.Sender, offer.Receiver)
						continue
					}
					if link.Get.Minutes() <= OfferExpireMinutes && link.Get.GetValueSize > MinimumOfferValueLength {
						pc, err := ConnectToIceServices(config)
						if err == nil {
							err = util.DecodeObject(offer.GetValue, &link.Offer)
							if err != nil {
								util.Info.Println("GetOffers error with DecodeObject", link.LinkAccount, link.LinkID(), err)
								continue
							}
							link.PeerConnection = pc
							link.State = StateSendAnswer
							util.Info.Println("GetOffers: Offer found for", link.LinkAccount, link.LinkID())
						}
					} else if link.Get.Minutes() > OfferExpireMinutes && link.Get.GetValueSize > MinimumOfferValueLength {
						util.Info.Println("GetOffers: Stale offer found for", link.LinkAccount, link.LinkID(), "minutes", link.Get.Minutes())
					}
				}
			}
		case <-stopchan:
			util.Info.Println("GetOffers stopped")
			return false
		}
	}
	return true
}

// ClearOffers sets all DHT link records to null
func ClearOffers() {
	util.Info.Println("ClearOffers started")
	l := len(linkBridges.unconnected)
	clearOffers := make(chan dynamic.DHTPutReturn, l)
	for _, link := range linkBridges.unconnected {
		var linkBridge = NewLinkBridge(link.LinkAccount, link.MyAccount, accounts)
		dynamicd.ClearLinkRecord(linkBridge.MyAccount, linkBridge.LinkAccount, clearOffers)
	}
	for i := 0; i < l; i++ {
		offer := <-clearOffers
		util.Info.Println("Offer cleared", offer)
	}
	l = len(linkBridges.connected)
	for _, link := range linkBridges.connected {
		var linkBridge = NewLinkBridge(link.LinkAccount, link.MyAccount, accounts)
		dynamicd.ClearLinkRecord(linkBridge.MyAccount, linkBridge.LinkAccount, clearOffers)
	}
	for i := 0; i < l; i++ {
		offer := <-clearOffers
		util.Info.Println("Offer cleared", offer)
	}
}

// PutOffers saves offers in the DHT for the link
func PutOffers(stopchan chan struct{}) bool {
	util.Info.Println("PutOffers started")
	l := len(linkBridges.unconnected)
	putOffers := make(chan dynamic.DHTPutReturn, l)
	for _, link := range linkBridges.unconnected {
		if link.State == StateNew {
			util.Info.Println("PutOffers for", link.LinkParticipants())
			var linkBridge = NewLinkBridge(link.LinkAccount, link.MyAccount, accounts)
			if link.PeerConnection == nil {
				pc, err := ConnectToIceServices(config)
				if err != nil {
					util.Error.Println("PutOffers error connecting tot ICE services", err)
					continue
				} else {
					link.PeerConnection = pc
					dataChannel, err := link.PeerConnection.CreateDataChannel(link.LinkParticipants(), nil)
					if err != nil {
						util.Error.Println("PutOffers error creating dataChannel for", link.LinkAccount, link.LinkID())
						continue
					}
					link.DataChannel = dataChannel
				}
			}
			link.Offer, _ = link.PeerConnection.CreateOffer(nil)
			encoded, err := util.EncodeObject(link.Offer)
			if err != nil {
				util.Error.Println("PutOffers error EncodeObject", err)
			}
			dynamicd.PutLinkRecord(linkBridge.MyAccount, linkBridge.LinkAccount, encoded, putOffers)
			link.State = StateWaitForAnswer
		} else {
			l--
		}
	}
	for i := 0; i < l; i++ {
		select {
		default:
			offer := <-putOffers
			linkBridge := NewLinkBridge(offer.Sender, offer.Receiver, accounts)
			link := linkBridges.unconnected[linkBridge.LinkID()]
			link.Put = offer.DHTPutJSON
			util.Info.Println("PutOffers Offer saved", offer)
		case <-stopchan:
			util.Info.Println("PutOffers stopped")
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
		if link.State == StateInit {
			util.Info.Println("DisconnectedLinks for", link.LinkParticipants(), link.LinkID())
			var linkBridge = NewLinkBridge(link.LinkAccount, link.MyAccount, accounts)
			linkBridge.SessionID = link.SessionID
			linkBridge.Get = link.Get
			pc, err := ConnectToIceServices(config)
			if err != nil {
				util.Error.Println("DisconnectedLinks error connecting tot ICE services", err)
				continue
			} else {
				linkBridge.PeerConnection = pc
				dataChannel, err := linkBridge.PeerConnection.CreateDataChannel(link.LinkParticipants(), nil)
				if err != nil {
					util.Error.Println("DisconnectedLinks error creating dataChannel for", link.LinkAccount, link.LinkID())
					continue
				}
				linkBridge.DataChannel = dataChannel
			}
			linkBridge.Offer, _ = linkBridge.PeerConnection.CreateOffer(nil)
			linkBridge.Answer = link.Answer
			encoded, err := util.EncodeObject(linkBridge.Offer)
			if err != nil {
				util.Info.Println("DisconnectedLinks error EncodeObject", err)
			}
			dynamicd.PutLinkRecord(linkBridge.MyAccount, linkBridge.LinkAccount, encoded, putOffers)
			linkBridge.State = StateWaitForAnswer
			linkBridges.unconnected[linkBridge.LinkID()] = &linkBridge
		} else {
			l--
		}
	}
	for i := 0; i < l; i++ {
		select {
		default:
			offer := <-putOffers
			linkBridge := NewLinkBridge(offer.Sender, offer.Receiver, accounts)
			link := linkBridges.unconnected[linkBridge.LinkID()]
			link.Put = offer.DHTPutJSON
			util.Info.Println("DisconnectedLinks Offer saved", offer)
		case <-stopchan:
			util.Info.Println("DisconnectedLinks stopped")
			return false
		}
	}
	return true
}
