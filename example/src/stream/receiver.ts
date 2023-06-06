import { SignallingMessage } from "./schema";
import { Subject, EventEmitter, createSubject } from "./subject";
import { InferType, any, either, exact, object, string } from "./validator";

type States = "CONNECTING" | "CONNECTED" | "DISCONNECTED";

/**
 * WsSession is a simple wrapper around a WebSocket that automatically
 * reconnects
 */
class WsSession {
	private closed: boolean = false;
	private ws: WebSocket | null = null;
	private messageBuffer: string[] = [];
	private _messageEvents: Subject<MessageEvent<unknown>> = createSubject();
	private _stateEvents: Subject<States> = createSubject();

	/**
	 * Creates a new WsSession. This differs from a WebSocket in that it will
	 * automatically reconnect if the connection is closed.
	 * @param address The address to connect to
	 */
	constructor(private address: string) {
		this.connect();
	}

	private connect() {
		if (this.closed) return;

		setTimeout(() => {
			this._stateEvents.emit("CONNECTING");
		});
		this.ws = new WebSocket(this.address);
		this.ws.addEventListener("close", () => {
			setTimeout(() => {
				this._stateEvents.emit("DISCONNECTED");
			});
			this.connect();
		});

		this.ws.addEventListener("open", () => {
			if (!this.ws) return;
			setTimeout(() => {
				this._stateEvents.emit("CONNECTED");
			});
			for (const m of this.messageBuffer) {
				this.ws.send(m);
			}
		});

		this.ws.addEventListener("message", (event) => {
			this._messageEvents.emit(event);
		});
	}

	send(data: string) {
		if (this.ws === null || this.ws.readyState !== WebSocket.OPEN) {
			this.messageBuffer.push(data);
		} else {
			this.ws.send(data);
		}
	}

	/**
	 * Closes the WebSocket connection.
	 */
	close() {
		this.closed = true;
		this.ws?.close();
	}

	/**
	 * An event emitter that emits messages received from the server
	 */
	get messageEvents(): EventEmitter<MessageEvent> {
		return { addEventListener: this._messageEvents.addEventListener };
	}

	get stateEvents(): EventEmitter<States> {
		return { addEventListener: this._stateEvents.addEventListener };
	}
}

const offerSchema = object({
	type: exact("offer"),
	sdp: string(),
});

const descriptionSchema = object({
	type: exact("DESCRIPTION"),
	data: offerSchema,
});

const candidateSchema = object({
	type: exact("CANDIDATE"),
	data: any(),
});

const messageSchema = either(
	object({
		type: exact("SIGNALLING"),
		data: either(descriptionSchema, candidateSchema),
	})
);

type ReceiverParams = {
	keyId: string;
	streamId: string;
	kind: string;
};

export class Receiver {
	private _mediaStream: MediaStream = new MediaStream();
	private peerConnection: RTCPeerConnection | null = null;

	// private keyId: string;
	// private streamId: string;
	private kind: string;

	private ws: WsSession;

	constructor(
		address: string,
		{ keyId, streamId, kind }: ReceiverParams,
		private rtcPCConfig?: RTCConfiguration
	) {
		// this.keyId = keyId;
		// this.streamId = streamId;
		this.kind = kind;

		// Create the URL for the websocket session
		const url = new URL(address);
		url.searchParams.set("keyId", keyId);
		url.searchParams.set("streamId", streamId);
		url.searchParams.set("kind", kind);

		this.ws = new WsSession(url.toString());

		this.ws.messageEvents.addEventListener((event) => {
			const message = JSON.parse(event.data);

			const validation = messageSchema.validate(message);
			if (!validation.isValid) {
				return;
			}

			if (validation.value.type === "SIGNALLING") {
				if (validation.value.data.type === "DESCRIPTION") {
					if (validation.value.data.data.type === "offer") {
						this.handleOffer(validation.value.data.data);
					} else {
						this.closePeerConnection();
					}
				} else if (validation.value.data.type === "CANDIDATE") {
					this.handleCandidate(validation.value.data.data);
				}
			}

			this.ws.stateEvents.addEventListener((state) => {
				if (state === "DISCONNECTED") {
					this.closePeerConnection();
				}
			});
		});
	}

	private getPeerConnection() {
		if (this.peerConnection === null) {
			this.peerConnection = new RTCPeerConnection(this.rtcPCConfig);
			this.peerConnection.addEventListener("track", (event) => {
				if (event.track.kind === this.kind) {
					const track = this._mediaStream
						.getTracks()
						.find((track) => track.kind === this.kind);
					if (track) {
						this._mediaStream.removeTrack(track);
					}
					this._mediaStream.addTrack(event.track);
				}
			});
			this.peerConnection.addEventListener("icecandidate", (event) => {
				if (event.candidate) {
					this.ws.send(
						JSON.stringify({
							type: "SIGNALLING",
							data: {
								type: "ICE_CANDIDATE",
								data: event.candidate,
							},
						} as SignallingMessage)
					);
				}
			});
		}

		return this.peerConnection;
	}

	private closePeerConnection() {
		this.peerConnection?.close();
		this.peerConnection = null;
	}

	private async handleOffer(offer: RTCSessionDescriptionInit) {
		const pc = this.getPeerConnection();
		await pc.setRemoteDescription(offer);
		const answer = await pc.createAnswer();
		await pc.setLocalDescription(answer);
		this.ws.send(
			JSON.stringify({
				type: "SIGNALLING",
				data: {
					type: "DESCRIPTION",
					data: answer,
				},
			} as SignallingMessage)
		);
	}

	private async handleCandidate(candidate: RTCIceCandidate) {
		const pc = this.getPeerConnection();
		await pc.addIceCandidate(candidate);
	}

	get mediaStream() {
		return this._mediaStream;
	}
}
