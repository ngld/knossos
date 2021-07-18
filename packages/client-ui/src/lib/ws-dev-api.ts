type Listener = (msg: ArrayBuffer[]) => void;
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

function openWS() {
  const ws = new WebSocket('ws://localhost:8100/ws');
  ws.addEventListener('message', (ev) => {
    if (ev.data instanceof Blob) {
      void (async () => {
        const content = await (ev.data as Blob).arrayBuffer();

        for (const listener of listeners) {
          listener([content]);
        }
      })();
    }
  });

  ws.addEventListener('open', () => {
    console.info('Connected to libknossos');
  });
  ws.addEventListener('close', (e) => {
    if (e instanceof CloseEvent) {
      console.info('libknossos WS closed, reconnecting');
    } else {
      console.error(e);
    }
    openWS();
  });
}
openWS();

export {};
