import { object, unknown, exact, string, InferType, either } from "./validator";

export const descriptionSchema = object({
	type: exact("DESCRIPTION"),
	data: object({
		type: either(exact("offer"), exact("answer")),
		sdp: string(),
	}),
});

export const candidateSchema = object({
	type: exact("ICE_CANDIDATE"),
	data: object({}), // An RTCIceCandidate instance
});

export const signallingMessageSchema = object({
	type: exact("SIGNALLING"),
	data: unknown(),
});

export type SignallingMessage = Omit<
	InferType<typeof signallingMessageSchema>,
	"data"
> & {
	data: InferType<typeof descriptionSchema> | InferType<typeof candidateSchema>;
};
