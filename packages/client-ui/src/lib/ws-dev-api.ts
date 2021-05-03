type Listener = (msg: ArrayBuffer) => void;
const listeners: Listener[] = [];

window.knAddMessageListener = (cb: Listener) => {
	listeners.push(cb);
};

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

export {};
