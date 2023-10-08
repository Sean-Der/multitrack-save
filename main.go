package main

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/oggwriter"
)

func saveToDisk(i media.Writer, track *webrtc.TrackRemote) {
	defer func() {
		if err := i.Close(); err != nil {
			panic(err)
		}
	}()

	for {
		rtpPacket, _, err := track.ReadRTP()
		if err != nil {
			fmt.Println(err)
			return
		}
		if err := i.WriteRTP(rtpPacket); err != nil {
			fmt.Println(err)
			return
		}
	}
}

func doSignaling(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	if r.Method == "OPTIONS" {
		return
	}

	offer, err := io.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}

	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		panic(err)
	}

	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) { //nolint: revive
		if strings.EqualFold(track.Codec().MimeType, webrtc.MimeTypeOpus) {
			fileName := randString(10) + ".ogg"
			fmt.Println("Got Opus track, saving to disk as " + fileName + " (48 kHz, 2 channels)")

			oggFile, err := oggwriter.New(fileName, 48000, 2)
			if err != nil {
				panic(err)
			}

			saveToDisk(oggFile, track)
		}
	})

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())

		if connectionState == webrtc.ICEConnectionStateFailed {
			peerConnection.Close()
		}
	})

	if err = peerConnection.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: string(offer)}); err != nil {
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	} else if err = peerConnection.SetLocalDescription(answer); err != nil {
		fmt.Println(answer.SDP)
		panic(err)
	}

	<-gatherComplete

	w.Header().Add("Location", "/")
	w.WriteHeader(http.StatusCreated)
	fmt.Fprint(w, peerConnection.LocalDescription().SDP)
}

func main() {
	rand.Seed(time.Now().UnixNano()) //nolint

	http.HandleFunc("/", doSignaling)

	fmt.Println("Running WHIP server at http://localhost:8080")
	// nolint: gosec
	panic(http.ListenAndServe(":8085", nil))
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
