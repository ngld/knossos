import React, { useState, useMemo } from 'react';
import { action, makeAutoObservable } from 'mobx';
import { observer } from 'mobx-react-lite';
import { fromPromise } from 'mobx-utils';
import type { RouteComponentProps } from 'react-router-dom';
import {
  Spinner,
  Callout,
  Checkbox,
  NonIdealState,
  HTMLSelect,
  HTMLTable,
  Tab,
  Tabs,
} from '@blueprintjs/core';
import styled from 'astroturf/react';

import { ModInfoResponse, ModDependencySnapshot, FlagInfo_Flag } from '@api/client';
import { Release, ModType } from '@api/mod';

import RefImage from '../elements/ref-image';
import { useGlobalState, GlobalState } from '../lib/state';
import bbparser from '../lib/bbparser';

async function getModDetails(gs: GlobalState, params: ModDetailsParams): Promise<ModInfoResponse> {
  const response = await gs.client.getModInfo({
    id: params.modid,
    version: params.version ?? '',
  });
  return response.response;
}

async function getModDependencies(
  gs: GlobalState,
  params: ModDetailsParams,
): Promise<ModDependencySnapshot> {
  const response = await gs.client.getModDependencies({
    id: params.modid,
    version: params.version ?? '',
  });
  return response.response;
}

async function getFlagInfos(
  gs: GlobalState,
  params: ModDetailsParams,
): Promise<[Record<string, FlagInfo_Flag[]>, string]> {
  const response = await gs.client.getModFlags({ id: params.modid, version: params.version ?? '' });

  const mappedFlags = {} as Record<string, FlagInfo_Flag[]>;
  for (const info of Object.values(response.response.flags)) {
    if (!mappedFlags[info.category]) {
      mappedFlags[info.category] = [];
    }

    mappedFlags[info.category].push(info);
  }

  const cats = Object.keys(mappedFlags);
  cats.sort();

  const sortedFlags = {} as Record<string, FlagInfo_Flag[]>;
  for (const cat of cats) {
    sortedFlags[cat] = mappedFlags[cat];
  }

  makeAutoObservable(sortedFlags);
  return [sortedFlags, response.response.freeform];
}

async function saveFlagInfos(
  gs: GlobalState,
  params: ModDetailsParams,
  flags: Record<string, boolean>,
  freeform: string,
): Promise<void> {
  try {
    void (await gs.client.saveModFlags({
      modid: params.modid,
      version: params.version ?? '',
      flags,
      freeform,
    }));
  } catch (e) {
    console.error(e);
    gs.toaster.show({
      icon: 'error',
      intent: 'danger',
      message: 'Failed to save flags',
    });
  }
}

interface DepInfoProps extends ModDetailsParams {
  release?: Release;
}

const DepInfo = observer(function DepInfo(props: DepInfoProps): React.ReactElement {
  const gs = useGlobalState();
  const deps = useMemo(() => fromPromise(getModDependencies(gs, props)), [gs, props]);

  return deps.case({
    pending: () => <span>Loading...</span>,
    rejected: (e: Error) => (
      <Callout intent="danger" title="Error">
        Could not resolve dependencies:
        <br />
        <pre>{e.toString()}</pre>
      </Callout>
    ),
    fulfilled: (response) => (
      <HTMLTable>
        <thead>
          <tr>
            <th>Mod</th>
            <th>Latest Local Version</th>
            <th>Latest Available Version</th>
            <th>Saved Version</th>
          </tr>
        </thead>
        <tbody>
          {Object.entries(response.dependencies).map(([modid, current]) => {
            return (
              <tr key={modid}>
                <td>{modid}</td>
                <td>{current}</td>
                <td>TBD</td>
                <td>
                  <HTMLSelect defaultValue={current}>
                    {response.available[modid].versions.map((version) => (
                      <option key={version} value={version}>
                        {version}
                      </option>
                    ))}
                  </HTMLSelect>
                </td>
              </tr>
            );
          })}
        </tbody>
      </HTMLTable>
    ),
  });
});

function renderFlags(
  gs: GlobalState,
  params: ModDetailsParams,
  flags: FlagInfo_Flag[],
  freeform: string,
): (React.ReactElement | null)[] {
  return flags.map((flag) => (
    <div key={flag.flag}>
      <Checkbox
        checked={flag.enabled}
        onChange={action((e) => {
          flag.enabled = e.currentTarget.checked;
          const flagMap: Record<string, boolean> = {};
          for (const item of flags) {
            flagMap[item.flag] = item.enabled;
          }

          void saveFlagInfos(gs, params, flagMap, freeform);
        })}
      >
        {flag.label === '' ? flag.flag : flag.label}
        {flag.help && (
          <span className="float-right">
            <a href={flag.help}>?</a>
          </span>
        )}
      </Checkbox>
    </div>
  ));
}

