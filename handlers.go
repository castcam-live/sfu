package main

import (
	"encoding/json"
	"log"
	"net/http"

	wskeyauth "github.com/castcam-live/ws-key-auth/go"
	"github.com/clubcabana/simple-forwarding-unit/finish"
	"github.com/clubcabana/simple-forwarding-unit/trackpipe"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/intervalpli"
	"github.com/pion/webrtc/v3"
)

// Things to test for:
//
// Sender connects to broadcast, sends a track
// Sender disconnects from WebSocket connection, but maintains PeerConnection
// Sender disconnects from PeerConnection, but maintains WebSocket connection
// Sender removes track from PeerConnection
// Sender replaces track in PeerConnection

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// allow all connections by default
		return true
	},
}

var peerConnectionConfig = webrtc.Configuration{
	ICEServers: []webrtc.ICEServer{
		{
			// TODO: soft code this
			URLs: []string{"stun:stun.l.google.com:19302"},
		},
	},
}

func CreateHandlers() http.Handler {
	router := mux.NewRouter()

	tracks := TracksSet{}
	tracksAndConnections := NewTracksAndConnectionManager()

	// Have clients request for "key ID", "kind" (either "audio" or "video"),
	// and "id" via query parameters, rather than URLs. Don't standardize things
	// too much. Let the implementers of WebRTC decide what the URL paths should
	// look like, and have clients just query those parts.

	router.HandleFunc("/broadcast/{id}", func(res http.ResponseWriter, req *http.Request) {
		// For broadcasting, we just need to be given the ID. Key ID is implied
		// during authentication, and the "kind" is implied when a track is added.
		//
		// Yes, a broadcast can accept multiple tracks, but only of different kinds.
		// Tracks of the same kind will be overriden.
		//
		// Receivers will need to create separate peer connection for each track
		// that they need.

		params := mux.Vars(req)
		id, ok := params["id"]
		if !ok {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		conn, err := upgrader.Upgrade(res, req, nil)
		if err != nil {
			log.Println(err)
			return
		}
		defer conn.Close()

		// First authenticate
		authenticated, keyID, err := wskeyauth.Handshake(conn)
		if err != nil {
			log.Println(err)
			return
		}

		if !authenticated {
			conn.WriteJSON(map[string]any{
				"type": "UNKNOWN_ERROR",
				"data": map[string]any{
					// TODO: provide more details here
					"type": "AUTHENTICATION_FAILED",
				},
			})
		}

		m := &webrtc.MediaEngine{}
		if err := m.RegisterDefaultCodecs(); err != nil {
			conn.WriteJSON(map[string]any{
				"type": "SERVER_ERROR",
				"data": map[string]any{
					"type": "CODEC_REGISTRATION_FAILED",
					// TODO: add more details
				},
			})
			return
		}

		i := &interceptor.Registry{}

		// Use the default set of Interceptors
		if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
			conn.WriteJSON(map[string]any{
				"type": "SERVER_ERROR",
				"data": map[string]any{
					"type": "INTERCEPTOR_REGISTRATION_FAILED",
				},
			})
			return
		}

		// TODO: Right now, we are sending a PLI every 3 seconds.
		//
		//   But, ideally, we should let the receiver decide how often they want to
		//   send PLIs.
		//
		//   Fix this ASAP.
		intervalPliFactory, err := intervalpli.NewReceiverInterceptor()
		if err != nil {
			conn.WriteJSON(map[string]any{
				"type": "SERVER_ERROR",
				"data": map[string]any{
					"type": "INTERCEPTOR_CREATION_FAILED",
				},
			})
			return
		}
		i.Add(intervalPliFactory)

		peerConnection, err := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i)).NewPeerConnection(peerConnectionConfig)
		if err != nil {
			conn.WriteJSON(map[string]any{
				"type": "SERVER_ERROR",
				"data": map[string]any{
					"type": "PEER_CONNECTION_CREATION_FAILED",
				},
			})
		}

		defer func() {
			if cErr := peerConnection.Close(); cErr != nil {
				log.Printf("cannot close peerConnection: %v\n", cErr)
			}
		}()

		if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
			conn.WriteJSON(map[string]any{
				"type": "SERVER_ERROR",
				"data": map[string]any{
					"type": "TRANSCIEVER_CREATION_FAILED",
				},
			})
			return
		}

		peerConnection.OnTrack(func(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
			localTrack, newTrackErr := webrtc.NewTrackLocalStaticRTP(remoteTrack.Codec().RTPCodecCapability, remoteTrack.Kind().String(), "pion")
			if newTrackErr != nil {
				log.Printf("cannot create new track: %v\n", newTrackErr)
				return
			}

			tracks.Set(
				KeyIDString(keyID),
				BroadcastIDString(id),
				trackpipe.New(remoteTrack, localTrack),
			)

			tracksAndConnections.SetTrack(
				KeyIDString(keyID),
				BroadcastIDString(id),
				localTrack,
			)
		})

		defer tracks.RemoveOfAllKind(KeyIDString(keyID), BroadcastIDString(id))

		done := finish.Done{}

		peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
			if s == webrtc.PeerConnectionStateClosed {
				done.Finish()
			}
		})

		peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
			if c == nil {
				return
			}

			if err := conn.WriteJSON(map[string]any{
				"type": "SIGNALLING",
				"data": map[string]any{
					"type": "ICE_CANDIDATE",
					"data": c,
				},
			}); err != nil {
				done.Finish()
				return
			}
		})

		for {
			if done.IsDone() {
				return
			}
			_, b, err := conn.ReadMessage()
			if err != nil {
				log.Printf("Reading message from client failed. I guess the client is closed? %s", err.Error())
				return
			}

			type TD struct {
				Type string          `json:"type"`
				Data json.RawMessage `json:"data"`
			}

			var t TD
			if err = json.Unmarshal(b, &t); err != nil {
				continue
			}

			switch t.Type {
			// We will be the one receiving offers, and responding with answers
			case "SIGNALLING":
				type Signalling struct {
					Type string          `json:"type"`
					Data json.RawMessage `json:"data"`
				}

				var s Signalling
				if err = json.Unmarshal(t.Data, &s); err != nil {
					continue
				}

				switch s.Type {
				case "ess":
					type Description struct {
						Type string `json:"type"`
						SDP  string `json:"sdp"`
					}
					var d webrtc.SessionDescription
					if err = json.Unmarshal(s.Data, &d); err != nil {
						log.Printf("Bad JSON message? %s", err.Error())
						continue
					}

					if d.Type == webrtc.SDPTypeAnswer {
						conn.WriteJSON(map[string]any{
							"type": "CLIENT_ERROR",
							"data": map[string]any{
								"type": "ANSWER_RECEIVED",
								"msg":  "Received answer from client; server can't accept answers; only offers",
							},
						})
						return
					}

					if err = peerConnection.SetRemoteDescription(d); err != nil {
						log.Printf("Failed to set remote description: %s", err.Error())
						conn.WriteJSON(map[string]any{
							"type": "SERVER_ERROR",
							"data": map[string]any{
								"type": "SET_REMOTE_DESCRIPTION_FAILED",
							},
						})
						return
					}

					answer, err := peerConnection.CreateAnswer(nil)
					if err != nil {
						log.Printf("Failed to create answer: %s", err.Error())
						conn.WriteJSON(map[string]any{
							"type": "SERVER_ERROR",
							"data": map[string]any{
								"type": "CREATE_ANSWER_FAILED",
							},
						})
						continue
					}
					if err = peerConnection.SetLocalDescription(answer); err != nil {
						log.Printf("Failed to set local description: %s", err.Error())
						conn.WriteJSON(map[string]any{
							"type": "SERVER_ERROR",
							"data": map[string]any{
								"type": "SET_LOCAL_DESCRIPTION_FAILED",
							},
						})
						continue
					}

					if err = conn.WriteJSON(map[string]any{
						"type": "SIGNALLING",
						"data": map[string]any{
							"type": "DESCRIPTION",
							"data": answer,
						},
					}); err != nil {
						return
					}
				case "ICE_CANDIDATE":
					var iceCandiate webrtc.ICECandidate
					if err = json.Unmarshal(s.Data, &iceCandiate); err != nil {
						log.Printf("Bad JSON message? %s", err.Error())
						continue
					}
					if err = peerConnection.AddICECandidate(iceCandiate.ToJSON()); err != nil {
						log.Printf("Failed to add ICE candidate: %s", err.Error())
						continue
					}
				}
			}
		}
	})

	router.HandleFunc("/get", func(res http.ResponseWriter, req *http.Request) {
		// Unlike `broadcast`, we don't need to authenticate.
		//
		// That said, if a local track keyed by the key ID, kind, and id does not
		// exist, create it.
		//
		// This means we will need a helper function to get the LocalTrack from
		// some store

		// Several steps:
		//
		// 1. Get the key ID, kind, and id from the query parameters
		// 2. Create a peer connection
		// 3. Add the peer connection to the list of peer connections
		// 4. Add the track to the peer connection
		// 5. Add listener for negotation needed events
		//    a. Create an offer
		//    b. Set the local description
		//    c. Send the offer to the client
		// 6. Add listener for ICE candidates
		//    a. Send the ICE candidate to the client
		// 8. Loop for messages from the client
		//    a. If the message is an answer, set the remote description
		//    b/ If the message is an ICE candidate, add the ICE candidate

		queryParams := ParseQuery(req.URL.RawQuery)

		keyID, ok := queryParams["keyid"]
		if !ok {
			res.WriteHeader(http.StatusBadRequest)
			res.Write([]byte("Missing key ID"))
			return
		}

		id, ok := queryParams["id"]
		if !ok {
			res.WriteHeader(http.StatusBadRequest)
			res.Write([]byte("Missing ID"))
			return
		}

		kind, ok := queryParams["kind"]
		if !ok {
			res.WriteHeader(http.StatusBadRequest)
			res.Write([]byte("Missing kind"))
			return
		}

		conn, err := upgrader.Upgrade(res, req, nil)
		if err != nil {
			log.Println(err)
			return
		}
		defer conn.Close()

		m := &webrtc.MediaEngine{}
		if err := m.RegisterDefaultCodecs(); err != nil {
			conn.WriteJSON(map[string]any{
				"type": "SERVER_ERROR",
				"data": map[string]any{
					"type": "CODEC_REGISTRATION_FAILED",
					// TODO: add more details
				},
			})
			return
		}

		i := &interceptor.Registry{}

		// Use the default set of Interceptors
		if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
			conn.WriteJSON(map[string]any{
				"type": "SERVER_ERROR",
				"data": map[string]any{
					"type": "INTERCEPTOR_REGISTRATION_FAILED",
				},
			})
			return
		}

		intervalPliFactory, err := intervalpli.NewReceiverInterceptor()
		if err != nil {
			conn.WriteJSON(map[string]any{
				"type": "SERVER_ERROR",
				"data": map[string]any{
					"type": "INTERCEPTOR_CREATION_FAILED",
				},
			})
			return
		}
		i.Add(intervalPliFactory)

		peerConnection, err := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i)).NewPeerConnection(peerConnectionConfig)
		if err != nil {
			conn.WriteJSON(map[string]any{
				"type": "SERVER_ERROR",
				"data": map[string]any{
					"type": "PEER_CONNECTION_CREATION_FAILED",
				},
			})
		}

		done := finish.Done{}

		peerConnection.OnNegotiationNeeded(func() {
			// TODO: I guess we will need another one of those channels to detect if
			//   the connection failed. In this callback, we will be signalling that
			//   the offer failed
			offer, err := peerConnection.CreateOffer(nil)

			if err != nil {
				conn.WriteJSON(map[string]any{
					"type": "SERVER_ERROR",
					"data": map[string]any{
						"type": "CREATE_OFFER_FAILED",
					},
				})
				done.Finish()
				return
			}

			if err = peerConnection.SetLocalDescription(offer); err != nil {
				done.Finish()
				return
			}

			if err = conn.WriteJSON(map[string]any{
				"type": "SIGNALLING",
				"data": map[string]any{
					"type": "DESCRIPTION",
					"data": offer,
				},
			}); err != nil {
				done.Finish()
				return
			}

		})

		peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
			if c == nil {
				return
			}

			if err = conn.WriteJSON(map[string]any{
				"type": "SIGNALLING",
				"data": map[string]any{
					"type": "ICE_CANDIDATE",
					"data": c,
				},
			}); err != nil {
				done.Finish()
				return
			}
		})

		defer func() {
			if cErr := peerConnection.Close(); cErr != nil {
				log.Printf("cannot close peerConnection: %v\n", cErr)
			}
		}()

		tracksAndConnections.AddPeerConnection(KeyIDString(keyID), BroadcastIDString(id), KindString(kind), peerConnection)
		defer tracksAndConnections.RemovePeerConnection(KeyIDString(keyID), BroadcastIDString(id), KindString(kind), peerConnection)

		for {
			if done.IsDone() {
				return
			}

			if done.IsDone() {
				return
			}
			_, b, err := conn.ReadMessage()
			if err != nil {
				log.Printf("Reading message from client failed. I guess the client is closed? %s", err.Error())
				return
			}

			type TD struct {
				Type string          `json:"type"`
				Data json.RawMessage `json:"data"`
			}

			var t TD
			if err = json.Unmarshal(b, &t); err != nil {
				continue
			}

			switch t.Type {
			// We will be the one receiving offers, and responding with answers
			case "SIGNALLING":
				type Signalling struct {
					Type string          `json:"type"`
					Data json.RawMessage `json:"data"`
				}

				var s Signalling
				if err = json.Unmarshal(t.Data, &s); err != nil {
					continue
				}

				switch s.Type {
				case "DESCRIPTION":
					type Description struct {
						Type string `json:"type"`
						SDP  string `json:"sdp"`
					}
					var d webrtc.SessionDescription
					if err = json.Unmarshal(s.Data, &d); err != nil {
						log.Printf("Bad JSON message? %s", err.Error())
						continue
					}

					if d.Type == webrtc.SDPTypeOffer {
						conn.WriteJSON(map[string]any{
							"type": "CLIENT_ERROR",
							"data": map[string]any{
								"type": "OFFER_RECEIVED",
								"msg":  "Received offer from client; server can't accept offers; only answers",
							},
						})
						return
					}

					if err = peerConnection.SetRemoteDescription(d); err != nil {
						log.Printf("Failed to set remote description: %s", err.Error())
						conn.WriteJSON(map[string]any{
							"type": "SERVER_ERROR",
							"data": map[string]any{
								"type": "SET_REMOTE_DESCRIPTION_FAILED",
							},
						})
						return
					}
				case "ICE_CANDIDATE":
					var iceCandiate webrtc.ICECandidate
					if err = json.Unmarshal(s.Data, &iceCandiate); err != nil {
						log.Printf("Bad JSON message? %s", err.Error())
						continue
					}
					if err = peerConnection.AddICECandidate(iceCandiate.ToJSON()); err != nil {
						log.Printf("Failed to add ICE candidate: %s", err.Error())
						continue
					}
				}
			}
		}
	})

	return router
}
