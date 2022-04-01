import { useState, useEffect } from 'react';
import { Button, NonIdealState, Spinner, Menu, MenuItem } from '@blueprintjs/core';
import { ContextMenu2 } from '@blueprintjs/popover2';
import { observer } from 'mobx-react-lite';
import { fromPromise } from 'mobx-utils';
import { useNavigate } from 'react-router-dom';
import cx from 'classnames';
import { SimpleModList_Item, ToolInfo } from '@api/client';
import { ModType } from '@api/mod';
import { GlobalState, useGlobalState } from '../lib/state';
import { API_URL } from '../lib/constants';
import { launchMod, LaunchModDialog } from '../dialogs/launch-mod';
import { maybeError } from '../dialogs/error-dialog';
import UninstallModDialog from '../dialogs/uninstall-mod';
import ModstockImage from '../resources/modstock.jpg';
import RetailImage from '../resources/mod-retail.png';

async function fetchMods(gs: GlobalState): Promise<SimpleModList_Item[]> {
  const result = await gs.client.getLocalMods({});
  console.log(result.response.mods);
  return result.response.mods;
}

export default observer(function LocalModList(): React.ReactElement {
  const gs = useGlobalState();
  const navigate = useNavigate();
  const [modList, setModList] = useState(() => fromPromise(fetchMods(gs)));

  gs.useSignal('reloadLocalMods', () => {
    setModList(fromPromise(fetchMods(gs)));
  });

  return (
    <div className="text-white">
      {modList.case({
        pending: () => <NonIdealState icon={<Spinner />} title="Loading mods..." />,
        rejected: (e: Error) => (
          <NonIdealState
            icon="error"
            title="Failed to load mods"
            description={e?.toString ? e.toString() : String(e)}
          />
        ),
        fulfilled: (mods: SimpleModList_Item[]) => (
          <div className="flex flex-row flex-wrap justify-between gap-4">
            {mods.length > 0 ? (
              mods.map((mod) => (
                <ContextMenu2 key={mod.modid} content={<ModActionMenu mod={mod} />}>
                  {({ className, contentProps: _, onContextMenu, popover, ref }) => (
                    <div
                      ref={ref}
                      className={cx(
                        'mod-tile bg-important flex flex-col overflow-hidden',
                        className,
                      )}
                      onContextMenu={onContextMenu}
                    >
                      {popover}
                      {mod.teaser?.fileid ? (
                        <img src={API_URL + '/ref/' + mod.teaser?.fileid} />
                      ) : mod.modid === 'FS2' ? (
                        <img src={RetailImage} />
                      ) : (
                        <img src={ModstockImage} />
                      )}
                      <div className="flex-1 flex flex-col justify-center text-white">
                        <div className="flex-initial text-center overflow-ellipsis overflow-hidden">
                          {mod.title}
                        </div>
                      </div>

                      <div className="cover flex flex-col justify-center gap-2">
                        {mod.type === ModType.MOD || mod.type === ModType.TOTAL_CONVERSION ? (
                          <Button onClick={() => launchMod(gs, mod.modid, mod.version)}>
                            Play
                          </Button>
                        ) : null}
                        <Button onClick={() => navigate('/mod/' + mod.modid + '/' + mod.version)}>
                          Details
                        </Button>
                        <Button
                          onClick={() =>
                            gs.launchOverlay(UninstallModDialog, {
                              modid: mod.modid,
                              version: mod.version,
                            })
                          }
                        >
                          Uninstall
                        </Button>
                        <Button onClick={(e) => onContextMenu(e)}>More...</Button>
                      </div>
                    </div>
                  )}
                </ContextMenu2>
              ))
            ) : (
              <NonIdealState
                icon="search"
                title="No mods found"
                description="You don't have any mods, go to the Explore tab to install some."
              />
            )}{' '}
          </div>
        ),
      })}
    </div>
  );
});

function ModActionMenu(props: { mod: SimpleModList_Item }): React.ReactElement {
  const gs = useGlobalState();
  const [tools, setTools] = useState<ToolInfo[]>([]);

  useEffect(() => {
    void (async () => {
      const resp = await gs.client.getModInfo({ id: props.mod.modid, version: props.mod.version });
      setTools(resp.response.tools);
    })();
  }, [gs, props.mod.modid, props.mod.version]);

  return (
    <Menu>
      {tools.map(
        (tool) =>
          tool.label === '' || (
            <MenuItem
              key={tool.label}
              icon="play"
              text={'Open ' + tool.label}
              onClick={() =>
                gs.launchOverlay(LaunchModDialog, {
                  modid: props.mod.modid,
                  version: props.mod.version,
                  label: tool.label,
                })
              }
            />
          ),
      )}
      <MenuItem
        icon="application"
        text="Open Debug Log"
        onClick={() => maybeError(gs, gs.client.openDebugLog({}))}
      />
    </Menu>
  );
}