const FlagsInfo = observer(function FlagsInfo(props: DepInfoProps): React.ReactElement {
  const gs = useGlobalState();
  const flags = useMemo(() => fromPromise(getFlagInfos(gs, props)), [gs, props]);
  const [currentCat, setCurrentCat] = useState<string>('combined');

  return flags.case({
    pending: () => <span>Loading...</span>,
    rejected: (e: Error) => (
      <Callout intent="danger" title="Error">
        Could not fetch flags:
        <br />
        <pre>{e.toString()}</pre>
      </Callout>
    ),
    fulfilled: ([mappedFlags, freeform]) => {
      return (
        <div>
          <div className="pb-2">
            <label className="text-sm pr-4">Category</label>
            <HTMLSelect defaultValue={currentCat} onChange={(e) => setCurrentCat(e.target.value)}>
              <option key="combined" value="combined">
                All
              </option>
              {Object.keys(mappedFlags).map((cat) => (
                <option key={cat} value={cat}>
                  {cat}
                </option>
              ))}
            </HTMLSelect>
          </div>
          <div className="p-4 border-black border">
            {currentCat === 'combined'
              ? Object.entries(mappedFlags).map(([cat, catFlags]) => (
                  <div key={cat}>
                    <div className="font-bold p-2">{cat}</div>
                    {renderFlags(gs, props, catFlags, freeform)}
                  </div>
                ))
              : renderFlags(gs, props, mappedFlags[currentCat] ?? [], freeform)}
          </div>
        </div>
      );
    },
  });
});

export interface ModDetailsParams {
  modid: string;
  version?: string;
}

const ModPageContainer = styled.div`
  > :global(.bp3-tabs) > :global(.bp3-tab-panel) {
    margin-top: 10px;
  }
`;

export default observer(function ModDetailsPage(
  props: RouteComponentProps<ModDetailsParams>,
): React.ReactElement {
  const gs = useGlobalState();
  const modDetails = useMemo(() => fromPromise(getModDetails(gs, props.match.params)), [
    gs,
    props.match.params,
  ]);

  const rawDesc = (modDetails.value as ModInfoResponse)?.release?.description;
  const description = useMemo(() => {
    const desc = rawDesc ?? '';
    return { __html: bbparser(desc === '' ? 'No description provided' : desc) };
  }, [rawDesc]);

  return (
    <ModPageContainer>
      {modDetails.case({
        pending: () => <Spinner />,
        rejected: (_e: Error) => (
          <Callout intent="danger" title="Failed to fetch mod info">
            Unfortunately, the mod details request failed. Please try again.
          </Callout>
        ),
        fulfilled: (response) =>
          !response ? (
            <NonIdealState
              icon="warning-sign"
              title="Mod not found"
              description="We couldn't find a mod for this URL."
            />
          ) : (
            <>
              <div className="relative">
                <div>
                  <h1 className="mb-2 text-white mod-title">
                    <span className="text-3xl">{response.mod?.title}</span>
                    <HTMLSelect
                      className="ml-2 -mt-2"
                      value={props.match.params.version ?? response.versions[0]}
                      onChange={(e) => {
                        props.history.push(`/mod/${props.match.params.modid}/${e.target.value}`);
                      }}
                    >
                      {response.versions.map((version) => (
                        <option key={version} value={version}>
                          {version}
                        </option>
                      ))}
                    </HTMLSelect>
                  </h1>
                </div>
                <RefImage className="object-contain w-full h-300px" src={response.release?.banner} />
              </div>
              <Tabs renderActiveTabPanelOnly={true}>
                <Tab
                  id="desc"
                  title="Description"
                  panel={
                    <div className="bg-base p-2 rounded text-white">
                      <p dangerouslySetInnerHTML={description} />
                    </div>
                  }
                />
                <Tab
                  id="deps"
                  title="Dependencies"
                  panel={
                    <div className="bg-base p-2 rounded text-white">
                      <DepInfo
                        release={response.release}
                        modid={props.match.params.modid}
                        version={props.match.params.version}
                      />
                    </div>
                  }
                />
                {(response.mod?.type === ModType.MOD || response.mod?.type === ModType.TOTAL_CONVERSION) && (
                  <Tab
                    id="flags"
                    title="Flags"
                    panel={
                      <div className="bg-base p-2 rounded text-white">
                        <FlagsInfo
                          release={response.release}
                          modid={props.match.params.modid}
                          version={props.match.params.version}
                        />
                      </div>
                    }
                  />
                )}
              </Tabs>
            </>
          ),
      })}
    </ModPageContainer>
  );
});
