type Listener = (msg: ArrayBuffer) => void;
const listeners: Listener[] = [];

// @ts-expect-error We never defined these as properties on window
window.knAddMessageListener = (cb: Listener) => {
	listeners.push(cb);
};

// @ts-expect-error We never defined these as properties on window
window.knRemoveMessageListener = (cb: Listener) => {
	const idx = listeners.indexOf(cb);
	if (idx > -1) {
		listeners.splice(idx, 1);
	}
};

const ws = new WebSocket('ws://localhost:8100/ws');
ws.addEventListener('message', (ev) => {
  console.log(ev.data);
});
