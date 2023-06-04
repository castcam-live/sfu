package main

import (
	"sync"

	"github.com/pion/webrtc/v3"
)

type KeyIDString string
type BroadcastIDString string
type KindString string

// TracksAndConnectionsManager is just a simple object, whose sole purpose is to
// manage tracks, and adding tracks to a peer connection, and nothing more.
type TracksAndConnectionsManager struct {
	lock *sync.RWMutex

	// Peer connections on the receiving end
	receivingPeerConnections Map3D[KeyIDString, BroadcastIDString, KindString, Set[*webrtc.PeerConnection]]

	// Tracks to send to the peer connections.
	tracks Map3D[KeyIDString, BroadcastIDString, KindString, webrtc.TrackLocal]
}

// NewTracksAndConnectionManager creates a new TracksAndConnectionsManager
func NewTracksAndConnectionManager() TracksAndConnectionsManager {
	return TracksAndConnectionsManager{
		lock:                     &sync.RWMutex{},
		receivingPeerConnections: Map3D[KeyIDString, BroadcastIDString, KindString, Set[*webrtc.PeerConnection]]{},
		tracks:                   Map3D[KeyIDString, BroadcastIDString, KindString, webrtc.TrackLocal]{},
	}
}

// NOT THREAD SAFE!
func setTrackForPeerConnection(pc *webrtc.PeerConnection, track webrtc.TrackLocal) error {
	// Check if a track exists. If it does, then replace it
	for _, t := range pc.GetTransceivers() {
		if t.Sender().Track().Kind() == track.Kind() {
			if t.Sender().Track() == track {
				return nil
			}
			return t.Sender().ReplaceTrack(track)
		}
	}

	// Otherwise, just add the track
	_, err := pc.AddTrack(track)
	return err
}

// NOT THREAD SAFE!
func (t TracksAndConnectionsManager) getTrack(
	keyId KeyIDString,
	broadcastId BroadcastIDString,
	kind KindString,
) (webrtc.TrackLocal, bool) {
	keyIds, ok := t.tracks[keyId]
	if !ok {
		keyIds = map[BroadcastIDString]map[KindString]webrtc.TrackLocal{}
		t.tracks[keyId] = keyIds
	}

	broadcasts, ok := keyIds[broadcastId]
	if !ok {
		broadcasts = map[KindString]webrtc.TrackLocal{}
		keyIds[broadcastId] = broadcasts
	}

	track, ok := broadcasts[kind]
	return track, ok
}

// SetTrack sets a track, and adds them to all the peer connections that are
// listening to the track.
func (t TracksAndConnectionsManager) SetTrack(
	keyId KeyIDString,
	broadcastId BroadcastIDString,
	track webrtc.TrackLocal,
) {
	// We iterate through each of the peer connections,
	t.lock.RLock()
	defer t.lock.RUnlock()

	kind := KindString(track.Kind().String())

	// Some notes:
	//
	// - We shouldn't have to care about how the ingress track (remote track
	//   coming from clients) is writing to the egress track
	// - When a track is set, then iterate through all peer connections associated
	//   with the key representing the key ID, broadcast ID, and kind, and then
	//   set the track to the peer connection.

	t.tracks.Set(keyId, broadcastId, kind, track)

	pcSet, ok := t.receivingPeerConnections.Get(keyId, broadcastId, kind)
	if !ok {
		return
	}

	for pc := range pcSet.Iterate() {
		setTrackForPeerConnection(pc, track)
	}
}

// AddReceivingPeerConnection adds a peer connection to the list of peer connections, and
// adds the track to the peer connection.
func (t TracksAndConnectionsManager) AddReceivingPeerConnection(
	keyId KeyIDString,
	broadcastId BroadcastIDString,
	kind KindString,
	pc *webrtc.PeerConnection,
) {
	t.lock.RLock()
	defer t.lock.RUnlock()

	// So, when we add a peer connection, we get a set, and ensure that the set
	// exists. If it does not, create it. Now with our set, we add the peer
	// but also, add tracks to the peer.

	connections, ok := t.receivingPeerConnections.Get(keyId, broadcastId, kind)
	if !ok {
		connections = Set[*webrtc.PeerConnection]{}
		t.receivingPeerConnections.Set(keyId, broadcastId, kind, connections)
	}
	connections.Add(pc)

	track, ok := t.tracks.Get(keyId, broadcastId, kind)
	if !ok {
		return
	}

	setTrackForPeerConnection(pc, track)
}

// RemoveReceivingPeerConnection removes a peer connection from the list of peers.
func (t TracksAndConnectionsManager) RemoveReceivingPeerConnection(
	keyId KeyIDString,
	broadcastId BroadcastIDString,
	kind KindString,
	pc *webrtc.PeerConnection,
) {
	t.lock.RLock()
	defer t.lock.RUnlock()

	pcSet, ok := t.receivingPeerConnections.Get(keyId, broadcastId, kind)
	if !ok {
		return
	}
	pcSet.Remove(pc)

	// Note: a track exists regardless of if any peer connections are listening
}

// RemoveTrack removes a track from the list of local tracks, but also removes
// it from all receiving peer connections.
func (t TracksAndConnectionsManager) RemoveTrack(
	keyId KeyIDString,
	broadcastId BroadcastIDString,
	kind KindString,
) {
	t.lock.RLock()
	defer t.lock.RUnlock()

	track, trackExists := t.tracks.Get(keyId, broadcastId, kind)
	t.tracks.Remove(keyId, broadcastId, kind)
	pcSet, ok := t.receivingPeerConnections.Get(keyId, broadcastId, kind)
	if !ok {
		return
	}

	if !trackExists {
		return
	}

	for pc := range pcSet {
		for _, sender := range pc.GetSenders() {
			if sender.Track() == track {
				pc.RemoveTrack(sender)
			}
		}
	}
}
