import React, { useState, useMemo } from 'react';
import { action, makeAutoObservable } from 'mobx';
import { observer } from 'mobx-react-lite';
import { fromPromise, IPromiseBasedObservable } from 'mobx-utils';
import { useParams, useNavigate, Params } from 'react-router-dom';
import {
  Spinner,
  Button,
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
import { gs } from '../lib/state';
import BBRenderer from '../elements/bbrenderer';
import ErrorDialog from '../dialogs/error-dialog';
import RetailBanner from '../resources/banner-retail.png';

async function getModDetails(params: ModDetailsParams): Promise<ModInfoResponse> {
  const response = await gs.client.getModInfo({
    id: params.modid ?? '',
    version: params.version ?? '',
  });
  return response.response;
}

async function getModDependencies(params: ModDetailsParams): Promise<ModDependencySnapshot> {
  const response = await gs.client.getModDependencies({
    id: params.modid ?? '',
    version: params.version ?? '',
  });
  return response.response;
}

async function getFlagInfos(
  params: ModDetailsParams,
): Promise<[Record<string, FlagInfo_Flag[]>, string]> {
  const response = await gs.client.getModFlags({
    id: params.modid ?? '',
    version: params.version ?? '',
  });

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
    sortedFlags[cat].sort((a, b) => (a.label < b.label ? -1 : a.label > b.label ? 1 : 0));
  }

  makeAutoObservable(sortedFlags);
  return [sortedFlags, response.response.freeform];
}

async function saveFlagInfos(
  params: ModDetailsParams,
  flags: Record<string, boolean>,
  freeform: string,
): Promise<void> {
  try {
    void (await gs.client.saveModFlags({
      modid: params.modid ?? '',
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

async function changeDepSnapshot(
  props: DepInfoProps,
  modid: string,
  version: string,
): Promise<void> {
  try {
    const result = await gs.client.depSnapshotChange({
      modid: props.modid ?? '',
      version: props.version ?? '',
      depModid: modid,
      depVersion: version,
    });

    if (!result.response.success) {
      gs.launchOverlay(ErrorDialog, { message: 'Failed to save the changed dependency!' });
    }
  } catch (e) {
    console.error(e);
    gs.launchOverlay(ErrorDialog, {
      title: 'Failed to change dependency snapshot change',
      message: <pre>{e instanceof Error ? e.message : String(e)}</pre>,
    });
  }
}

const DepInfo = observer(function DepInfo(props: DepInfoProps): React.ReactElement {
  const deps = useMemo(() => fromPromise(getModDependencies(props)), [props]);

  return deps.case({
    pending: () => <span>Loading...</span>,
    rejected: (e: Error) => (
      <Callout intent="danger" title="Error">
        Could not resolve dependencies:
        <br />
        <pre>{e.toString()}</pre>
      </Callout>
    ),
    fulfilled: (response) => {
      const depIDs = Object.keys(response.dependencies);
      depIDs.sort();
      return (
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
            {depIDs.map((modID) => {
              const current = response.dependencies[modID];
              return (
                <tr key={modID}>
                  <td>{modID}</td>
                  <td>{current}</td>
                  <td>TBD</td>
                  <td>
                    <HTMLSelect
                      defaultValue={current}
                      onChange={(e) => void changeDepSnapshot(props, modID, e.target.value)}
                    >
                      {response.available[modID].versions.map((version) => (
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
      );
    },
  });
});

function renderFlags(
  params: ModDetailsParams,
  cat: string,
  flags: Record<string, FlagInfo_Flag[]>,
  freeform: string,
): (React.ReactElement | null)[] {
  return (flags[cat] ?? []).map((flag) => (
    <div key={flag.flag}>
      <Checkbox
        checked={flag.enabled}
        onChange={action((e) => {
          flag.enabled = e.currentTarget.checked;
          const flagMap: Record<string, boolean> = {};
          for (const catFlags of Object.values(flags)) {
            for (const item of catFlags) {
              flagMap[item.flag] = item.enabled;
            }
          }

          void saveFlagInfos(params, flagMap, freeform);
        })}
      >
        {flag.label === '' ? flag.flag : flag.label}
        {flag.help && (
          <span className="float-right">
            <a
              href={flag.help}
              onClick={(e) => {
                e.preventDefault();
                void gs.client.openLink({ link: flag.help });
              }}
            >
              ?
            </a>
          </span>
        )}
      </Checkbox>
    </div>
  ));
}

async function resetModFlags(
  props: DepInfoProps,
  setFlags: (flags: IPromiseBasedObservable<[Record<string, FlagInfo_Flag[]>, string]>) => void,
): Promise<void> {
  try {
    await gs.client.resetModFlags({ id: props.modid ?? '', version: props.version ?? '' });
    setFlags(fromPromise(getFlagInfos(props)));
  } catch (e) {
    console.error(e);
    gs.launchOverlay(ErrorDialog, {
      title: 'Could not reset flags',
      message: <pre>{String(e)}</pre>,
    });
  }
}

const FlagsInfo = observer(function FlagsInfo(props: DepInfoProps): React.ReactElement {
  const [flags, setFlags] = useState(() => fromPromise(getFlagInfos(props)));
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
            <Button onClick={() => void resetModFlags(props, setFlags)}>Reset flags</Button>
          </div>
          <div className="p-4 border-black border">
            {currentCat === 'combined'
              ? Object.keys(mappedFlags).map((cat) => (
                  <div key={cat}>
                    <div className="font-bold p-2">{cat}</div>
                    {renderFlags(props, cat, mappedFlags, freeform)}
                  </div>
                ))
              : renderFlags(props, currentCat, mappedFlags, freeform)}
          </div>
        </div>
      );
    },
  });
});

export type ModDetailsParams = Partial<Params<'modid' | 'version'>>;

const ModPageContainer = styled.div`
  > :global(.bp3-tabs) > :global(.bp3-tab-panel) {
    margin-top: 10px;
  }
`;

export default observer(function ModDetailsPage(): React.ReactElement {
  const params = useParams<ModDetailsParams>();
  const navigate = useNavigate();
  const modDetails = useMemo(() => fromPromise(getModDetails(params)), [params]);

  let desc = (modDetails.value as ModInfoResponse)?.release?.description ?? '';
  desc = desc === '' ? 'No description provided' : desc;

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
                      value={params.version ?? response.versions[0]}
                      onChange={(e) => {
                        navigate(`/mod/${params.modid ?? 'missing'}/${e.target.value}`);
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
                {params.modid === 'FS2' ? (
                  <img src={RetailBanner} className="object-contain w-full h-300px" />
                ) : (
                  <RefImage
                    className="object-contain w-full h-300px"
                    src={response.release?.banner}
                  />
                )}
              </div>
              <Tabs renderActiveTabPanelOnly={true}>
                <Tab
                  id="desc"
                  title="Description"
                  panel={
                    <div className="bg-base p-2 rounded text-white">
                      <BBRenderer content={desc} />
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
                        modid={params.modid}
                        version={params.version}
                      />
                    </div>
                  }
                />
                {(response.mod?.type === ModType.MOD ||
                  response.mod?.type === ModType.TOTAL_CONVERSION) && (
                  <Tab
                    id="flags"
                    title="Flags"
                    panel={
                      <div className="bg-base p-2 rounded text-white">
                        <FlagsInfo
                          release={response.release}
                          modid={params.modid}
                          version={params.version}
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
