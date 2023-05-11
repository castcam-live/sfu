/**
MIT License

Copyright (c) 2023 Sal Rahman

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

/**
 * Encodes an array buffer as a base64 string
 * @param buffer An array buffer to encode
 * @returns A base64 encoded string
 */
export function encodeBase64(buffer: ArrayBuffer) {
	const base64chars =
		"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=";
	const bytes = new Uint8Array(buffer);
	let i = 0;
	let base64 = "";

	while (i < bytes.length) {
		const byte1 = bytes[i++] || 0;
		let byte2 = bytes[i++];
		let byte3 = bytes[i++];

		let padding = 0;

		if (byte2 === undefined) {
			padding++;
			byte2 = 0;
		}
		if (byte3 === undefined) {
			padding++;
			byte3 = 0;
		}

		const encoded1 = byte1 >> 2;
		const encoded2 = ((byte1 & 0x03) << 4) | (byte2 >> 4);
		let encoded3 = ((byte2 & 0x0f) << 2) | (byte3 >> 6);
		let encoded4 = byte3 & 0x3f;

		if (padding === 1) encoded4 = 64;
		if (padding === 2) encoded3 = encoded4 = 64;

		base64 += `${base64chars[encoded1]}${base64chars[encoded2]}${base64chars[encoded3]}${base64chars[encoded4]}`;
	}

	return base64;
}

/**
 * Decodes a base64 string into an array buffer
 * @param base64 A base64 encoded string
 * @returns An array buffer
 */
export function decodeBase64(base64: string) {
	const base64chars =
		"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

	if (!base64.match(/^[A-Za-z0-9+/]+={0,2}$/)) {
		throw new Error("Invalid base64 string");
	}

	const padding = base64.endsWith("==") ? 2 : base64.endsWith("=") ? 1 : 0;
	const bytes = new Uint8Array((base64.length * 6) / 8 - padding);
	let i = 0;
	let j = 0;

	while (i < base64.length) {
		const index1 = base64chars.indexOf(base64[i++]);
		const index2 = base64chars.indexOf(base64[i++]);
		const index3 = base64chars.indexOf(base64[i++]);
		const index4 = base64chars.indexOf(base64[i++]);

		const decoded1 = (index1 << 2) | (index2 >> 4);
		const decoded2 = ((index2 & 0x0f) << 4) | (index3 >> 2);
		const decoded3 = ((index3 & 0x03) << 6) | index4;

		bytes[j++] = decoded1;

		if (index3 !== 64) {
			bytes[j++] = decoded2;
		}
		if (index4 !== 64) {
			bytes[j++] = decoded3;
		}
	}

	return bytes.buffer;
}

function onOpen(ws: WebSocket): Promise<void> {
	return new Promise((resolve) => {
		if (ws.readyState === WebSocket.OPEN) {
			resolve();
		}
		ws.addEventListener("open", () => {
			resolve();
		});
	});
}

/**
 * Connects to a WebSocket server, and performs the initial handshake
 * @param ws The WebSocket instance that will perform the initial handshake
 * @param keyPair The keypair to form the handshake with
 * @param onMessage An optional on-message listener, for the purposes of
 *   debugging
 * @returns A promise that resolves when the handshake is complete
 */
export async function connect(
	ws: WebSocket,
	id: string,
	sign: (data: ArrayBuffer) => Promise<ArrayBuffer>
): Promise<void> {
	await onOpen(ws);

	let currentState: "CONNECTING" | "SENT_CHALLENGE" = "CONNECTING";

	const promise = new Promise<void>((resolve, reject) => {
		const removeListener = () => {
			ws.removeEventListener("message", listener);
		};

		const close = (error?: any) => {
			removeListener();
			reject(error);
			ws.close();
		};

		const listener = (event: MessageEvent<any>) => {
			const data = JSON.parse(event.data);

			switch (currentState) {
				case "CONNECTING":
					console.log("In connecting", data);
					switch (data.type) {
						case "CHALLENGE":
							// This is where we sign the challenge payload, and send it off
							sign(decodeBase64(data.data))
								.then((signature) => {
									ws.send(
										JSON.stringify({
											type: "CHALLENGE_RESPONSE",
											data: {
												signature: encodeBase64(signature),
												hash: "SHA-256",
											},
										})
									);
									console.log("Sent challenge response");
									currentState = "SENT_CHALLENGE";
								})
								.catch((e) => {
									close(e);
								});
							break;
						default:
							// This is where we got a message we didn't expect
							close(
								new Error(
									`Unexpected message. Connection state: ${currentState}}`
								)
							);
							break;
					}
					break;
				case "SENT_CHALLENGE":
					console.log("In sent challenge", data);
					switch (data.type) {
						case "SIGNATURE_MATCHES":
							removeListener();
							resolve();
							break;
						case "SIGNATURE_MISMATCH":
							close(new Error("Signature mismatch"));
							break;
						default:
							close(new Error("Unexpected message"));
							break;
					}
					break;
			}
		};
		ws.addEventListener("message", listener);
	});

	ws.send(
		JSON.stringify({
			type: "CLIENT_ID",
			data: id,
		})
	);

	return await promise;
}
