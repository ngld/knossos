import { useState, useEffect } from 'react';
import { Button, NonIdealState, Spinner } from '@blueprintjs/core';
import { History } from 'history';
import InfiniteScroll from 'react-infinite-scroll-component';
import { ModListResponse, ModListItem } from '@api/service';
import { GlobalState, useGlobalState } from '../lib/state';
import { launchMod } from '../dialogs/launch-mod';
import ModstockImage from '../resources/modstock.jpg';

async function fetchMods(gs: GlobalState, offset: number): Promise<ModListResponse> {
  const result = await gs.nebula.getModList({ offset, limit: 50, query: '' });
  console.log(result);
  return result.response;
}

export interface RemoteModListProps {
  history: History;
}

interface State {
  count: number;
  mods: ModListItem[];
  loading: boolean;
  error: string | null;
}

export default function RemoteModList(props: RemoteModListProps): React.ReactElement {
  const gs = useGlobalState();
  const [state, setState] = useState<State>({
    count: 0,
    mods: [],
    loading: true,
    error: null,
  });

  useEffect(() => {
    void (async function () {
      try {
        const resp = await fetchMods(gs, 0);
        setState((prev) => ({
          ...prev,
          loading: false,
          count: resp.count,
          mods: resp.mods,
        }));
      } catch (e) {
        console.error(e);
        setState((prev) => ({
          ...prev,
          loading: false,
          error: e.toString(),
        }));
      }
    })();
  }, [setState, gs]);

  async function fetchNext() {
    try {
      const resp = await fetchMods(gs, state.mods.length);
      setState((prev) => ({
        ...prev,
        mods: prev.mods.concat(resp.mods),
      }));
    } catch (e) {
      console.error(e);
      setState((prev) => ({
        ...prev,
        error: e.toString(),
      }));
    }
  }

  return (
    <div className="text-white">
      {state.loading ? (
        <NonIdealState icon={<Spinner />} title="Loading mods..." />
      ) : state.error ? (
        <NonIdealState icon="error" title="Failed to load mods" description={state.error} />
      ) : (
        <div className="flex">
          <InfiniteScroll
            className="flex flex-row flex-wrap justify-between gap-4"
            dataLength={state.mods.length}
            hasMore={state.mods.length < state.count}
            next={fetchNext}
            loader={<Spinner />}
            scrollableTarget="scroll-container"
          >
            {state.mods.map((mod) => (
              <div
                key={mod.modid}
                className="mod-tile bg-important flex flex-col overflow-hidden"
              >
                {mod.teaser ? <img src={mod.teaser} /> : <img src={ModstockImage} />}
                <div className="flex-1 flex flex-col justify-center text-white">
                  <div className="flex-initial text-center overflow-ellipsis overflow-hidden">
                    {mod.title}
                  </div>
                </div>

                <div className="cover flex flex-col justify-center gap-2">
                  <Button onClick={() => launchMod(gs, mod.modid, '')}>Play</Button>
                  <Button onClick={() => props.history.push('/mod/' + mod.modid + '/' + '')}>
                    Details
                  </Button>
                  <Button>Uninstall</Button>
                </div>
              </div>
            ))}{' '}
          </InfiniteScroll>
        </div>
      )}
    </div>
  );
}
