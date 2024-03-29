import React, { useMemo } from 'react';
import { observer } from 'mobx-react-lite';
import { fromPromise } from 'mobx-utils';
import { useParams, useNavigate, Params } from 'react-router-dom';
import { Button, Spinner, Callout, NonIdealState, HTMLSelect, Tab, Tabs } from '@blueprintjs/core';
import styled from 'astroturf/react';

import { ModInfoResponse } from '@api/client';

import { useGlobalState, GlobalState } from '../lib/state';
import BBRenderer from '../elements/bbrenderer';
import RefImage from '../elements/ref-image';
import { InstallModDialog } from '../dialogs/install-mod';

async function getModDetails(gs: GlobalState, params: ModDetailsParams): Promise<ModInfoResponse> {
  const response = await gs.client.getRemoteModInfo({
    id: params.modid ?? '',
    version: params.version ?? '',
  });
  return response.response;
}

export type ModDetailsParams = Partial<Params<'modid' | 'version'>>;

const ModPageContainer = styled.div`
  > :global(.bp3-tabs) > :global(.bp3-tab-panel) {
    margin-top: 10px;
  }
`;

export default observer(function RemoteModDetailsPage(): React.ReactElement {
  const gs = useGlobalState();
  const params = useParams<ModDetailsParams>();
  const navigate = useNavigate();
  const modDetails = useMemo(() => fromPromise(getModDetails(gs, params)), [gs, params]);

  let desc = (modDetails.value as ModInfoResponse | undefined)?.release?.description ?? '';
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
        fulfilled: (mod) =>
          !mod ? (
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
                    <span className="text-3xl">{mod.mod?.title}</span>
                    <HTMLSelect
                      className="ml-2 -mt-2"
                      value={params.version ?? mod.versions[0]}
                      onChange={(e) => {
                        navigate(`/rmod/${params.modid ?? 'missing'}/${e.target.value}`);
                      }}
                    >
                      {mod.versions.map((version) => (
                        <option key={version} value={version}>
                          {version}
                        </option>
                      ))}
                    </HTMLSelect>
                  </h1>
                </div>
                {mod.release?.banner && (
                  <RefImage className="object-contain w-full h-300px" src={mod.release?.banner} />
                )}
              </div>
              <Callout>
                <Button
                  onClick={() =>
                    gs.launchOverlay(InstallModDialog, {
                      modid: params.modid ?? '',
                      version: params.version,
                    })
                  }
                >
                  Install
                </Button>
              </Callout>
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
              </Tabs>
            </>
          ),
      })}
    </ModPageContainer>
  );
});
