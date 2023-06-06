package main

import "github.com/castcam-live/simple-forwarding-unit/trackpipe"

type TracksSet struct {
	tracks Map3D[KeyIDString, BroadcastIDString, KindString, trackpipe.Track]
}

func (t TracksSet) Set(keyID KeyIDString, broadcastID BroadcastIDString, track trackpipe.Track) {
	if t.tracks == nil {
		t.tracks = Map3D[KeyIDString, BroadcastIDString, KindString, trackpipe.Track]{}
	}
	t.tracks.Set(keyID, broadcastID, KindString(track.Kind()), track)
}

func (t TracksSet) RemoveOfAllKind(keyID KeyIDString, broadcastID BroadcastIDString) {
	if t.tracks == nil {
		return
	}
	t.tracks.RemoveLevel2(keyID, broadcastID)
}
