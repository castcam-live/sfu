export type EventEmitter<T> = {
	addEventListener: (listener: (value: T) => void) => () => void;
};

export type Subject<T> = EventEmitter<T> & {
	emit: (value: T) => void;
};

export function createSubject<T>(): Subject<T> {
	const listeners = new Set<(value: T) => void>();

	return {
		addEventListener: (listener: (value: T) => void) => {
			listeners.add(listener);
			return () => {
				listeners.delete(listener);
			};
		},
		emit: (value: T) => {
			for (const listener of listeners) {
				listener(value);
			}
		},
	};
}
