export class Receiver {
	private _mediaStream: MediaStream = new MediaStream();

	constructor(address: string) {}

	get mediaStream() {
		return this._mediaStream;
	}
}
