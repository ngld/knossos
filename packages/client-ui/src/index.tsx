import { createRoot } from 'react-dom/client';
import './tw-index.css';
import './blueprint.precss';
import '@blueprintjs/popover2/lib/css/blueprint-popover2.css';
import './resources/fonts/index.css';
import './style.css';
import { FocusStyleManager } from '@blueprintjs/core';
import { BrowserRouter } from 'react-router-dom';

FocusStyleManager.onlyShowFocusOnTabs();

// @ts-expect-error We don't have type definitions for window.knIsApp
if (process.env.NODE_ENV !== 'production' && !window.knIsApp) {
  if (!window.knAddMessageListener && !window.knRemoveMessageListener) {
    require('./lib/ws-dev-api');
  }
}

// eslint-disable-next-line
const Root = require('./elements/root').default;
const container = document.querySelector<HTMLDivElement>('div#container');

if (container) {
  createRoot(container).render(
    <BrowserRouter>
      <Root />
    </BrowserRouter>,
  );
}
