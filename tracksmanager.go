package main

import (
	"sync"

	"github.com/pion/webrtc/v3"
)

type KeyIDString string
type BroadcastIDString string
type KindString string

// TracksAndConnectionsManager is just a simple whose sole purpose is to manage tracks, and
// adding tracks to a peer connection, and nothing more.
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
	// Add our new track
	if _, err := pc.AddTrack(track); err != nil {
		panic(err)
	}

	// Check if a track exists. If it does, then replace it
	for _, t := range pc.GetTransceivers() {
		if t.Sender().Track().Kind() == track.Kind() {
			if t.Sender().Track() == track {
				return nil
			}
			t.Sender().ReplaceTrack(track)
			return nil
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
	kind KindString,
	track *webrtc.TrackLocalStaticRTP,
) {
	// We iterate through each of the peer connections,
	t.lock.RLock()
	defer t.lock.RUnlock()

	t.tracks.Set(keyId, broadcastId, kind, track)

	for _, peerConnections := range t.receivingPeerConnections[keyId][broadcastId] {
		for pc := range peerConnections.Iterate() {
			setTrackForPeerConnection(pc, track)
		}
	}
}

// AddPeerConnection adds a peer connection to the list of peer connections, and
// adds the track to the peer connection.
func (t TracksAndConnectionsManager) AddPeerConnection(
	keyId KeyIDString,
	broadcastId BroadcastIDString,
	kind KindString,
	pc *webrtc.PeerConnection,
) {
	t.lock.RLock()
	defer t.lock.RUnlock()

	connections, ok := t.receivingPeerConnections.Get(keyId, broadcastId, kind)
	if !ok {
		connections = Set[*webrtc.PeerConnection]{}
		t.receivingPeerConnections.Set(keyId, broadcastId, kind, connections)
	}
	connections.Add(pc)
}

// RemovePeerConnection removes a peer connection from the list of peers.
func (t TracksAndConnectionsManager) RemovePeerConnection(
	keyId KeyIDString,
	broadcastId BroadcastIDString,
	kind KindString,
	pc *webrtc.PeerConnection,
) {
	t.lock.RLock()
	defer t.lock.RUnlock()

	t.receivingPeerConnections.Remove(keyId, broadcastId, kind)

}

func (t TracksAndConnectionsManager) RemoveTrack(
	keyId KeyIDString,
	broadcastId BroadcastIDString,
	kind KindString,
) {
	t.lock.RLock()
	defer t.lock.RUnlock()

}
