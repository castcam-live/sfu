import { SignallingMessage } from "./schema";
import { WsKeySession } from "./ws-key-session";
import { encodeBase64 } from "./wskeyid";

/**
 * Generates a new key pair to be used for ECDSA signing
 * @returns A new SubtleCrypto KeyPair
 */
export async function generateKeys() {
	return await crypto.subtle.generateKey(
		{ name: "ECDSA", namedCurve: "P-256" },
		false,
		["sign", "verify"]
	);
}

/**
 * Derives a client ID from a key pair
 * @param keyPair The key pair to use to derive the client ID
 * @returns A string representing the client ID
 */
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
	return `WebCrypto-raw.EC.${
		(algo as KeyAlgorithm & { namedCurve: "P-256" | "P-384" | "P-256" })
			.namedCurve
	}$${encodedRaw}`;
}

/**
 * Sender is a container for sending a MediaStreamTrack.
 */
export class Sender {
	private pc: RTCPeerConnection | null = null;
	private isClosed = false;
	private session: WsKeySession;
	private kind: string;
	private track: MediaStreamTrack | null = null;

	private initializePc() {
		// NOTE: this function is idempotent if a PC is already set

		if (this.isClosed) return;
		if (this.pc) return;

		this.pc = new RTCPeerConnection(this.rtcPCConfig);
		this.pc.addEventListener("connectionstatechange", () => {
			if (this.pc?.connectionState === "disconnected") {
				this.initializePc();
			}
		});
		this.pc.addEventListener("icecandidate", (event) => {
			if (event.candidate) {
				const message: SignallingMessage = {
					type: "SIGNALLING",
					data: {
						type: "ICE_CANDIDATE",
						data: event.candidate,
					},
				};
				this.session.send(JSON.stringify(message));
			}
		});
		this.pc.addEventListener("negotiationneeded", () => {
			Promise.resolve()
				.then(async () => {
					const offer = await this.pc!.createOffer();
					this.pc!.setLocalDescription(offer);

					if (!offer.sdp) {
						// TODO: handle this edge case
						console.error("An SDP was not available for some reason");
						return;
					}

					const message: SignallingMessage = {
						type: "SIGNALLING",
						data: {
							type: "DESCRIPTION",
							data: {
								type: "offer",
								sdp: offer.sdp,
							},
						},
					};

					this.session.send(JSON.stringify(message));
				})
				// TODO: this silently fails. Figure out a way to make this fail by
				//   notifying the client code.
				.catch(console.error);
		});
		if (this.track) {
			this.setTrackOnly(this.track);
		}
	}

	private closePc() {
		this.pc?.close();
		this.pc = null;
	}

	/**
	 * Sets the track on the peer connection, without initializing the peer
	 * conneciton.
	 * @param track The track to set
	 */
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

	/**
	 * Sets the track on the peer connection, initializing the peer connection, if
	 * it hasn't been done so already.
	 * @param track The track to set
	 */
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

	/**
	 * Closes the peer connection and the session.
	 */
	close() {
		this.isClosed = true;
		this.closePc();
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
		// - create a WSKeyID session to server
		// - if a sender exists, then a track MUST be set (like hello, why should it
		//   not?)

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
