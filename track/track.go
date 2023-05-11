package track

import (
	"errors"
	"io"
	"log"

	"github.com/clubcabana/simple-forwarding-unit/finish"
	"github.com/pion/webrtc/v3"
)

type Track struct {
	done        *finish.Done
	remoteTrack *webrtc.TrackRemote
	localTrack  webrtc.TrackLocal
}

// New creates a new Track, but also, during the construction, starts a
// goroutine that will be reading incoming bytes from the remote track, and
// then writing it to the local track
func New(
	remoteTrack *webrtc.TrackRemote,
	localTrack *webrtc.TrackLocalStaticRTP,
) Track {
	done := finish.NewDone()

	go func() {
		rtpBuf := make([]byte, 1400)
		for !done.IsDone() {
			i, _, readErr := remoteTrack.Read(rtpBuf)
			if readErr != nil {
				log.Printf("")
				return
			}

			// ErrClosedPipe means we don't have any subscribers, this is ok if no
			// peers have connected yet
			if _, err := localTrack.Write(rtpBuf[:i]); err != nil && !errors.Is(err, io.ErrClosedPipe) {
				done.Finish()
			}
		}
	}()

	return Track{&done, remoteTrack, localTrack}
}

// Stop stops the goroutine that is reading from the remote track and writing
func (t Track) Stop() {
	t.done.Finish()
}
