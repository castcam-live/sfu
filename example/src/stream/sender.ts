import { WsKeySession } from "./ws-key-session";
import { connect, encodeBase64 } from "./wskeyid";

export async function generateKeys() {
	return await crypto.subtle.generateKey(
		{ name: "ECDSA", namedCurve: "P-256" },
		false,
		["sign", "verify"]
	);
}

export async function getClientId(keyPair: CryptoKeyPair) {
	const algo = keyPair.publicKey.algorithm;
	if (algo.name !== "ECDSA" && algo.name !== "ECDH") {
		throw new Error(
			`Unexpected key algorithm "${keyPair.publicKey.algorithm.name}"`
		);
	}
	const encodedRaw = encodeBase64(
		await crypto.subtle.exportKey("raw", keyPair.publicKey)
	);
	return `WebCrypto-raw.EC.${(algo as any).namedCurve}$${encodedRaw}`;
}

type SignallingMessage = {
	type: "SIGNALLING";
	data:
		| {
				type: "DESCRIPTION";
				data: {
					type: "offer";
					sdp: string;
				};
		  }
		| {
				type: "ICE_CANDIDATE";
				data: RTCIceCandidate;
		  };
};

export class Sender {
	private pc: RTCPeerConnection | null = null;
	private isClosed = false;
	private session: WsKeySession;
	private kind: string;
	private track: MediaStreamTrack | null = null;

	private initializePc() {
		if (this.isClosed) return;
		this.pc = new RTCPeerConnection(this.rtcPCConfig);
		this.pc.addEventListener("connectionstatechange", () => {
			if (this.pc?.connectionState === "disconnected") {
				this.initializePc();
			}
		});
		this.pc.addEventListener("icecandidate", (event) => {
			if (event.candidate) {
				this.session.send(
					JSON.stringify({
						type: "SIGNALLING",
						data: {
							type: "ICE_CANDIDATE",
							data: event.candidate,
						},
					})
				);
			}
		});
		this.pc.addEventListener("negotiationneeded", () => {
			Promise.resolve().then(async () => {
				const offer = await this.pc!.createOffer();
				this.pc!.setLocalDescription(offer);
				this.session.send(
					JSON.stringify({
						type: "SIGNALLING",
						data: {
							type: "DESCRIPTION",
							data: offer,
						},
					})
				);
			});
		});
		if (this.track) {
			this.setTrackOnly(this.track);
		}
	}

	private closePc() {
		this.pc?.close();
		this.pc = null;
	}

	private setTrackOnly(track: MediaStreamTrack) {
		const sender = this.pc!.getSenders().find(
			(sender) => sender.track === track
		);

		if (!sender) {
			this.pc!.addTrack(track);
		} else {
			sender.replaceTrack(track);
		}
	}

	setTrack(track: MediaStreamTrack) {
		if (track.kind !== this.kind) {
			this.closePc();
			throw new Error("Track kind does not match sender kind");
		}

		this.track = track;

		if (!this.pc) {
			this.initializePc();
		}

		this.initializePc();
	}

	removeTrack() {
		this.pc?.getSenders().forEach((sender) => {
			this.pc?.removeTrack(sender);
		});
	}

	close() {
		this.isClosed = true;
		this.pc?.close();
		this.session.close();
	}

	constructor(
		address: string,
		clientId: string,
		sign: (payload: ArrayBuffer) => Promise<ArrayBuffer>,
		kindOrTrack: string | MediaStreamTrack,
		private rtcPCConfig?: RTCConfiguration
	) {
		// General idea is this:
		//
		// - create a WSKeyID session
		// -

		this.session = new WsKeySession(address, clientId, sign);
		this.session.stateChangeEvents.addEventListener((state) => {
			if (state === "DISCONNECTED") {
				this.closePc();
			} else if (state === "CONNECTED" && this.pc === null) {
				this.initializePc();
			}
		});

		if (typeof kindOrTrack === "string") {
			this.kind = kindOrTrack;
		} else {
			this.kind = kindOrTrack.kind;
			this.setTrack(kindOrTrack);
		}
	}
}
