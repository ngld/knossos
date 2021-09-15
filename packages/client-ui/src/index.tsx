import React from 'react';
import ReactDOM from 'react-dom';
import './tw-index.css';
import './blueprint.scss';
import '@blueprintjs/popover2/lib/css/blueprint-popover2.css';
import './resources/fonts/index.css';
import './style.css';
import { FocusStyleManager } from '@blueprintjs/core';
import { BrowserRouter } from 'react-router-dom';
import { DefaultContext as IconConfig } from '@meronex/icons';

import Root from './elements/root';
import { GlobalState, StateProvider } from './lib/state';

FocusStyleManager.onlyShowFocusOnTabs();
IconConfig.className = 'icon';

// @ts-expect-error We don't have type definitions for window.knIsApp
if (process.env.NODE_ENV !== 'production' && !window.knIsApp) {
  if (!window.knAddMessageListener && !window.knRemoveMessageListener) {
    require('./lib/ws-dev-api');
  }
}

const gs = new GlobalState();

ReactDOM.render(
  <StateProvider value={gs}>
    <BrowserRouter>
      <Root />
    </BrowserRouter>
  </StateProvider>,
  document.querySelector('#container'),
);

