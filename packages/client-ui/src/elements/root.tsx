import {
  CiWindowMinLine,
  CiWindowRestoreLine,
  CiWindowMaxLine,
  CiWindowCloseLine,
  CiPictureLine,
  CiFilterLine,
  CiCogLine,
} from '@meronex/icons/ci';
import { useState, useEffect } from 'react';
import { observer } from 'mobx-react-lite';
import { Spinner } from '@blueprintjs/core';
import { Tooltip2 } from '@blueprintjs/popover2';
import cx from 'classnames';
import { Switch, Route, Redirect, useHistory, useLocation } from 'react-router-dom';
import ErrorBoundary from './error-boundary';
import HoverLink from './hover-link';
import { GlobalState, useGlobalState } from '../lib/state';
import { initStartup } from '../dialogs/startup';
import TaskDisplay from './task-display';
import LocalModList from '../pages/local-mod-list';
import RemoteModList from '../pages/remote-mod-list';
import Settings from '../pages/settings';
import LocalMod from '../pages/local-mod';
import RemoteMod from '../pages/remote-mod';

const NavTabs = function NavTabs(): React.ReactElement {
  const history = useHistory();
  const location = useLocation();
  const items = {
    play: 'Play',
    explore: 'Explore',
    build: 'Build',
  };

  return (
    <div className="ml-32 mt-2">
      {Object.entries(items).map(([key, label]) => (
        <a
          key={key}
          href="#"
          className={
            'text-white ml-10 pb-1 border-b-4' +
            (location.pathname === '/' + key ? '' : ' border-transparent')
          }
          onClick={(e) => {
            e.preventDefault();
            history.push('/' + key);
          }}
        >
          {label}
        </a>
      ))}
    </div>
  );
};

interface TooltipButtonProps {
  tooltip?: string;
  onClick?: () => void;
  children: React.ReactNode | React.ReactNode[];
}
function TooltipButton(props: TooltipButtonProps): React.ReactElement {
  return (
    <Tooltip2 content={props.tooltip} placement="bottom">
      <a
        href="#"
        onClick={(e) => {
          e.preventDefault();
          if (props.onClick) {
            props.onClick();
          }
        }}
      >
        {props.children}
      </a>
    </Tooltip2>
  );
}

const ModContainer = observer(function ModContainer(props: {
  gs: GlobalState;
}): React.ReactElement {
  const location = useLocation();

  return (
    <div
      id="scroll-container"
      className={cx(
        'flex-1',
        'relative',
        'mod-container',
        { 'pattern-bg': location.pathname !== '/settings' },
        'rounded-md',
        'm-3',
        'p-4',
        'overflow-y-scroll',
      )}
    >
      <ErrorBoundary>
        {props.gs.startupDone ? (
          <Switch>
            <Redirect exact from="/" to="/play" />
            <Redirect exact from="/index.html" to="/play" />
            <Route path="/play" component={LocalModList} />
            <Route path="/explore" component={RemoteModList} />
            <Route path="/settings">
              <Settings />
            </Route>
            <Route path="/mod/:modid/:version?" component={LocalMod} />
            <Route path="/rmod/:modid/:version?" component={RemoteMod} />
            <Route path="/">
              <div className="text-white">Page not found</div>
            </Route>
          </Switch>
        ) : props.gs.overlays.length > 0 ? null : (
          <Spinner />
        )}
      </ErrorBoundary>
    </div>
  );
});

const OverlayRenderer = observer(function OverlayRenderer({
  gs,
}: {
  gs: GlobalState;
}): React.ReactElement {
  return (
    <ErrorBoundary>
      {gs.overlays.map(([Component, props, overlayID], idx) => (
        <Component key={overlayID} onFinished={() => gs.removeOverlay(idx)} {...props} />
      ))}
    </ErrorBoundary>
  );
});

interface VersionInfo {
  version: string;
  commit: string;
}

export default function Root(): React.ReactElement {
  const gs = useGlobalState();
  const history = useHistory();
  const [isMaximized, setMaximized] = useState<boolean>(false);
  const [versionInfo, setVersionInfo] = useState<VersionInfo>({
    version: '',
    commit: '',
  });

  useEffect(() => {
    initStartup(gs);

    void (async () => {
      try {
        const result = await gs.client.getVersion({});
        setVersionInfo(result.response);
      } catch (e) {
        console.error('failed to fetch knossos version', e);
      }
    })();
  }, []);

  return (
    <div className="flex flex-col h-full">
      <div className="flex-initial">
        <div className="pt-5 pl-5 text-3xl text-white font-inria title-bar pb-1">
          <span>Knossos</span>
          <span className="ml-10">{versionInfo.version}</span>
          <span className="ml-1 text-sm align-top">+{versionInfo.commit}</span>
        </div>

        <div className="absolute top-0 right-0 text-white text-3xl traffic-lights">
          <HoverLink onClick={() => knMinimizeWindow()}>
            <CiWindowMinLine />
          </HoverLink>
          <HoverLink
            onClick={() => {
              if (isMaximized) {
                knRestoreWindow();
                setMaximized(false);
              } else {
                knMaximizeWindow();
                setMaximized(true);
              }
            }}
          >
            {isMaximized ? <CiWindowRestoreLine /> : <CiWindowMaxLine />}
          </HoverLink>
          <HoverLink onClick={() => knCloseWindow()}>
            <CiWindowCloseLine />
          </HoverLink>
        </div>

        <div className="relative">
          <div className="bg-dim h-px" />
          <TaskDisplay />
        </div>

        <div className="float-right mr-2 text-white text-2xl gap-2">
          <TooltipButton tooltip="Screenshots">
            <CiPictureLine className="ml-2" />
          </TooltipButton>
          <CiFilterLine className="ml-2" />
          <TooltipButton tooltip="Settings" onClick={() => history.push('/settings')}>
            <CiCogLine className="ml-2" />
          </TooltipButton>
        </div>

        <NavTabs />
      </div>
      <ModContainer gs={gs} />
      <OverlayRenderer gs={gs} />
    </div>
  );
}
