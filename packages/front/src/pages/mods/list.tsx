// It's hard to implement the debouncePromise helper without any.
/* eslint-disable @typescript-eslint/no-explicit-any */

import React, { useState, useMemo } from 'react';
import { observer } from 'mobx-react-lite';
import { fromPromise } from 'mobx-utils';
import { useNavigate } from 'react-router-dom';
import { Spinner, Callout, NonIdealState, InputGroup } from '@blueprintjs/core';

import { ModListRequest_SortType } from '@api/service';
import { useGlobalState, GlobalState } from '../../lib/state';

function debouncePromise<R, T extends (...args: any[]) => Promise<R>>(func: T, delay: number): T {
  let timer: NodeJS.Timeout | null = null;
  let callArgs: any[];
  let promise: Promise<R> | null = null;
  return ((...args: any[]): Promise<R> => {
    if (timer !== null) {
      clearTimeout(timer);
      timer = null;
    }

    callArgs = args;
    if (timer === null || promise === null) {
      promise = new Promise((resolve) => {
        timer = setTimeout(() => {
          // We're just passing through these parameters; we can't know which type they are and we can't tell TS
          // that it'll work out (our return value is identical to the passed function).
          // eslint-disable-next-line @typescript-eslint/no-unsafe-argument
          resolve(func(...callArgs));
          timer = null;
        }, delay);
      });
    }
    return promise;
  }) as T;
}

const listMods = debouncePromise(async function listMods(gs: GlobalState, query: string) {
  const response = await gs.runTwirpRequest(gs.client.getModList.bind(gs.client), {
    limit: 300,
    offset: 0,
    query,
    sort: ModListRequest_SortType.NAME,
  });

  return response;
}, 200);

export default observer(function ModListPage(): React.ReactElement {
  const gs = useGlobalState();
  const navigate = useNavigate();
  const [query, setQuery] = useState('');
  const modList = useMemo(
    () => fromPromise(listMods(gs, query) ?? Promise.resolve(null)),
    [gs, query],
  );

  return (
    <div>
      <InputGroup placeholder="Search" value={query} onChange={(e) => setQuery(e.target.value)} />
      <div>
        {modList.case({
          pending: () => <Spinner />,
          rejected: () => (
            <Callout intent="danger" title="Failed to fetch mods">
              Unfortunately, the mod list request failed. Please try again.
            </Callout>
          ),
          fulfilled: (mods) =>
            !mods ? (
              <NonIdealState
                icon="search"
                title="No mods found"
                description="We couldn't find any mods matching your request."
              />
            ) : (
              <div className="flex flex-wrap">
                {mods.mods.map((mod) => (
                  <div key={mod.modid} className="flex-initial bg-bg-tile m-2 mod-tile">
                    <a
                      href={`/mod/${mod.modid}`}
                      onClick={(e) => {
                        e.preventDefault();
                        navigate(`/mod/${mod.modid}`);
                      }}
                      className="block text-center no-underline pb-2"
                    >
                      <img src={mod.teaser} className="mx-auto mb-2" />
                      {mod.title}
                    </a>
                  </div>
                ))}
              </div>
            ),
        })}
      </div>
    </div>
  );
});
