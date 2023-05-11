import { connect, encodeBase64 } from "./wkeyid";

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

export class Sender {
	// I know, I know, the exclamation mark looks bad, but most of the code is
	// pretty much a bunch of `if (!this.pc) {throw new Error("...");}` anyway.
	private pc!: RTCPeerConnection;

	replaceTrack(track: MediaStreamTrack) {}

	removeTrack(track: MediaStreamTrack) {}

	static async send(
		address: string,
		id: string,
		sign: (data: ArrayBuffer) => Promise<ArrayBuffer>,
		track: MediaStreamTrack
	) {
		const ws = new WebSocket(address);
		await connect(ws, id, sign);
		const sender = new Sender();
		sender.pc = new RTCPeerConnection();
		sender.pc.addEventListener("icecandidate", (event) => {});
		sender.pc.addTrack(track);
	}
}
