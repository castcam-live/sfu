import { useEffect, useState } from "react";
import StreamPlayer from "./StreamPlayer";
import { generateKeys, getClientId } from "./stream/sender";

class MediaManager {
	constructor() {}
}

/**
 * Get the list of available media devices of a specific kind
 * @param kind The media device kind that we want to fetch
 * @returns A list of media device info
 */
function getMediaDevicesList(kind: MediaDeviceKind) {
	return navigator.mediaDevices.enumerateDevices().then((devices) => {
		return devices.filter((device) => device.kind === kind);
	});
}

type MediaDevicesSelectorProps = {
	kind: MediaDeviceKind;
	onSelect: (device: MediaDeviceInfo["deviceId"]) => void;
};
function MediaDeviceSelector({ kind, onSelect }: MediaDevicesSelectorProps) {
	const [devices, setDevices] = useState<MediaDeviceInfo[]>([]);

	useEffect(() => {
		let cancelled = false;

		const refreshMediaDevicesList = () => {
			if (cancelled) return;
			getMediaDevicesList(kind).then((devices) => {
				if (cancelled) return;
				setDevices(devices);
			});
		};

		refreshMediaDevicesList();

		navigator.mediaDevices.addEventListener(
			"devicechange",
			refreshMediaDevicesList
		);

		return () => {
			cancelled = true;
			navigator.mediaDevices.removeEventListener(
				"devicechange",
				refreshMediaDevicesList
			);
		};
	}, []);

	return (
		<select
			disabled={devices.length === 0}
			onChange={(e) => {
				onSelect(e.target.value);
			}}
		>
			{devices.map((device) => (
				<option key={device.deviceId} value={device.deviceId}>
					{device.label}
				</option>
			))}
		</select>
	);
}

export function Stream() {
	const [videoDevice, setVideoDevice] = useState<
		MediaDeviceInfo["deviceId"] | null
	>(null);
	const [videoStream, setVideoStream] = useState<MediaStream | null>(null);
	const [videoStreamId, setVideoStreamId] = useState<string | null>(null);

	useEffect(() => {
		if (videoDevice) {
			navigator.mediaDevices
				.getUserMedia({
					video: { deviceId: videoDevice },
					audio: false,
				})
				.then((stream) => {
					setVideoStream(stream);
				});
		} else {
			setVideoStream(null);
		}
	}, [videoDevice]);

	return (
		<div>
			{!videoStream ? null : (
				<div>
					<StreamPlayer
						style={{
							width: 256,
							height: 100,
						}}
						stream={videoStream}
					/>
				</div>
			)}

			<div>
				<button
					onClick={() => {
						generateKeys()
							.then(async (keys) => {
								const clientId = await getClientId(keys.publicKey);
								console.log(clientId);
							})
							// TODO: properly handle errors!
							.catch(console.error);
					}}
				>
					Publish
				</button>
			</div>

			<MediaDeviceSelector kind="videoinput" onSelect={setVideoDevice} />
		</div>
	);
}
