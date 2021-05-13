import { useState } from 'react';
import { Button, NonIdealState, Spinner } from '@blueprintjs/core';
import { History } from 'history';
import { observer } from 'mobx-react-lite';
import { fromPromise } from 'mobx-utils';
import { SimpleModList_Item } from '@api/client';
import { GlobalState, useGlobalState } from '../lib/state';
import RefImage from '../elements/ref-image';
import ModstockImage from '../resources/modstock.jpg';

async function fetchMods(gs: GlobalState): Promise<SimpleModList_Item[]> {
  const result = await gs.client.getRemoteMods({});
  console.log(result.response.mods);
  return result.response.mods;
}

function refreshRemoteMods(gs: GlobalState): void {
  const taskId = gs.tasks.startTask('Refreshing remote mod list');
  void gs.client.syncRemoteMods({ ref: taskId });
}

export interface RemoteModListProps {
  history: History;
}

export default observer(function RemoteModList(props: RemoteModListProps): React.ReactElement {
  const gs = useGlobalState();
  const [modList] = useState(() => fromPromise(fetchMods(gs)));

  return (
    <div className="text-white">
      {modList.case({
        pending: () => <NonIdealState icon={<Spinner />} title="Loading mods..." />,
        rejected: (e: Error) => (
          <NonIdealState icon="error" title="Failed to load mods" description={e.message} />
        ),
        fulfilled: (mods) => (
          <>
            <Button onClick={() => refreshRemoteMods(gs)}>Refresh</Button>
            <div className="flex flex-row flex-wrap justify-between gap-4">
              {mods.map((mod) => (
                <div
                  key={mod.modid}
                  className="mod-tile bg-important flex flex-col overflow-hidden"
                >
                  {mod.teaser ? <RefImage src={mod.teaser} /> : <img src={ModstockImage} />}
                  <div className="flex-1 flex flex-col justify-center text-white">
                    <div className="flex-initial text-center overflow-ellipsis overflow-hidden">
                      {mod.title}
                    </div>
                  </div>

                  <div className="cover flex flex-col justify-center gap-2">
                    <Button
                      onClick={() => props.history.push('/rmod/' + mod.modid + '/' + mod.version)}
                    >
                      Details
                    </Button>
                  </div>
                </div>
              ))}{' '}
            </div>
          </>
        ),
      })}
    </div>
  );
});
