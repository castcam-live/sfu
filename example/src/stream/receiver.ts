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
	private _messageEvents: Subject<MessageEvent> = createSubject();

	constructor(private address: string) {
		this.connect();
	}

	private connect() {
		if (this.closed) return;

		this.ws = new WebSocket(this.address);
		this.ws.addEventListener("close", () => {
			this.connect();
		});

		this.ws.addEventListener("open", () => {
			for (const m of this.messageBuffer) {
				this.ws!.send(m);
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

type Message = InferType<typeof messageSchema>;

type ReceiverParams = {
	keyId: string;
	streamId: string;
	kind: string;
};

export class Receiver {
	private _mediaStream: MediaStream = new MediaStream();
	private peerConnection: RTCPeerConnection | null = null;

	private keyId: string;
	private streamId: string;
	private kind: string;

	constructor(
		address: string,
		{ keyId, streamId, kind }: ReceiverParams,
		private rtcPCConfig?: RTCConfiguration
	) {
		this.keyId = keyId;
		this.streamId = streamId;
		this.kind = kind;

		const ws = new WebSocket(address);

		ws.addEventListener("message", (event) => {
			const message = JSON.parse(event.data);

			const validation = messageSchema.validate(message);
			if (validation.isValid) {
			}

			if (message.type === "SIGNALLING") {
				if (message.data.type === "DESCRIPTION") {
					if (message.data.data.type === "offer") {
						this.handleOffer(message.data.data);
					}
				} else if (message.data.type === "CANDIDATE") {
					this.handleCandidate(message.data.data);
				}
			}
		});
	}

	private getPeerConnection() {
		if (this.peerConnection === null) {
			this.peerConnection = new RTCPeerConnection(this.rtcPCConfig);
			this.peerConnection.addEventListener("track", (event) => {});
			this.peerConnection.addEventListener("icecandidate", (event) => {});
		}

		return this.peerConnection;
	}

	private closePeerConnection() {
		this.peerConnection?.close();
	}

	private async handleOffer(offer: RTCSessionDescriptionInit) {
		this.getPeerConnection();

		await this.peerConnection!.setRemoteDescription(offer);
	}

	private async handleCandidate(candidate: RTCIceCandidate) {
		const pc = this.getPeerConnection();

		await pc.addIceCandidate(candidate);
	}

	get mediaStream() {
		return this._mediaStream;
	}
}
